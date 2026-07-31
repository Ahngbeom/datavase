// Package ui assembles the terminal interface.
//
// The rule this package follows is that widgets never hold application
// state. Anything a decision depends on — the current statement, the guard
// verdict, the result buffer — lives outside tview, so it can be tested
// without a terminal. Widgets only render what they are handed.
package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/complete"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/guard"
	"github.com/Ahngbeom/datavase/internal/history"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/Ahngbeom/datavase/internal/sqlparse"
	"github.com/Ahngbeom/datavase/internal/vim"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// App is the running interface for one datasource.
type App struct {
	app  *tview.Application
	conn *db.Conn
	cfg  *config.Config

	tree      *tview.TreeView
	editor    *tview.TextArea
	grid      *tview.Table
	statusBar *statusBar

	// vim is the modal input state. It exists whatever the preset is, so
	// switching to the vim keyboard mid-session starts from normal mode
	// rather than from whatever the last session left behind.
	vim              *vim.State
	register         string
	registerLinewise bool
	// listPending holds a half-typed sequence for the panels that are lists.
	listPending rune
	pages       *tview.Pages

	buf    *result.Buffer
	status status

	// keys resolves key events to actions. Holding it here rather than
	// consulting package-level state is what lets the bindings come from
	// configuration.
	keys *keymap.Map

	// sidebarVisible tracks the schema pane, which the layout is rebuilt
	// around when it is toggled.
	sidebarVisible bool
	body           *tview.Flex
	rightPane      tview.Primitive

	// screen is captured during drawing. tview offers no accessor for it,
	// but the clipboard needs it to reach the terminal over OSC 52.
	screen tcell.Screen
	// clipboard is the session-local copy, used for pasting since OSC 52
	// reads are asynchronous and usually refused.
	clipboard string

	// selectionAnchor and selectionCaret track which end of a selection the
	// user is dragging. tview normalises both ends, so the direction cannot
	// be read back from the widget.
	selectionAnchor int
	selectionCaret  int

	// schemaTabs and resultTabs are the two tabbed panes.
	schemaTabs *tabbed
	resultTabs *tabbed

	// The tables tab, and the schema it lists. selectedSchema follows the
	// tree; empty means the datasource's configured default.
	tableFilter    *tview.InputField
	tableList      *tview.List
	selectedSchema string
	// listedTables mirrors what the list currently shows, so a highlighted
	// row can be mapped back to the table it stands for.
	listedTables []catalog.Table

	// ddlView shows a table definition on demand.
	ddlView *tview.TextView
	ddlText string

	// completion is nil until the schema cache is available; the popup says
	// so rather than appearing broken.
	completion *complete.Engine
	cache      *catalog.Cache
	history    *history.Store

	// running is the statement in flight, if any. Only the UI goroutine
	// touches it, which is what makes Ctrl+C unambiguous.
	running *db.Stream

	// batch is a "run everything" in flight, nil otherwise. Like running, it
	// is touched only from the UI goroutine: each statement resumes the batch
	// from the same callback that reports the last one finished.
	batch *batch
}

// batch is the state of a Run-everything.
//
// It is a queue rather than a loop because every step can stop to ask
// something: the guard may refuse, or want a phrase typed, and both answers
// arrive from a dialog long after the statement that raised them returned.
type batch struct {
	stmts []sqlparse.Statement
	// next is the index of the statement to consider next, so it doubles as
	// the human number of the statement being considered once incremented.
	next int
	// ran counts the statements that reached the server and finished without
	// an error.
	ran int
}

// batchSummary is what the status bar says when a Run-everything ends.
//
// It always reports how many of the statements actually ran, including when
// everything succeeded. A batch that stopped in the middle has left the
// database in a state neither the buffer nor the editor shows, and with no
// transaction to unwind it the only remedy is knowing exactly where it
// stopped — so the count is not an error-only detail.
func batchSummary(total, ran int, why string) string {
	summary := fmt.Sprintf("%d statements · %d ran", total, ran)
	if why != "" {
		summary += " · " + why
	}
	return summary
}

// Deps are the optional collaborators an interface can be given.
//
// They are optional so that the interface still opens when the cache cannot
// be created — a read-only home directory should cost the user completion,
// not the whole application.
type Deps struct {
	Keys    *keymap.Map
	Cache   *catalog.Cache
	History *history.Store
}

