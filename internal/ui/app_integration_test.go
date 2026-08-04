//go:build integration

package ui

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/history"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/recent"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/gdamore/tcell/v2"
)

// harness drives the real interface against a simulated terminal, so the
// wiring between key presses, guard and the database is exercised end to
// end without needing a tty.
type harness struct {
	app     *App
	screen  tcell.SimulationScreen
	cache   *catalog.Cache
	history *history.Store
	t       *testing.T

	// keysHandled counts the keys the interface has finished processing.
	keysHandled int64
}

// seedCache replaces the completion cache with a known schema.
//
// Opening the interface kicks off a background refresh that reads the real
// server, so this waits for that to land first — otherwise it would overwrite
// the seed a moment later and the test would complete against whatever
// happens to be in the test database.
func (h *harness) seedCache(snap catalog.Snapshot) {
	h.t.Helper()

	name := h.app.conn.DataSource().Name

	if err := h.cache.Save(context.Background(), name, snap); err != nil {
		h.t.Fatalf("seeding the cache: %v", err)
	}
}

// waitForBackgroundRefresh blocks until the refresh started at startup has
// written something.
func (h *harness) waitForBackgroundRefresh(datasource string) {
	h.t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		schemas, err := h.cache.Schemas(context.Background(), datasource)
		if err != nil {
			h.t.Fatalf("reading the cache: %v", err)
		}
		if len(schemas) > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatal("the background schema refresh never populated the cache")
}

func newHarness(t *testing.T, env config.Env) *harness {
	t.Helper()
	return newHarnessWithIntro(t, env, "")
}

// newHarnessWithIntro is newHarness for the tests that need the first-run card.
//
// An empty marker path is what every other test gets, and is what keeps the
// card out of their way: a session with nowhere to record it never shows it.
func newHarnessWithIntro(t *testing.T, env config.Env, introMarker string) *harness {
	t.Helper()

	ds, password := testmysql.DataSource(t)
	ds.Env = env

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := db.Open(ctx, ds, password, "")
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	return harnessWith(t, &session.Session{Conn: conn}, ds, introMarker)
}

// harnessOver builds the interface over a session that is already open.
//
// Separated so a test can supply one that reaches its server through a real
// bastion — which is the only way to exercise what the interface says when
// that bastion goes away.
func harnessOver(t *testing.T, sess *session.Session, ds *config.DataSource) *harness {
	t.Helper()
	return harnessWith(t, sess, ds, "")
}

func harnessWith(t *testing.T, sess *session.Session, ds *config.DataSource, introMarker string) *harness {
	t.Helper()

	t.Cleanup(func() { sess.Close() })

	cfg := &config.Config{
		DataSources: []config.DataSource{*ds},
		Defaults: config.Defaults{
			AutoLimit:  config.DefaultAutoLimit,
			FetchChunk: config.DefaultFetchChunk,
			BufferMax:  config.DefaultBufferMax,
		},
	}

	cache, err := catalog.OpenCache(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("OpenCache() error = %v", err)
	}
	t.Cleanup(func() { cache.Close() })

	screen := tcell.NewSimulationScreen("UTF-8")
	screen.SetSize(120, 40)

	hist, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("history.Open() error = %v", err)
	}
	t.Cleanup(func() { hist.Close() })

	// The keyboard is stated rather than taken from the default. These tests
	// are about behaviour that predates the modal editor — typing into the
	// editor, undo, completion — and would otherwise start failing the day
	// the default changed, for reasons that have nothing to do with them.
	// Modal behaviour has its own harness.
	keys, err := keymap.ForPreset(keymap.PresetDataGrip)
	if err != nil {
		t.Fatalf("ForPreset(datagrip) error = %v", err)
	}

	// Backed by a temporary file so that attaching a directory in a test never
	// writes into the developer's own state directory.
	recents, err := recent.Open(filepath.Join(t.TempDir(), "recent-dirs.json"))
	if err != nil {
		t.Fatalf("recent.Open() error = %v", err)
	}

	app := New(sess, cfg, Deps{
		Keys: keys, Cache: cache, History: hist, Recent: recents, IntroPath: introMarker,
	})
	app.SetScreen(screen)

	h := &harness{app: app, screen: screen, cache: cache, history: hist, t: t}
	h.countKeys()

	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	t.Cleanup(func() {
		app.Stop()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the interface did not stop")
		}
	})

	h.settle()
	// Opening the interface starts a background schema refresh. Letting it
	// finish here keeps every test from racing against it.
	h.waitForBackgroundRefresh(ds.Name)
	return h
}

