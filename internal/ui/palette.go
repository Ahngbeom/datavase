package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ahngbeom/datavase/internal/export"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/match"
	"github.com/Ahngbeom/datavase/internal/procs"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/Ahngbeom/datavase/internal/vim"
)

// paletteNameColumn is the width the command names are padded to, so the
// summaries line up in a second column on the same row.
const paletteNameColumn = 17

// The two commands the guard's refusal has to be able to name. They are
// constants so the message and the palette entry cannot drift apart — a test
// checks the hint names a command the palette really offers.
//
// They say "writes" rather than "write" because a ":" command line resolves
// these same names, and to a vim user ":write" saves the file. Leaving the
// unlock called "write" would have put the most dangerous thing here behind
// the most reflexive thing a vim user types.
const (
	cmdEnableWrites  = "unlock writes"
	cmdDisableWrites = "lock writes"
)

// command is one entry of the command palette.
type command struct {
	name    string
	summary string

	// exact keeps the command off the ":" line's abbreviations, so only its
	// whole name reaches it.
	//
	// The palette can afford to guess because it shows the row it picked and
	// Enter chooses from a visible list. A command line runs on Enter, and a
	// prefix that is unique today stops being unique the moment a command is
	// added — which is a fine way to reach "history" and no way at all to
	// reach an unlock on production.
	exact bool

	// covers names the action this command performs, where there is one.
	//
	// It is what lets a test prove that no action is reachable only through a
	// chord some other application can claim — which is not hypothetical: a
	// terminal that keeps ⌘ for its own menus left the schema tree with no way
	// in at all, since ⌘B and Ctrl+B were the only two ways to ask for it.
	covers keymap.Action

	run func(a *App)
}

// paletteExempt lists the actions the palette deliberately does not offer.
//
// The rule the test enforces is that every action is reachable without a
// chord: bound to a plain key, or named here in the palette. These are the
// exceptions, and each needs a reason rather than an omission.
var paletteExempt = map[keymap.Action]bool{
	// Moving the caret by a word. Opening a palette and typing a command to go
	// one word left is not a route anyone would take — these are keystrokes or
	// they are nothing.
	//
	// Only the word motions are here: start and end of line are Home and End,
	// which no chord is involved in and nothing upstream claims.
	keymap.ActionWordLeft:        true,
	keymap.ActionWordRight:       true,
	keymap.ActionSelectWordLeft:  true,
	keymap.ActionSelectWordRight: true,

	// The terminal's own copy and paste are what claimed these keys in the
	// first place, and they do the same job. A paste that goes through a
	// palette is worse than no paste at all.
	keymap.ActionCopyOrCancel: true,
	keymap.ActionCopy:         true,
	keymap.ActionCut:          true,
	keymap.ActionPaste:        true,

	// Faster ways to do what plain Backspace already does. Losing the shortcut
	// costs keystrokes, not the ability.
	keymap.ActionDeleteWordLeft:    true,
	keymap.ActionDeleteToLineStart: true,
}

