//go:build integration

package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/intro"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// introPath is a marker location for a session that has not seen the card.
func introPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "datavase", "intro-seen")
}

func newIntroHarness(t *testing.T, path string) *harness {
	t.Helper()
	return newHarnessWithIntro(t, config.EnvDev, path)
}

func newIntroHarnessFor(t *testing.T, path string, env config.Env) *harness {
	t.Helper()
	return newHarnessWithIntro(t, env, path)
}

// Every other test in this package runs without a marker path, and none of
// them expects a dialog over the interface. A session that has nowhere to
// record the card must not show it.
func TestNoIntroductionWithoutSomewhereToRecordIt(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.inspect(func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		if name == pageIntro {
			t.Errorf("the introduction is shown with no marker path")
		}
		return true
	})
}

// The first run is the only one that gets it.
func TestTheIntroductionIsShownOnceAndRecorded(t *testing.T) {
	path := introPath(t)
	h := newIntroHarness(t, path)

	h.waitFor("the introduction", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageIntro
	})

	h.press(tcell.KeyEnter)
	h.waitFor("the introduction to close", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name != pageIntro
	})

	if !intro.Seen(path) {
		t.Error("closing the introduction did not record that it was seen")
	}

	// Closing has to hand the keyboard back to the editor, or the session
	// starts with focus nowhere anyone would guess.
	h.waitFor("focus to reach the editor", func(a *App) bool {
		return a.app.GetFocus() == a.editor
	})
}

func TestTheIntroductionIsNotShownAgain(t *testing.T) {
	path := introPath(t)
	if err := intro.MarkSeen(path); err != nil {
		t.Fatalf("MarkSeen() error = %v", err)
	}

	h := newIntroHarness(t, path)
	h.inspect(func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		if name == pageIntro {
			t.Error("the introduction is shown again after it was recorded")
		}
		return true
	})
}

// Hardcoding the keys would make the card lie to precisely the people who
// rebound them — the same reason the key reference is generated.
func TestTheIntroductionNamesTheKeysThatAreInForce(t *testing.T) {
	h := newIntroHarness(t, introPath(t))

	h.waitFor("the introduction", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageIntro
	})

	h.inspect(func(a *App) bool {
		card := a.introText()
		for _, action := range startHere {
			if !strings.Contains(card, a.keyLabel(action)) {
				t.Errorf("the introduction does not name the key for %s:\n%s", action, card)
			}
		}
		return true
	})
}

// What the guard will do is the one thing about this session that cannot be
// worked out by pressing keys, and it is different on production.
func TestTheIntroductionSaysWhatTheGuardWillDo(t *testing.T) {
	for _, tc := range []struct {
		env  config.Env
		want string
	}{
		{config.EnvDev, "ask before they run"},
		{config.EnvProd, "refused"},
	} {
		t.Run(string(tc.env), func(t *testing.T) {
			h := newIntroHarnessFor(t, introPath(t), tc.env)

			h.inspect(func(a *App) bool {
				if card := a.introText(); !strings.Contains(card, tc.want) {
					t.Errorf("on %s the introduction does not say %q:\n%s", tc.env, tc.want, card)
				}
				return true
			})
		})
	}
}

// It has to be reachable again: someone who pressed Enter to make the dialog
// go away has not read it, and that is the commonest thing to do with a dialog
// that appears before you asked for anything.
func TestTheIntroductionCanBeOpenedAgainFromThePalette(t *testing.T) {
	path := introPath(t)
	if err := intro.MarkSeen(path); err != nil {
		t.Fatalf("MarkSeen() error = %v", err)
	}

	h := newIntroHarness(t, path)

	h.do(keymap.ActionCommandPalette)
	h.waitFor("the palette", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pagePalette
	})

	h.typeInto(cmdGettingStarted)
	h.press(tcell.KeyEnter)

	h.waitFor("the introduction", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageIntro
	})
}

// A marker that could not be written leaves the card to appear again, which is
// a moment's annoyance. Refusing to start, or deciding it had been seen, are
// both worse answers.
func TestAMarkerThatCannotBeWrittenDoesNotStopTheSession(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// The marker's parent is a file, so creating the directory cannot succeed.
	h := newIntroHarness(t, filepath.Join(blocked, "intro-seen"))

	h.waitFor("the introduction", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name == pageIntro
	})

	h.press(tcell.KeyEnter)
	h.waitFor("the introduction to close", func(a *App) bool {
		name, _ := a.pages.GetFrontPage()
		return name != pageIntro
	})

	// Still usable: the failure costs the once-only-ness and nothing else.
	h.waitFor("focus to reach the editor", func(a *App) bool {
		return a.app.GetFocus() == a.editor
	})
}
