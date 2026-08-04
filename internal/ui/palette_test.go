package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// chordMods are the modifiers another application can take before the key ever
// reaches a terminal program: a window manager's shortcut, a terminal's own
// menu, a multiplexer's prefix.
//
// Shift is not among them. A shifted key is a distinct terminfo entry rather
// than a chord, and nothing upstream claims it on its own.
const chordMods = tcell.ModCtrl | tcell.ModMeta | tcell.ModAlt

// reachableWithoutAChord reports whether the map binds the action to a key
// that no host application is in a position to intercept.
func reachableWithoutAChord(m *keymap.Map, a keymap.Action) bool {
	for _, b := range m.Bindings(a) {
		if b.Mods&chordMods == 0 {
			return true
		}
	}
	return false
}

func paletteCoverage() map[keymap.Action]bool {
	covered := make(map[keymap.Action]bool)
	for _, c := range paletteCommands() {
		if c.covers != keymap.ActionNone {
			covered[c.covers] = true
		}
	}
	return covered
}

// The invariant this whole arrangement exists for.
//
// A terminal that keeps ⌘ for its own menus, or a multiplexer whose prefix is
// Ctrl+B, takes those keys before datavase sees them. When that happened, the
// schema tree had no way in at all: ⌘B and Ctrl+B were its only two bindings
// and the palette did not offer it. Every action now has a route that survives
// losing its chord.
func TestEveryActionIsReachableWithoutAChord(t *testing.T) {
	covered := paletteCoverage()

	for _, preset := range keymap.Presets() {
		m, err := keymap.ForPreset(preset)
		if err != nil {
			t.Fatalf("%s: %v", preset, err)
		}

		for _, action := range keymap.AllActions() {
			switch {
			case reachableWithoutAChord(m, action):
			case covered[action]:
			case paletteExempt[action]:
			default:
				t.Errorf("%s preset: %s can only be reached by a chord — bind it to a plain key, "+
					"offer it in the palette, or say in paletteExempt why it does not need either",
					preset, action)
			}
		}
	}
}

// An exemption that stopped being needed is a note that has quietly become
// wrong, and the next reader will believe it.
func TestNoExemptionCoversAnActionThatIsAlreadyReachable(t *testing.T) {
	covered := paletteCoverage()
	m := keymap.Default()

	for action := range paletteExempt {
		if covered[action] {
			t.Errorf("%s is exempt from the palette and also offered by it", action)
		}
		if reachableWithoutAChord(m, action) {
			t.Errorf("%s is exempt as unreachable but has a plain key; drop the exemption", action)
		}
	}
}

// Two commands claiming one action means the palette does the same thing twice
// under two names, and neither reads as the real one.
func TestNoTwoCommandsCoverTheSameAction(t *testing.T) {
	seen := make(map[keymap.Action]string)

	for _, c := range paletteCommands() {
		if c.covers == keymap.ActionNone {
			continue
		}
		if first, taken := seen[c.covers]; taken {
			t.Errorf("%q and %q both perform %s", first, c.name, c.covers)
		}
		seen[c.covers] = c.name
	}
}

// The palette is the route to everything that has no key, so it cannot be the
// one thing you need a key to reach.
func TestThePaletteItselfIsReachableWithoutAChord(t *testing.T) {
	for _, preset := range keymap.Presets() {
		m, err := keymap.ForPreset(preset)
		if err != nil {
			t.Fatalf("%s: %v", preset, err)
		}
		if !reachableWithoutAChord(m, keymap.ActionCommandPalette) {
			t.Errorf("%s preset: the palette is only reachable by a chord", preset)
		}
	}
}

// A command with no category is one that browsing cannot find: it would sit
// under whichever heading happened to come last, which is worse than being
// filed wrongly on purpose.
func TestEveryCommandIsFiledUnderAnOfferedCategory(t *testing.T) {
	offered := make(map[string]bool, len(paletteCategories))
	for _, c := range paletteCategories {
		offered[c] = true
	}

	for _, cmd := range paletteCommands() {
		switch {
		case cmd.category == "":
			t.Errorf("%q has no category", cmd.name)
		case !offered[cmd.category]:
			t.Errorf("%q is filed under %q, which paletteCategories does not list", cmd.name, cmd.category)
		}
	}
}

