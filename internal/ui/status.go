package ui

import (
	"fmt"
	"strings"
	"time"

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
	// columnsLeft is how many of the result's columns have scrolled off the
	// left of the grid. Zero when the grid is at its first column, which is
	// the ordinary case and says nothing.
	columnsLeft int
	message     string
	// hints are what the bar offers when it has nothing else to say. Most
	// actionable first: the bar can drop a clause whole and can only cut a
	// sentence in half, and a list of independent hints loses far less to a
	// dropped clause than to a chopped one.
	//
	// Anything that actually happened displaces them, which is what keeps
	// them out of a working session's way.
	hints []string
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
	// target is what a click on this field answers. zoneNone for most fields
	// — a zone is only worth publishing for a state someone can forget they
	// are in and a click can act on, which today is the mode and the
	// unlocked-writes notice.
	target zoneTarget
}

// defaultStatusWidth is used when the real width is not known yet, on the
// first draw before tview has laid the bar out.
const defaultStatusWidth = 100

// render produces the line at the bar's default width.
func (s status) render() string {
	line, _ := s.renderWidth(defaultStatusWidth)
	return line
}

// Separators, widest first. Tightening the spacing costs nothing, so it is
// tried before any field is dropped — losing whitespace is always better
// than losing information.
const (
	wideSeparator   = "  ·  "
	narrowSeparator = " · "
)

// renderWidth produces the line, tightening and then dropping fields until
// it fits, and the zones a click on the result can land in.
//
// A zone's columns are known only after the join, since dropping and
// tightening both happen first — the same reason topBarState.renderWidth
// accumulates them here rather than in fields().
func (s status) renderWidth(width int) (string, []zone) {
	fields := s.fields()

	separator := wideSeparator
	if statusWidth(fields, runewidth.StringWidth(separator)) > width {
		separator = narrowSeparator
	}
	sepCost := runewidth.StringWidth(separator)

	for statusWidth(fields, sepCost) > width && dropOne(&fields) {
	}

	var (
		line  strings.Builder
		zones []zone
		at    int
	)
	for i, f := range fields {
		if i > 0 {
			line.WriteString(separator)
			at += sepCost
		}
		if f.target != zoneNone {
			zones = append(zones, zone{from: at, to: at + f.visibleCost, target: f.target, index: -1})
		}
		line.WriteString(f.text)
		at += f.visibleCost
	}

	// On a terminal too narrow even for the warnings, something has to give.
	// Truncating is the last resort and keeps the leftmost fields, which are
	// the environment badge and whatever warning followed it. Truncating
	// drops the zones with the columns they described — a partially cut field
	// has nothing intact left for a click to mean.
	if out := line.String(); visibleCost(out) > width {
		return truncateMarkup(out, width), nil
	}
	return line.String(), zones
}

// truncateMarkup shortens a tagged string to a visible width, leaving colour
// tags intact so the line does not lose its colours partway through.
//
// A cut line ends in an ellipsis, paid for out of the width rather than hung
// off the end of a terminal that had no room for it. Without one the bar
// simply stopped mid-word, which reads as a sentence that ended rather than
// one that was cut — and this application abbreviates a file name in the
// region header with an ellipsis two rows above, so the two sat on the same
// screen disagreeing about what a cut looks like.
func truncateMarkup(s string, width int) string {
	const ellipsis = "…"

	// One cell has to be left for the ellipsis. On a terminal with only that
	// one cell, saying "there was more" is the whole of what can be said.
	room := width - 1
	if room < 0 {
		room = 0
	}

	var (
		b       strings.Builder
		inTag   bool
		visible int
		runes   = []rune(s)
	)
	for i := 0; i < len(runes); i++ {
		switch {
		case runes[i] == '[' && i+1 < len(runes) && runes[i+1] == '[':
			if visible >= room {
				return b.String() + ellipsis + "[-]"
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
			if visible >= room {
				return b.String() + ellipsis + "[-]"
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
		// It is where someone reads which keyboard they are on, so it is also
		// where they reach to change it.
		out[len(out)-1].target = zoneStatusMode
	}

	if s.writesEnabled {
		out = add(out, tag(colourNotice, "writes on"), false, 0)
		// A state someone can forget they are in; a click is the shortest way
		// back to locked.
		out[len(out)-1].target = zoneStatusWrites
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
			out = add(out, plural(s.rows, "row"), true, 25)
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
			out = add(out, tag(colourDanger, oneLine(s.err.Error())), false, 0)
		}
	}

	// Outside the switch, and never dropped. A grid scrolled sideways is the
	// same kind of fact as a truncated result — what is on screen is not the
	// whole answer — and it outlives the statement that produced it, so it
	// cannot belong to any one phase. Its absence is silence rather than
	// "0 columns", which is what leaves the notice any force.
	if s.columnsLeft > 0 {
		out = add(out, tag(colourNotice, plural(s.columnsLeft, "column")+" left of view"), false, 0)
	}

	if s.message != "" {
		// Cut rather than dropped. A message is the whole of what this line
		// says when nothing is running, and shedding the field left the bar
		// blank on a narrow terminal — half a sentence beats none. It is last,
		// so truncation takes it and leaves the warnings above intact.
		out = add(out, result.EscapeTags(oneLine(s.message)), false, 0)
	}

	// Hints only appear when the bar has nothing else to say — the moment
	// anything happens, the phase, the message or the error stops being idle
	// and the answer the user was waiting for stops competing with a hint for
	// the same room. Each clause is a field of its own, ranked so the least
	// useful one goes first, which is what keeps a narrow terminal from
	// cutting a hint in half and leaving "for the s".
	if s.phase == phaseIdle && s.message == "" && s.err == nil {
		for i, clause := range s.hints {
			out = add(out, result.EscapeTags(oneLine(clause)), true, hintRank+i)
		}
	}
	return out
}

// hintRank is where the hints sit in the shedding order. Above the row count
// and the elapsed time, because by the time either of those exists the bar
// has something more important to say than what is under the keyboard.
const hintRank = 40

// visibleCost is how many cells a field occupies, ignoring colour tags.
func visibleCost(s string) int {
	return runewidth.StringWidth(visibleText(s))
}

// visibleText is what the terminal shows, colour tags removed.
//
// Separate from visibleCost because callers want different things from the
// same parse: a zone boundary is a count of cells, computed by visibleCost
// against the string this returns — and a rune count and a cell count are
// not the same number once anything double-width is on the line.
func visibleText(s string) string {
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
	return b.String()
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
	// record hands the zones of this frame to the application's hitmap,
	// offset into screen columns. Nil in a bar nobody is clicking.
	record func(row int, zones []zone)
}

func newStatusBar(current func() status) *statusBar {
	return &statusBar{
		TextView: tview.NewTextView().SetDynamicColors(true),
		current:  current,
	}
}

func (b *statusBar) Draw(screen tcell.Screen) {
	x, y, width, _ := b.GetInnerRect()
	if width <= 0 {
		width = defaultStatusWidth
	}

	text, zones := b.current().renderWidth(width)
	b.SetText(text)
	if b.record != nil {
		b.record(y, offsetZones(zones, x))
	}
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