// countKeys makes injected keys observable.
//
// An injected key goes onto the screen's event queue while QueueUpdateDraw
// uses the application's, and tview interleaves the two — so settling a fixed
// number of times after injecting a key is a guess. Counting the keys the
// interface has actually handled turns that guess into a condition, which is
// what stops these tests failing a few runs in a hundred.
//
// It wraps the interface's own capture from the outside rather than adding a
// counter to it: nothing here should exist in the application because of a
// test.
func (h *harness) countKeys() {
	inner := h.app.app.GetInputCapture()

	h.app.app.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		out := inner(ev)
		// After inner has returned, whatever the key was going to do to the
		// application state has been done.
		atomic.AddInt64(&h.keysHandled, 1)
		return out
	})
}

// awaitKeys blocks until the interface has handled every key injected so far.
func (h *harness) awaitKeys(want int64) {
	h.t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&h.keysHandled) >= want {
			h.settle()
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	h.t.Fatalf("the interface handled %d of %d injected keys",
		atomic.LoadInt64(&h.keysHandled), want)
}

// inject sends keys and waits for all of them to be handled.
func (h *harness) inject(keys ...*tcell.EventKey) {
	h.t.Helper()

	want := atomic.LoadInt64(&h.keysHandled) + int64(len(keys))
	for _, ev := range keys {
		h.screen.InjectKey(ev.Key(), ev.Rune(), ev.Modifiers())
	}
	h.awaitKeys(want)
}

// settle lets queued UI updates finish before the screen is inspected.
func (h *harness) settle() {
	h.t.Helper()
	for i := 0; i < 3; i++ {
		done := make(chan struct{})
		h.app.app.QueueUpdateDraw(func() { close(done) })
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			h.t.Fatal("the interface stopped responding")
		}
	}
}

