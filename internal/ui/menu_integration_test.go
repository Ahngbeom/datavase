//go:build integration

package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/gdamore/tcell/v2"
)

// clickMenuText finds a right-click menu's row by its rendered text and
// clicks it immediately — the way someone who already knows what a right
// click offers actually uses it, reading the row and clicking it without a
// deliberate pause. Product code is what makes that safe now (bindMouse
// rewrites the double click tview's shared click timer produces here into
// the click it actually is); this helper exercises that real timing rather
// than waiting it out, which would only prove the workaround worked, not
// the fix.
//
// The column is found by rune, not by byte: the menu's own border draws
// multi-byte box characters ("║") before the row's text, and a byte offset
// from strings.Index would count each of those as three columns instead of
// one, landing the click short of where the text actually is.
func (h *harness) clickMenuText(want string) {
	h.t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for row, line := range strings.Split(h.text(), "\n") {
			if idx := strings.Index(line, want); idx >= 0 {
				col := len([]rune(line[:idx]))
				// Not h.click: that waits for tview to resolve the event as
				// MouseLeftClick specifically, which right after a right
				// click it does not — this is the exact case the fix
				// exists for. What proves the click worked is the caller's
				// own wait for what the chosen command actually does.
				h.screen.InjectMouse(col, row, tcell.Button1, tcell.ModNone)
				h.screen.InjectMouse(col, row, tcell.ButtonNone, tcell.ModNone)
				h.settle()
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("menu text %q never appeared on screen:\n%s", want, h.text())
}

// The menu is where someone learns the key, so a key written out rather than
// looked up would teach a rebound user the wrong one.
func TestTheResultMenuNamesTheKeyThatIsBound(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("a result", func(a *App) bool { return a.buf.RowCount() > 0 })

	x, y := h.gridHeaderPosition(0)
	h.rightClick(x, y+1)

	// Read on the interface's own goroutine, and only once: h.text() calls
	// h.settle(), which queues its own update, so calling it from inside an
	// h.inspect/h.waitFor callback — already running inside one queued
	// update — deadlocks waiting for a turn of the loop it is blocking.
	var key string
	h.inspect(func(a *App) bool {
		key = a.keyLabel(keymap.ActionInspect)
		return true
	})

	if !h.waitForScreen("copy row") {
		t.Fatalf("the menu never named copy row:\n%s", h.text())
	}
	if !strings.Contains(h.text(), key) {
		t.Fatalf("the menu named copy row without its bound key %q:\n%s", key, h.text())
	}
}

// A menu entry and the palette entry it came from are the same command; if
// they were not, one of them would be a second source of truth.
func TestAMenuEntryAndThePaletteEntryDoTheSameThing(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 2 AS n UNION ALL SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("two rows", func(a *App) bool { return a.buf.RowCount() == 2 })

	x, y := h.gridHeaderPosition(0)
	h.click(x, y)
	h.waitFor("a sort", func(a *App) bool { return a.content.sorted() })

	h.runCommand("clear sort")
	h.waitFor("the sort to be cleared from the palette", func(a *App) bool {
		return !a.content.sorted()
	})
}

// A menu that cannot fit a row's key column on the real terminal has to drop
// the key rather than let the row get clipped — the same choice layoutMenu
// makes when it is called directly, now pinned against the screen showMenu
// actually draws to. layoutMenu used to be handed a fixed budget regardless
// of how wide the terminal really was, so this narrow-terminal choice was
// never actually reached in production even though menu_test.go covered
// layoutMenu itself.
//
// The width picked here matters: anything narrow enough to hide a key
// entirely looks identical whether layoutMenu decided to drop it or
// menu.Draw's own clamp pushed it off the visible edge — that check would
// have passed against the unfixed code too. The width below leaves just
// enough room that an unfixed layout, still sized against the wrong wider
// budget, shows a jagged prefix of the longest key before the screen cuts
// it off; a fixed layout, sized against this exact width, has already
// decided that key does not fit at all and shows none of it.
func TestANarrowTerminalMenuDropsTheKeyRatherThanClipTheRow(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("a result", func(a *App) bool { return a.buf.RowCount() > 0 })

	var widestName int
	var longestKeyRow, longestKey string
	var longestKeyLen int
	h.inspect(func(a *App) bool {
		for _, e := range menuEntries(paletteCommands(), ctxResult, a.keyLabel) {
			if n := visibleCost(e.name); n > widestName {
				widestName = n
			}
			if n := visibleCost(e.key); n > longestKeyLen {
				longestKeyLen, longestKeyRow, longestKey = n, e.name, e.key
			}
		}
		return true
	})
	if longestKeyLen < 2 {
		t.Fatalf("no ctxResult command carries a key long enough to tell a dropped key from a clipped one (longest is %q)", longestKey)
	}

	partial := longestKeyLen / 2
	h.resize(widestName+2+menuKeyGap+partial, 24)

	x, y := h.gridHeaderPosition(0)
	h.rightClick(x, y+1)

	if !h.waitForScreen(longestKeyRow) {
		t.Fatalf("the menu never opened on a narrow terminal:\n%s", h.text())
	}

	leak := string([]rune(longestKey)[:partial])
	if strings.Contains(h.text(), leak) {
		t.Errorf("a narrow terminal's menu showed %q of %s's key %q instead of dropping it cleanly:\n%s",
			leak, longestKeyRow, longestKey, h.text())
	}
}

// Right-clicking a row that is not the one already selected and choosing a
// command must act on the row the pointer landed on, not on whatever the
// keyboard had selected before — the entire promise of a menu answering
// "what can I do here" is false in the context it is used in most
// otherwise.
func TestRightClickingAResultRowActsOnThatRowNotTheOldSelection(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL("SELECT 'first' AS word UNION ALL SELECT 'second' UNION ALL SELECT 'third'", 3)

	// Row 1 ("first") is selected by default; right-click row 3 ("third").
	x, headerY := h.gridHeaderPosition(0)
	h.rightClick(x, headerY+3)

	h.clickMenuText("copy row")

	h.waitFor("the clicked row's value, not the selected row's, to be copied", func(a *App) bool {
		return a.readClipboard() == "third"
	})
}

// The tree half of the same promise: right-clicking a schema that is not the
// one already selected and choosing "use this schema" must put the clicked
// schema in force.
func TestRightClickingATreeNodeActsOnThatNodeNotTheOldSelection(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.do(keymap.ActionToggleSidebar)
	h.waitFor("at least two schemas", func(a *App) bool {
		return len(a.tree.GetRoot().GetChildren()) >= 2
	})

	var first, second string
	h.inspect(func(a *App) bool {
		children := a.tree.GetRoot().GetChildren()
		first = children[0].GetReference().(*nodeRef).schema
		second = children[1].GetReference().(*nodeRef).schema
		// Already selected, so a right click that failed to move the
		// selection would leave this one in force instead of the one
		// actually clicked.
		a.tree.SetCurrentNode(children[0])
		return true
	})
	if first == second {
		t.Fatalf("the two schemas used in this test were not actually distinct: %q", first)
	}

	// Offset 1 is the first schema (already selected); right-click offset 2,
	// the second.
	x, y := h.treeNodePosition(2)
	h.rightClick(x, y)

	h.clickMenuText("use this schema")

	h.waitFor("the clicked schema, not the previously selected one, to be in force", func(a *App) bool {
		return a.currentSchema() == second
	})
}

// tview reads a left click within DoubleClickInterval of any button's last
// click as completing a double click rather than firing a click of its own
// (Application.fireMouseActions keeps one lastMouseClick timestamp shared
// across all three buttons) — so the right click that opens the menu primes
// exactly this, and List.MouseHandler does nothing with a double click.
// Reading the row and clicking it, with no deliberate pause in between, is
// the gesture this feature exists for; it must not be the one that gets
// silently eaten.
func TestMenuRespondsToAnImmediateClickAfterTheRightClickThatOpenedIt(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL("SELECT 'x' AS word", 1)

	x, y := h.gridHeaderPosition(0)
	h.rightClick(x, y+1)

	h.clickMenuText("copy row")

	h.waitFor("the immediate click to have run copy row", func(a *App) bool {
		return a.readClipboard() == "x"
	})
}

// seedTwoRealTables makes sure two distinctly named tables exist and are in
// the cache, so the tables tab has more than one row to click between —
// mirroring seedSequenceTable (panel_integration_test.go), which needs only
// one.
func seedTwoRealTables(t *testing.T, h *harness, names ...string) {
	t.Helper()

	ctx := context.Background()
	for _, name := range names {
		if _, err := h.app.conn.Exec(ctx,
			"CREATE TABLE IF NOT EXISTS "+name+" (n INT PRIMARY KEY)"); err != nil {
			t.Fatalf("creating %s: %v", name, err)
		}
	}

	snap, err := catalog.FetchSnapshot(ctx, h.app.conn)
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}
	if err := h.cache.Save(ctx, h.app.conn.DataSource().Name, snap); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

// The tables tab half of the same promise as the grid and the tree:
// right-clicking a table that is not the one already selected and choosing
// "inspect" must show the clicked table's definition, not the selected
// one's.
func TestRightClickingATablesTabRowActsOnThatRowNotTheOldSelection(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	seedTwoRealTables(t, h, "dv_menu_a", "dv_menu_b")

	h.focusSchemaPane()
	h.do(keymap.ActionCycleTab)
	h.app.app.QueueUpdateDraw(func() {
		h.app.selectedSchema = testmysql.DefaultDatabase
		h.app.renderTables()
	})
	h.waitFor("at least two tables listed", func(a *App) bool {
		return len(a.listedTables) >= 2
	})

	var target int
	var targetName string
	h.inspect(func(a *App) bool {
		current := a.tableList.GetCurrentItem()
		for i, table := range a.listedTables {
			if i != current {
				target, targetName = i, table.Name
				return true
			}
		}
		return false
	})

	x, y := h.tableItemPosition(target)
	h.rightClick(x, y)

	h.clickMenuText("inspect")

	h.waitFor("the clicked table's definition, not the selected one's, to be shown", func(a *App) bool {
		return strings.Contains(a.ddlText, targetName)
	})
}

// A right click over DDL, plan or sessions must not be classified as the
// result: a.grid keeps its last-drawn rect after a.resultTabs switches away
// from it, the same as a.tree does for the schema pane, and would otherwise
// win the switch in contextAt on a stale match rather than the tab that is
// actually showing.
func TestRightClickingANonGridResultTabIsNotClassifiedAsTheResult(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.runSQL("SELECT 1 AS n", 1)

	x, y := h.gridHeaderPosition(0)

	h.app.app.QueueUpdateDraw(func() { h.app.resultTabs.show(tabDDL) })
	h.settle()

	if got := h.inspect(func(a *App) bool { return a.contextAt(x, y) == ctxResult }); got {
		t.Error("a right click over the DDL tab, where the grid used to be, was still classified as the result")
	}
}
