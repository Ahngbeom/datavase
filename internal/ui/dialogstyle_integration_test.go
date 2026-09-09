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
// Quitting with a transaction open is the Modal that meets this, and the one
// the user sees when they are already leaving.
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