// inspect evaluates a predicate on the interface's own goroutine.
//
// Reading application state from the test goroutine is a data race whatever
// the timing looks like, and tview state is only safe to touch from inside
// its update queue.
func (h *harness) inspect(cond func(*App) bool) bool {
	h.t.Helper()

	var ok bool
	done := make(chan struct{})
	h.app.app.QueueUpdateDraw(func() {
		ok = cond(h.app)
		close(done)
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		h.t.Fatal("the interface stopped responding")
	}
	return ok
}

// waitFor blocks until a condition holds, or fails the test saying what it
// was waiting for.
//
// An injected key goes onto the screen's event queue while QueueUpdateDraw
// uses the application's, and tview interleaves the two. "Settle a fixed
// number of times, then read" is therefore a guess that is usually right,
// which is the worst kind of test — it fails a few runs in a hundred, on a
// machine that is not yours.
func (h *harness) waitFor(what string, cond func(*App) bool) {
	h.t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if h.inspect(cond) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("timed out waiting for %s; screen:\n%s", what, h.text())
}

func (h *harness) typeSQL(sql string) {
	h.t.Helper()
	h.app.app.QueueUpdateDraw(func() {
		h.app.editor.SetText(sql, true)
		h.app.app.SetFocus(h.app.editor)
	})
	h.settle()
}

func (h *harness) press(key tcell.Key) {
	h.t.Helper()
	h.inject(tcell.NewEventKey(key, 0, tcell.ModNone))
}

// do triggers an action through its real binding.
//
// Tests name actions rather than keys so that rebinding a key does not
// invalidate every behavioural test — which key means what is the keymap
// package's business, and it has its own tests for that.
func (h *harness) do(action keymap.Action) {
	h.t.Helper()

	bindings := h.app.keys.Bindings(action)
	if len(bindings) == 0 {
		h.t.Fatalf("action %v has no binding", action)
	}

	// The last binding is the plainest fallback, which the simulation screen
	// can always deliver: it needs no extended keyboard protocol.
	b := bindings[len(bindings)-1]
	h.inject(tcell.NewEventKey(b.Key, b.Rune, b.Mods))
}

// editorText reads the editor buffer from the UI goroutine.
func (h *harness) editorText() string {
	h.t.Helper()

	var text string
	done := make(chan struct{})
	h.app.app.QueueUpdateDraw(func() {
		text = h.app.editor.GetText()
		close(done)
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		h.t.Fatal("the interface stopped responding")
	}
	return text
}

// undo sends tview's own undo binding, which datavase deliberately leaves
// to the widget.
func (h *harness) undo() {
	h.t.Helper()
	h.screen.InjectKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl)
	h.settle()
}

// moveCaret places the caret at a byte offset in the editor.
func (h *harness) moveCaret(offset int) {
	h.t.Helper()
	h.app.app.QueueUpdateDraw(func() { h.app.editor.Select(offset, offset) })
	h.settle()
}

// selectAllText selects the whole buffer without going through a key, so
// tests that are about something else do not depend on that binding.
func (h *harness) selectAllText() {
	h.t.Helper()
	h.app.app.QueueUpdateDraw(func() { h.app.selectAll() })
	h.settle()
}

// showSidebar brings the schema pane on screen.
//
// It starts hidden — the finders answer "where is that table" better than a
// permanent tree does — so a test about the tree has to ask for it, the same
// way a user would.
func (h *harness) showSidebar() {
	h.t.Helper()

	if h.inspect(func(a *App) bool { return a.sidebarVisible }) {
		return
	}
	h.do(keymap.ActionToggleSidebar)
	h.waitFor("the schema pane", func(a *App) bool { return a.sidebarVisible })
}

// text is everything currently drawn, with trailing spaces collapsed so
// assertions can look for phrases rather than exact layout.
func (h *harness) text() string {
	h.t.Helper()
	h.settle()

	cells, width, height := h.screen.GetContents()
	var b strings.Builder
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			b.Write(cells[row*width+col].Bytes)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// waitForScreen polls until the drawn text contains want.
func (h *harness) waitForScreen(want string) bool {
	h.t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(h.text(), want) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func TestInterfaceShowsTheEnvironmentAndDataSource(t *testing.T) {
	h := newHarness(t, config.EnvProd)

	got := h.text()
	// The environment is set as a filled chip, in capitals, so that it cannot
	// be mistaken for the red of an error message.
	if !strings.Contains(strings.ToLower(got), "prod") {
		t.Errorf("screen does not show the environment:\n%s", got)
	}
	if !strings.Contains(got, "integration") {
		t.Errorf("screen does not show the datasource name:\n%s", got)
	}
	// The schema an unqualified statement reaches is on the top line beside
	// the environment; nothing else on screen says which it is.
	if !strings.Contains(got, "@"+testmysql.DefaultDatabase) {
		t.Errorf("screen does not show the current schema:\n%s", got)
	}
	// The result region names itself. The editor deliberately does not: its
	// header carries which file is open, and the caret says the rest.
	if !strings.Contains(got, "results") {
		t.Errorf("screen does not show the result region:\n%s", got)
	}
}

// The route that survives a host application taking ⌘B.
//
// This is the whole point of the palette carrying every command and of its own
// plain key: nothing here presses a chord, and the schema tree still appears.
func TestTheSchemaTreeIsReachableWithoutItsChord(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	// F3, not ⌘⇧A: the escape hatch has to open with a key nothing upstream
	// is in a position to claim.
	h.press(tcell.KeyF3)
	h.waitFor("the palette", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pagePalette
	})

	h.typeInto("schema tree")
	h.press(tcell.KeyEnter)

	h.waitFor("the schema pane", func(a *App) bool { return a.sidebarVisible })
	if !h.waitForScreen(tabTables) {
		t.Errorf("the schema pane did not appear:\n%s", h.text())
	}
}

// The schema tree is one key away rather than a third of the screen, because
// this application already answers "where is that table" with a finder.
func TestTheSchemaPaneStartsHiddenAndComesBackOnRequest(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	if h.inspect(func(a *App) bool { return a.sidebarVisible }) {
		t.Fatal("the schema pane is on screen before anyone asked for it")
	}
	// The opening line is the only place its existence is announced.
	if !strings.Contains(h.text(), "schema tree") {
		t.Errorf("nothing says the schema tree exists:\n%s", h.text())
	}

	h.showSidebar()
	if !h.waitForScreen("tree") {
		t.Errorf("the schema pane did not come back:\n%s", h.text())
	}
}

func TestRunningASelectFillsTheGrid(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 42 AS answer, 'hello' AS greeting")

	h.do(keymap.ActionRun)

	if !h.waitForScreen("answer") {
		t.Errorf("the column header never appeared:\n%s", h.text())
	}
	if !h.waitForScreen("hello") {
		t.Errorf("the row value never appeared:\n%s", h.text())
	}
	if !h.waitForScreen("1 row") {
		t.Errorf("the status bar never reported the row count:\n%s", h.text())
	}
}

// A file of migrations is the reason the worktree exists, and a migration
// file is almost never one statement. Run-everything has to run all of them.
func TestRunEverythingRunsEveryStatementInTheBuffer(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1; SELECT 2; SELECT 3")

	h.do(keymap.ActionRunAll)

	h.waitFor("the batch to report that all three ran", func(a *App) bool {
		return strings.Contains(a.status.message, "3 statements") &&
			strings.Contains(a.status.message, "3 ran")
	})
}

// A refusal stops the rest. The statements after it were written to follow
// the one that did not run, and with no transaction to unwind what already
// happened, the count of what ran is the only thing that says where to look.
func TestRunEverythingStopsAtARefusalAndSaysHowFarItGot(t *testing.T) {
	h := newHarness(t, config.EnvProd)
	h.typeSQL("SELECT 1; DELETE FROM dv_seq; SELECT 3")

	h.do(keymap.ActionRunAll)

	h.waitFor("the batch to report where it stopped", func(a *App) bool {
		return strings.Contains(a.status.message, "refused at statement 2")
	})
	h.waitFor("the third statement to have been left alone", func(a *App) bool {
		return strings.Contains(a.status.message, "1 ran")
	})

	if !h.waitForScreen("Refused") {
		t.Errorf("the refusal itself never reached the screen:\n%s", h.text())
	}
}

// Declining the confirmation is a decision about the whole batch, not about
// one statement: carrying on would run the statements that were written to
// follow the one just refused.
func TestDecliningAConfirmationStopsTheRestOfTheBatch(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1; DELETE FROM dv_seq WHERE id = 1; SELECT 3")

	h.do(keymap.ActionRunAll)

	if !h.waitForScreen("Run it?") {
		t.Fatalf("no confirmation appeared:\n%s", h.text())
	}

	// Cancel is the button the dialog opens on, so Enter declines.
	h.press(tcell.KeyEnter)

	h.waitFor("the batch to stop where it was declined", func(a *App) bool {
		return strings.Contains(a.status.message, "cancelled at statement 2") &&
			strings.Contains(a.status.message, "1 ran")
	})
}

// The guard stops the user and makes them agree to a write. Reporting "0
// rows" afterwards told them nothing about what they had just agreed to.
func TestAWriteReportsHowManyRowsItChangedOnTheStatusBar(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	seedRows(t, h, 5)
	h.typeSQL("UPDATE dv_ui SET n = n + 100 WHERE n <= 2")

	h.do(keymap.ActionRun)
	confirmWrite(t, h)

	h.waitFor("the bar to say what the write changed", func(a *App) bool {
		return a.status.written != nil && a.status.written.RowsAffected == 2
	})
	if !h.waitForScreen("2 rows affected") {
		t.Errorf("the count never reached the screen:\n%s", h.text())
	}
}

// The guard's decision has to reach the screen, not just the policy engine.
func TestProductionDeleteIsRefusedOnScreen(t *testing.T) {
	h := newHarness(t, config.EnvProd)
	h.typeSQL("DELETE FROM dv_seq")

	h.do(keymap.ActionRun)

	got := h.text()
	if !strings.Contains(got, "Refused") {
		t.Fatalf("no refusal dialog appeared:\n%s", got)
	}
	if !strings.Contains(strings.ToUpper(got), "WHERE") {
		t.Errorf("the refusal does not explain the missing WHERE:\n%s", got)
	}
}

// Outside production the same statement is possible, but only deliberately.
func TestDevelopmentDeleteAsksForTypedConfirmation(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("DELETE FROM dv_seq")

	h.do(keymap.ActionRun)

	got := h.text()
	if !strings.Contains(strings.ToLower(got), "confirm") {
		t.Fatalf("no confirmation dialog appeared:\n%s", got)
	}
	if !strings.Contains(got, "DELETE") {
		t.Errorf("the dialog does not name the phrase to type:\n%s", got)
	}
}

// A syntax error belongs on the status bar, not in a crash.
func TestSyntaxErrorIsReportedOnTheStatusBar(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT FROM WHERE")

	h.do(keymap.ActionRun)

	if !h.waitForScreen("1064") {
		t.Errorf("the server error never reached the status bar:\n%s", h.text())
	}
}

// An unbounded SELECT must run, and the added LIMIT must be disclosed.
func TestUnboundedSelectRunsWithADisclosedLimit(t *testing.T) {
	h := newHarness(t, config.EnvProd)
	h.typeSQL("SELECT 1")

	h.do(keymap.ActionRun)

	if !h.waitForScreen("LIMIT 1000 added") {
		t.Errorf("the injected LIMIT was not disclosed:\n%s", h.text())
	}
}

func TestHelpOpensAndCloses(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionHelp)
	if !strings.Contains(h.text(), "run the statement under the cursor") {
		t.Fatalf("help did not open:\n%s", h.text())
	}

	h.press(tcell.KeyEscape)
	if strings.Contains(h.text(), "run the statement under the cursor") {
		t.Errorf("help did not close:\n%s", h.text())
	}
}

// Help that names a key the application does not actually respond to is
// worse than no help, and it misleads exactly the people consulting it.
func TestHelpShowsTheBindingsThatAreInForce(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	// Rebind to something no default uses, so a hardcoded help text would
	// fail to mention it.
	h.app.app.QueueUpdateDraw(func() {
		if err := h.app.keys.Apply(map[string][]string{"run": {"f8"}}); err != nil {
			t.Errorf("Apply() error = %v", err)
		}
	})
	h.settle()

	h.do(keymap.ActionHelp)

	got := h.text()
	line := lineContaining(t, got, keymap.ActionRun.Describe())

	if !strings.Contains(line, "F8") {
		t.Errorf("the run line does not show the rebound key: %q", line)
	}
	// Checked on this line alone: Shift+F5 still belongs to run-all, and a
	// whole-screen search would match it.
	if strings.Contains(line, "F5") {
		t.Errorf("the run line still shows the replaced default: %q", line)
	}
}

// lineContaining returns the screen line holding want.
func lineContaining(t *testing.T, screen, want string) string {
	t.Helper()

	for _, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, want) {
			return line
		}
	}
	t.Fatalf("no line contains %q:\n%s", want, screen)
	return ""
}

