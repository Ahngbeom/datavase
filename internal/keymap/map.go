package keymap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// Map resolves key events to actions.
type Map struct {
	// byBinding is the lookup table, keyed by the canonical binding.
	byBinding map[Binding]Action
	// byAction preserves each action's bindings for the help screen.
	byAction map[Action][]Binding
	// preset is which named keyboard this map was built from.
	preset Preset
}

// Lookup returns the action bound to the event, or ActionNone.
func (m *Map) Lookup(ev *tcell.EventKey) Action {
	return m.byBinding[normalize(ev)]
}

// Bindings returns the bindings for an action, most idiomatic first.
func (m *Map) Bindings(a Action) []Binding {
	out := make([]Binding, len(m.byAction[a]))
	copy(out, m.byAction[a])
	return out
}

// bind registers every spelling of one action.
func (m *Map) bind(a Action, bindings ...Binding) {
	for _, b := range bindings {
		m.byBinding[b] = a
	}
	m.byAction[a] = append(m.byAction[a], bindings...)
	sortBindings(m.byAction[a])
}

// Apply replaces the bindings of the named actions.
//
// It is all-or-nothing: a map with one bad entry changes nothing, so a typo
// in configuration cannot leave the user with a half-rebound keyboard.
func (m *Map) Apply(overrides map[string][]string) error {
	parsed := make(map[Action][]Binding, len(overrides))

	// Iterate in a fixed order so the same configuration always reports the
	// same first error.
	names := make([]string, 0, len(overrides))
	for name := range overrides {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		action, ok := actionByName[name]
		if !ok {
			return fmt.Errorf("unknown action %q in keymap; valid actions are: %s",
				name, strings.Join(sortedActionNames(), ", "))
		}

		for _, spec := range overrides[name] {
			b, err := ParseBinding(spec)
			if err != nil {
				return fmt.Errorf("keymap action %q: %w", name, err)
			}
			parsed[action] = append(parsed[action], b)
		}
	}

	// Everything parsed; now it is safe to mutate.
	for action, bindings := range parsed {
		m.clear(action)
		m.bind(action, bindings...)
	}
	return nil
}

// clear drops an action's existing bindings so Apply replaces rather than
// accumulates — otherwise a user who rebinds a key would still be able to
// trigger it with the old one.
func (m *Map) clear(a Action) {
	for _, b := range m.byAction[a] {
		delete(m.byBinding, b)
	}
	delete(m.byAction, a)
}