// paletteCommands are what the palette offers. Keeping them in one list means
// the palette, the key reference and any future ":" prompt cannot drift apart.
//
// It is a function rather than a package variable because the list names
// showHelp, and the help screen now lists the commands: as a variable that is
// an initialisation cycle the compiler refuses, even though nothing recurses
// at run time.
func paletteCommands() []command {
	cmds := []command{
		{
			name:    cmdEnableWrites,
			summary: "allow writes to this production datasource for the session",
			exact:   true,
			run:     (*App).enableWrites,
		},
		{
			name:    cmdDisableWrites,
			summary: "refuse them again",
			run:     (*App).disableWrites,
		},
		{
			name:    "begin",
			summary: "open a transaction — work stays undoable until you commit",
			run:     func(a *App) { a.transactionControl("BEGIN") },
		},
		{
			name:    "commit",
			summary: "keep the open transaction's work",
			run:     func(a *App) { a.transactionControl("COMMIT") },
		},
		{
			name:    "rollback",
			summary: "discard the open transaction's work",
			run:     func(a *App) { a.transactionControl("ROLLBACK") },
		},
		{
			name:    "export csv",
			summary: "write the current result to a CSV file",
			run:     func(a *App) { a.exportResult(formatCSV) },
		},
		{
			name:    "export json",
			summary: "write the current result to a JSON file",
			run:     func(a *App) { a.exportResult(formatJSON) },
		},
		{
			name:    "cancel",
			summary: "stop the running statement",
			covers:  keymap.ActionCancel,
			run:     (*App).cancelRunning,
		},
		{
			name:    "find in text",
			summary: "search the editor, or the results when they have focus",
			covers:  keymap.ActionFind,
			run:     func(a *App) { a.showTextSearch(false) },
		},
		{
			name:    "find next",
			summary: "go to the next match of the last search",
			covers:  keymap.ActionFindNext,
			run:     func(a *App) { a.searchAgain(false) },
		},
		{
			name:    "find previous",
			summary: "go to the previous match of the last search",
			covers:  keymap.ActionFindPrev,
			run:     func(a *App) { a.searchAgain(true) },
		},
		{
			name:    "history",
			summary: "search previously run statements",
			covers:  keymap.ActionSearchHistory,
			run:     (*App).showHistory,
		},
		{
			name:    "explain",
			summary: "show how the server would run the statement under the cursor",
			covers:  keymap.ActionExplain,
			run:     (*App).explainStatement,
		},
		{
			name:    "analyze",
			summary: "run the statement under the cursor and show what it actually did",
			covers:  keymap.ActionAnalyze,
			run:     (*App).analyzeStatement,
		},
		{
			name:    "sessions",
			summary: "list what else is running on the server",
			covers:  keymap.ActionSessions,
			run:     (*App).showSessions,
		},
		{
			name:    "locks",
			summary: "show which connections are waiting on which",
			covers:  keymap.ActionLocks,
			run:     (*App).showLocks,
		},
		{
			name:    "stop a statement",
			summary: "stop the statement running on another connection",
			covers:  keymap.ActionKillSession,
			run:     func(a *App) { a.showKillSession(procs.StopStatement) },
		},
		{
			name:    "stop a connection",
			summary: "end another connection, rolling back anything it held open",
			exact:   true,
			run:     func(a *App) { a.showKillSession(procs.StopConnection) },
		},
		{
			name:    "sort by column",
			summary: "order the results by the selected column, and back again",
			covers:  keymap.ActionSortColumn,
			run:     (*App).sortColumn,
		},
		{
			name:    "copy row",
			summary: "put the selected result row on the clipboard, tab separated",
			run:     (*App).copyRow,
		},
		{
			name:    "inspect",
			summary: "show the selected table or result row in full",
			covers:  keymap.ActionInspect,
			run:     (*App).inspect,
		},
		{
			name:    "go to table",
			summary: "find a table anywhere on the server by name",
			covers:  keymap.ActionGoToTable,
			run:     (*App).showGoToTable,
		},
		{
			name:    "schema tree",
			summary: "show or hide the schema pane",
			covers:  keymap.ActionToggleSidebar,
			run:     (*App).toggleSidebar,
		},
		{
			name:    "complete",
			summary: "complete the word at the cursor",
			covers:  keymap.ActionComplete,
			run:     (*App).showCompletion,
		},
		{
			name:    "attach directory",
			summary: "point this session at a worktree of SQL files",
			run:     (*App).showAttachDirectory,
		},
		{
			name:    "detach directory",
			summary: "forget the attached worktree",
			run:     (*App).detach,
		},
		{
			name:    "open file",
			summary: "open a SQL file from the attached worktree",
			covers:  keymap.ActionFindFile,
			run:     (*App).showFindFile,
		},
		{
			name:    "save file",
			summary: "write the editor back to the file it came from",
			covers:  keymap.ActionSaveFile,
			run:     (*App).saveFile,
		},
		{
			name:    "switch datasource",
			summary: "move this session to another configured datasource",
			covers:  keymap.ActionSwitchDataSource,
			run:     (*App).showDataSources,
		},
		{
			name:    "use schema",
			summary: "choose the schema unqualified names resolve against",
			covers:  keymap.ActionUseSchema,
			run:     (*App).showUseSchema,
		},
		{
			name:    "refresh schema",
			summary: "reload the schema tree and completion cache",
			covers:  keymap.ActionRefreshSchema,
			run:     (*App).loadSchemas,
		},

		// Editing, for the keyboards where these arrive as ⌘ chords and never
		// reach us. They go through the editor's own dispatcher, so the palette
		// cannot drift from what the keys do.
		{
			name:    "select all",
			summary: "select the whole editor buffer",
			covers:  keymap.ActionSelectAll,
			run:     func(a *App) { a.editorAction(keymap.ActionSelectAll) },
		},
		{
			name:    "comment",
			summary: "comment or uncomment the selected lines",
			covers:  keymap.ActionToggleComment,
			run:     func(a *App) { a.editorAction(keymap.ActionToggleComment) },
		},
		{
			name:    "duplicate line",
			summary: "duplicate the current line",
			covers:  keymap.ActionDuplicateLine,
			run:     func(a *App) { a.editorAction(keymap.ActionDuplicateLine) },
		},
		{
			name:    "delete line",
			summary: "delete the current line",
			covers:  keymap.ActionDeleteLine,
			run:     func(a *App) { a.editorAction(keymap.ActionDeleteLine) },
		},

		{
			name:    "help",
			summary: "show the key reference",
			covers:  keymap.ActionHelp,
			run:     (*App).showHelp,
		},
		{
			name:    "quit",
			summary: "leave datavase",
			covers:  keymap.ActionQuit,
			run:     (*App).quit,
		},
	}

	// One entry per preset, generated rather than listed: a preset that the
	// palette does not offer is one nobody can reach without editing a file.
	for _, p := range keymap.Presets() {
		preset := p
		cmds = append(cmds, command{
			name:    "keymap " + string(preset),
			summary: "switch to the " + string(preset) + " keyboard for this session",
			run:     func(a *App) { a.setPreset(preset) },
		})
	}
	return cmds
}