// Every key the help prints must resolve back to the action it is listed
// under.
func TestHelpKeysResolveToTheirActions(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	for _, group := range helpGroups {
		for _, action := range group.actions {
			for _, binding := range h.app.keys.DisplayBindings(action) {
				if got := h.app.keys.Lookup(binding.Event()); got != action {
					t.Errorf("help lists %s under %v, but it resolves to %v",
						binding.Label(false), action, got)
				}
			}
		}
	}
}

// Cancelling is something the user asked for, so it must read as a normal
// outcome — not as the server error KILL QUERY actually produces.
func TestCancellingAStatementReadsAsCancelledNotAsAnError(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT SLEEP(30)")

	h.do(keymap.ActionRun)
	if !h.waitForScreen("running") {
		t.Fatalf("the statement never started:\n%s", h.text())
	}

	h.do(keymap.ActionCopyOrCancel)

	if !h.waitForScreen("cancelled") {
		t.Fatalf("the status bar never reported the cancellation:\n%s", h.text())
	}
	if got := h.text(); strings.Contains(got, "1317") || strings.Contains(strings.ToLower(got), "interrupted") {
		t.Errorf("the raw server interruption error was shown to the user:\n%s", got)
	}
}

// Ctrl+C with nothing to copy and nothing to stop must not be mistaken for
// "quit", and must say why it did nothing.
func TestCopyOrCancelWithNothingToDoKeepsTheInterfaceAlive(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionCopyOrCancel)

	got := h.text()
	if !strings.Contains(got, "nothing selected") {
		t.Errorf("the status bar did not explain the no-op:\n%s", got)
	}
	// Still alive: the editor must still accept text.
	h.typeSQL("SELECT 1")
	if !strings.Contains(h.text(), "SELECT 1") {
		t.Errorf("the interface stopped responding after an idle Ctrl+C:\n%s", h.text())
	}
}

