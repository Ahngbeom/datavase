// Command dv is datavase's entry point: a terminal MySQL client.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/cli"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/history"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/ui"
	"golang.org/x/term"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Before flag parsing and before configuration: "which build is this"
	// has to be answerable on a machine that has never been set up, and the
	// flag spellings would otherwise be rejected as undefined.
	if cli.HandleVersion(os.Stdout, os.Args[1:]) {
		return 0
	}

	configPath := flag.String("c", "", "path to config.yaml (default: $XDG_CONFIG_HOME/datavase/config.yaml)")
	flag.Parse()

	path := *configPath
	if path == "" {
		p, err := config.DefaultPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		path = p
	}

	cfg, err := config.Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		cfg, err = config.Empty(), nil
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	for _, key := range cfg.Ignored() {
		fmt.Fprintf(os.Stderr, "config: %q is no longer used and was ignored\n", key)
	}

	app := &cli.App{
		Config:       cfg,
		Secrets:      secrets(),
		Out:          os.Stdout,
		Err:          os.Stderr,
		ReadPassword: readPassword,
		Probe:        probe,
		OpenUI: func(ctx context.Context, ds *config.DataSource, password string, cfg *config.Config) error {
			return openUI(ctx, ds, password, cfg, path)
		},
		Launch: func() error { return launch(cfg, path) },
	}
	return app.Run(flag.Args())
}

// openUI connects and hands control to the terminal interface. The context
// bounds the connection attempt only; the interface itself runs until the
// user quits.
func openUI(ctx context.Context, ds *config.DataSource, password string, cfg *config.Config, path string) error {
	sess, err := session.Open(ctx, ds, password)
	if err != nil {
		return err
	}
	return openSession(sess, cfg, path)
}

// launch shows the datasource list, then opens the interface on the session
// it produced.
func launch(cfg *config.Config, path string) error {
	sess, err := ui.Launch(ui.LaunchDeps{
		Config: cfg, ConfigPath: path, Secrets: secrets(), Probe: probe, Connect: connectTo,
	})
	if err != nil || sess == nil {
		return err
	}
	return openSession(sess, cfg, path)
}

// openSession hands an already-open session to the terminal interface.
//
// The interface owns the session from here: switching datasource closes the
// one it leaves, and a session closed twice or by nobody is how a tunnel
// outlives the thing it was carrying.
func openSession(sess *session.Session, cfg *config.Config, path string) error {
	// The schema cache is optional: a read-only home directory should cost
	// completion, not the whole session.
	var cache *catalog.Cache
	if p, err := catalog.DefaultCachePath(); err == nil {
		if opened, err := catalog.OpenCache(p); err == nil {
			cache = opened
			defer cache.Close()
		} else {
			fmt.Fprintf(os.Stderr, "completion disabled: %v\n", err)
		}
	}

	// History is optional for the same reason as the cache.
	var hist *history.Store
	if p, err := history.DefaultPath(); err == nil {
		if opened, err := history.Open(p); err == nil {
			hist = opened
			defer hist.Close()
		}
	}

	return ui.New(sess, cfg, ui.Deps{
		Cache:   cache,
		History: hist,
		Connect: connectTo,
	}).Run()
}

// secrets is where every password is read and written.
//
// One function rather than separate constructions, so the environment layer
// cannot end up on one caller's reads but not another's — which would show up
// as a datasource that works until you switch to it.
func secrets() secret.Store { return secret.WithEnv(secret.NewKeychain()) }

// connectTo opens another datasource for a switch mid-session.
//
// The password comes from the keychain or the environment and nowhere else. A
// switch that could prompt would mean a modal password field over a running
// interface, and a datasource nobody has run "dv auth" for is one this session
// was never in a position to reach.
func connectTo(ctx context.Context, ds *config.DataSource) (*session.Session, error) {
	password, err := secrets().Get(ds.Name)
	if err != nil {
		// Both ways out, because the first one does not exist on a machine with
		// no keychain — which is exactly the machine this is most likely to
		// fail on.
		return nil, fmt.Errorf("no password for %q; run: dv auth %s — or set %s",
			ds.Name, ds.Name, secret.EnvVarName(ds.Name))
	}
	return session.Open(ctx, ds, password)
}

// probe verifies reachability, raising the tunnel first when one is needed,
// so `dv check` tests the same path the interface will take.
func probe(ctx context.Context, ds *config.DataSource, password string) (string, error) {
	sess, err := session.Open(ctx, ds, password)
	if err != nil {
		return "", err
	}
	defer sess.Close()

	return sess.Conn.ServerVersion(), nil
}

// readPassword reads from the terminal with echo disabled so the password
// never appears on screen or in scrollback.
func readPassword(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New("stdin is not a terminal; run dv auth interactively")
	}

	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
