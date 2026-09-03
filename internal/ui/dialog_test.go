package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/guard"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

// The help screen is built from an explicit list of groups, so an action
// added to the keymap and forgotten here has a key that works and no way to
// discover it. This is the check that stops that happening quietly.
func TestEveryActionAppearsOnTheHelpScreen(t *testing.T) {
	seen := make(map[keymap.Action]int)
	for _, group := range helpGroups {
		for _, action := range group.actions {
			seen[action]++
		}
	}

	for _, action := range keymap.AllActions() {
		switch seen[action] {
		case 1:
		case 0:
			t.Errorf("%s is bindable but is not on the help screen", action)
		default:
			t.Errorf("%s appears on the help screen %d times", action, seen[action])
		}
	}
}

// "Start here" repeats keys that appear again further down, which is
// deliberate: the reference has to be complete and the opening has to be
// short. What it must not do is advertise an action that no longer exists.
func TestStartHereOnlyNamesActionsTheReferenceAlsoCarries(t *testing.T) {
	inGroups := make(map[keymap.Action]bool)
	for _, group := range helpGroups {
		for _, action := range group.actions {
			inGroups[action] = true
		}
	}

	for _, action := range startHere {
		if !inGroups[action] {
			t.Errorf("Start here offers %s, which the key reference does not list", action)
		}
	}
}

// Five is the whole point. A "start here" that grows into a second reference
// is one nobody reads either.
func TestStartHereStaysShort(t *testing.T) {
	const most = 6

	if len(startHere) > most {
		t.Errorf("Start here lists %d keys, want at most %d", len(startHere), most)
	}
}

// A palette command carries no key of its own, so the key reference is the
// only place it can be discovered. One left off is one that only its author
// knows exists — which is what happened to attaching a worktree.
func TestEveryPaletteCommandAppearsOnTheHelpScreen(t *testing.T) {
	help := commandHelpText("⌘⇧A")

	for _, c := range paletteCommands() {
		if !strings.Contains(help, c.name) {
			t.Errorf("the palette offers %q but the help screen does not list it", c.name)
		}
		if !strings.Contains(help, c.summary) {
			t.Errorf("%q is listed without its summary", c.name)
		}
	}
}

// The refusal a locked production write produces is the one message a user
// reads at the moment they are stopped, so the way out it names has to exist.
// It once read "unlock with :write" — a command no preset ever had.
//
// The comparison is exact on purpose. A substring check accepts ":write", the
// very string this test exists to reject, because the palette offers "write"
// and the colon is the entire defect.
func TestTheUnlockHintNamesACommandThePaletteActuallyOffers(t *testing.T) {
	hint := unlockHint("⌘⇧A")

	quoted := regexp.MustCompile(`"([^"]*)"`).FindStringSubmatch(hint)
	if quoted == nil {
		t.Fatalf("the unlock hint names no command:\n%s", hint)
	}

	var named bool
	for _, c := range paletteCommands() {
		if c.name == quoted[1] {
			named = true
		}
	}
	if !named {
		t.Errorf("the hint names %q, which the palette does not offer:\n%s", quoted[1], hint)
	}
	if !strings.Contains(hint, "⌘⇧A") {
		t.Errorf("the unlock hint does not say how to open the palette:\n%s", hint)
	}
}

// A refusal the unlock cannot lift must not carry the hint, or the hint stops
// meaning anything. This is the interface half of the guard's Unlockable flag.
func TestOnlyAnUnlockableRefusalCarriesTheHint(t *testing.T) {
	locked := guard.Decision{Verdict: guard.Deny, Reason: "locked", Unlockable: true}
	final := guard.Decision{Verdict: guard.Deny, Reason: "no WHERE clause"}

	if !strings.Contains(refusalText(locked, "⌘⇧A"), "⌘⇧A") {
		t.Error("an unlockable refusal does not offer the way past it")
	}
	if strings.Contains(refusalText(final, "⌘⇧A"), "⌘⇧A") {
		t.Error("a refusal the unlock cannot lift offers a way past it anyway")
	}
}

// The section is useless without the key that opens the palette: a list of
// commands with no way in is a list of things you cannot do.
func TestTheCommandSectionNamesTheKeyThatOpensThePalette(t *testing.T) {
	if !strings.Contains(commandHelpText("⌘⇧A"), "⌘⇧A") {
		t.Errorf("the command section does not name the palette key:\n%s", commandHelpText("⌘⇧A"))
	}
}

// A command with no summary renders as a blank line on the help screen, and
// two commands with one name cannot be told apart in the palette.
func TestEveryPaletteCommandHasADistinctNameAndASummary(t *testing.T) {
	cmds := paletteCommands()
	seen := make(map[string]bool, len(cmds))

	for _, c := range cmds {
		if strings.TrimSpace(c.name) == "" {
			t.Error("a palette command has no name")
			continue
		}
		if strings.TrimSpace(c.summary) == "" {
			t.Errorf("palette command %q has no summary", c.name)
		}
		if seen[c.name] {
			t.Errorf("two palette commands are both called %q", c.name)
		}
		seen[c.name] = true
	}
}

// A context that opens an empty menu reads as a dead region.
func TestEveryContextOffersSomething(t *testing.T) {
	cmds := paletteCommands()

	for _, ctx := range allMenuContexts() {
		if got := menuEntries(cmds, ctx, func(keymap.Action) string { return "" }); len(got) == 0 {
			t.Errorf("right-clicking in context %v would open an empty menu", ctx)
		}
	}
}

// A typo'd context shows up in no menu and is otherwise silent, so the
// compiler cannot catch it and this must.
func TestEveryCommandNamesRealContexts(t *testing.T) {
	known := make(map[menuContext]bool, len(allMenuContexts()))
	for _, ctx := range allMenuContexts() {
		known[ctx] = true
	}

	for _, cmd := range paletteCommands() {
		for _, ctx := range cmd.contexts {
			if !known[ctx] {
				t.Errorf("command %q names context %v, which does not exist", cmd.name, ctx)
			}
		}
	}
}
