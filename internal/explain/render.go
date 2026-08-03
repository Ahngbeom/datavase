package explain

import (
	"fmt"
	"strings"
)

// Render draws the plan as a tree, wrapped to fit width.
//
// Fitting is the whole point rather than a nicety. The reason EXPLAIN is hard
// to read in a grid is that it is a dozen narrow columns spread sideways, and
// a tree that overflows has reinvented the problem in a different shape — so
// nothing here is allowed to reach past the width it was given.
func Render(n *Node, width int) string {
	// Below this there is no room for a tree and a word on the same line, and
	// clamping is kinder than emitting something unreadable.
	if width < 20 {
		width = 20
	}

	var b strings.Builder
	render(&b, n, "", "", width)
	return strings.TrimRight(b.String(), "\n")
}

// render writes one step and its children.
//
// prefix is what precedes this line, and childPrefix what precedes everything
// below it — the difference is the elbow, which only the step itself gets.
func render(b *strings.Builder, n *Node, prefix, childPrefix string, width int) {
	head := n.Name
	if summary := n.summary(); summary != "" {
		head += "  " + summary
	}
	if len(n.Warnings) > 0 {
		head += "  ⚠ " + strings.Join(n.Warnings, ", ")
	}

	writeWrapped(b, prefix, childPrefix+"   ", head, width)

	// The details go under the step rather than beside it. A condition is
	// routinely longer than the terminal, and it is the part worth reading.
	for _, a := range n.details() {
		writeWrapped(b, childPrefix+"   ", childPrefix+"     ",
			fmt.Sprintf("%s: %s", a.Key, a.Value), width)
	}

	for i, child := range n.Children {
		last := i == len(n.Children)-1

		elbow, spine := "├─ ", "│  "
		if last {
			elbow, spine = "└─ ", "   "
		}
		render(b, child, childPrefix+elbow, childPrefix+spine, width)
	}
}

// summaryKeys are the fields shown on the step's own line: what it does, how
// it gets there, and how much of the table it expects to touch.
var summaryKeys = []string{"access_type", "key", "rows", "r_rows", "cost"}

func (n *Node) summary() string {
	var parts []string
	for _, key := range summaryKeys {
		if v := n.Attr(key); v != "" {
			// access_type is the one that reads as a phrase on its own; the
			// rest need their name to mean anything.
			if key == "access_type" {
				parts = append(parts, v)
				continue
			}
			parts = append(parts, fmt.Sprintf("%s %s", key, v))
		}
	}
	return strings.Join(parts, "  ")
}

// details are the attributes not already on the step's line.
func (n *Node) details() []Attr {
	var out []Attr
	for _, a := range n.Attrs {
		if a.Key == "table_name" || inList(summaryKeys, a.Key) {
			continue
		}
		out = append(out, a)
	}
	return out
}

// writeWrapped emits text under prefix, breaking it to fit and indenting the
// continuation under cont.
//
// A word longer than the room left — a hundred-character condition with no
// spaces — is broken rather than allowed to overflow. Losing the word boundary
// is worse than nothing; losing the width guarantee is worse than that.
func writeWrapped(b *strings.Builder, prefix, cont, text string, width int) {
	room := width - len([]rune(prefix))
	if room < 8 {
		room = 8
	}

	first := true
	for _, line := range wrap(text, room, width-len([]rune(cont))) {
		if first {
			fmt.Fprintf(b, "%s%s\n", prefix, line)
			first = false
			continue
		}
		fmt.Fprintf(b, "%s%s\n", cont, line)
	}
}

// wrap breaks text into lines of at most firstRoom, then restRoom.
func wrap(text string, firstRoom, restRoom int) []string {
	if restRoom < 8 {
		restRoom = 8
	}

	var (
		lines []string
		room  = firstRoom
		line  []rune
	)
	flush := func() {
		lines = append(lines, string(line))
		line = nil
		room = restRoom
	}

	for _, word := range strings.Fields(text) {
		runes := []rune(word)

		// A word that cannot fit on a line of its own is cut across lines. It
		// is unpleasant to read and it is still every character the server
		// sent.
		for len(runes) > room {
			if len(line) > 0 {
				flush()
				continue
			}
			line = runes[:room]
			runes = runes[room:]
			flush()
		}

		if len(line) > 0 && len(line)+1+len(runes) > room {
			flush()
		}
		if len(line) > 0 {
			line = append(line, ' ')
		}
		line = append(line, runes...)
	}

	if len(line) > 0 || len(lines) == 0 {
		lines = append(lines, string(line))
	}
	return lines
}

func inList(all []string, want string) bool {
	for _, s := range all {
		if s == want {
			return true
		}
	}
	return false
}
