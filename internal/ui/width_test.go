package ui

import (
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"
)

// underCJKLocale makes go-runewidth behave as it does when LANG is ko_KR,
// ja_JP or zh_*: East Asian Ambiguous characters count two cells.
func underCJKLocale(t *testing.T) {
	t.Helper()
	was := runewidth.DefaultCondition.EastAsianWidth
	runewidth.DefaultCondition.EastAsianWidth = true
	t.Cleanup(func() { runewidth.DefaultCondition.EastAsianWidth = was })
}

// Under a Korean locale the bar counted ▌ and · as two cells while tview drew
// them as one, so header click zones sat a column off and the status bar
// shed hints that would have fitted.
func TestTheBarMeasuresTextAsTheScreenDrawsItUnderACJKLocale(t *testing.T) {
	underCJKLocale(t)

	for _, s := range []string{
		tag(colourAccent, focusMarker) + " results",
		"F1 for keys" + wideSeparator + "server 11.4",
		narrowSeparator,
		"← → ↩ ⇥",
		"한글 테이블",
	} {
		if got, drawn := visibleCost(s), tview.TaggedStringWidth(s); got != drawn {
			t.Errorf("visibleCost(%q) = %d, but the screen draws it in %d cells", s, got, drawn)
		}
	}
}