// New builds the interface for an open connection.
func New(conn *db.Conn, cfg *config.Config, deps Deps) *App {
	ds := conn.DataSource()

	keys := deps.Keys
	if keys == nil {
		keys = keymap.Default()
	}

	a := &App{
		app:             tview.NewApplication(),
		conn:            conn,
		cfg:             cfg,
		keys:            keys,
		cache:           deps.Cache,
		history:         deps.History,
		buf:             result.NewBuffer(cfg.Defaults.BufferMax),
		vim:             vim.New(),
		sidebarVisible:  true,
		selectionAnchor: noAnchor,
		status: status{
			dsName: ds.Name,
			env:    ds.Env,
			schema: ds.Database,
		},
	}
	if deps.Cache != nil {
		a.completion = complete.New(deps.Cache.Names(), ds.Name, ds.Database)
	}
	a.status.message = a.openingMessage(conn.ServerVersion())

	a.buildWidgets()
	a.buildLayout()
	a.bindKeys()
	a.bindEditor()
	a.captureScreen()
	a.loadSchemas()

	return a
}

// captureScreen keeps a reference to the terminal tview is drawing on.
// tview exposes no accessor for it, and the clipboard needs it to emit OSC 52.
func (a *App) captureScreen() {
	a.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		a.screen = screen
		return false
	})
}

// openingMessage greets the user and, when the terminal cannot deliver the
// primary bindings, says so straight away.
//
// Finding out that Ctrl+Enter does nothing by pressing it and watching
// nothing happen is the worst way to learn it; the status bar is already
// there and costs nothing to read.
func (a *App) openingMessage(serverVersion string) string {
	msg := fmt.Sprintf("server %s · %s for keys", serverVersion, a.helpKeyLabel())

	// A modal editor that nobody was told about is one where the first
	// keystroke does nothing, which reads as a broken application rather
	// than as a mode.
	if a.keys.Modal() {
		msg += " · vim keys: i to type, Esc for normal"
	}
	if advice := keymap.TerminalAdvice(os.Getenv("TERM"), a.keys); advice != "" {
		msg += " · " + advice
	}
	return msg
}

// keyLabel names an action's key as this terminal can actually deliver it.
//
// On a terminal without the extended keyboard protocol the primary binding is
// one the user cannot press, so the plain function key at the end of the list
// is named instead — advice nobody can follow is worse than none.
//
// It returns "" for an unbound action, leaving each caller to say what belongs
// there in its place.
func (a *App) keyLabel(action keymap.Action) string {
	bindings := a.keys.DisplayBindings(action)
	if len(bindings) == 0 {
		return ""
	}
	if !keymap.SupportsExtendedKeys(os.Getenv("TERM")) {
		return bindings[len(bindings)-1].Label(onMac)
	}
	return bindings[0].Label(onMac)
}

// helpKeyLabel names the key that opens the key reference. It is looked up
// rather than hardcoded because a rebound help key that the status bar still
// advertises as F1 is worse than no hint at all.
func (a *App) helpKeyLabel() string {
	if label := a.keyLabel(keymap.ActionHelp); label != "" {
		return label
	}
	return "F1"
}

// editorPlaceholder names the run key, preferring one this terminal can
// actually deliver so the hint is not advice the user cannot follow.
func (a *App) editorPlaceholder() string {
	label := a.keyLabel(keymap.ActionRun)
	if label == "" {
		return "SELECT …"
	}

	hint := "SELECT … then " + label + " to run"
	if a.keys.Modal() {
		// The empty buffer is exactly where a modal editor is most confusing:
		// typing does nothing until insert mode is entered.
		return "press i to type · " + hint
	}
	return hint
}

// Run starts the event loop and blocks until the user quits.
func (a *App) Run() error {
	return a.app.SetRoot(a.pages, true).EnableMouse(true).EnablePaste(true).Run()
}

// SetScreen replaces the terminal the interface draws on. Tests pass a
// tcell simulation screen so the real interface — layout, key handling,
// guard dialogs — can be exercised without a terminal.
func (a *App) SetScreen(screen tcell.Screen) {
	a.app.SetScreen(screen)
}

// Stop ends the event loop.
func (a *App) Stop() { a.app.Stop() }