func sortedActionNames() []string {
	names := make([]string, 0, len(actionByName))
	for name := range actionByName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ctrlAndCmd returns the same key bound under both Ctrl and Cmd.
//
// This pairing is what makes "⌘ and Ctrl both work" true: a Mac user with a
// terminal configured to forward Cmd gets DataGrip's keys unchanged, and
// everyone else gets the Ctrl form, from one registration.
func ctrlAndCmd(key tcell.Key, r rune, extra tcell.ModMask) []Binding {
	return []Binding{
		{Key: key, Rune: r, Mods: tcell.ModCtrl | extra},
		{Key: key, Rune: r, Mods: tcell.ModMeta | extra},
	}
}

// ctrlAndCmdRune is ctrlAndCmd for character keys.
func ctrlAndCmdRune(r rune, extra tcell.ModMask) []Binding {
	return ctrlAndCmd(tcell.KeyRune, r, extra)
}

// altAndCtrl returns the same key bound under both Alt and Ctrl.
//
// This is the pairing word movement needs: macOS spells it Option, everywhere
// else it is Ctrl. Deliberately not ⌘, which on a Mac means line movement.
func altAndCtrl(key tcell.Key, extra tcell.ModMask) []Binding {
	return []Binding{
		{Key: key, Mods: tcell.ModAlt | extra},
		{Key: key, Mods: tcell.ModCtrl | extra},
	}
}

// backspace returns the canonical Backspace binding.
//
// Terminals send either 0x08 (KeyBackspace) or 0x7F (KeyBackspace2) depending
// on their erase setting, but normalize already folds the former into the
// latter, so one registration serves both.
func backspace(mods tcell.ModMask) []Binding {
	return []Binding{{Key: tcell.KeyBackspace2, Mods: mods}}
}

// plain is a binding with no modifiers.
func plain(key tcell.Key) []Binding {
	return []Binding{{Key: key}}
}

// Default returns the key map for the default preset.
func Default() *Map {
	m, err := ForPreset(DefaultPreset)
	if err != nil {
		// The default preset is a constant in this package; if it does not
		// resolve, the package is broken, not the caller's input.
		panic(err)
	}
	return m
}

// baseMap returns the DataGrip-flavoured key map every preset starts from.
//
// Where DataGrip and VS Code agree, that key is used. Where they differ,
// DataGrip wins — this is a SQL tool, and that is the muscle memory being
// served. Fallbacks exist for every combination a terminal without the
// extended keyboard protocol cannot express.
func baseMap() *Map {
	m := &Map{
		byBinding: make(map[Binding]Action),
		byAction:  make(map[Action][]Binding),
	}

	// Running. Ctrl+Enter is DataGrip's; F5 and the Ctrl+J stand-in cover
	// terminals that cannot report a modified Enter.
	m.bind(ActionRun,
		append(ctrlAndCmd(tcell.KeyEnter, 0, 0),
			Binding{Key: tcell.KeyF5},
			Binding{Key: tcell.KeyRune, Rune: enterStandIn, Mods: tcell.ModCtrl},
		)...)

	m.bind(ActionRunAll,
		append(ctrlAndCmd(tcell.KeyEnter, 0, tcell.ModShift),
			Binding{Key: tcell.KeyF5, Mods: tcell.ModShift},
		)...)

	// DataGrip stops a statement with Cmd+F2.
	m.bind(ActionCancel, ctrlAndCmd(tcell.KeyF2, 0, 0)...)

	// Ctrl+C is resolved at runtime: copy when there is a selection, cancel
	// otherwise.
	m.bind(ActionCopyOrCancel, ctrlAndCmdRune('c', 0)...)

	// Cursor movement.
	//
	// This is the one place ⌘ and Ctrl are not interchangeable. On macOS ⌘←
	// goes to the start of the line while ⌥← moves by word, and Ctrl← is the
	// Windows and Linux spelling of the latter. Collapsing them the way every
	// other binding does would put the wrong thing under a Mac user's fingers.
	m.bind(ActionWordLeft, altAndCtrl(tcell.KeyLeft, 0)...)
	m.bind(ActionWordRight, altAndCtrl(tcell.KeyRight, 0)...)
	m.bind(ActionSelectWordLeft, altAndCtrl(tcell.KeyLeft, tcell.ModShift)...)
	m.bind(ActionSelectWordRight, altAndCtrl(tcell.KeyRight, tcell.ModShift)...)

	m.bind(ActionLineStart,
		Binding{Key: tcell.KeyLeft, Mods: tcell.ModMeta},
		Binding{Key: tcell.KeyHome})
	m.bind(ActionLineEnd,
		Binding{Key: tcell.KeyRight, Mods: tcell.ModMeta},
		Binding{Key: tcell.KeyEnd})
	m.bind(ActionSelectLineStart,
		Binding{Key: tcell.KeyLeft, Mods: tcell.ModMeta | tcell.ModShift},
		Binding{Key: tcell.KeyHome, Mods: tcell.ModShift})
	m.bind(ActionSelectLineEnd,
		Binding{Key: tcell.KeyRight, Mods: tcell.ModMeta | tcell.ModShift},
		Binding{Key: tcell.KeyEnd, Mods: tcell.ModShift})

	// Terminals disagree on which backspace key they send, so both are bound.
	m.bind(ActionDeleteWordLeft, backspace(tcell.ModAlt)...)
	m.bind(ActionDeleteWordLeft, backspace(tcell.ModCtrl)...)
	m.bind(ActionDeleteToLineStart, backspace(tcell.ModMeta)...)

	// Editing.
	m.bind(ActionCut, ctrlAndCmdRune('x', 0)...)
	m.bind(ActionPaste, ctrlAndCmdRune('v', 0)...)
	m.bind(ActionSelectAll, ctrlAndCmdRune('a', 0)...)
	m.bind(ActionToggleComment, ctrlAndCmdRune('/', 0)...)
	m.bind(ActionDuplicateLine, ctrlAndCmdRune('d', 0)...)
	m.bind(ActionDeleteLine, ctrlAndCmdRune('y', 0)...)

	// Completion. Ctrl+Space is the one key DataGrip and VS Code share
	// outright, so no Cmd variant is offered.
	m.bind(ActionComplete,
		Binding{Key: tcell.KeyRune, Rune: ' ', Mods: tcell.ModCtrl},
	)

	// ⌘F means the same thing here as in every other editor: find in the text
	// in front of you. The query history, which used to answer to this key, is
	// a different question and now has its own.
	m.bind(ActionFind, ctrlAndCmdRune('f', 0)...)
	// F9 alongside the chord because a shifted Ctrl letter needs the extended
	// keyboard protocol to be reported at all, and it is the last free function
	// key that is not already spoken for.
	m.bind(ActionSearchHistory,
		append(ctrlAndCmdRune('f', tcell.ModShift),
			Binding{Key: tcell.KeyF9})...)
	// ⌘G is what "again" is called on this platform. The modal editor has n
	// and N, which need no modifier at all; these are for the keyboards where
	// an unmodified letter is text.
	m.bind(ActionFindNext, ctrlAndCmdRune('g', 0)...)
	m.bind(ActionFindPrev, ctrlAndCmdRune('g', tcell.ModShift)...)
	// F3 alongside the chord, and not merely as a courtesy. The palette is how
	// every command without a key of its own is reached, so it is the one
	// binding that must survive a host application claiming ⌘⇧A — and
	// Ctrl+Shift+A is no safety net, since a shifted Ctrl letter needs the
	// extended keyboard protocol to be reported at all.
	m.bind(ActionCommandPalette,
		append(ctrlAndCmdRune('a', tcell.ModShift),
			Binding{Key: tcell.KeyF3})...)
	m.bind(ActionGoToTable, ctrlAndCmdRune('n', 0)...)

	// The worktree's files. ⌘⇧O is DataGrip's "go to file"; ⌘⇧N, which the
	// Windows keyboard uses for it, is already the schema picker here.
	// F2 carries saving for terminals that still treat Ctrl+S as XOFF and
	// freeze rather than deliver it. Cancelling is ⌘F2, so plain F2 is free.
	// Tab switching and inspection.
	//
	// ⌘⇥ is not available: macOS reserves it for the application switcher at
	// system level, so a terminal can never deliver it. Ctrl+⇥ needs the
	// extended keyboard protocol to be told apart from ⇥, which is why F6
	// carries it everywhere else.
	m.bind(ActionCycleTab,
		Binding{Key: tcell.KeyTab, Mods: tcell.ModCtrl},
		Binding{Key: tcell.KeyF6})

	// Ctrl+I cannot be used: it is byte 0x09, the same as ⇥.
	m.bind(ActionInspect,
		Binding{Key: tcell.KeyRune, Rune: 'i', Mods: tcell.ModMeta},
		Binding{Key: tcell.KeyRune, Rune: 'i', Mods: tcell.ModCtrl | tcell.ModShift},
		Binding{Key: tcell.KeyF4})

	// Panes.
	m.bind(ActionNextPane, plain(tcell.KeyTab)...)
	m.bind(ActionPrevPane, plain(tcell.KeyBacktab)...)
	m.bind(ActionToggleSidebar, ctrlAndCmdRune('b', 0)...)
	m.bind(ActionRefreshSchema, ctrlAndCmdRune('r', 0)...)

	// Choosing the schema. ⌘⇧N mirrors DataGrip's "new" family; F7 carries it
	// on terminals that cannot report a shifted ⌘ letter.
	m.bind(ActionUseSchema,
		append(ctrlAndCmdRune('n', tcell.ModShift),
			Binding{Key: tcell.KeyF7})...)

	// Application.
	m.bind(ActionHelp, plain(tcell.KeyF1)...)
	m.bind(ActionQuit,
		append(ctrlAndCmdRune('q', 0), Binding{Key: tcell.KeyF10})...)

	return m
}
