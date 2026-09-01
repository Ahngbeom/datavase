package ui

import (
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// topBarState is where the session is.
//
// The top line carries the facts that do not change from keystroke to
// keystroke; the bottom line carries what just happened. They used to share
// one line, where a schema name and a row count competed for the same space
// and the loser vanished — including, on a narrow terminal, the environment.
type topBarState struct {
	env    config.Env
	dsName string
	schema string
	branch string
	// helpKey names the key that opens the reference, looked up rather than
	// hardcoded so a rebound one is not advertised as F1.
	helpKey string
}

// topBarForm is one degradation of the line: which of the optional parts it
// still carries.
//
// The forms are enumerated rather than the fields ranked and shed one at a
// time, which is what the status bar does. That machinery earns its keep
// there — a dozen fields whose importance depends on what just happened — but
// this line holds four things and a fixed opinion about the order they go in,
// and a list says that opinion out loud.
type topBarForm struct{ helpKey, dsName, branch bool }

// topBarForms, most complete first. The environment and the schema appear in
// none of them: those two are what a production mistake is made of, so they
// are not on the table.
var topBarForms = []topBarForm{
	{helpKey: true, dsName: true, branch: true},
	{dsName: true, branch: true},
	{branch: true},
	{},
}

const topBarSeparator = "  ·  "

// renderWidth produces the line, degrading until it fits, alongside the
// zones for what it drew.
func (t topBarState) renderWidth(width int) (string, []zone) {
	for _, form := range topBarForms {
		if line, zones := t.line(form, width); visibleCost(line) <= width {
			return line, zones
		}
	}

	// Narrower than the environment and the schema together. Truncating keeps
	// the leftmost, which is the environment — the one thing that has to
	// survive a terminal of any size.
	last, _ := t.line(topBarForms[len(topBarForms)-1], width)
	// Truncating drops the zones with the columns they described: the line
	// that survives is the environment, which is not a control.
	return truncateMarkup(last, width), nil
}

func (t topBarState) line(form topBarForm, width int) (string, []zone) {
	line := t.chip()
	var zones []zone

	// mark records a zone over the run just appended, measured against the
	// markup so far so that the columns agree with the rendered string by
	// construction rather than by a parallel calculation that can drift.
	mark := func(before string, target zoneTarget) {
		if from, to := visibleCost(before), visibleCost(line); to > from {
			zones = append(zones, zone{from: from, to: to, target: target, index: -1})
		}
	}

	// The datasource and the schema written as one. A datasource is often
	// named after its main schema, and the two side by side read as a
	// repetition rather than as two facts — hence the "@", and hence dropping
	// the datasource still leaves "@app_db" rather than a bare word. They are
	// marked as separate zones so a click on either resolves to that field
	// alone.
	if (form.dsName && t.dsName != "") || t.schema != "" {
		line += " "
		if form.dsName && t.dsName != "" {
			before := line
			line += result.EscapeTags(t.dsName)
			mark(before, zoneDataSource)
		}
		if t.schema != "" {
			// The "@" precedes the schema whenever the schema is drawn, not
			// only when the datasource half also survived: dropping the
			// datasource must still leave "@app_db" rather than a bare word.
			line += "@"
			before := line
			line += result.EscapeTags(t.schema)
			mark(before, zoneSchema)
		}
	}

	if form.branch && t.branch != "" {
		line += topBarSeparator + result.EscapeTags(oneLine(t.branch))
	}

	if form.helpKey && t.helpKey != "" {
		help := tag(colourMuted, t.helpKey+" keys")
		// Two cells of gap at minimum, or the hint reads as part of the branch
		// name rather than as a thing of its own.
		if pad := width - visibleCost(line) - visibleCost(help); pad >= 2 {
			line += strings.Repeat(" ", pad)
			before := line
			line += help
			mark(before, zoneHelp)
		}
	}
	return line, zones
}

// chip is the environment, filled rather than merely coloured.
//
// Filled because an error message is red text: if the environment were red
// text too, the cue and the failure would be the same thing worn twice. The
// chip also butts against the spine, so the two read as one band of colour.
func (t topBarState) chip() string {
	style := envStyleFor(t.env)
	return fmt.Sprintf("[%s] %s [-:-]", colourTag(style.fg, style.bg), strings.ToUpper(string(t.env)))
}

// topBar draws the line at whatever width it actually has.
//
// The width is read during Draw for the same reason the status bar reads it
// there: asking earlier returns the zero rect tview holds before layout, and
// rendering against that sheds every field but the environment.
type topBar struct {
	*tview.TextView
	current func() topBarState
	// record hands the zones of this frame to the application's hitmap,
	// offset into screen columns. Nil in a bar nobody is clicking.
	record func(row int, zones []zone)
}

func newTopBar(current func() topBarState) *topBar {
	return &topBar{
		TextView: tview.NewTextView().SetDynamicColors(true),
		current:  current,
	}
}

func (b *topBar) Draw(screen tcell.Screen) {
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
