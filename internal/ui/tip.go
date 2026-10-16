package ui

import "github.com/Ahngbeom/datavase/internal/keymap"

// tips are the keys nothing on screen leads to. The region headers already
// offer run, complete, copy, sort and the row view while they apply; these
// have no header, menu or button that would let someone stumble on them,
// which is the whole of the case for spending the bar's spare room on them.
var tips = []struct {
	action keymap.Action
	does   string
}{
	{keymap.ActionSearchHistory, "searches what you ran"},
	{keymap.ActionFormat, "lays the query out one clause per line"},
	{keymap.ActionUseSchema, "picks the schema plain names resolve against"},
	{keymap.ActionFind, "finds text in the editor or the result"},
	{keymap.ActionCopyRow, "copies the selected row, tab separated"},
	{keymap.ActionSwitchDataSource, "switches to another datasource"},
}

// tipFor is the tip for the session with this many before it.
//
// Keyed to the session count rather than the clock or a random draw, so a
// tip someone mentions can be found again, and the next session always
// brings a different one.
func tipFor(sessions int, km *keymap.Map) string {
	tip := tips[sessions%len(tips)]
	return "Tip: " + tipKeyLabel(km, tip.action) + " " + tip.does
}

// tipKeyLabel spells a key the way App.keyLabel does, without an App.
func tipKeyLabel(km *keymap.Map, action keymap.Action) string {
	b, ok := km.PreferredBinding(action, onMac)
	if !ok {
		return action.String()
	}
	return b.Label(onMac)
}