func (a *App) buildWidgets() {
	ds := a.conn.DataSource()

	a.tree = tview.NewTreeView().
		SetRoot(tview.NewTreeNode(rootLabel(ds, sidebarWidth-4)).
			SetColor(envColour(ds.Env)))
	// Explicit tree lines: indentation alone reads as a weak hierarchy, and
	// a weak hierarchy is what made schemas look nested under the server.
	a.tree.SetGraphics(true)
	a.tree.SetSelectedFunc(a.onTreeSelect)

	a.editor = tview.NewTextArea().
		SetPlaceholder(a.editorPlaceholder()).
		SetWrap(false)
	a.editor.SetBorder(true).SetTitle(" editor ")
	a.editor.SetClipboard(a.setClipboard, a.readClipboard)

	a.grid = tview.NewTable().
		SetContent(newGridContent(a.buf)).
		SetFixed(1, 0).
		SetSelectable(true, true)

	// Both panes are tabbed, and share one component so they cannot drift
	// apart in behaviour.
	a.schemaTabs = newTabbed(" schema ")
	a.schemaTabs.add(tabTree, a.tree)
	a.schemaTabs.add(tabTables, a.buildTablesTab())

	a.resultTabs = newTabbed(" results ")
	a.resultTabs.add(tabResults, a.grid)
	a.resultTabs.add(tabDDL, a.buildDDLTab())

	a.statusBar = newStatusBar(a.currentStatus)
}

// sidebarWidth is the schema pane's fixed width. Fixed rather than
// proportional so the editor does not reflow when the window is resized.
const sidebarWidth = 34

func (a *App) buildLayout() {
	a.rightPane = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.editor, 0, 2, true).
		AddItem(a.resultTabs, 0, 3, false)

	a.body = tview.NewFlex()
	a.layoutBody()

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.body, 0, 1, true).
		AddItem(a.statusBar, 1, 0, false)

	a.pages = tview.NewPages().AddPage(pageMain, root, true, true)
}

// layoutBody rebuilds the horizontal split, which is how the sidebar is
// shown or hidden.
func (a *App) layoutBody() {
	a.body.Clear()
	if a.sidebarVisible {
		a.body.AddItem(a.schemaTabs, sidebarWidth, 0, false)
	}
	a.body.AddItem(a.rightPane, 0, 1, true)
}

// setSchema points completion, the tables tab and the status bar at a schema.
//
// One place to change it, so the three can never disagree about which schema
// an unqualified query will reach.
func (a *App) setSchema(schema string) {
	a.selectedSchema = schema
	a.status.schema = schema

	if a.completion != nil {
		a.completion.SetSchema(schema)
	}
	if a.schemaTabs != nil && a.schemaTabs.current() == tabTables {
		a.renderTables()
	}
}

// cycleTab switches tabs in whichever pane has focus.
func (a *App) cycleTab() {
	if a.schemaHasFocus() {
		a.schemaTabs.cycle()
		a.focusVisibleSchemaTab()
		return
	}
	a.resultTabs.cycle()
	a.app.SetFocus(a.resultPrimitive())
}

// focusVisibleSchemaTab moves focus onto whatever the newly shown tab holds,
// and refreshes the tables list so it reflects the current schema.
func (a *App) focusVisibleSchemaTab() {
	if a.schemaTabs.current() == tabTables {
		a.renderTables()
	}
	a.app.SetFocus(a.schemaPrimitive())
}

func (a *App) toggleSidebar() {
	a.sidebarVisible = !a.sidebarVisible
	a.layoutBody()

	// Focus cannot stay on a pane that is no longer on screen.
	if !a.sidebarVisible && a.schemaHasFocus() {
		a.app.SetFocus(a.editor)
	}
}

// Tab names for the two tabbed panes.
const (
	tabTree    = "tree"
	tabTables  = "tables"
	tabResults = "results"
	tabDDL     = "DDL"
)

const (
	pageMain      = "main"
	pageConfirm   = "confirm"
	pageHelp      = "help"
	pageComplete  = "complete"
	pagePalette   = "palette"
	pageHistory   = "history"
	pageGoTo      = "goto"
	pageUseSchema = "useschema"
)

