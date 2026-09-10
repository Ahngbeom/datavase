// Package ui assembles the terminal interface.
//
// The rule this package follows is that widgets never hold application
// state. Anything a decision depends on — the current statement, the
// result buffer — lives outside tview, so it can be tested without a
// terminal. Widgets only render what they are handed.
package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/complete"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/history"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/sqlparse"
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
	connect    func(context.Context, *config.DataSource) (*session.Session, error)
	configPath string
	secrets    secret.Store
	probe      func(ctx context.Context, ds *config.DataSource, password string) (string, error)
	// picker is the open datasource dialog, or nil between sessions of it.
	picker *dsPicker

	tree      *tview.TreeView
	editor    *tview.TextArea
	grid      *tview.Table
	topBar    *topBar
	statusBar *statusBar

	pages *tview.Pages

	buf *result.Buffer
	// content is held rather than handed to the table and forgotten, because
	// it owns the row ordering and so is the only thing that can say which
	// buffer row a grid row draws.
	content *gridContent
	status  status

	// keys resolves key events to actions.
	keys *keymap.Map

	// sidebarVisible tracks the schema pane, which the layout is rebuilt
	// around when it is toggled. sidebarRule is the hairline beside it.
	//
	// It starts true. Knowing what is in the database is the first thing
	// anyone wants from a client they have just opened, and a tree that has
	// to be asked for is a tree most people never learn is there. ⌘B takes it
	// back for the session when the width is wanted for the result.
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

	// mouseEnabled says whether a click means anything. config.Defaults.Mouse
	// is a *bool so "not written" can be told apart from "written as false";
	// this is the resolved value, true when the config left it unset.
	mouseEnabled bool

	// hits is where the last frame's regions recorded what a click on them
	// means. Rebuilt every frame in captureScreen's SetBeforeDrawFunc, the
	// same reason a region's own header is rebuilt every frame rather than
	// updated: the only way it can be wrong is if a renderer is.
	hits hitmap
	// paneRows maps a header's screen row to the tabbed region that drew it.
	// The hitmap alone answers what a zone means and which tab; it does not
	// say which of the three regions the row belonged to, and a tab click has
	// to know that to find the pane to switch. Rebuilt alongside hits, for
	// the same reason.
	paneRows map[int]*tabbed

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

	// completion is nil until the schema cache is available; the popup says
	// so rather than appearing broken.
	completion *complete.Engine
	cache      *catalog.Cache
	history    *history.Store

	// search is the last pattern looked for and where, so that n and N have
	// something to repeat once the prompt has closed.
	search searchState

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
// It is a queue rather than a loop because a statement's result arrives
// asynchronously: each step resumes the batch from the same callback that
// reports the last one finished.
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
	Cache   *catalog.Cache
	History *history.Store
	// Connect opens another datasource, for switching mid-session. Nil leaves
	// the session on the datasource it started with, and the switch says so
	// rather than failing silently.
	Connect func(context.Context, *config.DataSource) (*session.Session, error)
	// ConfigPath is where the datasource dialog saves. Empty means a save
	// from the dialog is refused, with a message saying why; connecting
	// still works.
	ConfigPath string
	// Secrets stores passwords typed into the dialog. Nil means they cannot
	// be stored here, which the form says instead of pretending.
	Secrets secret.Store
	// Probe answers the dialog's Test button.
	Probe func(ctx context.Context, ds *config.DataSource, password string) (string, error)
}

// New builds the interface for an open session.
//
// The session rather than the connection, because switching datasource has to
// close what it left — and a tunnel closed by nobody outlives the thing it
// was carrying.
func New(sess *session.Session, cfg *config.Config, deps Deps) *App {
	conn := sess.Conn
	ds := conn.DataSource()

	a := &App{
		app:             tview.NewApplication(),
		sess:            sess,
		conn:            conn,
		connect:         deps.Connect,
		configPath:      deps.ConfigPath,
		secrets:         deps.Secrets,
		probe:           deps.Probe,
		cfg:             cfg,
		keys:            keymap.Default(),
		cache:           deps.Cache,
		history:         deps.History,
		buf:             result.NewBuffer(cfg.Defaults.BufferMax),
		selectionAnchor: noAnchor,
		sidebarVisible:  true,
		mouseEnabled:    cfg.Defaults.Mouse == nil || *cfg.Defaults.Mouse,
	}
	if deps.Cache != nil {
		a.completion = complete.New(deps.Cache.Names(), ds.Name, ds.Database)
	}
	a.status.hints = a.openingClauses(conn.ServerVersion())

	// Before any widget is built: tview copies its palette into each one as it
	// is created, so a default claimed afterwards would reach nothing.
	applyTheme()

	a.buildWidgets()
	a.buildLayout()
	a.bindKeys()
	a.bindMouse()
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
		// The zones of the frame about to be drawn replace the last one's. A
		// region that is no longer on screen — the sidebar, toggled off —
		// must not still answer for the rows it used to hold.
		a.hits.clear()
		a.paneRows = nil
		return false
	})
}

