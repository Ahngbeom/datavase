package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"
)

// runPhase is where the current statement stands.
type runPhase int

const (
	phaseIdle runPhase = iota
	phaseRunning
	phaseDone
	phaseFailed
)

// status is what just happened.
//
// Where the session is — the environment, the datasource, the schema, the
// branch — lives on the top bar, and the open file on the editor's own header.
// Splitting them is what stopped a schema name and a row count competing for
// the same space, with the loser silently gone.
//
// It is a plain value with a pure render method, so what the user is told
// about a production database can be tested without starting a terminal.
type status struct {
	// vimMode and vimPending describe the modal keyboard, and are empty on
	// the keyboards that do not have one.
	vimMode       string
	vimPending    string
	writesEnabled bool

	phase         runPhase
	rows          int
	elapsed       time.Duration
	err           error
	limitInjected int
	truncated     bool
	message       string
}

// field is one item of the bar, with whether it may be dropped when the line
// does not fit.
//
// Some of these are the only warning the user gets that a result is
// incomplete, so "make it fit" cannot be allowed to quietly remove them.
type field struct {
	text        string
	expendable  bool
	expendRank  int // higher goes first when space runs out
	visibleCost int
}

// defaultStatusWidth is used when the real width is not known yet, on the
// first draw before tview has laid the bar out.
const defaultStatusWidth = 100

// render produces the line at the bar's default width.
func (s status) render() string { return s.renderWidth(defaultStatusWidth) }

// Separators, widest first. Tightening the spacing costs nothing, so it is
// tried before any field is dropped — losing whitespace is always better
// than losing information.
const (
	wideSeparator   = "  ·  "
	narrowSeparator = " · "
)

// renderWidth produces the line, tightening and then dropping fields until
// it fits.
func (s status) renderWidth(width int) string {
	fields := s.fields()

	separator := wideSeparator
	if statusWidth(fields, runewidth.StringWidth(separator)) > width {
		separator = narrowSeparator
	}
	sepCost := runewidth.StringWidth(separator)

	for statusWidth(fields, sepCost) > width && dropOne(&fields) {
	}

	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = f.text
	}
	line := strings.Join(parts, separator)

	// On a terminal too narrow even for the warnings, something has to give.
	// Truncating is the last resort and keeps the leftmost fields, which are
	// the environment badge and whatever warning followed it.
	if visibleCost(line) > width {
		return truncateMarkup(line, width)
	}
	return line
}

// truncateMarkup shortens a tagged string to a visible width, leaving colour
// tags intact so the line does not lose its colours partway through.
func truncateMarkup(s string, width int) string {
	var (
		b       strings.Builder
		inTag   bool
		visible int
		runes   = []rune(s)
	)
	for i := 0; i < len(runes); i++ {
		switch {
		case runes[i] == '[' && i+1 < len(runes) && runes[i+1] == '[':
			if visible >= width {
				return b.String() + "[-]"
			}
			b.WriteString("[[")
			visible++
			i++
		case runes[i] == '[':
			inTag = true
			b.WriteRune(runes[i])
		case runes[i] == ']' && inTag:
			inTag = false
			b.WriteRune(runes[i])
		case inTag:
			b.WriteRune(runes[i])
		default:
			if visible >= width {
				return b.String() + "[-]"
			}
			b.WriteRune(runes[i])
			visible++
		}
	}
	return b.String()
}

// statusWidth is the visible cost of the assembled line.
func statusWidth(fields []field, sepCost int) int {
	if len(fields) == 0 {
		return 0
	}

	total := sepCost * (len(fields) - 1)
	for _, f := range fields {
		total += f.visibleCost
	}
	return total
}

// dropOne removes the most expendable field, reporting whether it could.
func dropOne(fields *[]field) bool {
	best := -1
	for i, f := range *fields {
		if !f.expendable {
			continue
		}
		if best < 0 || f.expendRank > (*fields)[best].expendRank {
			best = i
		}
	}
	if best < 0 {
		return false
	}

	*fields = append((*fields)[:best], (*fields)[best+1:]...)
	return true
}

