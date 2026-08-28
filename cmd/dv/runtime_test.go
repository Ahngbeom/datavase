package main

import (
	"context"
	"io"
	"testing"

	"github.com/Ahngbeom/datavase/internal/daemon"
	"github.com/gdamore/tcell/v2"
)

// fakeSession is a minimal statefulSession stand-in: enough to run
// closingSession to completion without a real interface or database behind
// it.
type fakeSession struct{ err error }

func (f *fakeSession) SetScreen(tcell.Screen)                      {}
func (f *fakeSession) Stop()                                       {}
func (f *fakeSession) Run() error                                  { return f.err }
func (f *fakeSession) SwitchTo(string)                             {}
func (f *fakeSession) State(context.Context) (daemon.State, error) { return daemon.State{}, nil }

// fakeCloser records whether Close was called, standing in for the
// *catalog.Cache and *history.Store handles buildSession actually opens.
type fakeCloser struct{ closed bool }

func (c *fakeCloser) Close() error { c.closed = true; return nil }

// The cache and history handles buildSession opens must not outlive the
// session that used them: a daemon-hosted dv server has no defer to close
// them the way openUI does for the monolithic path, only the moment Run
// returns.
func TestClosingSessionClosesHandlesWhenRunReturns(t *testing.T) {
	cache, hist := &fakeCloser{}, &fakeCloser{}
	s := closingSession{
		statefulSession: &fakeSession{},
		closers:         []io.Closer{cache, hist},
	}

	if err := s.Run(); err != nil {
		t.Fatalf("Run() = %v, want nil", err)
	}
	if !cache.closed {
		t.Error("the cache handle was never closed")
	}
	if !hist.closed {
		t.Error("the history handle was never closed")
	}
}
