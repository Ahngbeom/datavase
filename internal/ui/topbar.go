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

// renderWidth produces the line, degrading until it fits.
func (t topBarState) renderWidth(width int) string {
	for _, form := range topBarForms {
		if line := t.line(form, width); visibleCost(line) <= width {
			return line
		}
	}

	// Narrower than the environment and the schema together. Truncating keeps
	// the leftmost, which is the environment — the one thing that has to
	// survive a terminal of any size.
	return truncateMarkup(t.line(topBarForms[len(topBarForms)-1], width), width)
}

func (t topBarState) line(form topBarForm, width int) string {
	line := t.chip()

	if place := t.place(form.dsName); place != "" {
		line += " " + place
	}
	if form.branch && t.branch != "" {
		line += topBarSeparator + result.EscapeTags(oneLine(t.branch))
	}

	if form.helpKey && t.helpKey != "" {
		help := tag(colourMuted, t.helpKey+" keys")
		// Two cells of gap at minimum, or the hint reads as part of the branch
		// name rather than as a thing of its own.
		if pad := width - visibleCost(line) - visibleCost(help); pad >= 2 {
			line += strings.Repeat(" ", pad) + help
		}
	}
	return line
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

// place is the datasource and the schema written as one.
//
// A datasource is often named after its main schema, and the two side by side
// read as a repetition rather than as two facts — hence the "@", and hence
// dropping the datasource still leaves "@app_db" rather than a bare word.
func (t topBarState) place(withDataSource bool) string {
	name := ""
	if withDataSource {
		name = result.EscapeTags(t.dsName)
	}
	if t.schema == "" {
		return name
	}
	return name + "@" + result.EscapeTags(t.schema)
}

// topBar draws the line at whatever width it actually has.
//
// The width is read during Draw for the same reason the status bar reads it
// there: asking earlier returns the zero rect tview holds before layout, and
// rendering against that sheds every field but the environment.
type topBar struct {
	*tview.TextView
	current func() topBarState
}

func newTopBar(current func() topBarState) *topBar {
	return &topBar{
		TextView: tview.NewTextView().SetDynamicColors(true),
		current:  current,
	}
}

func (b *topBar) Draw(screen tcell.Screen) {
	_, _, width, _ := b.GetInnerRect()
	if width <= 0 {
		width = defaultStatusWidth
	}

	b.SetText(b.current().renderWidth(width))
	b.TextView.Draw(screen)
}
