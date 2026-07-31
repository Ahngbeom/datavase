package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
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

// status is everything the bottom bar reports.
//
// It is a plain value with a pure render method, so what the user is told
// about a production database can be tested without starting a terminal.
type status struct {
	dsName string
	env    config.Env
	// schema is the one an unqualified query will hit. It is shown at all
	// times because nothing else on screen says which it is.
	schema string
	// vimMode and vimPending describe the modal keyboard, and are empty on
	// the keyboards that do not have one.
	vimMode       string
	vimPending    string
	writesEnabled bool
	// inTransaction says the connection is pinned and the work so far is
	// undoable, which changes what several other fields mean.
	inTransaction bool

	phase runPhase
	rows  int
	// written is the outcome of a statement that changed rows rather than
	// returning them, and nil for one that returned a result set. A write has
	// no rows to count, so reporting "0 rows" for it said nothing at all.
	written *db.Result
	// warnings are what the server said about a statement it nonetheless
	// called a success — a truncated value, an implicit conversion.
	warnings      []db.Warning
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
// The ranks encode a judgement: an elapsed time is a nicety, the environment
// badge and the warnings about an incomplete result are not.
func (s status) fields() []field {
	add := func(list []field, text string, expendable bool, rank int) []field {
		return append(list, field{
			text:        text,
			expendable:  expendable,
			expendRank:  rank,
			visibleCost: visibleCost(text),
		})
	}

	// Never dropped.
	out := add(nil, envBadge(s.env), false, 0)

	// The mode comes second, before anything that can be dropped: on a modal
	// keyboard it is what explains why an ordinary letter did nothing, so it
	// has to survive both the dropping and the truncating.
	if s.vimMode != "" {
		mode := s.vimMode
		if s.vimPending != "" {
			mode += " " + s.vimPending
		}
		out = add(out, tag(colourNotice, mode), false, 0)
	}

	// The datasource is usually obvious from context; the schema is not.
	out = add(out, result.EscapeTags(s.dsName), true, 20)
	if s.schema != "" {
		// "@schema" rather than the bare name: a datasource is often called
		// the same thing as its main schema, and the two would be
		// unreadable side by side.
		out = add(out, "@"+result.EscapeTags(s.schema), true, 10)
	}
	if s.writesEnabled {
		out = add(out, tag(colourNotice, "writes on"), false, 0)
	}
	// Never dropped. Whether the work so far can be undone is not a detail
	// that should vanish because the terminal got narrow.
	if s.inTransaction {
		out = add(out, tag(colourNotice, "TX"), false, 0)
	}

	switch s.phase {
	case phaseRunning:
		out = add(out, tag(colourNotice, "running… ^C cancels"), false, 0)

	case phaseDone:
		if s.written != nil {
			out = add(out, writeSummary(*s.written), true, 25)
		} else {
			out = add(out, fmt.Sprintf("%d rows", s.rows), true, 25)
		}
		out = add(out, formatElapsed(s.elapsed), true, 30)
		if s.limitInjected > 0 {
			// A quietly added LIMIT makes a partial result look complete.
			out = add(out, tag(colourNotice, fmt.Sprintf("LIMIT %d added", s.limitInjected)), false, 0)
		}
		if s.truncated {
			out = add(out, tag(colourNotice, "truncated"), false, 0)
		}
		// Never dropped: a warning is the only sign that a statement the
		// server called a success did something other than what was asked.
		if len(s.warnings) > 0 {
			out = add(out, tag(colourNotice, warningSummary(s.warnings)), false, 0)
		}

	case phaseFailed:
		if s.err != nil {
			out = add(out, tag(colourError, oneLine(s.err.Error())), false, 0)
		}
	}

	if s.message != "" {
		out = add(out, result.EscapeTags(oneLine(s.message)), true, 15)
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

// envBadge renders the environment label. Production is red wherever it
// appears, because it is the cue that has to survive being glanced at.
func envBadge(env config.Env) string {
	switch env {
	case config.EnvProd:
		return tag(colourEnvProd, " "+string(env)+" ")
	case config.EnvStage:
		return tag(colourEnvStage, " "+string(env)+" ")
	default:
		return tag(colourEnvDev, " "+string(env)+" ")
	}
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

// writeSummary is what a statement that changed rows reports instead of a
// row count.
//
// The singular is spelled out because this is the number that says whether a
// write went as intended: "1 row affected" and "4812 rows affected" are the
// difference between a routine edit and an incident, and it is read in a
// hurry.
//
// MySQL counts rows *changed* rather than matched, so an UPDATE setting a
// column to the value it already held reports zero. That is the server's own
// answer, and the wording says "affected" rather than "matched" so it cannot
// be read as the other one.
func writeSummary(r db.Result) string {
	noun := "rows affected"
	if r.RowsAffected == 1 {
		noun = "row affected"
	}

	summary := fmt.Sprintf("%d %s", r.RowsAffected, noun)
	if r.LastInsertID > 0 {
		summary += fmt.Sprintf(" · id %d", r.LastInsertID)
	}
	return summary
}

// warningSummary is how the server's own complaint reaches the bar.
//
// The first message is carried in full rather than replaced by a count. A
// count tells the user something happened without telling them what, and
// "Data truncated for column 's' at row 1" is the whole of what they need to
// know to go and look.
func warningSummary(warnings []db.Warning) string {
	noun := "warnings"
	if len(warnings) == 1 {
		noun = "warning"
	}

	return fmt.Sprintf("%d %s: %s",
		len(warnings), noun, result.EscapeTags(oneLine(warnings[0].Message)))
}
