package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/keymap"
)

// An empty editor is the first thing a newcomer looks at. The header a row
// above already names run and complete, so what the placeholder adds is an
// invitation and the way to everything else.
func TestTheEmptyEditorInvitesAQueryAndPointsAtTheKeys(t *testing.T) {
	got := editorPlaceholder(false, "F1")

	if !strings.HasPrefix(got, "Type a query") {
		t.Errorf("the placeholder does not invite a query first: %q", got)
	}
	if !strings.Contains(got, "F1") {
		t.Errorf("the placeholder does not name the key reference: %q", got)
	}
}

// The preview is offered only when there is a tree to double-click. Naming it
// with the schema pane hidden points at something that is not on screen.
func TestThePlaceholderOffersThePreviewOnlyWithTheTreeShowing(t *testing.T) {
	if got := editorPlaceholder(false, "F1"); strings.Contains(got, "dbl-click") {
		t.Errorf("the hidden tree's preview is offered: %q", got)
	}
	if got := editorPlaceholder(true, "F1"); !strings.Contains(got, "dbl-click a table") {
		t.Errorf("the visible tree's preview is not offered: %q", got)
	}
}

// The welcome is the one screen that has to answer "what do I do now" for
// someone who has never seen dv. It does that with the same five keys the
// key reference opens with, so the two cannot come to disagree.
func TestTheWelcomeListsTheKeysTheReferenceStartsWith(t *testing.T) {
	km := keymap.Default()
	got := visibleText(welcomeText(km))

	for _, action := range startHere {
		if action != keymap.ActionHelp && !strings.Contains(got, action.Describe()) {
			t.Errorf("the welcome does not mention %q:\n%s", action.Describe(), got)
		}
		if label := km.DisplayBindings(action)[0].Label(onMac); !strings.Contains(got, label) {
			t.Errorf("the welcome does not name %s's key %q:\n%s", action, label, got)
		}
	}
	if !strings.Contains(got, "Enter") || !strings.Contains(got, "Esc") {
		t.Errorf("the welcome does not say how to get past it:\n%s", got)
	}
	// Borrowed from the help screen, "this help" would name the welcome.
	if strings.Contains(got, "this help") {
		t.Errorf("the welcome calls itself the help:\n%s", got)
	}
}

// A tip that changed on every redraw, or that two sessions with the same
// count disagreed on, could not be pinned down by anyone reporting it.
// Consecutive sessions differ, so a returning user meets something new.
func TestTheTipFollowsTheSessionCount(t *testing.T) {
	km := keymap.Default()

	if tipFor(3, km) != tipFor(3, km) {
		t.Error("the same session count chose two different tips")
	}
	if tipFor(0, km) == tipFor(1, km) {
		t.Errorf("two sessions in a row got the same tip: %q", tipFor(0, km))
	}
	if tipFor(0, km) != tipFor(len(tips), km) {
		t.Error("the tips do not come round again once every one has been shown")
	}
}

// A tip is worth showing only if the key it names is the one that works
// here, so each one is spelled from the key map in force.
func TestEveryTipNamesABoundKey(t *testing.T) {
	km := keymap.Default()

	for i, tip := range tips {
		got := tipFor(i, km)
		if !strings.HasPrefix(got, "Tip: ") {
			t.Errorf("tip %d does not say it is one: %q", i, got)
		}
		if len(km.DisplayBindings(tip.action)) == 0 {
			t.Errorf("tip %d advertises %s, which has no key", i, tip.action)
		}
		if !strings.Contains(got, tipKeyLabel(km, tip.action)) {
			t.Errorf("tip %d does not name the key for %s: %q", i, tip.action, got)
		}
	}
}

// The tip is the least of what the bar has to say. Anything that actually
// happened takes the line, and the tip does not come back over it.
func TestATipGivesWayToAnythingThatHappened(t *testing.T) {
	tip := tipFor(0, keymap.Default())
	s := baseStatus()
	s.hints = openingClauses(opening{helpKey: "F1", sidebarKey: "^B", tip: tip})

	if got := s.renderWidth(200); !strings.Contains(got, tip) {
		t.Fatalf("an idle bar does not show the tip: %q", got)
	}

	s.message = "nothing to run"
	if got := s.renderWidth(200); strings.Contains(got, tip) {
		t.Errorf("the tip is still competing with a real message: %q", got)
	}
}

// Of everything on the opening line, the tip is what goes before the schema
// tree and the help key do: those are how the rest is found.
func TestTheTipIsShedBeforeTheWaysToFindEverythingElse(t *testing.T) {
	tip := tipFor(0, keymap.Default())
	s := baseStatus()
	s.hints = openingClauses(opening{
		serverVersion: "11.4.12-MariaDB-ubu2404",
		helpKey:       "F1",
		sidebarKey:    "^B",
		tip:           tip,
	})

	got := s.renderWidth(40)
	if strings.Contains(got, tip) {
		t.Errorf("a narrow bar kept the tip: %q", got)
	}
	for _, want := range []string{"schema tree", "F1 for keys"} {
		if !strings.Contains(got, want) {
			t.Errorf("a narrow bar dropped %q for something else: %q", want, got)
		}
	}
}
