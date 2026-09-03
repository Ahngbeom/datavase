package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/keymap"
)

// A menu entry that names a key it does not have teaches the wrong one, and
// this is the first place someone reads a key at all.
func TestAMenuEntryCarriesTheKeyThatIsActuallyBound(t *testing.T) {
	cmds := []command{{
		name:     "copy row",
		category: catResults,
		contexts: []menuContext{ctxResult},
		covers:   keymap.ActionInspect,
	}}

	entries := menuEntries(cmds, ctxResult, func(a keymap.Action) string { return "Shift+F4" })

	if len(entries) != 1 {
		t.Fatalf("menuEntries returned %d entries, want 1", len(entries))
	}
	if entries[0].key != "Shift+F4" {
		t.Errorf("entry key = %q, want the bound key %q", entries[0].key, "Shift+F4")
	}
}

// A command that belongs everywhere makes every menu longer without making
// any of them more useful.
func TestACommandWithNoContextsStaysOutOfEveryMenu(t *testing.T) {
	cmds := []command{{name: "export csv", category: catResults}}

	for _, ctx := range allMenuContexts() {
		if got := menuEntries(cmds, ctx, func(keymap.Action) string { return "" }); len(got) != 0 {
			t.Errorf("context %v offers a command that named no contexts", ctx)
		}
	}
}

// The key is recoverable from the palette; the name is the whole of the row.
// So a narrow menu drops the key column before it abbreviates a name.
func TestANarrowMenuDropsTheKeyBeforeTheName(t *testing.T) {
	entries := []menuEntry{{name: "copy row", key: "Ctrl+Shift+I"}}

	wide := layoutMenu(entries, 40)
	if !strings.Contains(wide[0], "Ctrl+Shift+I") {
		t.Fatalf("a wide menu dropped the key: %q", wide[0])
	}

	narrow := layoutMenu(entries, 12)
	if strings.Contains(narrow[0], "Ctrl") {
		t.Errorf("a narrow menu kept the key instead of the name: %q", narrow[0])
	}
	if !strings.Contains(narrow[0], "copy row") {
		t.Errorf("a narrow menu abbreviated the name while a key was still on the row: %q", narrow[0])
	}
}
