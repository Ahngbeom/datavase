//go:build integration

package ui

import (
	"fmt"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// backgrounds is every background colour currently on screen, with a cell
// that carries it.
func (h *harness) backgrounds() map[tcell.Color]string {
	h.t.Helper()
	h.settle()

	found := map[tcell.Color]string{}
	cells, width, height := h.screen.GetContents()
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			c := cells[row*width+col]
			_, bg, _ := c.Style.Decompose()
			if _, seen := found[bg]; !seen {
				found[bg] = fmt.Sprintf("row %d col %d %q", row, col, string(c.Bytes))
			}
		}
	}
	return found
}

// tview builds a Modal out of three primitives and its own SetBackgroundColor
// reaches only two of them; the Box underneath — the one that draws the border
// — keeps the library's blue.
//
// So the refusal dialog, which is the whole reason this application exists,
// was a ring of blue around black. Every other dialog here draws its border
// on the background it was given, and the guard's was the one that did not.
func TestTheGuardDialogDrawsNoColourThisApplicationDidNotChoose(t *testing.T) {
	h := newHarness(t, config.EnvProd)

	h.typeSQL("DELETE FROM dv_absent WHERE id = 1")
	h.do(keymap.ActionRun)
	if !h.waitForScreen("Refused") {
		t.Fatalf("the guard never refused:\n%s", h.text())
	}

	if where, found := h.backgrounds()[tcell.ColorBlue]; found {
		t.Errorf("the refusal is drawn on tview's default blue at %s:\n%s", where, h.text())
	}
}

// The same Modal is what asks before a write and before quitting on an open
// transaction, so fixing one of them and not the others would only move the
// ring.
func TestTheWriteConfirmationDrawsNoColourThisApplicationDidNotChoose(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	seedRows(t, h, 1)

	h.typeSQL("UPDATE dv_ui SET n = 2 WHERE n = 1")
	h.do(keymap.ActionRun)
	if !h.waitForScreen("Run it?") {
		t.Fatalf("no confirmation appeared:\n%s", h.text())
	}

	if where, found := h.backgrounds()[tcell.ColorBlue]; found {
		t.Errorf("the confirmation is drawn on tview's default blue at %s:\n%s", where, h.text())
	}
}

// Quitting with a transaction open is the third Modal, and the one the user
// meets when they are already leaving.
func TestTheTransactionDialogDrawsNoColourThisApplicationDidNotChoose(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("BEGIN")
	h.do(keymap.ActionRun)
	h.waitFor("the transaction", func(a *App) bool { return a.conn.InTransaction() })

	h.do(keymap.ActionQuit)
	if !h.waitForScreen("Quitting rolls it back") {
		t.Fatalf("no transaction dialog appeared:\n%s", h.text())
	}

	if where, found := h.backgrounds()[tcell.ColorBlue]; found {
		t.Errorf("the transaction dialog is drawn on tview's default blue at %s:\n%s", where, h.text())
	}
}
