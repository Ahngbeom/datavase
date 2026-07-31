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
	// ActionCycleTab moves through the tabs of whichever pane has focus.
	ActionCycleTab
	// ActionInspect shows the definition of the selected table.
	ActionInspect

	// Application.
	ActionHelp
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
	ActionCycleTab:          "cycle-tab",
	ActionInspect:           "inspect",
	ActionHelp:              "help",
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
	ActionCycleTab:          "switch tab in the focused pane",
	ActionInspect:           "show the selected table's definition",
	ActionHelp:              "show this help",
	ActionQuit:              "quit",
}

// reserved lists actions whose feature is not implemented yet. The UI says so
// rather than ignoring the key, which would read as a broken binding.
//
// Empty today: every bound action does something. It stays because binding a
// key before the feature lands is a normal step, and an unimplemented key
// must announce itself rather than appear dead.
var reserved = map[Action]bool{}

// order fixes how actions appear on the help screen, grouped by purpose.
var order = []Action{
	ActionRun, ActionRunAll, ActionCancel,
	ActionWordLeft, ActionWordRight, ActionSelectWordLeft, ActionSelectWordRight,
	ActionLineStart, ActionLineEnd, ActionSelectLineStart, ActionSelectLineEnd,
	ActionDeleteWordLeft, ActionDeleteToLineStart,
	ActionComplete, ActionCopyOrCancel, ActionCut, ActionPaste,
	ActionSelectAll, ActionToggleComment, ActionDuplicateLine, ActionDeleteLine,
	ActionFind, ActionFindNext, ActionFindPrev, ActionSearchHistory,
	ActionCommandPalette, ActionGoToTable, ActionInspect,
	ActionNextPane, ActionPrevPane, ActionCycleTab, ActionToggleSidebar,
	ActionRefreshSchema, ActionUseSchema,
	ActionHelp, ActionQuit,
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