// With a selection, Ctrl+C copies rather than cancelling — the behaviour a
// VS Code or DataGrip user expects from that key.
func TestCopyOrCancelCopiesWhenTextIsSelected(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 42")
	h.selectAllText()

	h.do(keymap.ActionCopyOrCancel)

	if !h.waitForScreen("copied") {
		t.Fatalf("the status bar did not report a copy:\n%s", h.text())
	}
	if got := h.app.clipboard; got != "SELECT 42" {
		t.Errorf("clipboard = %q, want %q", got, "SELECT 42")
	}
}

// With a query in flight and no selection, the same key stops the query.
func TestCopyOrCancelStopsTheQueryWhenNothingIsSelected(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT SLEEP(30)")

	h.do(keymap.ActionRun)
	if !h.waitForScreen("running") {
		t.Fatalf("the statement never started:\n%s", h.text())
	}

	h.do(keymap.ActionCopyOrCancel)

	if !h.waitForScreen("cancelled") {
		t.Errorf("the query was not cancelled:\n%s", h.text())
	}
}

// Editing commands must reach the editor rather than tview's own emacs-style
// bindings, and must be reversible in one step.
func TestEditingActionsRunThroughTheKeymap(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1")

	h.do(keymap.ActionToggleComment)
	if got := h.editorText(); got != "-- SELECT 1" {
		t.Fatalf("after toggle-comment the editor holds %q, want %q", got, "-- SELECT 1")
	}

	h.do(keymap.ActionToggleComment)
	if got := h.editorText(); got != "SELECT 1" {
		t.Errorf("toggling twice left %q, want the original text", got)
	}
}

