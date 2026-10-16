package ui

import (
	"strings"
	"testing"
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
