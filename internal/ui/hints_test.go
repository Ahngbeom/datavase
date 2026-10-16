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
