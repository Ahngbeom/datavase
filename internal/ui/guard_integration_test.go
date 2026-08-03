//go:build integration

package ui

import (
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// The hole, driven through the interface a user actually has.
//
// "ANALYZE FORMAT=JSON DELETE FROM t" runs the delete — verified against the
// server, three rows in and none out — and the classifier called it a read, so
// against production it went straight through with nothing asked.
func TestAnAnalyzeThatDeletesIsRefusedAgainstProduction(t *testing.T) {
	h := newHarness(t, config.EnvProd)

	h.typeSQL("ANALYZE FORMAT=JSON DELETE FROM dv_hole")
	h.do(keymap.ActionRun)

	if !h.waitForScreen("Refused") {
		t.Fatalf("an ANALYZE that deletes was not refused; screen:\n%s", h.text())
	}
}

// Planning without running has to stay free, or the fix has been paid for by
// making the safe thing expensive.
func TestExplainingADeleteIsNotRefused(t *testing.T) {
	h := newHarness(t, config.EnvProd)

	h.typeSQL("EXPLAIN DELETE FROM dv_hole")
	h.do(keymap.ActionRun)

	h.waitFor("the plan to be produced", func(a *App) bool { return a.running == nil })
	if h.screenHas("Refused") {
		t.Errorf("EXPLAIN was refused, though it runs nothing; screen:\n%s", h.text())
	}
}
