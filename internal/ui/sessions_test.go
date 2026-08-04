package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/procs"
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
