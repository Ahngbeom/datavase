//go:build integration

package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/gdamore/tcell/v2"
)

func (h *harness) typeRunes(s string) {
	h.t.Helper()
	keys := make([]*tcell.EventKey, 0, len(s))
	for _, r := range s {
		keys = append(keys, tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	h.inject(keys...)
}

// A result that has to leave the terminal as a file, not a paste: fifty
// thousand rows on a clipboard are nowhere useful, and a spreadsheet takes
// a CSV.
func TestTheCopyKeyCanSaveTheResultAsACSVFile(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	path := filepath.Join(t.TempDir(), "rows.csv")

	h.typeSQL("SELECT 1 AS a, 'x,y' AS b UNION ALL SELECT 2, 'z'")
	h.do(keymap.ActionRun)
	h.waitFor("two rows", func(a *App) bool { return a.status.phase == phaseDone && a.buf.RowCount() == 2 })

	h.do(keymap.ActionCopyResult)
	h.waitFor("the format chooser", func(a *App) bool { return a.pages.HasPage(pageCopyFormat) })
	h.typeRunes("c")
	h.waitFor("the path prompt", func(a *App) bool { return a.pages.HasPage(pageSavePath) })

	h.press(tcell.KeyCtrlU)
	h.typeRunes(path)
	h.press(tcell.KeyEnter)

	h.waitFor("the notice", func(a *App) bool { return strings.Contains(a.status.message, "2 rows written to "+path) })
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the file was not written: %v", err)
	}
	if want := "a,b\n1,\"x,y\"\n2,z\n"; string(got) != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	if !h.inspect(func(a *App) bool { return !a.pages.HasPage(pageSavePath) && a.app.GetFocus() == a.grid }) {
		t.Error("the prompt did not close and hand the keyboard back to the grid")
	}
}

func TestEscapeLeavesTheSavePromptWithoutWriting(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("a result", func(a *App) bool { return a.status.phase == phaseDone })

	h.do(keymap.ActionCopyResult)
	h.waitFor("the format chooser", func(a *App) bool { return a.pages.HasPage(pageCopyFormat) })
	h.typeRunes("c")
	h.waitFor("the path prompt", func(a *App) bool { return a.pages.HasPage(pageSavePath) })
	h.press(tcell.KeyEscape)
	h.waitFor("the prompt to close", func(a *App) bool { return !a.pages.HasPage(pageSavePath) })

	if h.inspect(func(a *App) bool { return strings.Contains(a.status.message, "written") }) {
		t.Error("Escape wrote a file")
	}
}

func newReadOnlyHarness(t *testing.T) *harness {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	ds.Env = config.EnvDev
	ds.ReadOnly = true

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := db.Open(ctx, ds, password, "")
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	return harnessWith(t, &session.Session{Conn: conn}, ds)
}

// Whether a session can write is a fact about where you are, and the top
// line is where those live. Nothing else on screen would say it until a
// write failed.
func TestAReadOnlyDataSourceIsMarkedOnTheTopLine(t *testing.T) {
	h := newReadOnlyHarness(t)

	top := strings.SplitN(h.text(), "\n", 2)[0]
	if !strings.Contains(top, "read-only") {
		t.Errorf("the top line does not say the datasource is read-only:\n%s", top)
	}
}

// The server's words say a transaction is read-only, which sends someone
// looking for a BEGIN they never typed. The setting is in the configuration.
func TestARefusedWriteSaysTheDataSourceIsReadOnly(t *testing.T) {
	h := newReadOnlyHarness(t)

	h.typeSQL("CREATE TABLE dv_ro_probe (id INT)")
	h.do(keymap.ActionRun)
	h.waitFor("the failure", func(a *App) bool { return a.status.phase == phaseFailed })

	if !h.waitForScreen("read_only") {
		t.Errorf("the failure does not point at the read_only setting:\n%s", h.text())
	}
}
