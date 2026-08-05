package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/procs"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Stopping somebody else's work is the operation that most wants a
// confirmation nobody can wave through — and demanding a typed word for every
// one of them is how a demand stops being read.
func TestWhatAKillHasToCost(t *testing.T) {
	for _, tt := range []struct {
		name string
		env  config.Env
		stop procs.Stop
		want string
	}{
		{
			// The client reruns its query. A button is the whole of what this
			// is worth outside production.
			name: "stopping a statement on dev is a button",
			env:  config.EnvDev, stop: procs.StopStatement, want: "",
		},
		{
			name: "and on stage",
			env:  config.EnvStage, stop: procs.StopStatement, want: "",
		},
		{
			name: "against production it is typed",
			env:  config.EnvProd, stop: procs.StopStatement, want: "KILL",
		},
		{
			// The connection goes and any transaction it held is rolled back,
			// so this is somebody's work rather than somebody's wait.
			name: "and ending the connection costs more than stopping the statement",
			env:  config.EnvProd, stop: procs.StopConnection, want: "KILL CONNECTION",
		},
		{
			name: "ending a connection off production is still a button",
			env:  config.EnvDev, stop: procs.StopConnection, want: "",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := killPhrase(tt.env, tt.stop); got != tt.want {
				t.Errorf("killPhrase(%v, %v) = %q, want %q", tt.env, tt.stop, got, tt.want)
			}
		})
	}
}

// A short list and a quiet server look identical, so the summary has to say
// which one is on screen.
func TestTheSessionsSummarySaysWhatCouldNotBeSeen(t *testing.T) {
	full := procs.Listing{
		Complete: true,
		Processes: []procs.Process{
			{ID: 1, Command: "Query"},
			{ID: 2, Command: "Sleep"},
		},
	}
	if got := sessionsSummary(full); got != "2 connections · 1 working" {
		t.Errorf("sessionsSummary(complete) = %q", got)
	}

	partial := full
	partial.Complete = false
	if got := sessionsSummary(partial); got == sessionsSummary(full) {
		t.Error("a partial listing reads exactly like a complete one")
	}
}

// An empty tree and a server that will not answer look identical on screen,
// and only one of them means nothing is stuck.
func TestALockTreeDistinguishesQuietFromSilent(t *testing.T) {
	quiet := lockTreeText(procs.LockTree{Supported: true}, 80)
	silent := lockTreeText(procs.LockTree{Supported: false}, 80)

	if quiet == silent {
		t.Fatalf("both read %q", quiet)
	}
	if !strings.Contains(quiet, "Nothing is waiting") {
		t.Errorf("a quiet server reads %q", quiet)
	}
	if !strings.Contains(silent, "does not expose") {
		t.Errorf("a silent server reads %q", silent)
	}

	if got := lockSummary(procs.LockTree{Supported: true}); got != "nothing is waiting on a lock" {
		t.Errorf("the quiet summary reads %q", got)
	}
	if got := lockSummary(procs.LockTree{}); got == lockSummary(procs.LockTree{Supported: true}) {
		t.Error("an unsupported server summarises exactly like a quiet one")
	}
}

// The blocker at the bottom is what the tree exists to point at, and the
// reason it is stuck — a transaction left open with nothing running — is the
// one a list of statements cannot show.
func TestALockTreeNamesTheIdleHolder(t *testing.T) {
	tree := procs.LockTree{
		Supported: true,
		Roots: procs.Tree([]procs.Wait{
			{
				Waiter:  procs.Thread{ID: 20, User: "app", Host: "h", SQL: "UPDATE orders SET total = 0"},
				Blocker: procs.Thread{ID: 10, User: "dba", Host: "h", Idle: true},
				Waited:  42 * time.Second,
			},
		}),
	}

	out := lockTreeText(tree, 80)
	for _, want := range []string{"idle in transaction", "waiting 42s", "UPDATE orders", "└─ "} {
		if !strings.Contains(out, want) {
			t.Errorf("the tree does not show %q:\n%s", want, out)
		}
	}
}

// The process list is laid out for a width, and the width it was laid out for
// used to be read when the text was composed rather than when it was drawn.
//
// A tab that has never been shown has no width to give, so the reply was the
// twenty-column floor and a statement came out folded into a stack of
// four-word lines on an eighty-column terminal. This is the same failure the
// plan pane, the status bar and the top bar are all built to avoid.
func TestTheProcessListIsLaidOutForTheWidthItIsDrawnAt(t *testing.T) {
	var laidOutFor int

	pane := newSessionsPane()
	pane.show(func(width int) string {
		laidOutFor = width
		return "listing"
	})

	// Never drawn, so nothing has been laid out and nothing has been guessed.
	if laidOutFor != 0 {
		t.Fatalf("the listing was laid out for %d before it was ever drawn", laidOutFor)
	}

	drawAt(t, pane, 100, 10)
	if laidOutFor < 90 {
		t.Errorf("laid out for %d columns on a pane 100 wide", laidOutFor)
	}
}

// A window that is made narrower has to fold the statements again; one that
// has not changed must not pay to.
func TestTheProcessListIsLaidOutAgainOnlyWhenTheWidthChanges(t *testing.T) {
	var layouts []int

	pane := newSessionsPane()
	pane.show(func(width int) string {
		layouts = append(layouts, width)
		return "listing"
	})

	drawAt(t, pane, 100, 10)
	drawAt(t, pane, 100, 10)
	if len(layouts) != 1 {
		t.Errorf("an unchanged width was laid out %d times: %v", len(layouts), layouts)
	}

	drawAt(t, pane, 60, 10)
	if len(layouts) != 2 {
		t.Errorf("a resize did not lay the listing out again: %v", layouts)
	}
	if layouts[1] >= layouts[0] {
		t.Errorf("the narrower window was laid out for %v", layouts)
	}
}

// drawAt draws a primitive at a size, on a screen no test has to look at.
func drawAt(t *testing.T, p tview.Primitive, width, height int) {
	t.Helper()

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)

	p.SetRect(0, 0, width, height)
	p.Draw(screen)
}
