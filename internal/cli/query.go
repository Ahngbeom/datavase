package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/headless"
	"github.com/Ahngbeom/datavase/internal/secret"
)

// DefaultQueryTimeout is how long `dv query` lets a statement run when no
// -timeout is given.
const DefaultQueryTimeout = 30 * time.Second

// MaxQueryBytes bounds the statement read from stdin. A query someone wrote
// is far shorter; anything this long is a file piped in by mistake.
const MaxQueryBytes = 1 << 20

// query runs one read statement from stdin and writes the result as JSON
// lines, for a program rather than a person.
func (a *App) query(args []string) int {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	maxRows := fs.Int("max-rows", a.Config.Defaults.AutoLimit, "most rows to write")
	timeout := fs.Duration("timeout", DefaultQueryTimeout, "how long the statement may run before the server is asked to end it")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	if fs.NArg() != 1 {
		fmt.Fprint(a.Err, "usage: dv query [-max-rows N] [-timeout D] <datasource>  (the statement on stdin)\n")
		return exitUsage
	}
	if *maxRows <= 0 || *maxRows > a.Config.Defaults.BufferMax {
		fmt.Fprintf(a.Err, "-max-rows must be between 1 and %d (buffer_max)\n", a.Config.Defaults.BufferMax)
		return exitUsage
	}
	if *timeout <= 0 {
		fmt.Fprint(a.Err, "-timeout must be positive\n")
		return exitUsage
	}

	ds, err := a.Config.Find(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitError
	}

	// Before stdin is read, so a mistaken datasource costs nothing: without
	// read_only there is no server-side refusal behind the checks below.
	if !ds.ReadOnly {
		fmt.Fprintf(a.Err, "refused: dv query runs only on a datasource with read_only: true, and %q is not one\n", ds.Name)
		return exitRefused
	}

	src, err := io.ReadAll(io.LimitReader(a.In, MaxQueryBytes+1))
	if err != nil {
		fmt.Fprintf(a.Err, "reading the statement: %v\n", err)
		return exitError
	}
	if len(src) > MaxQueryBytes {
		fmt.Fprintf(a.Err, "refused: the statement is longer than %d bytes\n", MaxQueryBytes)
		return exitRefused
	}

	stmt, err := headless.Admit(string(src))
	if err != nil {
		fmt.Fprintf(a.Err, "refused: %v\n", err)
		return exitRefused
	}

	password, err := a.Secrets.Get(ds.Name)
	if errors.Is(err, secret.ErrNotFound) {
		// The variable first: a machine running dv for a program is the one
		// with no keychain.
		fmt.Fprintf(a.Err, "no password for %q; set %s, or run: dv auth %s\n",
			ds.Name, secret.EnvVarName(ds.Name), ds.Name)
		return exitError
	}
	if err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitError
	}

	err = a.Query(context.Background(), ds, password, stmt,
		headless.Limits{MaxRows: *maxRows, Timeout: *timeout}, a.Out)

	var refused *headless.Refusal
	switch {
	case err == nil:
		return exitOK
	case errors.As(err, &refused), db.IsReadOnlyRefusal(err):
		fmt.Fprintf(a.Err, "refused: %v\n", err)
		return exitRefused
	default:
		fmt.Fprintf(a.Err, "%v\n", err)
		if hint := db.Diagnose(err).Hint(); hint != "" {
			fmt.Fprintf(a.Err, "%s\n", hint)
		}
		return exitError
	}
}