func TestDuplicateAndDeleteLineActions(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1")

	h.do(keymap.ActionDuplicateLine)
	if got := h.editorText(); got != "SELECT 1\nSELECT 1" {
		t.Fatalf("after duplicate-line the editor holds %q", got)
	}

	h.do(keymap.ActionDeleteLine)
	if got := h.editorText(); got != "SELECT 1" {
		t.Errorf("after delete-line the editor holds %q, want %q", got, "SELECT 1")
	}
}

// A single undo has to reverse an edit command; that is the whole reason
// these go through TextArea.Replace instead of SetText.
func TestEditingActionsAreASingleUndoStep(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT 1")

	h.do(keymap.ActionDuplicateLine)
	if got := h.editorText(); got == "SELECT 1" {
		t.Fatal("duplicate-line did nothing")
	}

	h.undo()

	if got := h.editorText(); got != "SELECT 1" {
		t.Errorf("after one undo the editor holds %q, want %q", got, "SELECT 1")
	}
}

// The sidebar toggle also has to move focus off a pane that disappeared.
// The pane starts hidden, so the first press brings it and the second takes it
// away. The screen has to agree with the state both times — the pane names
// itself in its tab strip now, having no border title to do it.
func TestSidebarToggle(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.do(keymap.ActionToggleSidebar)
	h.waitFor("the schema pane", func(a *App) bool { return a.sidebarVisible })
	if !strings.Contains(h.text(), tabTables) {
		t.Errorf("the schema pane is not on screen after toggling it on:\n%s", h.text())
	}

	h.do(keymap.ActionToggleSidebar)
	h.waitFor("the schema pane to go", func(a *App) bool { return !a.sidebarVisible })
	if strings.Contains(h.text(), tabTables) {
		t.Errorf("the schema pane is still visible after toggling it off:\n%s", h.text())
	}
}

