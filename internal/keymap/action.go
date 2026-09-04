// Package keymap turns key events into named actions.
//
// Keeping key knowledge out of the UI has two payoffs: the whole mapping can
// be tested without a terminal, and the bindings become data the user can
// override in configuration rather than constants only a rebuild can change.
package keymap

// Action is something the user asked for, independent of how they asked.
type Action int

const (
	// ActionNone means the event is not a command; the widget should handle
	// it as ordinary input.
	ActionNone Action = iota

	// Running.
	ActionRun
	ActionRunAll
	ActionCancel
	// ActionCopyOrCancel is Ctrl+C, whose meaning depends on whether text is
	// selected. The keymap cannot see that state, so it names the ambiguity
	// instead of guessing.
	ActionCopyOrCancel

	// Cursor movement. These are the one place ⌘ and Ctrl differ: on macOS
	// ⌘← goes to the start of a line while Ctrl← moves by word.
	ActionWordLeft
	ActionWordRight
	ActionSelectWordLeft
	ActionSelectWordRight
	ActionLineStart
	ActionLineEnd
	ActionSelectLineStart
	ActionSelectLineEnd
	ActionDeleteWordLeft
	ActionDeleteToLineStart

	// Editing.
	ActionCopy
	ActionCut
	ActionPaste
	ActionSelectAll
	ActionToggleComment
	ActionDuplicateLine
	ActionDeleteLine

	// Navigation and panes.
	ActionNextPane
	ActionPrevPane
	ActionToggleSidebar
	ActionRefreshSchema
	ActionUseSchema

	ActionComplete
	ActionFind
	ActionFindNext
	ActionFindPrev
	ActionSearchHistory
	ActionCommandPalette
	ActionGoToTable
	// ActionFindFile opens the attached worktree's SQL files.
	ActionFindFile
	// ActionSaveFile writes the editor back to the file it was loaded from.
	ActionSaveFile
	// ActionCycleTab moves through the tabs of whichever pane has focus.
	ActionCycleTab
	// ActionInspect shows whatever is selected in full: a table's definition,
	// or a result row read down the page instead of across it.
	ActionInspect
	// ActionSortColumn orders the results by the selected column, and back
	// again — the third press restores the order the server sent.
	ActionSortColumn
	// ActionSwitchDataSource moves the session to another configured
	// datasource.
	ActionSwitchDataSource
	// ActionExplain asks the server how it would run the statement under the
	// cursor, without running it.
	ActionExplain
	// ActionAnalyze runs the statement under the cursor and reports what it
	// actually did, against what was expected.
	ActionAnalyze
	// ActionSessions lists what else is running on the server.
	ActionSessions
	// ActionKillSession stops another connection's statement.
	ActionKillSession
	// ActionLocks shows which connections are waiting on which.
	ActionLocks

	// Application.
	ActionHelp
	ActionDetach
	ActionQuit
)

// actionNames are the identifiers used in configuration and in errors.
var actionNames = map[Action]string{
	ActionRun:               "run",
	ActionRunAll:            "run-all",
	ActionCancel:            "cancel",
	ActionCopyOrCancel:      "copy-or-cancel",
	ActionWordLeft:          "word-left",
	ActionWordRight:         "word-right",
	ActionSelectWordLeft:    "select-word-left",
	ActionSelectWordRight:   "select-word-right",
	ActionLineStart:         "line-start",
	ActionLineEnd:           "line-end",
	ActionSelectLineStart:   "select-line-start",
	ActionSelectLineEnd:     "select-line-end",
	ActionDeleteWordLeft:    "delete-word-left",
	ActionDeleteToLineStart: "delete-to-line-start",
	ActionCopy:              "copy",
	ActionCut:               "cut",
	ActionPaste:             "paste",
	ActionSelectAll:         "select-all",
	ActionToggleComment:     "toggle-comment",
	ActionDuplicateLine:     "duplicate-line",
	ActionDeleteLine:        "delete-line",
	ActionNextPane:          "next-pane",
	ActionPrevPane:          "prev-pane",
	ActionToggleSidebar:     "toggle-sidebar",
	ActionRefreshSchema:     "refresh-schema",
	ActionUseSchema:         "use-schema",
	ActionComplete:          "complete",
	ActionFind:              "find",
	ActionFindNext:          "find-next",
	ActionFindPrev:          "find-previous",
	ActionSearchHistory:     "search-history",
	ActionCommandPalette:    "command-palette",
	ActionGoToTable:         "go-to-table",
	ActionFindFile:          "find-file",
	ActionSaveFile:          "save-file",
	ActionCycleTab:          "cycle-tab",
	ActionInspect:           "inspect",
	ActionSortColumn:        "sort-column",
	ActionSwitchDataSource:  "switch-datasource",
	ActionExplain:           "explain",
	ActionAnalyze:           "analyze",
	ActionSessions:          "sessions",
	ActionKillSession:       "kill-session",
	ActionLocks:             "locks",
	ActionHelp:              "help",
	ActionDetach:            "detach",
	ActionQuit:              "quit",
}