// focusOrder is the Tab cycle. A hidden sidebar is skipped rather than
// focused invisibly, which would look like Tab stopped working.
func (a *App) focusOrder() []tview.Primitive {
	order := []tview.Primitive{a.editor, a.resultPrimitive()}
	if a.sidebarVisible {
		order = append(order, a.schemaPrimitive())
	}
	return order
}

// resultPrimitive and schemaPrimitive return whichever widget the visible tab
// holds, so Tab lands on something focusable rather than on the tab strip.
func (a *App) resultPrimitive() tview.Primitive {
	if a.resultTabs.current() == tabDDL {
		return a.ddlView
	}
	return a.grid
}

func (a *App) schemaPrimitive() tview.Primitive {
	if a.schemaTabs.current() == tabTables {
		return a.tableFilter
	}
	return a.tree
}

// schemaHasFocus reports whether any widget of the schema pane holds focus.
func (a *App) schemaHasFocus() bool {
	focus := a.app.GetFocus()
	return focus == a.tree || focus == a.tableFilter || focus == a.tableList
}

func (a *App) bindKeys() {
	a.app.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		// While a dialog is open it owns the keyboard.
		if name, _ := a.pages.GetFrontPage(); name != pageMain {
			return ev
		}

		if a.dispatch(a.keys.Lookup(ev)) {
			return nil
		}
		return ev
	})
}

// dispatch performs an action and reports whether it consumed the key.
//
// Every key decision lives in the keymap; this switch only says what each
// action does. That split is what lets the bindings become configuration
// without the UI knowing anything about terminals.
func (a *App) dispatch(action keymap.Action) bool {
	switch action {
	case keymap.ActionNone:
		return false

	case keymap.ActionRun:
		a.execute()
	case keymap.ActionRunAll:
		a.executeAll()
	case keymap.ActionCancel:
		a.cancelRunning()
	case keymap.ActionCopyOrCancel:
		a.copyOrCancel()

	case keymap.ActionNextPane:
		a.cycleFocus(1)
	case keymap.ActionPrevPane:
		a.cycleFocus(-1)
	case keymap.ActionToggleSidebar:
		a.toggleSidebar()
	case keymap.ActionRefreshSchema:
		a.loadSchemas()

	case keymap.ActionUseSchema:
		a.showUseSchema()

	case keymap.ActionComplete:
		a.showCompletion()
	case keymap.ActionCycleTab:
		a.cycleTab()
	case keymap.ActionInspect:
		a.inspectTable()
	case keymap.ActionCommandPalette:
		a.showCommandPalette()
	case keymap.ActionFind:
		a.showHistory()
	case keymap.ActionGoToTable:
		a.showGoToTable()

	case keymap.ActionHelp:
		a.showHelp()
	case keymap.ActionQuit:
		a.quit()

	default:
		// Actions the editor owns are handled closer to the widget; anything
		// still unhandled here belongs to a feature that is not built yet.
		if action.Reserved() {
			a.notice(fmt.Sprintf("%s is not built yet", action.Describe()))
			return true
		}
		return false
	}
	return true
}

func (a *App) cycleFocus(delta int) {
	order := a.focusOrder()
	current := a.app.GetFocus()

	for i, p := range order {
		if p == current {
			next := (i + delta + len(order)) % len(order)
			a.app.SetFocus(order[next])
			return
		}
	}
	a.app.SetFocus(order[0])
}

func (a *App) quit() {
	if a.running != nil {
		// Leaving a statement running server-side after the client exits
		// would keep burning resources with nobody watching.
		a.running.Cancel()
	}
	a.app.Stop()
}

// execute runs the statement under the cursor.
func (a *App) execute() {
	if a.running != nil {
		a.notice("a statement is already running; ^C cancels it")
		return
	}

	text := a.editor.GetText()
	row, column, _, _ := a.editor.GetCursor()

	stmt, ok := sqlparse.StatementAt(text, offsetAt(text, row, column))
	if !ok {
		a.notice("no statement under the cursor")
		return
	}
	a.runStatement(stmt)
}

// runStatement puts one statement through the guard and then the engine.
func (a *App) runStatement(stmt sqlparse.Statement) {
	decision := guard.Evaluate(stmt, a.policy())
	switch decision.Verdict {
	case guard.Deny:
		a.refuse(decision)
	case guard.Confirm:
		a.confirm(stmt, decision)
	default:
		a.start(stmt, decision)
	}
}

