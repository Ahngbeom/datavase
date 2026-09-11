// Package cli implements datavase's command-line surface.
//
// App takes its dependencies as fields so every command can be exercised in
// tests without a terminal, a keychain or a database.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/Ahngbeom/datavase/internal/version"
)

// Exit codes. 2 is reserved for usage mistakes, matching the flag package.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

// App holds everything the commands need.
type App struct {
	Config  *config.Config
	Secrets secret.Store
	Out     io.Writer
	Err     io.Writer

	// ReadPassword prompts for a password without echoing it. It is a field
	// so tests can supply input without a terminal.
	ReadPassword func(prompt string) (string, error)

	// Probe opens a connection and returns the server version. It is a field
	// so the command can be tested without a database.
	Probe func(ctx context.Context, ds *config.DataSource, password string) (string, error)

	// OpenUI connects and runs the terminal interface. It is a field so the
	// dispatch logic can be tested without starting a terminal.
	OpenUI func(ctx context.Context, ds *config.DataSource, password string, cfg *config.Config) error

	// Launch shows the datasource list without a session and opens the
	// interface on whichever entry the user connects to. It is how a machine
	// with no configuration gets one.
	Launch func() error
}

// HandleVersion answers a request for the version, reporting whether it did.
//
// It is a free function rather than a command on App because it has to run
// before the configuration is read: someone checking which build they have
// should not first be told to write a config file. That also means the flag
// spellings never reach flag.Parse, which would reject them as undefined.
func HandleVersion(w io.Writer, args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(w, "dv %s\n", version.String())
		return true
	}
	return false
}

// Run dispatches args (excluding the program name) and returns an exit code.
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		return a.open("")
	}

	switch args[0] {
	case "open":
		return a.openCmd(args[1:])
	case "ls":
		return a.list()
	case "auth":
		return a.auth(args[1:])
	case "check":
		return a.check(args[1:])
	case "version", "--version", "-v":
		// Reached when -c came first: HandleVersion saw the flag, not the
		// word, and flag.Parse has since taken the flag away.
		HandleVersion(a.Out, args)
		return exitOK
	case "help", "-h", "--help":
		a.usage()
		return exitOK
	default:
		fmt.Fprintf(a.Err, "unknown command %q\n\n", args[0])
		a.usage()
		return exitUsage
	}
}

func (a *App) usage() {
	fmt.Fprint(a.Out, `datavase — terminal MySQL client

usage:
  dv                    open the interface; the datasource list unless exactly one is configured
  dv open <name>        open a named datasource
  dv ls                 list configured datasources
  dv auth <name>        store a datasource password in the keychain
  dv auth -rm <name>    remove a stored password
  dv check <name>       verify that the datasource is reachable
  dv version            print the version
  dv help               show this message
`)
}

func (a *App) list() int {
	for i := range a.Config.DataSources {
		ds := &a.Config.DataSources[i]
		stored := "no password"
		if _, err := a.Secrets.Get(ds.Name); err == nil {
			stored = "password stored"
		}
		fmt.Fprintf(a.Out, "%-16s %s:%d/%s  (%s)\n",
			ds.Name, ds.Host, ds.Port, ds.Database, stored)
	}
	return exitOK
}

// CheckTimeout bounds how long `dv check` waits before giving up.
const CheckTimeout = 15 * time.Second

func (a *App) openCmd(args []string) int {
	switch len(args) {
	case 0:
		return a.open("")
	case 1:
		return a.open(args[0])
	default:
		fmt.Fprintf(a.Err, "dv open takes one datasource name; got %q and %q\n", args[0], args[1])
		return exitUsage
	}
}

// open connects and hands control to the TUI.
//
// With no name given it picks the single configured datasource; with none or
// several it defers to Launch instead of guessing, which would risk opening
// production when dev was meant.
func (a *App) open(name string) int {
	if name == "" {
		if len(a.Config.DataSources) == 1 {
			name = a.Config.DataSources[0].Name
		} else {
			if err := a.Launch(); err != nil {
				fmt.Fprintln(a.Err, err)
				return exitError
			}
			return exitOK
		}
	}

	ds, err := a.Config.Find(name)
	if err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitError
	}

	ctx, cancel := context.WithTimeout(context.Background(), CheckTimeout)
	defer cancel()

	password, err := a.Secrets.Get(ds.Name)
	if errors.Is(err, secret.ErrNotFound) {
		fmt.Fprintf(a.Err, "no password stored for %q; run: dv auth %s\n", ds.Name, ds.Name)
		return exitError
	}
	if err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitError
	}

	if err := a.OpenUI(ctx, ds, password, a.Config); err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitError
	}
	return exitOK
}

func (a *App) check(args []string) int {
	if len(args) != 1 {
		fmt.Fprint(a.Err, "usage: dv check <datasource>\n")
		return exitUsage
	}

	ds, err := a.Config.Find(args[0])
	if err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitError
	}

	password, err := a.Secrets.Get(ds.Name)
	if errors.Is(err, secret.ErrNotFound) {
		// The driver would report this as "access denied", which sends
		// people looking for a server-side problem that isn't there.
		fmt.Fprintf(a.Err, "no password stored for %q; run: dv auth %s\n", ds.Name, ds.Name)
		return exitError
	}
	if err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitError
	}

	ctx, cancel := context.WithTimeout(context.Background(), CheckTimeout)
	defer cancel()

	version, err := a.Probe(ctx, ds, password)
	if err != nil {
		fmt.Fprintf(a.Err, "cannot reach %q: %v\n", ds.Name, err)
		return exitError
	}

	fmt.Fprintf(a.Out, "%s is reachable — server %s\n", ds.Name, version)
	return exitOK
}

func (a *App) auth(args []string) int {
	fs := flag.NewFlagSet("auth", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	remove := fs.Bool("rm", false, "remove the stored password instead of setting it")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	if fs.NArg() != 1 {
		fmt.Fprint(a.Err, "usage: dv auth [-rm] <datasource>\n")
		return exitUsage
	}
	name := fs.Arg(0)

	// Resolve the name first: storing a password for a datasource that does
	// not exist would leave an orphaned keychain entry.
	ds, err := a.Config.Find(name)
	if err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitError
	}

	if *remove {
		if err := a.Secrets.Delete(ds.Name); err != nil {
			fmt.Fprintf(a.Err, "%v\n", err)
			return exitError
		}
		fmt.Fprintf(a.Out, "removed the stored password for %q\n", ds.Name)
		return exitOK
	}

	password, err := a.ReadPassword(fmt.Sprintf("password for %s@%s: ", ds.User, ds.Name))
	if err != nil {
		fmt.Fprintf(a.Err, "reading password: %v\n", err)
		return exitError
	}

	if err := a.Secrets.Set(ds.Name, password); err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitError
	}
	fmt.Fprintf(a.Out, "stored the password for %q\n", ds.Name)
	return exitOK
}