// openingMessage greets the user with what the first line of a session has to
// say.
func (a *App) openingClauses(serverVersion string) []string {
	return openingClauses(opening{
		serverVersion: serverVersion,
		helpKey:       a.helpKeyLabel(),
		sidebarKey:    a.keyLabel(keymap.ActionToggleSidebar),
	})
}

// opening is what the first line of a session has to say.
type opening struct {
	serverVersion string
	helpKey       string
	sidebarKey    string
}

// openingClauses is the greeting, most actionable first.
//
// They are separate clauses rather than one sentence because the bar can drop
// a field whole and can only cut a sentence in half. As one string the last
// clause was chopped mid-word on an eighty-column terminal — which is where a
// default one starts — and the reader was left with "for the s".
//
// The order decides what survives, and it is the whole of the ranking: a
// clause's position in this slice is its shedding priority, most protected
// first. F1 comes first: it reaches the full reference, and every other key
// with it. The schema tree follows — it is on screen, so this says how to
// get the width back rather than that it exists. The server version brings
// up the rear: it is a greeting rather than an instruction, and knowing
// which MariaDB answered has never stood between anyone and their first
// query.
func openingClauses(o opening) []string {
	var out []string

	if o.helpKey != "" {
		out = append(out, o.helpKey+" for keys")
	}
	// The tree is on screen; what is worth saying is how to put it away.
	if o.sidebarKey != "" {
		out = append(out, o.sidebarKey+" hides the schema tree")
	}
	if o.serverVersion != "" {
		out = append(out, "server "+o.serverVersion)
	}
	return out
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

// keyLabel names an action's key.
func (a *App) keyLabel(action keymap.Action) string {
	bindings := a.keys.DisplayBindings(action)
	if len(bindings) == 0 {
		return action.String()
	}
	return bindings[0].Label(onMac)
}

// editorPlaceholder is what an empty editor offers.
//
// It names the other way in rather than the run key: the header a row above
// already says how to run, and repeating it wastes the one line a reader
// looks at before they have typed anything. A table in the tree is the
// fastest thing anyone new can do here, and nothing else on screen says the
// gesture exists.
func (a *App) editorPlaceholder() string {
	if !a.sidebarVisible {
		return "SELECT …"
	}
	return "SELECT … or double-click a table"
}

// Run starts the event loop and blocks until the user quits.
func (a *App) Run() error {
	return a.app.SetRoot(a.pages, true).EnableMouse(true).EnablePaste(true).Run()
}

// SetScreen replaces the terminal the interface draws on. Tests pass a
// tcell simulation screen so the real interface — layout, key handling,
// dialogs — can be exercised without a terminal.
func (a *App) SetScreen(screen tcell.Screen) {
	a.app.SetScreen(screen)
}

// Stop ends the event loop.
func (a *App) Stop() { a.app.Stop() }

func (a *App) buildWidgets() {
	ds := a.conn.DataSource()

	// The root is the tree's one heading.
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
		SetSelectable(true, true).
		// SetBorders(true) draws a rule between every row too, halving how
		// many fit on screen — row density matters more here than a box.
		SetSeparator('│').
		SetBordersColor(colourMuted)
	a.grid.SetInputCapture(a.gridKey)

	// Every region shares one component so they cannot drift apart in
	// behaviour, including the editor, which has no tabs at all.
	a.editorRegion = newTabbed().watch(a.editorDetail)
	a.editorRegion.only(a.editor)
	a.editorRegion.record = a.recorderFor(a.editorRegion)

	a.schemaTabs = newTabbed().watch(a.schemaDetail)
	a.schemaTabs.add(tabTree, a.tree)
	a.schemaTabs.add(tabTables, a.buildTablesTab())
	a.schemaTabs.record = a.recorderFor(a.schemaTabs)

	a.resultTabs = newTabbed().watch(a.resultDetail)
	a.resultTabs.detailTarget = func() zoneTarget {
		if a.buf.ColumnCount() > 0 && a.running == nil {
			return zoneCopyResult
		}
		return zoneNone
	}
	a.resultTabs.add(tabResults, a.grid)
	a.resultTabs.record = a.recorderFor(a.resultTabs)

	a.topBar = newTopBar(a.currentTopBar)
	a.topBar.record = a.hits.set
	a.statusBar = newStatusBar(a.currentStatus)
}

// schemaDetail is the trailing note on the schema pane's header, read at draw
// time like the other two regions'.
// affordances are the key labels the header hints name, read from the map in
// force rather than written out, so a rebinding moves the hint with the key.
func (a *App) affordances() affordanceKeys {
	k := affordanceKeys{
		run:      a.keyLabel(keymap.ActionRun),
		runAll:   a.keyLabel(keymap.ActionRunAll),
		complete: a.keyLabel(keymap.ActionComplete),
		history:  a.keyLabel(keymap.ActionSearchHistory),
		copy:     a.keyLabel(keymap.ActionCopyResult),
		sort:     a.keyLabel(keymap.ActionSortColumn),
		inspect:  a.keyLabel(keymap.ActionInspect),
		row:      a.keyLabel(keymap.ActionCopyRow),
	}
	k.short.run = a.shortKeyLabel(keymap.ActionRun)
	k.short.complete = a.shortKeyLabel(keymap.ActionComplete)
	k.short.copy = a.shortKeyLabel(keymap.ActionCopyResult)
	k.short.sort = a.shortKeyLabel(keymap.ActionSortColumn)
	k.short.inspect = a.shortKeyLabel(keymap.ActionInspect)
	k.short.row = a.shortKeyLabel(keymap.ActionCopyRow)
	return k
}

// shortKeyLabel is an action's briefest label, for a header too narrow for
// the one this application teaches.
//
// Briefest is usually the function key, which is also the one that reaches
// every terminal — so where the space runs out, what is left is the binding
// most likely to work.
func (a *App) shortKeyLabel(action keymap.Action) string {
	best := ""
	for _, b := range a.keys.DisplayBindings(action) {
		if label := b.Label(onMac); best == "" || visibleCost(label) < visibleCost(best) {
			best = label
		}
	}
	return best
}

// editorDetail offers what the editor does beyond taking the typing.
func (a *App) editorDetail(room int) string {
	return editorAffordance(a.editor.HasFocus(), room, a.affordances())
}

// schemaDetail explains the marker, or offers the preview while the tree has
// the keyboard.
//
// One or the other, never both: the pane is thirty-four columns wide and the
// tab strip has most of them, so the two together are truncated into saying
// neither. The legend answers a question the marker raises, and it can wait —
// the marker is still there when the keyboard moves on, and the hint names a
// gesture nothing else on screen mentions.
func (a *App) schemaDetail(int) string {
	if hint := treeAffordance(a.schemaTabs.current() == tabTree && a.tree.HasFocus(),
		a.affordances()); hint != "" {
		return hint
	}
	return schemaPaneDetail(a.schemaTabs.current(), a.currentSchema())
}

// schemaPaneDetail explains the marker beside a schema in the tree.
//
// The dot is the only thing on screen that says which schema an unqualified
// statement will reach, and a dot explains nothing. The schema picker spells
// the same marker out on the row beside it; the tree, where it matters most,
// left the reader to work it out.
//
// Nothing is said when no schema is current, because then nothing is marked
// and a legend for an absent marker is one more thing to work out rather than
// one fewer. Nothing is said on the tables tab either, which draws no markers.
func schemaPaneDetail(tab, currentSchema string) string {
	if tab != tabTree || currentSchema == "" {
		return ""
	}
	return currentSchemaMarker + " current schema"
}

// resultDetail says what the empty results tab would otherwise not say, or
// offers the copy key once there is something to copy.
func (a *App) resultDetail(room int) string {
	k := a.affordances()

	if a.buf.ColumnCount() > 0 && a.running == nil {
		// With the keyboard here, all three things a result can do; without
		// it, the copy label alone, because that one is also a click target.
		if hint := gridAffordance(a.grid.HasFocus(), true, room, k); hint != "" {
			return hint
		}
		return k.copy + " copy"
	}
	return resultHint(resultState{
		columns: a.buf.ColumnCount(),
		running: a.running != nil,
		wrote:   a.status.written != nil,
	}, k)
}

// resultState is everything the hint depends on.
type resultState struct {
	columns int
	running bool
	// wrote says the last statement changed rows instead of returning them,
	// which is an empty grid for a reason rather than an empty grid.
	wrote bool
}

// resultHint is the trailing line on the results tab's header.
//
// With no box around it, an empty region is simply blank — which reads as a
// gap in the layout rather than as a pane waiting for a statement. So the
// header says which it is, and it has to say the right one: it used to know
// only "run a statement to see rows here", and went on saying it while a
// statement was running and the bar two rows below said so. A pane telling
// the user to do the thing they are watching happen is the same contradiction
// the finders opened with.
func resultHint(s resultState, k affordanceKeys) string {
	if s.columns > 0 {
		return ""
	}

	switch {
	case s.running:
		return "waiting for the first row…"
	case s.wrote:
		// The grid is empty because the server sent a count rather than rows,
		// which the bar reports and the pane otherwise contradicts.
		return "no rows: that statement changed data"
	default:
		// An invitation rather than a description of the gap: the reader can
		// see the pane is empty, and what they cannot see is which key fills
		// it.
		return k.run + " runs the statement"
	}
}

// currentTopBar is where the session is, read at draw time so the schema
// cannot lag behind the thing that changed it.
func (a *App) currentTopBar() topBarState {
	ds := a.conn.DataSource()
	return topBarState{
		dsName:   ds.Name,
		schema:   a.currentSchema(),
		readOnly: ds.ReadOnly,
		helpKey:  a.helpKeyLabel(),
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

	// The spine runs down the outside of everything.
	root := tview.NewFlex().
		AddItem(newSpine(), 1, 0, false).
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
)

const (
	pageMain       = "main"
	pageConfirm    = "confirm"
	pageHelp       = "help"
	pageComplete   = "complete"
	pageHistory    = "history"
	pageUseSchema  = "useschema"
	pageSearch     = "search"
	pageDataSource = "datasource"
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

// resultPrimitive and schemaPrimitive return whichever widget Tab should
// land on, rather than the tab strip.
func (a *App) resultPrimitive() tview.Primitive {
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
	case keymap.ActionCopyRow:
		a.copyRow()
	case keymap.ActionCopyResult:
		a.showCopyFormats()
	case keymap.ActionSwitchDataSource:
		a.showDataSources()
	case keymap.ActionFind:
		a.showTextSearch(false)
	case keymap.ActionFindNext:
		a.searchAgain(false)
	case keymap.ActionFindPrev:
		a.searchAgain(true)
	case keymap.ActionSearchHistory:
		a.showHistory()

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

// quit leaves, asking first when there is an open transaction to roll back.
func (a *App) quit() {
	if a.conn.InTransaction() {
		a.confirmDiscardTransaction()
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

// runStatement sends one statement, or opens or ends a transaction when
// that is what it says.
func (a *App) runStatement(stmt sqlparse.Statement) {
	if stmt.IsEmpty() {
		a.notice("nothing to run")
		return
	}
	// Transaction control opens or ends the pinned connection rather than
	// running on one, so it never becomes a Stream. Typing BEGIN works because
	// that is what a DBA types.
	if opensOrEndsTransaction(stmt) {
		a.transactionControl(stmt.Verb())
		return
	}
	a.start(stmt, sqlparse.AutoLimit(stmt, a.cfg.Defaults.AutoLimit))
}

// executeAll runs every statement in the editor, in order, stopping at the
// first failure so a statement after it is never run against a database the
// one before it left in an unknown state.
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

// advanceBatch sends the next statement of a Run-everything, or opens or
// ends a transaction when that is what it says.
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

	if opensOrEndsTransaction(stmt) {
		a.transactionControl(stmt.Verb())
		a.advanceBatch()
		return
	}
	a.start(stmt, sqlparse.AutoLimit(stmt, a.cfg.Defaults.AutoLimit))
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
		onGrid:       a.app.GetFocus() == a.grid,
		hasSelection: a.editor.HasSelection(),
	}.resolve()

	switch intent {
	case intentCancel:
		a.cancelRunning()
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

// start sends the statement and streams the result into the buffer.
func (a *App) start(stmt sqlparse.Statement, limit int) {
	sql := stmt.SQL
	if limit > 0 {
		sql = sqlparse.AppendLimit(stmt, limit)
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
	a.status.limitInjected = limit
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
			cause := failureCause(err, a.transportFailure(), a.bastionName())
			a.status.err = readOnlyRefusal(cause, a.conn.DataSource().ReadOnly)
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
	// The greeting is what the bar says until something happens; once
	// something has, keeping it would leave it competing for room with the
	// answer the user was waiting for. fields() already withholds it
	// whenever message is set, so this is belt and braces.
	a.status.hints = nil
}

// currentStatus is what the status bar renders.
func (a *App) currentStatus() status {
	s := a.status
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
func (a *App) columnsOffView() int {
	if a.resultTabs == nil {
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
	modal := newModal().
		SetText("A transaction is open.\n\nQuitting rolls it back.").
		AddButtons([]string{"Cancel", "Roll back and quit"}).
		SetDoneFunc(func(_ int, label string) {
			a.closeDialog()
			if label == "Roll back and quit" {
				a.forceQuit()
			}
		})

	a.openDialog(modal)
}

// inspect shows the selected result row in full.
func (a *App) inspect() {
	if a.app.GetFocus() != a.grid {
		a.notice("select a result row first")
		return
	}
	a.showRow()
}