// A category listed and never used is a heading that renders empty, or worse,
// a note that has quietly stopped being true.
func TestEveryCategoryHasCommandsInIt(t *testing.T) {
	used := make(map[string]bool)
	for _, cmd := range paletteCommands() {
		used[cmd.category] = true
	}

	for _, c := range paletteCategories {
		if !used[c] {
			t.Errorf("paletteCategories lists %q and nothing is filed under it", c)
		}
	}
}

// noop stands in for what the App would do with a chosen command, so the rows
// can be built without a terminal.
func noop(command) func() { return func() {} }

// headings are the rows that name a group rather than offering a command.
func headings(items []searchItem) []string {
	var out []string
	for _, it := range items {
		if it.accept == nil {
			out = append(out, it.primary)
		}
	}
	return out
}

// Browsing is the case the categories exist for: someone who does not know
// what the command is called cannot type its name, and forty rows in one run
// is a list you scroll past rather than read.
func TestBrowsingThePaletteGroupsTheCommandsUnderHeadings(t *testing.T) {
	items := paletteItems("", noop)

	got := headings(items)
	if len(got) != len(paletteCategories) {
		t.Fatalf("got %d headings, want %d: %v", len(got), len(paletteCategories), got)
	}
	for i, want := range paletteCategories {
		if got[i] != want {
			t.Errorf("heading %d = %q, want %q", i, got[i], want)
		}
	}

	// Every command still reachable, exactly once: a grouping that drops one is
	// worse than no grouping, because the command looks as though it went away.
	seen := make(map[string]int)
	for _, it := range items {
		if it.accept != nil {
			seen[it.primary]++
		}
	}
	for _, cmd := range paletteCommands() {
		count := 0
		for row, n := range seen {
			if strings.HasPrefix(row, cmd.name+" ") || row == cmd.name {
				count += n
			}
		}
		if count != 1 {
			t.Errorf("%q appears %d times while browsing, want 1", cmd.name, count)
		}
	}
}

// Once something is typed the list is ranked by how well it matches, and a
// heading in the middle of that ordering means nothing. Enter also takes the
// best match, so a heading there would be a row that runs nothing.
func TestTypingDropsTheHeadings(t *testing.T) {
	items := paletteItems("quit", noop)

	if got := headings(items); len(got) != 0 {
		t.Errorf("typing left %d headings in the list: %v", len(got), got)
	}
	if len(items) == 0 || items[0].accept == nil {
		t.Fatal("the first row of a filtered palette does not run anything")
	}
	if !strings.HasPrefix(items[0].primary, "quit") {
		t.Errorf("first row = %q, want the command that was typed", items[0].primary)
	}
}

// Enter on an opened palette runs the first row that does anything. What that
// row is has to stay something a stray keypress can survive.
//
// It used to be "unlock writes" — the palette's first entry and the single
// most dangerous thing here — reached by opening the palette and pressing
// Enter, which is two keys and no reading.
func TestEnterOnAnUnfilteredPaletteCannotUnlockWrites(t *testing.T) {
	items := paletteItems("", noop)

	for _, it := range items {
		if it.accept == nil {
			continue
		}
		if strings.HasPrefix(it.primary, cmdEnableWrites) {
			t.Errorf("the first command Enter reaches is %q", cmdEnableWrites)
		}
		return
	}
	t.Fatal("the palette offers nothing to run")
}

// The ":" command line resolves palette command names, so a palette command
// that borrows a vim command's name and means something else makes the line
// lie about what it is about to do.
//
// "write" was the one that mattered: in vim it saves, and here it used to
// unlock writes against production. A vim user types ":w" and then ":write"
// without looking, which would have made the single most dangerous thing this
// application can do the thing a reflex reaches.
//
// "quit" is deliberately not listed. It means what vim means by it, so the
// line saying "quit" and quitting is agreement, not collision.
func TestThePaletteDoesNotClaimVimsFileCommands(t *testing.T) {
	// Spelled out rather than derived: this list is the claim being made, and
	// a derived one would only restate whatever the palette already does.
	taken := map[string]string{
		"w": "saves the file", "write": "saves the file",
		"wq": "saves and quits", "x": "saves and quits",
		"e": "opens a file", "edit": "opens a file",
		"r": "reads a file in", "read": "reads a file in",
	}

	for _, c := range paletteCommands() {
		if means, clash := taken[c.name]; clash {
			t.Errorf("the palette offers %q for %q, but to a vim user %q %s — a \":\" line cannot mean both",
				c.name, c.summary, c.name, means)
		}
	}
}
