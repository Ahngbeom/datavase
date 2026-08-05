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
	"github.com/Ahngbeom/datavase/internal/procs"
	"github.com/Ahngbeom/datavase/internal/recent"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/sqlparse"
	"github.com/Ahngbeom/datavase/internal/vim"
	"github.com/Ahngbeom/datavase/internal/worktree"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// App is the running interface for one datasource.
type App struct {
	app *tview.Application
	// sess is the open datasource, and conn is its connection — held apart
	// only because conn is what almost everything wants. Switching replaces
	// both and closes what it replaced, tunnel included.
	sess *session.Session
	conn *db.Conn
	cfg  *config.Config

	// connect opens another datasource. It is a field so that the interface
	// does not have to know where a password comes from, and so a test can
	// switch without a keychain.
	connect func(context.Context, *config.DataSource) (*session.Session, error)

	// spine is the environment colour down the left. Held because it is the
	// one piece of the frame that has to be repainted when the datasource
	// changes, and a stale one is worse than none.
	spine *tview.Box

	tree      *tview.TreeView
	editor    *tview.TextArea
	grid      *tview.Table
	topBar    *topBar
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

	buf *result.Buffer
	// content is held rather than handed to the table and forgotten, because
	// it owns the row ordering and so is the only thing that can say which
	// buffer row a grid row draws.
	content *gridContent
	status  status

	// keys resolves key events to actions. Holding it here rather than
	// consulting package-level state is what lets the bindings come from
	// configuration.
	keys *keymap.Map

	// sidebarVisible tracks the schema pane, which the layout is rebuilt
	// around when it is toggled. sidebarRule is the hairline beside it.
	//
	// It starts false. This application already chose overlay finders as its
	// way around — a table, a schema and a file each have a key that opens a
	// searchable list — and a permanent tree on top of them is a third of the
	// screen spent saying what those already answer. It is one key away.
	sidebarVisible bool
	sidebarRule    *rule
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

	// The three regions of the body. Each owns a one-line header; none draws
	// a box.
	editorRegion *tabbed
	schemaTabs   *tabbed
	resultTabs   *tabbed

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

	// planView shows the query plan, laying it out at whatever width it has
	// when it is drawn.
	planView *planPane

	// sessionsView lists what else is running on the server.
	sessionsView *tview.TextView

	// completion is nil until the schema cache is available; the popup says
	// so rather than appearing broken.
	completion *complete.Engine
	cache      *catalog.Cache
	history    *history.Store

	// wt is the attached directory of SQL work, nil until one is attached.
	// wtSnap is the last listing taken from it, held so the finder can filter
	// without asking git on every keystroke.
	wt     *worktree.Worktree
	wtSnap worktree.Snapshot
	// openFile is the file the editor was loaded from, if any.
	openFile openFile
	// recentDirs are the directories attached before, offered by the attach
	// dialog before anything has been typed. Nil when the state directory
	// could not be read.
	recentDirs *recent.List
	// introPath is where the first-run card records that it has been shown.
	// Empty means there is nowhere to record it, which is also the session
	// that never shows it.
	introPath string
	// search is the last pattern looked for and where, so that n and N have
	// something to repeat once the prompt has closed.
	search searchState

	// running is the statement in flight, if any. Only the UI goroutine
	// touches it, which is what makes Ctrl+C unambiguous.
	running *db.Stream

	// lastChange is what "." repeats.
	lastChange change

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
// database in a state neither the buffer nor the editor shows, so the count is
// not an error-only detail.
//
// Inside a transaction that count means something different: the work is still
// undoable, and the sentence says so. Reporting where a batch stopped without
// saying whether it can be taken back would leave the reader to guess the one
// thing they need.
func batchSummary(total, ran int, why string, inTransaction bool) string {
	summary := fmt.Sprintf("%s · %d ran", plural(total, "statement"), ran)
	if why != "" {
		summary += " · " + why
	}
	if inTransaction {
		summary += " · in transaction, rollback still possible"
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
	// Worktree is the directory named by --dir, if any. Nil means the session
	// starts unattached, which is the ordinary case.
	Worktree *worktree.Worktree
	// Recent is the list of directories attached before. Nil costs the
	// shortcut and nothing else.
	Recent *recent.List
	// Connect opens another datasource, for switching mid-session. Nil leaves
	// the session on the datasource it started with, and the switch says so
	// rather than failing silently.
	Connect func(context.Context, *config.DataSource) (*session.Session, error)
	// IntroPath is where "the first-run card has been shown" is recorded.
	// Empty means never show it, which is what a session with no usable state
	// directory gets — and what every test that is not about the card gets.
	IntroPath string
}

// New builds the interface for an open session.
//
// The session rather than the connection, because switching datasource has to
// close what it left — and a tunnel closed by nobody outlives the thing it
// was carrying.
func New(sess *session.Session, cfg *config.Config, deps Deps) *App {
	conn := sess.Conn
	ds := conn.DataSource()

	keys := deps.Keys
	if keys == nil {
		keys = keymap.Default()
	}

	a := &App{
		app:             tview.NewApplication(),
		sess:            sess,
		conn:            conn,
		connect:         deps.Connect,
		cfg:             cfg,
		keys:            keys,
		cache:           deps.Cache,
		history:         deps.History,
		wt:              deps.Worktree,
		recentDirs:      deps.Recent,
		introPath:       deps.IntroPath,
		buf:             result.NewBuffer(cfg.Defaults.BufferMax),
		vim:             vim.New(),
		selectionAnchor: noAnchor,
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
	// A worktree given on the command line is listed before the first draw, so
	// the status bar names the branch rather than filling in a moment later.
	a.rescan()
	// Last, so the card is drawn over an interface that is already built: it
	// names the datasource and the keys, and both have to be settled first.
	a.showIntroOnce()

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
	// The schema tree is not on screen, so this is where anyone learns it
	// exists at all.
	msg := fmt.Sprintf("server %s · %s for keys · %s for the schema tree",
		serverVersion, a.helpKeyLabel(), a.keyLabel(keymap.ActionToggleSidebar))

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

// helpKeyLabel names the key that opens the key reference. It is looked up
// rather than hardcoded because a rebound help key that the status bar still
// advertises as F1 is worse than no hint at all.
func (a *App) helpKeyLabel() string {
	bindings := a.keys.DisplayBindings(keymap.ActionHelp)
	if len(bindings) == 0 {
		return "F1"
	}
	return bindings[0].Label(onMac)
}

// keyLabel names an action's key as this terminal can actually deliver it.
//
// On a terminal without the extended keyboard protocol the primary binding is
// one the user cannot press, so the function-key fallback is named instead —
// advice nobody can follow is worse than none.
func (a *App) keyLabel(action keymap.Action) string {
	bindings := a.keys.DisplayBindings(action)
	if len(bindings) == 0 {
		return action.String()
	}
	if !keymap.SupportsExtendedKeys(os.Getenv("TERM")) {
		return bindings[len(bindings)-1].Label(onMac)
	}
	return bindings[0].Label(onMac)
}

// editorPlaceholder names the run key, preferring one this terminal can
// actually deliver so the hint is not advice the user cannot follow.
func (a *App) editorPlaceholder() string {
	bindings := a.keys.DisplayBindings(keymap.ActionRun)
	if len(bindings) == 0 {
		return "SELECT …"
	}

	binding := bindings[0]
	if !keymap.SupportsExtendedKeys(os.Getenv("TERM")) {
		// Fall back to the last binding, which is the plain function key
		// that works everywhere.
		binding = bindings[len(bindings)-1]
	}
	hint := "SELECT … then " + binding.Label(onMac) + " to run"
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

	// The root is the tree's one heading. It no longer carries the
	// environment's colour: the spine does that now, and saying it twice is
	// what made the palette run out of meanings.
	a.tree = tview.NewTreeView().
		SetRoot(tview.NewTreeNode(rootLabel(ds, sidebarWidth-4)).
			SetColor(colourAccent))
	// Explicit tree lines: indentation alone reads as a weak hierarchy, and
	// a weak hierarchy is what made schemas look nested under the server.
	a.tree.SetGraphics(true)
	a.tree.SetSelectedFunc(a.onTreeSelect)

	a.editor = tview.NewTextArea().
		SetPlaceholder(a.editorPlaceholder()).
		SetWrap(false)
	a.editor.SetClipboard(a.setClipboard, a.readClipboard)

	a.content = newGridContent(a.buf)
	// The header row is pinned, and so is the first column. Saying how many
	// columns have scrolled off is half the answer; the other half is keeping
	// the one that says which row this is, since a screenful beginning at
	// "total_cents" is unreadable however honestly the bar reports it — and the
	// first column of a SELECT is where people put the id.
	a.grid = tview.NewTable().
		SetContent(a.content).
		SetFixed(1, 1).
		SetSelectable(true, true)
	a.grid.SetInputCapture(a.gridKey)

	// Every region shares one component so they cannot drift apart in
	// behaviour, including the editor — which has no tabs, only a header
	// saying which file it holds.
	a.editorRegion = newTabbed().watch(a.editorDetail)
	a.editorRegion.only(a.editor)

	a.schemaTabs = newTabbed()
	a.schemaTabs.add(tabTree, a.tree)
	a.schemaTabs.add(tabTables, a.buildTablesTab())

	a.resultTabs = newTabbed().watch(a.resultDetail)
	a.resultTabs.add(tabResults, a.grid)
	a.resultTabs.add(tabDDL, a.buildDDLTab())
	a.resultTabs.add(tabPlan, a.buildPlanTab())
	a.resultTabs.add(tabSessions, a.buildSessionsTab())

	a.topBar = newTopBar(a.currentTopBar)
	a.statusBar = newStatusBar(a.currentStatus)
}

// editorDetail names the file the buffer came from, and whether it has
// diverged from it. Empty for a scratch buffer, which has no file to name.
func (a *App) editorDetail() string {
	if !a.openFile.isOpen() {
		return ""
	}
	if a.fileDirty() {
		return a.openFile.rel + " *"
	}
	return a.openFile.rel
}

// resultDetail says what the empty results tab would otherwise not say.
//
// With no box around it, an empty region is simply blank — which reads as a
// gap in the layout rather than as a pane waiting for a statement.
func (a *App) resultDetail() string {
	if a.resultTabs.current() == tabResults && a.buf.ColumnCount() == 0 {
		return "run a statement to see rows here"
	}
	return ""
}

// currentTopBar is where the session is, read at draw time so the schema and
// the branch cannot lag behind the thing that changed them.
func (a *App) currentTopBar() topBarState {
	ds := a.conn.DataSource()
	return topBarState{
		env:     ds.Env,
		dsName:  ds.Name,
		schema:  a.currentSchema(),
		branch:  a.worktreeLabel(),
		helpKey: a.helpKeyLabel(),
	}
}

// sidebarWidth is the schema pane's fixed width. Fixed rather than
// proportional so the editor does not reflow when the window is resized.
const sidebarWidth = 34

func (a *App) buildLayout() {
	a.rightPane = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.editorRegion, 0, 2, true).
		AddItem(newRule(false), 1, 0, false).
		AddItem(a.resultTabs, 0, 3, false)

	// Held rather than built per toggle, so showing the sidebar does not leave
	// a discarded primitive behind on every press.
	a.sidebarRule = newRule(true)

	a.body = tview.NewFlex()
	a.layoutBody()

	// The top line says where the session is, the bottom what just happened.
	inner := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.topBar, 1, 0, false).
		AddItem(newRule(false), 1, 0, false).
		AddItem(a.body, 0, 1, true).
		AddItem(newRule(false), 1, 0, false).
		AddItem(a.statusBar, 1, 0, false)

	// The environment runs down the outside of everything. Held, because
	// switching datasource has to repaint it in the same step.
	a.spine = newSpine(a.conn.DataSource().Env)
	root := tview.NewFlex().
		AddItem(a.spine, 1, 0, false).
		AddItem(inner, 0, 1, true)

	a.pages = tview.NewPages().AddPage(pageMain, root, true, true)
}

// layoutBody rebuilds the horizontal split, which is how the sidebar is
// shown or hidden.
func (a *App) layoutBody() {
	a.body.Clear()
	if a.sidebarVisible {
		a.body.AddItem(a.schemaTabs, sidebarWidth, 0, false)
		a.body.AddItem(a.sidebarRule, 1, 0, false)
	}
	a.body.AddItem(a.rightPane, 0, 1, true)
}

// setSchema points completion and the tables tab at a schema.
//
// One place to change it, so the two can never disagree about which schema an
// unqualified query will reach. The top bar is not told: it reads
// currentSchema() when it draws, so it cannot be left behind.
func (a *App) setSchema(schema string) {
	a.selectedSchema = schema

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
	// Lower case like the others. A single shouted name in a strip of quiet
	// ones reads as an error rather than as an acronym.
	tabDDL      = "ddl"
	tabPlan     = "plan"
	tabSessions = "sessions"
)

const (
	pageMain       = "main"
	pageConfirm    = "confirm"
	pageHelp       = "help"
	pageComplete   = "complete"
	pagePalette    = "palette"
	pageHistory    = "history"
	pageGoTo       = "goto"
	pageUseSchema  = "useschema"
	pageFiles      = "files"
	pageAttach     = "attach"
	pageSearch     = "search"
	pageCommand    = "command"
	pageDataSource = "datasource"
	pageKill       = "kill"
	pageIntro      = "intro"
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
		a.inspect()
	case keymap.ActionSortColumn:
		a.sortColumn()
	case keymap.ActionSwitchDataSource:
		a.showDataSources()
	case keymap.ActionExplain:
		a.explainStatement()
	case keymap.ActionAnalyze:
		a.analyzeStatement()
	case keymap.ActionSessions:
		a.showSessions()
	case keymap.ActionKillSession:
		a.showKillSession(procs.StopStatement)
	case keymap.ActionLocks:
		a.showLocks()
	case keymap.ActionCommandPalette:
		a.showCommandPalette()
	case keymap.ActionFind:
		a.showTextSearch(false)
	case keymap.ActionFindNext:
		a.searchAgain(false)
	case keymap.ActionFindPrev:
		a.searchAgain(true)
	case keymap.ActionSearchHistory:
		a.showHistory()
	case keymap.ActionGoToTable:
		a.showGoToTable()
	case keymap.ActionFindFile:
		a.showFindFile()
	case keymap.ActionSaveFile:
		a.saveFile()

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

// quit leaves, asking first when the buffer holds unsaved file changes.
//
// The confirmation only appears for an edited file, not for a scratch buffer:
// text typed into datavase has never been anywhere else, and prompting about
// every session's leftovers is how a prompt stops being read.
func (a *App) quit() {
	// Two things can be lost by leaving, and both get asked about, dearest
	// first. An open transaction is unsaved work of the same kind as an
	// unsaved buffer and rather more expensive, since quitting rolls it back.
	if a.conn.InTransaction() {
		a.confirmDiscardTransaction()
		return
	}
	a.quitWithUnsavedFile()
}

// quitWithUnsavedFile is the second question quitting has to ask, and where
// the transaction dialog continues once it has been answered — otherwise
// agreeing to roll back would take an unsaved file down with it, unasked.
func (a *App) quitWithUnsavedFile() {
	if a.fileDirty() {
		a.confirmDiscard(
			fmt.Sprintf("%s has unsaved changes.\n\nQuit and lose them?", a.openFile.rel),
			"Quit",
			a.forceQuit)
		return
	}
	a.forceQuit()
}

// forceQuit leaves without asking anything further.
func (a *App) forceQuit() {
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

	// Transaction control opens or ends the pinned connection rather than
	// running on one, so it never becomes a Stream. Typing BEGIN works because
	// that is what a DBA types; there is a palette entry for the same thing,
	// not instead of it.
	if decision.Verdict == guard.Allow && opensOrEndsTransaction(stmt) {
		a.transactionControl(stmt.Verb())
		return
	}

	switch decision.Verdict {
	case guard.Deny:
		a.refuse(decision)
	case guard.Confirm:
		a.confirm(stmt, decision)
	default:
		a.start(stmt, decision)
	}
}

// executeAll runs every statement in the editor, in order, stopping at the
// first refusal so a rejected statement cannot be skipped over silently.
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
	a.notice(batchSummary(len(b.stmts), b.ran, why, a.conn.InTransaction()))
}

// copyOrCancel resolves Ctrl+C, whose meaning depends on what is on screen.
//
// A selection means the user wants the text; no selection during a query
// means they want it stopped. This mirrors how a terminal behaves and avoids
// forcing a choice between the two most expected behaviours of one key.
func (a *App) copyOrCancel() {
	intent := copyContext{
		running:      a.running != nil,
		onDDL:        a.app.GetFocus() == a.ddlView,
		onPlan:       a.app.GetFocus() == a.planView,
		onGrid:       a.app.GetFocus() == a.grid,
		hasSelection: a.editor.HasSelection(),
	}.resolve()

	switch intent {
	case intentCancel:
		a.cancelRunning()
	case intentDefinition:
		if !a.copyDDL() {
			a.notice("nothing to copy")
		}
	case intentPlan:
		if !a.copyPlan() {
			a.notice("nothing to copy")
		}
	case intentSelection:
		a.copySelection()
	case intentCell:
		if !a.copyCell() {
			a.notice("nothing to copy")
		}
	default:
		a.notice("nothing selected and nothing running")
	}
}

func (a *App) policy() guard.Policy {
	return guard.Policy{
		Env:           a.conn.DataSource().Env,
		AutoLimit:     a.cfg.Defaults.AutoLimit,
		WritesEnabled: a.status.writesEnabled,
		InTransaction: a.conn.InTransaction(),
	}
}

// start sends the statement and streams the result into the buffer.
func (a *App) start(stmt sqlparse.Statement, decision guard.Decision) {
	sql := stmt.SQL
	if decision.InjectLimit > 0 {
		sql = sqlparse.AppendLimit(stmt, decision.InjectLimit)
	}

	a.buf.Reset()
	// A column chosen on the last result says nothing about this one, which
	// may not even have that many columns.
	a.content.unsort()
	a.grid.ScrollToBeginning()
	a.resultTabs.show(tabResults)

	a.status.phase = phaseRunning
	a.status.err = nil
	a.status.rows = 0
	a.status.message = ""
	a.status.limitInjected = decision.InjectLimit
	a.status.truncated = false
	a.status.written = nil
	a.status.warnings = nil

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

	// Whether the answer is a plan is settled here, from the statement that
	// was sent, rather than remembered from the key that asked: a confirmation
	// dialog can sit between the two for as long as the user likes.
	go a.consume(stream, sql, started, stmt.PlansAsJSON())
}

// consume forwards stream events onto the UI goroutine.
//
// tview may only be touched from its own goroutine, hence QueueUpdateDraw.
// Rows land in the buffer immediately so the grid can show them as they
// arrive rather than waiting for the statement to finish.
func (a *App) consume(stream *db.Stream, sqlText string, started time.Time, plansJSON bool) {
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
	warnings := stream.Warnings()
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
		a.status.warnings = warnings

		a.record(sqlText, rows, elapsed)

		switch {
		case err == nil:
			a.status.phase = phaseDone
			// A statement that answered in JSON answered with a plan, and a
			// screenful of braces in the grid is not what was asked for.
			if plansJSON && a.buf.RowCount() > 0 {
				a.showPlanFrom(a.buf)
			}

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
			// What failed and where are different questions, and the driver
			// only answers the first: a bastion that has stopped forwarding
			// looks exactly like a database that has.
			a.status.err = failureCause(err, a.transportFailure(), a.bastionName())
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
	s.columnsLeft = a.columnsOffView()
	return s
}

// columnsOffView is how many of the result's columns have scrolled off the
// left of the grid.
//
// Read here rather than pushed on every keypress: the grid scrolls itself in
// response to the arrow keys, so there is no moment this application could
// hook that the grid does not already know about. Reading the offset is also
// the only exact answer — how many columns fit on the right depends on widths
// tview works out while it draws.
//
// It reports nothing while another tab is in front. The offset survives the
// tab switch, and a bar warning about a grid nobody is looking at is a warning
// that gets learned as noise.
func (a *App) columnsOffView() int {
	if a.resultTabs == nil || a.resultTabs.current() != tabResults {
		return 0
	}
	if a.buf.ColumnCount() == 0 {
		return 0
	}

	_, column := a.grid.GetOffset()
	if column < 0 {
		return 0
	}
	return column
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

// opensOrEndsTransaction picks out the statements that change whether a
// transaction exists, which the connection handles rather than a Stream.
//
// SAVEPOINT and RELEASE SAVEPOINT are not among them: they are ordinary
// statements that happen to need an open transaction, and they run on the
// pinned connection like anything else does.
func opensOrEndsTransaction(stmt sqlparse.Statement) bool {
	if stmt.Kind() != sqlparse.StmtTransaction {
		return false
	}
	switch stmt.Verb() {
	case "BEGIN", "START", "COMMIT", "ROLLBACK":
		return true
	}
	return false
}

// transactionControl opens or ends a transaction, on the connection rather
// than through a Stream.
func (a *App) transactionControl(verb string) {
	if a.running != nil {
		a.notice("a statement is already running; ^C cancels it")
		return
	}

	ctx := context.Background()
	var err error
	var said string

	switch verb {
	case "BEGIN", "START":
		err, said = a.conn.Begin(ctx), "transaction open — nothing is visible to anyone else until commit"
	case "COMMIT":
		err, said = a.conn.Commit(ctx), "committed"
	case "ROLLBACK":
		err, said = a.conn.Rollback(ctx), "rolled back"
	}

	if err != nil {
		a.notice(err.Error())
		return
	}
	a.status.inTransaction = a.conn.InTransaction()
	a.notice(said)
}

// confirmDiscardTransaction asks before quitting with work that has not been
// committed. Leaving would roll it back, which is the safe reading of a
// session that ended without saying commit — but not one to do silently.
func (a *App) confirmDiscardTransaction() {
	modal := tview.NewModal().
		SetText("A transaction is open.\n\nQuitting rolls it back.").
		AddButtons([]string{"Cancel", "Roll back and quit"}).
		SetDoneFunc(func(_ int, label string) {
			a.closeDialog()
			if label == "Roll back and quit" {
				a.quitWithUnsavedFile()
			}
		})

	modal.SetBackgroundColor(tcell.ColorBlack)
	a.openDialog(modal)
}

// inspect shows whatever has focus in full: a table's definition from the
// schema pane, a row from the results.
//
// One word for one intent, resolved by where the caret is — the same shape as
// copy-or-cancel and as "/", which searches whichever pane is focused. A
// second key for "the same thing but over there" is a key nobody remembers.
func (a *App) inspect() {
	if a.app.GetFocus() == a.grid {
		a.showRow()
		return
	}
	a.inspectTable()
}