// executeAll runs every statement in the editor, stopping at the first
// refusal so a rejected statement cannot be skipped over silently.
func (a *App) executeAll() {
	if a.running != nil {
		a.notice("a statement is already running; ^C cancels it")
		return
	}

	stmts := sqlparse.Split(a.editor.GetText())
	switch len(stmts) {
	case 0:
		a.notice("nothing to run")
		return
	case 1:
		// One statement is not a batch. Going through the queue would only
		// add a summary line saying "1 statement · 1 ran" to something the
		// row count already describes.
		a.runStatement(stmts[0])
		return
	}

	a.batch = &batch{stmts: stmts}
	a.advanceBatch()
}

// advanceBatch puts the next statement of a Run-everything through the guard.
//
// Each verdict either continues the queue or ends it. Nothing is skipped: a
// refusal in the middle stops the rest, because the statements after it were
// written to follow the one that did not run.
func (a *App) advanceBatch() {
	b := a.batch
	if b == nil {
		return
	}
	if b.next >= len(b.stmts) {
		a.finishBatch("")
		return
	}

	stmt := b.stmts[b.next]
	b.next++

	decision := guard.Evaluate(stmt, a.policy())
	switch decision.Verdict {
	case guard.Deny:
		a.finishBatch(fmt.Sprintf("refused at statement %d", b.next))
		a.refuse(decision)
	case guard.Confirm:
		a.confirm(stmt, decision)
	default:
		a.start(stmt, decision)
	}
}

// resumeBatch continues the queue after a statement has finished, or ends it
// if that statement did not.
//
// A failure stops the rest. The statements in a file are written to run in
// order, so continuing past a broken one applies the later half of a change
// to a database that never received the first.
func (a *App) resumeBatch(err error) {
	b := a.batch
	if b == nil {
		return
	}

	switch {
	case err == nil:
		b.ran++
		a.advanceBatch()
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), db.IsInterrupted(err):
		a.finishBatch(fmt.Sprintf("cancelled at statement %d", b.next))
	default:
		a.finishBatch(fmt.Sprintf("failed at statement %d", b.next))
	}
}

// abandonBatch ends the queue because the user declined a confirmation.
func (a *App) abandonBatch() {
	if a.batch == nil {
		return
	}
	a.finishBatch(fmt.Sprintf("cancelled at statement %d", a.batch.next))
}

// finishBatch ends a Run-everything and reports how far it got.
func (a *App) finishBatch(why string) {
	b := a.batch
	if b == nil {
		return
	}
	a.batch = nil
	a.notice(batchSummary(len(b.stmts), b.ran, why))
}

// copyOrCancel resolves Ctrl+C, whose meaning depends on what is on screen.
//
// A selection means the user wants the text; no selection during a query
// means they want it stopped. This mirrors how a terminal behaves and avoids
// forcing a choice between the two most expected behaviours of one key.
func (a *App) copyOrCancel() {
	// Inside the DDL tab the obvious thing to copy is the definition on
	// screen, not whatever happens to be selected in the editor.
	if a.app.GetFocus() == a.ddlView && a.copyDDL() {
		return
	}
	if a.editor.HasSelection() {
		a.copySelection()
		return
	}
	if a.running != nil {
		a.cancelRunning()
		return
	}
	a.notice("nothing selected and nothing running")
}

func (a *App) policy() guard.Policy {
	return guard.Policy{
		Env:           a.conn.DataSource().Env,
		AutoLimit:     a.cfg.Defaults.AutoLimit,
		WritesEnabled: a.status.writesEnabled,
	}
}

// start sends the statement and streams the result into the buffer.
func (a *App) start(stmt sqlparse.Statement, decision guard.Decision) {
	sql := stmt.SQL
	if decision.InjectLimit > 0 {
		sql = sqlparse.AppendLimit(stmt, decision.InjectLimit)
	}

	a.buf.Reset()
	a.grid.ScrollToBeginning()
	a.resultTabs.show(tabResults)

	a.status.phase = phaseRunning
	a.status.err = nil
	a.status.rows = 0
	a.status.message = ""
	a.status.limitInjected = decision.InjectLimit
	a.status.truncated = false
	a.status.written = nil

	started := time.Now()
	stream := a.conn.Query(context.Background(), sql, db.Options{
		ChunkSize: a.cfg.Defaults.FetchChunk,
		MaxRows:   a.cfg.Defaults.BufferMax,
		// The chosen schema travels with the statement. Without this the
		// picker would change what the status bar says and nothing else.
		Schema: a.selectedSchema,
		// A write is sent for its count rather than for rows, which is the
		// only moment the server's answer can still be read.
		Exec: !stmt.Kind().ReturnsRows(),
	})
	a.running = stream

	go a.consume(stream, sql, started)
}