// completionSnapshot is a small schema the completion tests complete against.
func completionSnapshot() catalog.Snapshot {
	schema := testmysql.DefaultDatabase
	return catalog.Snapshot{
		Schemas: []string{schema},
		Tables: map[string][]catalog.Table{
			schema: {{Name: "customers"}, {Name: "customer_notes"}, {Name: "invoices"}},
		},
		Columns: map[string][]catalog.Column{
			catalog.ColumnKey(schema, "customers"): {
				{Name: "id", IsPrimaryKey: true}, {Name: "email"}, {Name: "signed_up_at"},
			},
			catalog.ColumnKey(schema, "invoices"): {
				{Name: "id", IsPrimaryKey: true}, {Name: "customer_id"}, {Name: "amount"},
			},
		},
	}
}

// With several matches the popup opens and lists them.
func TestCompletionOffersTables(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())
	h.typeSQL("SELECT * FROM customer")

	h.do(keymap.ActionComplete)

	got := h.text()
	for _, want := range []string{"customers", "customer_notes"} {
		if !strings.Contains(got, want) {
			t.Errorf("the completion popup does not offer %q:\n%s", want, got)
		}
	}
}

// The alias has to resolve, or qualified completion is useless.
func TestCompletionResolvesAnAlias(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())
	h.typeSQL("SELECT i. FROM customers c JOIN invoices i ON i.customer_id = c.id")

	// Put the caret just after "i.".
	h.moveCaret(len("SELECT i."))
	h.do(keymap.ActionComplete)

	got := h.text()
	if !strings.Contains(got, "amount") {
		t.Errorf("the popup does not offer invoices' columns:\n%s", got)
	}
	if strings.Contains(got, "signed_up_at") {
		t.Errorf("the popup offers customers' columns behind the i. qualifier:\n%s", got)
	}
}

// A single match needs no menu — inserting it directly is the whole point of
// pressing the key.
func TestCompletionInsertsASoleMatchDirectly(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())
	h.typeSQL("SELECT * FROM invoi")

	h.do(keymap.ActionComplete)

	if got := h.editorText(); got != "SELECT * FROM invoices" {
		t.Errorf("editor holds %q, want the completed table name", got)
	}
}

// Accepting a candidate replaces the typed prefix rather than appending to it.
func TestAcceptingACandidateReplacesThePrefix(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())
	h.typeSQL("SELECT * FROM customer")

	h.do(keymap.ActionComplete)
	h.press(tcell.KeyEnter) // accept the highlighted entry

	got := h.editorText()
	if strings.Contains(got, "customercustomer") {
		t.Errorf("editor holds %q; the prefix was appended to instead of replaced", got)
	}
	if !strings.HasPrefix(got, "SELECT * FROM customer") {
		t.Errorf("editor holds %q, want a completed table name", got)
	}
}

// Escape must return control to the editor rather than trapping it.
func TestCompletionPopupCanBeDismissed(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.seedCache(completionSnapshot())
	h.typeSQL("SELECT * FROM customer")

	h.do(keymap.ActionComplete)
	if !strings.Contains(h.text(), "complete") {
		t.Fatalf("the popup did not open:\n%s", h.text())
	}

	h.press(tcell.KeyEscape)

	if strings.Contains(h.text(), "customer_notes") {
		t.Errorf("the popup is still open after Escape:\n%s", h.text())
	}
	// The editor still works.
	h.typeSQL("SELECT 1")
	if got := h.editorText(); got != "SELECT 1" {
		t.Errorf("the editor did not regain focus; it holds %q", got)
	}
}

// Having nothing to offer must be said out loud, or the key looks broken.
func TestCompletionWithNoMatchesSaysSo(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.typeSQL("SELECT * FROM zzzznosuchtable")

	h.do(keymap.ActionComplete)

	if !h.waitForScreen("nothing to complete") {
		t.Errorf("no feedback when there was nothing to offer:\n%s", h.text())
	}
}

