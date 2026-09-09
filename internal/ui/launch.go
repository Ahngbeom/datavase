package ui

import (
	"context"
	"fmt"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// LaunchDeps is what the datasource list needs before there is a session.
type LaunchDeps struct {
	Config     *config.Config
	ConfigPath string
	Secrets    secret.Store
	Probe      func(ctx context.Context, ds *config.DataSource, password string) (string, error)
	Connect    func(ctx context.Context, ds *config.DataSource) (*session.Session, error)
	// Screen replaces the terminal, for tests. Nil is the real one.
	Screen tcell.Screen
}

// Launch shows the datasource list on its own and returns the session the
// user connected to, or nil when they closed the list instead.
//
// It is its own small application rather than a state of App because App
// assumes a connection in nearly every method; a launcher that ends by
// handing over a session keeps that assumption true.
func Launch(deps LaunchDeps) (*session.Session, error) {
	app := tview.NewApplication()
	if deps.Screen != nil {
		app.SetScreen(deps.Screen)
	}

	var (
		picker *dsPicker
		got    *session.Session
		fail   error
	)
	picker = newDSPicker(app, pickerDeps{
		cfg:     deps.Config,
		save:    func() error { return config.Save(deps.ConfigPath, deps.Config) },
		secrets: deps.Secrets,
		probe:   deps.Probe,
		close:   app.Stop,
		connect: func(ds *config.DataSource) {
			picker.setBusy(true)
			picker.setStatus(fmt.Sprintf("connecting to %s…", ds.Name))
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
				defer cancel()
				sess, err := deps.Connect(ctx, ds)
				app.QueueUpdateDraw(func() {
					picker.setBusy(false)
					if err != nil {
						picker.setStatus(tag(colourDanger, err.Error()))
						return
					}
					got = sess
					app.Stop()
				})
			}()
		},
	})

	root := tview.NewPages().AddPage("picker", centred(picker.Primitive(), 80, 24), true, true)
	if err := app.SetRoot(root, true).EnableMouse(true).Run(); err != nil {
		fail = err
	}
	return got, fail
}