// consume forwards stream events onto the UI goroutine.
//
// tview may only be touched from its own goroutine, hence QueueUpdateDraw.
// Rows land in the buffer immediately so the grid can show them as they
// arrive rather than waiting for the statement to finish.
func (a *App) consume(stream *db.Stream, sqlText string, started time.Time) {
	for ev := range stream.Events {
		switch ev.Kind {
		case db.EventColumns:
			a.buf.SetColumns(ev.Columns, ev.Types)
			a.app.QueueUpdateDraw(func() {})

		case db.EventRows:
			a.buf.Append(ev.Rows)
			rows := a.buf.RowCount()
			a.app.QueueUpdateDraw(func() {
				a.status.rows = rows
				a.status.elapsed = time.Since(started)
			})
		}
	}

	err := stream.Err()
	truncated := stream.Truncated() || a.buf.AtCapacity()
	written, wrote := stream.Result()
	elapsed := time.Since(started)
	rows := a.buf.RowCount()

	a.app.QueueUpdateDraw(func() {
		a.running = nil
		a.status.rows = rows
		a.status.elapsed = elapsed
		a.status.truncated = truncated
		if wrote {
			a.status.written = &written
		}

		a.record(sqlText, rows, elapsed)

		switch {
		case err == nil:
			a.status.phase = phaseDone

		// A cancellation is an outcome the user asked for, so it reads as a
		// normal finish. The driver usually notices the cancelled context
		// first, but KILL QUERY can also surface as a server interruption —
		// both mean the same thing here.
		case errors.Is(err, context.Canceled),
			errors.Is(err, context.DeadlineExceeded),
			db.IsInterrupted(err):
			a.status.phase = phaseDone
			a.status.message = "cancelled"

		default:
			a.status.phase = phaseFailed
			a.status.err = err
		}

		// The queue is resumed from here rather than from start(), because
		// this is the first moment the next statement may be sent: MySQL will
		// not accept one until this result set has been read to the end.
		a.resumeBatch(err)
	})
	stream.Close()
}

func (a *App) cancelRunning() {
	if a.running == nil {
		a.notice("nothing is running")
		return
	}
	a.notice("cancelling…")
	go a.running.Cancel()
}

// notice puts a one-off message on the status bar. The bar reads the status
// value itself on the next draw, so setting the field is the whole job.
func (a *App) notice(msg string) {
	a.status.message = msg
}

func (a *App) refreshStatus() {
}

// renderStatus asks for a redraw; the bar reads the status itself at Draw
// time, when it knows how much room it has.
func (a *App) renderStatus() {}

// currentStatus is what the status bar renders.
//
// The modal fields are filled in here rather than pushed on every keystroke:
// the bar reads this at draw time, so the mode and any half-typed sequence
// are always the live ones without anything having to remember to update it.
func (a *App) currentStatus() status {
	s := a.status
	if a.keys.Modal() {
		s.vimMode = a.vim.Mode().String()
		s.vimPending = a.vim.Pending()
	}
	return s
}

func envColour(env config.Env) tcell.Color {
	switch env {
	case config.EnvProd:
		return colourEnvProd
	case config.EnvStage:
		return colourEnvStage
	default:
		return colourEnvDev
	}
}

// record adds a finished statement to the query history.
//
// Failures are ignored: history is a convenience, and a full disk should not
// interrupt the work the user came to do.
func (a *App) record(sqlText string, rows int, elapsed time.Duration) {
	if a.history == nil {
		return
	}

	entry := history.Entry{
		DataSource: a.conn.DataSource().Name,
		SQL:        sqlText,
		Rows:       rows,
		Elapsed:    elapsed,
		At:         time.Now(),
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
		defer cancel()
		a.history.Add(ctx, entry)
	}()
}