// descriptions are shown on the help screen.
var descriptions = map[Action]string{
	ActionRun:               "run the statement under the cursor",
	ActionRunAll:            "run every statement in the editor",
	ActionCancel:            "cancel the running statement",
	ActionCopyOrCancel:      "copy the selection, or cancel if nothing is selected",
	ActionWordLeft:          "move one word left",
	ActionWordRight:         "move one word right",
	ActionSelectWordLeft:    "extend the selection one word left",
	ActionSelectWordRight:   "extend the selection one word right",
	ActionLineStart:         "move to the start of the line",
	ActionLineEnd:           "move to the end of the line",
	ActionSelectLineStart:   "select to the start of the line",
	ActionSelectLineEnd:     "select to the end of the line",
	ActionDeleteWordLeft:    "delete the word before the cursor",
	ActionDeleteToLineStart: "delete to the start of the line",
	ActionCopy:              "copy",
	ActionCut:               "cut",
	ActionPaste:             "paste",
	ActionSelectAll:         "select all",
	ActionToggleComment:     "comment or uncomment the selected lines",
	ActionDuplicateLine:     "duplicate the current line",
	ActionDeleteLine:        "delete the current line",
	ActionNextPane:          "move to the next pane",
	ActionPrevPane:          "move to the previous pane",
	ActionToggleSidebar:     "show or hide the schema tree",
	ActionRefreshSchema:     "reload the schema tree",
	ActionUseSchema:         "choose the schema unqualified names resolve against",
	ActionComplete:          "complete the word at the cursor",
	ActionFind:              "find in the editor or results",
	ActionFindNext:          "go to the next match",
	ActionFindPrev:          "go to the previous match",
	ActionSearchHistory:     "search the query history",
	ActionCommandPalette:    "open the command palette",
	ActionGoToTable:         "jump to a table",
	ActionFindFile:          "open a SQL file from the attached worktree",
	ActionSaveFile:          "save the open file",
	ActionCycleTab:          "switch tab in the focused pane",
	ActionInspect:           "show the selected table or result row in full",
	ActionSortColumn:        "sort the results by the selected column",
	ActionSwitchDataSource:  "switch to another datasource",
	ActionExplain:           "explain the statement under the cursor",
	ActionAnalyze:           "run it and report what it actually did",
	ActionSessions:          "list what else is running on the server",
	ActionKillSession:       "stop another connection's statement",
	ActionLocks:             "show which connections are waiting on which",
	ActionHelp:              "show this help",
	ActionDetach:            "leave the terminal, keeping the session running",
	ActionQuit:              "quit",
}

// reserved lists actions whose feature is not implemented yet. The UI says so
// rather than ignoring the key, which would read as a broken binding.
//
// Empty today: every bound action does something. It stays because binding a
// key before the feature lands is a normal step, and an unimplemented key
// must announce itself rather than appear dead.
var reserved = map[Action]bool{}