// setPreset swaps the keyboard mid-session.
//
// Every lookup goes through a.keys at the moment a key arrives, so replacing
// the map is the whole switch — nothing needs rebinding. The user's own
// overrides are re-applied on top, since they were an opinion about keys, not
// about which keyboard those keys sat on.
func (a *App) setPreset(p keymap.Preset) {
	km, err := keymap.FromConfig(string(p), a.cfg.Keymap.Actions)
	if err != nil {
		a.notice(err.Error())
		return
	}

	a.keys = km
	// Switching keyboards mid-session starts the modal state over: being
	// dropped into insert mode by a keyboard change is not something anyone
	// would guess had happened.
	a.vim = vim.New()
	a.notice(fmt.Sprintf("keymap: %s — %s for keys", p, a.helpKeyLabel()))
}

// showCommandPalette offers every command, filtered as the user types.
//
// The filter is not decoration: the list outgrew a modest terminal once the
// keyboards were added to it, and a command you have to scroll blindly to
// find is one you will not find.
func (a *App) showCommandPalette() {
	box := a.newSearchBox("command: ", " commands ", pagePalette, func(term string) []searchItem {
		cmds := paletteCommands()
		rows := make([]ranked, 0, len(cmds))

		for _, c := range cmds {
			cmd := c
			tier, score, ok := rankCommand(cmd, term)
			if !ok {
				continue
			}
			rows = append(rows, ranked{
				item: searchItem{
					// Name and summary share one row so that every command fits
					// on a modest terminal. A palette you have to scroll to find
					// "quit" in is one where the keyboard was quicker.
					primary: fmt.Sprintf("%-*s %s", paletteNameColumn, cmd.name, cmd.summary),
					accept: func() {
						a.closeSearchBox(pagePalette)
						cmd.run(a)
					},
				},
				tier:  tier,
				score: score,
			})
		}
		if len(rows) == 0 {
			return []searchItem{message("no matching command", "press Escape to close")}
		}
		return sortRanked(rows)
	})

	// The height is a maximum — centred shrinks it to whatever the terminal
	// has — so asking for room enough for every command costs nothing.
	a.pages.AddPage(pagePalette, centred(box, 80, 40), true, true)
}

// Match tiers for the palette, highest first.
const (
	// tierName is a hit on what the command is called.
	tierName = 1
	// tierSummary is a hit only in the description, which is what lets
	// "keyboard" find the preset commands even though none of them says the
	// word. It never outranks a name: someone typing "quit" means the command
	// called quit, not one that mentions quitting.
	tierSummary = 0
)