// The refresh that runs at startup has to actually populate the cache —
// without it completion would stay empty until a manual reload.
func TestSchemaCacheIsPopulatedInTheBackground(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	name := h.app.conn.DataSource().Name

	h.waitForBackgroundRefresh(name)

	tables, err := h.cache.Tables(context.Background(), name, testmysql.DefaultDatabase)
	if err != nil {
		t.Fatalf("Tables() error = %v", err)
	}
	if len(tables) == 0 {
		t.Error("the cache holds no tables after the background refresh")
	}
}

// seedRows gives the interface tests a table of their own to write to, so a
// write test cannot disturb the fixture the streaming tests count rows from.
func seedRows(t *testing.T, h *harness, n int) {
	t.Helper()
	ctx := context.Background()

	for _, sql := range []string{
		"DROP TABLE IF EXISTS dv_ui",
		"CREATE TABLE dv_ui (n INT PRIMARY KEY)",
	} {
		if _, err := h.app.conn.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	for i := 1; i <= n; i++ {
		if _, err := h.app.conn.Exec(ctx, fmt.Sprintf("INSERT INTO dv_ui (n) VALUES (%d)", i)); err != nil {
			t.Fatalf("seeding dv_ui: %v", err)
		}
	}
	t.Cleanup(func() { h.app.conn.Exec(ctx, "DROP TABLE IF EXISTS dv_ui") })
}

// confirmWrite answers the guard's confirmation dialog with Run.
func confirmWrite(t *testing.T, h *harness) {
	t.Helper()
	if !h.waitForScreen("Run it?") {
		t.Fatalf("no confirmation appeared:\n%s", h.text())
	}
	h.press(tcell.KeyRight)
	h.press(tcell.KeyEnter)
}

// The whole point, driven the way a DBA drives it: type BEGIN, change
// something, look at it, change your mind. Before this the ROLLBACK reported
// success and the row stayed.
func TestATypedRollbackUndoesTheWork(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	seedRows(t, h, 3)

	h.typeSQL("BEGIN")
	h.do(keymap.ActionRun)
	h.waitFor("the transaction to open", func(a *App) bool {
		return a.status.inTransaction
	})
	if !h.waitForScreen("TX") {
		t.Errorf("the status bar does not say a transaction is open:\n%s", h.text())
	}

	h.typeSQL("DELETE FROM dv_ui WHERE n <= 2")
	h.do(keymap.ActionRun)
	confirmWrite(t, h)
	h.waitFor("the delete to report what it removed", func(a *App) bool {
		return a.status.written != nil && a.status.written.RowsAffected == 2
	})

	h.typeSQL("ROLLBACK")
	h.do(keymap.ActionRun)
	h.waitFor("the transaction to close", func(a *App) bool {
		return !a.status.inTransaction
	})

	if got := rowCount(t, h, "dv_ui"); got != 3 {
		t.Errorf("dv_ui has %d rows after ROLLBACK, want 3 — the delete was not undone", got)
	}
}

// Session state was refused because it could not reach the next statement.
// Inside a transaction the connection is held, so it can, and refusing would
// now be the wrong answer.
func TestSetIsRefusedOutsideATransactionAndAcceptedInside(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SET SESSION sql_mode = 'STRICT_ALL_TABLES'")
	h.do(keymap.ActionRun)
	if !h.waitForScreen("Refused") {
		t.Fatalf("SET was not refused outside a transaction:\n%s", h.text())
	}
	h.press(tcell.KeyEnter)

	h.typeSQL("BEGIN")
	h.do(keymap.ActionRun)
	h.waitFor("the transaction to open", func(a *App) bool { return a.status.inTransaction })

	h.typeSQL("SET SESSION sql_mode = 'STRICT_ALL_TABLES'")
	h.do(keymap.ActionRun)
	h.waitFor("the SET to run", func(a *App) bool {
		return a.status.phase == phaseDone && a.status.err == nil
	})

	h.typeSQL("ROLLBACK")
	h.do(keymap.ActionRun)
	h.waitFor("the transaction to close", func(a *App) bool { return !a.status.inTransaction })
}

func rowCount(t *testing.T, h *harness, table string) int {
	t.Helper()

	var n int
	err := h.app.conn.WithControl(context.Background(), func(c *sql.Conn) error {
		return c.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&n)
	})
	if err != nil {
		t.Fatalf("counting %s: %v", table, err)
	}
	return n
}