// familiar says whether an action's binding is the one every editor and every
// macOS application already uses, so it is not something dv teaches.
//
// The test is sameness, not shape: ⌘D looks conventional and means "select
// the next occurrence" in VS Code, which is the most expensive kind of
// difference — a user believes they already know it. Where a call is close,
// it goes here as false, because a key wrongly listed as known is a key
// nobody is taught.
//
// It is a property of the action rather than of the chord. A preset may
// rebind anything; what a reader already knows does not move with it.
var familiar = map[Action]bool{
	// Cursor movement and selection, identical in every text field.
	ActionWordLeft:          true,
	ActionWordRight:         true,
	ActionSelectWordLeft:    true,
	ActionSelectWordRight:   true,
	ActionLineStart:         true,
	ActionLineEnd:           true,
	ActionSelectLineStart:   true,
	ActionSelectLineEnd:     true,
	ActionDeleteWordLeft:    true,
	ActionDeleteToLineStart: true,

	// The clipboard, and the keys that mean the same thing everywhere.
	// ⌘C is deliberately absent: its action is CopyOrCancel.
	ActionCut:       true,
	ActionPaste:     true,
	ActionSelectAll: true,
	ActionSaveFile:  true,
	ActionFind:      true,
	ActionQuit:      true,

	ActionRun:              false,
	ActionRunAll:           false,
	ActionCancel:           false,
	ActionCopyOrCancel:     false,
	ActionToggleComment:    false,
	ActionDuplicateLine:    false,
	ActionDeleteLine:       false,
	ActionNextPane:         false,
	ActionPrevPane:         false,
	ActionToggleSidebar:    false,
	ActionRefreshSchema:    false,
	ActionUseSchema:        false,
	ActionComplete:         false,
	ActionFindNext:         false,
	ActionFindPrev:         false,
	ActionSearchHistory:    false,
	ActionCommandPalette:   false,
	ActionGoToTable:        false,
	ActionFindFile:         false,
	ActionCycleTab:         false,
	ActionInspect:          false,
	ActionSortColumn:       false,
	ActionSwitchDataSource: false,
	ActionExplain:          false,
	ActionAnalyze:          false,
	ActionSessions:         false,
	ActionKillSession:      false,
	ActionLocks:            false,
	ActionHelp:             false,
	ActionDetach:           false,
}

// Familiar reports that dv does not have to teach this action's key.
func (a Action) Familiar() bool { return familiar[a] }

// order fixes how actions appear on the help screen, grouped by purpose.
var order = []Action{
	ActionRun, ActionRunAll, ActionCancel, ActionExplain, ActionAnalyze,
	ActionWordLeft, ActionWordRight, ActionSelectWordLeft, ActionSelectWordRight,
	ActionLineStart, ActionLineEnd, ActionSelectLineStart, ActionSelectLineEnd,
	ActionDeleteWordLeft, ActionDeleteToLineStart,
	ActionComplete, ActionCopyOrCancel, ActionCut, ActionPaste,
	ActionSelectAll, ActionToggleComment, ActionDuplicateLine, ActionDeleteLine,
	ActionSaveFile,
	ActionFind, ActionFindNext, ActionFindPrev, ActionSearchHistory,
	ActionCommandPalette, ActionGoToTable, ActionFindFile, ActionInspect,
	ActionSortColumn,
	ActionNextPane, ActionPrevPane, ActionCycleTab, ActionToggleSidebar,
	ActionRefreshSchema, ActionUseSchema, ActionSwitchDataSource, ActionSessions, ActionKillSession, ActionLocks,
	ActionHelp, ActionDetach, ActionQuit,
}

func (a Action) String() string {
	if name, ok := actionNames[a]; ok {
		return name
	}
	if a == ActionNone {
		return "none"
	}
	return "unknown"
}

// Describe returns the help-screen text for the action.
func (a Action) Describe() string { return descriptions[a] }

// Reserved reports whether the action names a feature that is not built yet.
func (a Action) Reserved() bool { return reserved[a] }

// AllActions lists every bindable action in help-screen order.
func AllActions() []Action {
	out := make([]Action, len(order))
	copy(out, order)
	return out
}

// actionByName is the reverse of actionNames, for configuration parsing.
var actionByName = func() map[string]Action {
	m := make(map[string]Action, len(actionNames))
	for a, name := range actionNames {
		m[name] = a
	}
	return m
}()