// rankCommand scores a command against what has been typed.
func rankCommand(c command, term string) (tier, score int, ok bool) {
	term = strings.TrimSpace(term)
	if term == "" {
		return tierName, 0, true
	}
	if score, ok := match.Fuzzy(term, c.name); ok {
		return tierName, score, true
	}
	if score, ok := match.Fuzzy(term, c.summary); ok {
		return tierSummary, score, true
	}
	return 0, 0, false
}

// enableWrites unlocks writes against production for this session only.
//
// It is never persisted: an unlock that outlived the session would quietly
// become the default, which is precisely the state the guard exists to
// prevent.
func (a *App) enableWrites() {
	a.status.writesEnabled = true
	a.notice("writes unlocked for this session — the status bar will keep saying so")
}

func (a *App) disableWrites() {
	a.status.writesEnabled = false
	a.notice("writes locked")
}

// exportFormat selects the writer.
type exportFormat int

const (
	formatCSV exportFormat = iota
	formatJSON
)

func (f exportFormat) extension() string {
	if f == formatJSON {
		return "json"
	}
	return "csv"
}

// exportResult writes the buffered result to a file.
//
// Only the rows actually held are written, and the name says as much when the
// result was truncated — a file that silently contains part of the answer is
// worse than one that admits it.
func (a *App) exportResult(format exportFormat) {
	if a.buf.ColumnCount() == 0 {
		a.notice("no result to export")
		return
	}

	path, err := a.exportPath(format)
	if err != nil {
		a.notice(fmt.Sprintf("export failed: %v", err))
		return
	}

	columns := a.buf.Columns()
	rows := make([][]any, a.buf.RowCount())
	for i := range rows {
		rows[i] = a.buf.Row(i)
	}

	file, err := os.Create(path)
	if err != nil {
		a.notice(fmt.Sprintf("export failed: %v", err))
		return
	}
	defer file.Close()

	if format == formatJSON {
		err = export.JSON(file, columns, rows)
	} else {
		err = export.CSV(file, columns, rows)
	}
	if err != nil {
		a.notice(fmt.Sprintf("export failed: %v", err))
		return
	}

	message := fmt.Sprintf("wrote %d rows to %s", len(rows), path)
	if a.buf.AtCapacity() || a.status.truncated {
		message += " (truncated — the result was larger than the buffer)"
	}
	a.notice(message)
}

// exportPath builds a unique name in the working directory, so an export
// never silently overwrites an earlier one.
func (a *App) exportPath(format exportFormat) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	base := fmt.Sprintf("%s-%s.%s",
		sanitiseFilename(a.conn.DataSource().Name),
		time.Now().Format("20060102-150405"),
		format.extension())
	return filepath.Join(dir, base), nil
}

// sanitiseFilename keeps a datasource name usable as part of a filename.
func sanitiseFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "result"
	}
	return b.String()
}

// showHistory opens a searchable list of previously run statements.
func (a *App) showHistory() {
	if a.history == nil {
		a.notice("query history is unavailable")
		return
	}

	box := a.newSearchBox("search: ", " history ", pageHistory, func(term string) []searchItem {
		ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
		defer cancel()

		entries, err := a.history.Search(ctx, term, 100)
		if err != nil {
			return []searchItem{message("search failed", err.Error())}
		}
		if len(entries) == 0 {
			return []searchItem{message("no matching statements", "")}
		}

		items := make([]searchItem, len(entries))
		for i, e := range entries {
			entry := e
			items[i] = searchItem{
				primary: oneLineSQL(entry.SQL),
				secondary: fmt.Sprintf("%s · %d rows · %s",
					entry.At.Local().Format("2006-01-02 15:04"), entry.Rows, entry.DataSource),
				accept: func() {
					a.closeSearchBox(pageHistory)
					a.editor.SetText(entry.SQL, true)
				},
			}
		}
		return items
	})

	a.pages.AddPage(pageHistory, centred(box, 90, 24), true, true)
}

// oneLineSQL flattens a statement so each history entry occupies one row.
func oneLineSQL(sql string) string {
	return result.Truncate(strings.Join(strings.Fields(sql), " "), 110)
}
