package ui

import (
	"testing"

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
