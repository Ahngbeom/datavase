package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// tabbed is a region of the screen: a one-line header above its content.
//
// It draws no box. Three boxes cost six rows and six columns of chrome, and
// each of their titles repeated what the tab strip two rows below already
// said — " results " above "▸results DDL". The header is now the only place a
// region names itself, and a hairline separates it from its neighbour.
//
// The schema pane, the result pane and the editor all use it, so there is no
// reason for two headers in the same application to behave differently.
type tabbed struct {
	*tview.Flex

	header *tview.TextView
	pages  *tview.Pages
	names  []string
	active int

	// width is the last width the header was drawn at, so the header can be
	// abbreviated to fit.
	width int

	// detail is the trailing note this region owns — the open file, a hint
	// that no statement has run yet.
	//
	// It is a function read at draw time rather than a string pushed on every
	// change: with no box to double, the header is the only cue a region has,
	// and one something forgot to update is worse than none.
	//
	// Focus is not a function here. Asking the application who has focus from
	// inside Draw deadlocks — tview holds its mutex for the whole frame and
	// GetFocus takes the same one — and the Flex already knows, because the
	// focused widget is one of its children.
	detail func() string

	// record hands this frame's header zones to the application's hitmap.
	record func(row int, zones []zone)
	// headerRow and headerCol are where the header was last drawn, in screen
	// coordinates.
	headerRow int
	headerCol int
}

func newTabbed() *tabbed {
	t := &tabbed{
		Flex:   tview.NewFlex().SetDirection(tview.FlexRow),
		header: tview.NewTextView().SetDynamicColors(true),
		pages:  tview.NewPages(),
		width:  40,
	}

	t.AddItem(t.header, 1, 0, false).
		AddItem(t.pages, 0, 1, true)

	return t
}

// add registers a tab. The first one added is shown.
func (t *tabbed) add(name string, p tview.Primitive) {
	t.names = append(t.names, name)
	t.pages.AddPage(name, p, true, len(t.names) == 1)
	t.renderHeader()
}

// only holds a single primitive under no name at all.
//
// A header exists to disambiguate, and a region holding one thing has nothing
// to disambiguate — so the editor's header carries which file is open and
// whether it has focus, and never the word "editor". The caret is already
// there; nobody needs telling.
func (t *tabbed) only(p tview.Primitive) {
	t.pages.AddPage("", p, true, true)
	t.renderHeader()
}

// watch supplies the trailing note the header reads at draw time.
func (t *tabbed) watch(detail func() string) *tabbed {
	t.detail = detail
	return t
}

func (t *tabbed) detailText() string {
	if t.detail == nil {
		return ""
	}
	return t.detail()
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

// Draw rebuilds the header every frame.
//
// Focus and the trailing detail are read here rather than pushed, so the only
// way the header can be wrong is if the predicate is — there is no update to
// forget. The strip is a handful of string joins; the status bar has always
// been rendered this way for the same reason.
func (t *tabbed) Draw(screen tcell.Screen) {
	if x, y, width, _ := t.GetInnerRect(); width > 0 {
		t.width, t.headerCol, t.headerRow = width, x, y
	}
	t.renderHeader()
	t.Flex.Draw(screen)
}

func (t *tabbed) renderHeader() {
	text, zones := regionHeader(t.names, t.active, t.HasFocus(), t.detailText(), t.width)
	t.header.SetText(text)
	if t.record != nil {
		t.record(t.headerRow, offsetZones(zones, t.headerCol))
	}
}

// focusMarker is the bar that says which region the keyboard is in. It is a
// glyph and not only a colour, because colour alone is lost on a monochrome
// terminal and to a colour-blind reader — the same reason the active tab keeps
// its "▸".
const focusMarker = "▌"

// regionHeader assembles the marker, the tab strip and the trailing detail.
//
// The detail is the first thing to go when the region is narrow: which tab you
// are on is structural, while a file name or a hint is a convenience.
func regionHeader(names []string, active int, focused bool, detail string, width int) (string, []zone) {
	if width < 1 {
		width = 1
	}

	// The region marker and the active tab's "▸" say different things — which
	// region the keyboard is in, and which tab you are looking at — so they are
	// kept a column apart. Flush against each other they read as one compound
	// glyph and neither is legible.
	marker := "  "
	if focused {
		marker = fmt.Sprintf("[%s]%s[-] ", colourAccent, focusMarker)
	}
	remaining := width - 2

	tabs, zones := tabHeader(names, active, remaining)
	line := marker + tabs
	used := visibleCost(tabs)

	// The strip is drawn after the marker, so its zones sit that far in.
	zones = offsetZones(zones, visibleCost(marker))

	const gap = "  "
	// Four cells is the least that can carry a legible fragment; below that the
	// detail is noise rather than information.
	if room := remaining - used - len(gap); detail != "" && room >= 4 {
		line += gap + tag(colourMuted, result.Truncate(detail, room))
	}

	// The region-name zone spans the whole header and is appended last, after
	// every tab zone: hitmap.at returns the first zone that covers a column,
	// so a tab keeps answering for itself and every other column — the
	// marker, the gap, the detail, and the whole of a header with no tabs at
	// all — falls through to this one. Clicking anywhere on a header that is
	// not a tab still has to focus the region; that is what the header is.
	zones = append(zones, zone{from: 0, to: width, target: zoneRegionName, index: -1})

	return line, zones
}

// tabHeader renders the strip of tab names, marking the active one.
//
// Names are abbreviated when the pane is narrow, but the active marker is
// never dropped: a header that does not say which tab you are looking at has
// no reason to exist.
func tabHeader(names []string, active, width int) (string, []zone) {
	if len(names) == 0 {
		return "", nil
	}
	if active < 0 || active >= len(names) {
		active = 0
	}
	if width < 1 {
		width = 1
	}

	for _, budget := range abbreviationBudgets(names, width) {
		if header, zones, ok := renderTabs(names, active, budget, width); ok {
			return header, zones
		}
	}

	// Nothing fits; show the active tab's initial so the pane is not silent.
	// One initial is not a target — there is nothing to distinguish it from.
	return fmt.Sprintf("[%s]%s[-]", colourAccent, result.EscapeTags(firstRune(names[active]))), nil
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
func renderTabs(names []string, active, budget, width int) (string, []zone, bool) {
	const separator = " "

	var (
		markup  strings.Builder
		zones   []zone
		visible int
	)
	for i, name := range names {
		label := result.Truncate(name, budget)

		if i > 0 {
			markup.WriteString(separator)
			visible += utf8.RuneCountInString(separator)
		}

		from := visible
		// The active tab is bracketed as well as coloured: colour alone is
		// lost on a monochrome terminal or to a colour-blind reader.
		if i == active {
			markup.WriteString(fmt.Sprintf("[%s]▸%s[-]", colourAccent, result.EscapeTags(label)))
			visible += utf8.RuneCountInString(label) + 1
		} else {
			markup.WriteString(fmt.Sprintf("[%s]%s[-]", colourMuted, result.EscapeTags(label)))
			visible += utf8.RuneCountInString(label)
		}
		// An unnamed tab is a region holding one thing, and there is nothing
		// to switch to.
		if name != "" {
			zones = append(zones, zone{from: from, to: visible, target: zoneTab, index: i})
		}
	}

	if visible > width {
		return "", nil, false
	}
	return markup.String(), zones, true
}

func firstRune(s string) string {
	for _, r := range s {
		return string(r)
	}
	return "?"
}
