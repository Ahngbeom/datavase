package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// tabbed is a pane split into named tabs.
//
// tview has no tab widget, so this is the thinnest thing that works: a
// one-line header above a Pages. The schema pane and the result pane share
// it — there is no reason for two tab strips in the same application to
// behave differently.
type tabbed struct {
	*tview.Flex

	header *tview.TextView
	pages  *tview.Pages
	names  []string
	active int

	// width is the last width the header was drawn at, so the header can be
	// abbreviated to fit.
	width int
}

func newTabbed(title string) *tabbed {
	t := &tabbed{
		Flex:   tview.NewFlex().SetDirection(tview.FlexRow),
		header: tview.NewTextView().SetDynamicColors(true),
		pages:  tview.NewPages(),
		width:  40,
	}

	t.AddItem(t.header, 1, 0, false).
		AddItem(t.pages, 0, 1, true)
	t.SetBorder(true).SetTitle(title)

	return t
}

// add registers a tab. The first one added is shown.
func (t *tabbed) add(name string, p tview.Primitive) {
	t.names = append(t.names, name)
	t.pages.AddPage(name, p, true, len(t.names) == 1)
	t.renderHeader()
}

// show switches to a named tab. Unknown names are ignored, so a caller can
// ask for a tab that has not been built yet without guarding every call.
func (t *tabbed) show(name string) {
	for i, n := range t.names {
		if n == name {
			t.active = i
			t.pages.SwitchToPage(name)
			t.renderHeader()
			return
		}
	}
}

// current returns the name of the visible tab.
func (t *tabbed) current() string {
	if t.active < 0 || t.active >= len(t.names) {
		return ""
	}
	return t.names[t.active]
}

// cycle moves to the next tab, wrapping around.
func (t *tabbed) cycle() {
	if len(t.names) < 2 {
		return
	}
	t.show(t.names[(t.active+1)%len(t.names)])
}

// Draw records the width so the header can be abbreviated to fit.
func (t *tabbed) Draw(screen tcell.Screen) {
	if _, _, width, _ := t.GetInnerRect(); width != t.width {
		t.width = width
		t.renderHeader()
	}
	t.Flex.Draw(screen)
}

func (t *tabbed) renderHeader() {
	t.header.SetText(tabHeader(t.names, t.active, t.width))
}

// tabHeader renders the strip of tab names, marking the active one.
//
// Names are abbreviated when the pane is narrow, but the active marker is
// never dropped: a header that does not say which tab you are looking at has
// no reason to exist.
func tabHeader(names []string, active, width int) string {
	if len(names) == 0 {
		return ""
	}
	if active < 0 || active >= len(names) {
		active = 0
	}
	if width < 1 {
		width = 1
	}

	for _, budget := range abbreviationBudgets(names, width) {
		if header, ok := renderTabs(names, active, budget, width); ok {
			return header
		}
	}

	// Nothing fits; show the active tab's initial so the pane is not silent.
	return fmt.Sprintf("[%s]%s[-]", colourTabActive, result.EscapeTags(firstRune(names[active])))
}

// abbreviationBudgets are the per-name lengths to try, longest first.
func abbreviationBudgets(names []string, width int) []int {
	longest := 0
	for _, n := range names {
		if c := utf8.RuneCountInString(n); c > longest {
			longest = c
		}
	}

	budgets := make([]int, 0, longest)
	for n := longest; n >= 1; n-- {
		budgets = append(budgets, n)
	}
	return budgets
}

// renderTabs lays out the strip with each name capped at budget runes.
func renderTabs(names []string, active, budget, width int) (string, bool) {
	const separator = " "

	var (
		markup  strings.Builder
		visible int
	)
	for i, name := range names {
		label := result.Truncate(name, budget)

		if i > 0 {
			markup.WriteString(separator)
			visible += utf8.RuneCountInString(separator)
		}

		// The active tab is bracketed as well as coloured: colour alone is
		// lost on a monochrome terminal or to a colour-blind reader.
		if i == active {
			markup.WriteString(fmt.Sprintf("[%s]▸%s[-]", colourTabActive, result.EscapeTags(label)))
			visible += utf8.RuneCountInString(label) + 1
			continue
		}
		markup.WriteString(fmt.Sprintf("[%s]%s[-]", colourTabIdle, result.EscapeTags(label)))
		visible += utf8.RuneCountInString(label)
	}

	if visible > width {
		return "", false
	}
	return markup.String(), true
}

func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return "?"
}
