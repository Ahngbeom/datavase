//go:build integration

package ui

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/gdamore/tcell/v2"
)

func TestTheLauncherConnectsToTheChosenEntry(t *testing.T) {
	ds, password := testmysql.DataSource(t)
	cfg := config.Empty()
	cfg.DataSources = []config.DataSource{*ds}
	store := secret.NewMemory()
	_ = store.Set(ds.Name, password)

	screen := tcell.NewSimulationScreen("UTF-8")
	screen.SetSize(100, 30)

	done := make(chan struct {
		sess *session.Session
		err  error
	}, 1)
	go func() {
		sess, err := Launch(LaunchDeps{
			Config: cfg, ConfigPath: filepath.Join(t.TempDir(), "config.yaml"), Secrets: store,
			Connect: func(ctx context.Context, ds *config.DataSource) (*session.Session, error) {
				pw, _ := store.Get(ds.Name)
				return session.Open(ctx, ds, pw)
			},
			Screen: screen,
		})
		done <- struct {
			sess *session.Session
			err  error
		}{sess, err}
	}()

	time.Sleep(200 * time.Millisecond) // let the list draw
	screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)

	select {
	case r := <-done:
		if r.err != nil || r.sess == nil {
			t.Fatalf("Launch() = %v, %v; want a session", r.sess, r.err)
		}
		r.sess.Close()
	case <-time.After(20 * time.Second):
		t.Fatal("the launcher never returned")
	}
}

func TestClosingTheLauncherReturnsNoSession(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.SetSize(100, 30)

	done := make(chan *session.Session, 1)
	go func() {
		sess, _ := Launch(LaunchDeps{Config: config.Empty(), ConfigPath: filepath.Join(t.TempDir(), "c.yaml"), Screen: screen})
		done <- sess
	}()
	time.Sleep(200 * time.Millisecond)
	screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)

	select {
	case sess := <-done:
		if sess != nil {
			t.Error("Escape on an empty list produced a session")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the launcher never returned")
	}
}

func TestAddingAnEntryInTheLauncherWritesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.Empty()
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.SetSize(100, 40)

	done := make(chan struct{}, 1)
	go func() {
		Launch(LaunchDeps{Config: cfg, ConfigPath: path, Secrets: secret.NewMemory(), Screen: screen})
		done <- struct{}{}
	}()
	time.Sleep(200 * time.Millisecond)

	type_ := func(s string) {
		for _, r := range s {
			screen.InjectKey(tcell.KeyRune, r, tcell.ModNone)
		}
	}
	screen.InjectKey(tcell.KeyRune, 'a', tcell.ModNone) // add
	time.Sleep(100 * time.Millisecond)
	type_("local")
	screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone)
	type_("127.0.0.1")
	screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone) // port
	screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone) // user
	type_("root")
	// Tab through password, database, read only, tls, tls_ca, tunnel host/port/user/identity, Test → Save.
	for i := 0; i < 11; i++ {
		screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone) // Save
	time.Sleep(300 * time.Millisecond)
	screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the launcher never returned")
	}
	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() after the form saved: %v", err)
	}
	if len(saved.DataSources) != 1 || saved.DataSources[0].Name != "local" || saved.DataSources[0].User != "root" {
		t.Errorf("saved = %+v", saved.DataSources)
	}
}