// fields lists what the bar would show at unlimited width.
//
// The ranks encode a judgement: an elapsed time is a nicety, and the warnings
// about an incomplete result are not.
func (s status) fields() []field {
	add := func(list []field, text string, expendable bool, rank int) []field {
		return append(list, field{
			text:        text,
			expendable:  expendable,
			expendRank:  rank,
			visibleCost: visibleCost(text),
		})
	}

	// The mode comes first, before anything that can be dropped: on a modal
	// keyboard it is what explains why an ordinary letter did nothing, so it
	// has to survive both the dropping and the truncating.
	var out []field
	if s.vimMode != "" {
		mode := s.vimMode
		if s.vimPending != "" {
			mode += " " + s.vimPending
		}
		out = add(out, tag(colourNotice, mode), false, 0)
	}

	if s.writesEnabled {
		out = add(out, tag(colourNotice, "writes on"), false, 0)
	}

	switch s.phase {
	case phaseRunning:
		out = add(out, tag(colourNotice, "running… ^C cancels"), false, 0)

	case phaseDone:
		out = add(out, plural(s.rows, "row"), true, 25)
		out = add(out, formatElapsed(s.elapsed), true, 30)
		if s.limitInjected > 0 {
			// A quietly added LIMIT makes a partial result look complete.
			out = add(out, tag(colourNotice, fmt.Sprintf("LIMIT %d added", s.limitInjected)), false, 0)
		}
		if s.truncated {
			out = add(out, tag(colourNotice, "truncated"), false, 0)
		}

	case phaseFailed:
		if s.err != nil {
			out = add(out, tag(colourDanger, oneLine(s.err.Error())), false, 0)
		}
	}

	if s.message != "" {
		// Cut rather than dropped. A message is the whole of what this line
		// says when nothing is running, and shedding the field left the bar
		// blank on a narrow terminal — half a sentence beats none. It is last,
		// so truncation takes it and leaves the warnings above intact.
		out = add(out, result.EscapeTags(oneLine(s.message)), false, 0)
	}
	return out
}

// visibleCost is how many cells a field occupies, ignoring colour tags.
func visibleCost(s string) int {
	var (
		b     strings.Builder
		inTag bool
		runes = []rune(s)
	)
	for i := 0; i < len(runes); i++ {
		switch {
		case runes[i] == '[' && i+1 < len(runes) && runes[i+1] == '[':
			b.WriteRune('[')
			i++
		case runes[i] == '[':
			inTag = true
		case runes[i] == ']' && inTag:
			inTag = false
		case !inTag:
			b.WriteRune(runes[i])
		}
	}
	return runewidth.StringWidth(b.String())
}

// tag wraps text in a tview colour tag. The text is escaped first: server
// messages and datasource names can contain "[", which tview would read as
// the start of a tag and swallow.
func tag(colour fmt.Stringer, text string) string {
	return fmt.Sprintf("[%s]%s[-]", colour, result.EscapeTags(text))
}

// oneLine flattens a multi-line message; the status bar is one line by
// contract, and a driver error with newlines would push the layout apart.
func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

// plural counts a thing without saying "1 rows", which reads as a bug in the
// counter rather than as a result of one.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// formatElapsed favours the unit that keeps the number readable.
func formatElapsed(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// statusBar draws the status line at whatever width it actually has.
//
// The width has to be read during Draw: asking the widget earlier returns the
// zero rect it holds before tview lays it out, and rendering against that
// drops every field but the environment badge. Drawing per frame also means
// the bar re-flows when the window is resized.
type statusBar struct {
	*tview.TextView
	current func() status
}

func newStatusBar(current func() status) *statusBar {
	return &statusBar{
		TextView: tview.NewTextView().SetDynamicColors(true),
		current:  current,
	}
}

func (b *statusBar) Draw(screen tcell.Screen) {
	_, _, width, _ := b.GetInnerRect()
	if width <= 0 {
		width = defaultStatusWidth
	}

	b.SetText(b.current().renderWidth(width))
	b.TextView.Draw(screen)
}
