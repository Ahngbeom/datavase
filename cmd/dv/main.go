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
	"github.com/Ahngbeom/datavase/internal/gitlab"
	"github.com/Ahngbeom/datavase/internal/glab"
	"github.com/Ahngbeom/datavase/internal/history"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/recent"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/ui"
	"github.com/Ahngbeom/datavase/internal/worktree"
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
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			printGettingStarted(path)
			return 1
		}
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	app := &cli.App{
		Config:       cfg,
		Secrets:      secret.NewKeychain(),
		Out:          os.Stdout,
		Err:          os.Stderr,
		ReadPassword: readPassword,
		Probe:        probe,
		OpenUI:       openUI,
	}
	return app.Run(flag.Args())
}

// openUI connects and hands control to the terminal interface. The context
// bounds the connection attempt only; the interface itself runs until the
// user quits.
func openUI(ctx context.Context, ds *config.DataSource, password string, cfg *config.Config, opt cli.UIOptions) error {
	// Key bindings are resolved before connecting: a typo in the keymap
	// should fail immediately, not after a password prompt and a handshake.
	keys, err := keymap.FromConfig(cfg.Keymap.Preset, cfg.Keymap.Actions)
	if err != nil {
		return fmt.Errorf("keymap: %w", err)
	}

	// The schema cache is optional: a read-only home directory should cost
	// completion, not the whole session.
	var cache *catalog.Cache
	if path, err := catalog.DefaultCachePath(); err == nil {
		if opened, err := catalog.OpenCache(path); err == nil {
			cache = opened
			defer cache.Close()
		} else {
			fmt.Fprintf(os.Stderr, "completion disabled: %v\n", err)
		}
	}

	// History is optional for the same reason as the cache.
	var hist *history.Store
	if path, err := history.DefaultPath(); err == nil {
		if opened, err := history.Open(path); err == nil {
			hist = opened
			defer hist.Close()
		}
	}

	// The list of directories attached before, optional for the same reason.
	var recents *recent.List
	if path, err := recent.DefaultPath(); err == nil {
		if opened, err := recent.Open(path); err == nil {
			recents = opened
		}
	}

	// GitLab borrows its credential from glab rather than keeping one of its
	// own. Nothing runs here: the lookup reads the OS keyring and costs half a
	// second, so it happens the first time the GitLab command is used.
	glCLI := glab.New()
	openGitLab := func(ctx context.Context, host string) (ui.GitLabSource, error) {
		token, err := glCLI.Token(ctx, host)
		if err != nil {
			return nil, err
		}
		return gitlab.New("https://"+host, token), nil
	}

	// The worktree is optional in the same way: a path that no longer exists —
	// a branch cleaned up since the command was last run — should cost the
	// file list, not the session the user is trying to start.
	var wt *worktree.Worktree
	if opt.WorkDir != "" {
		opened, err := worktree.Open(opt.WorkDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "no worktree attached: %v\n", err)
		} else {
			wt = opened
		}
	}

	sess, err := session.Open(ctx, ds, password)
	if err != nil {
		return err
	}
	defer sess.Close()

	return ui.New(sess.Conn, cfg, ui.Deps{
		Keys:       keys,
		Cache:      cache,
		History:    hist,
		Worktree:   wt,
		Recent:     recents,
		GitLab:     openGitLab,
		GitLabHost: cfg.GitLab.Host,
	}).Run()
}

// probe verifies reachability, raising the tunnel first when one is needed,
// so `dv check` tests the same path the interface will take.
func probe(ctx context.Context, ds *config.DataSource, password string) (string, error) {
	// History is optional for the same reason as the cache.
	var hist *history.Store
	if path, err := history.DefaultPath(); err == nil {
		if opened, err := history.Open(path); err == nil {
			hist = opened
			defer hist.Close()
		}
	}

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

func printGettingStarted(path string) {
	fmt.Fprintf(os.Stderr, `no configuration found at %s

create it with a datasource, for example:

  datasources:
    - name: local
      env: dev            # prod | stage | dev
      host: 127.0.0.1
      port: 3306
      user: root
      database: app_db

then store the password:

  dv auth local

`, path)
}
