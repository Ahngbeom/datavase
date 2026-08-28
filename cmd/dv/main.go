// Command dv is datavase's entry point: a terminal MySQL client.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/cli"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/daemon"
	"github.com/Ahngbeom/datavase/internal/history"
	"github.com/Ahngbeom/datavase/internal/intro"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/Ahngbeom/datavase/internal/proto"
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
	noSession := flag.Bool("no-session", false, "run without a session server")
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

	// `dv init` is dispatched here rather than by cli.App for the same reason
	// `dv version` is: App holds a *config.Config, and the whole point of this
	// command is that there is not one yet. The parsing lives in cli so that
	// it is checkable without a terminal.
	if wanted, code := cli.HandleInit(os.Stderr, flag.Args()); wanted {
		if code != 0 {
			return code
		}
		return runWizard(path)
	}

	cfg, err := config.Load(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}

		// Nothing is configured yet. Someone sitting at a terminal can answer
		// the questions now, which is the whole difference between "datavase
		// does not work" and a session. A pipe cannot answer them, and blocking
		// on one that never will is worse than saying what to run.
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			printGettingStarted(path)
			return 1
		}
		if code := runWizard(path); code != 0 {
			return code
		}
		if cfg, err = config.Load(path); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
	}

	app := &cli.App{
		Config:       cfg,
		Secrets:      secrets(),
		Out:          os.Stdout,
		Err:          os.Stderr,
		ReadPassword: readPassword,
		Probe:        probe,
		OpenUI:       openUI,
		RunServer:    func() error { return runServer(cfg) },
		StopServer:   stopServer,
		ServerStatus: serverStatus,
	}
	if !*noSession {
		// The resolved path, not the flag as typed: a server spawned from
		// here is told which file to read rather than working it out again,
		// so a datasource name cannot mean one host on this side of the
		// socket and another host on the other.
		app.Attach = func(ctx context.Context, ds *config.DataSource, cfg *config.Config, opt cli.UIOptions) error {
			return attachSession(ctx, ds, cfg, opt, path)
		}
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

	// Whether the first-run card has been shown. Optional for the same reason:
	// a state directory that cannot be written costs the card being shown once
	// more, not the session.
	var introPath string
	if path, err := intro.DefaultPath(); err == nil {
		introPath = path
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

	// The interface owns the session from here: switching datasource closes
	// the one it leaves, and a session closed twice or by nobody is how a
	// tunnel outlives the thing it was carrying.
	return ui.New(sess, cfg, ui.Deps{
		Keys:      keys,
		Cache:     cache,
		History:   hist,
		Worktree:  wt,
		Recent:    recents,
		IntroPath: introPath,
		Connect:   connectTo,
	}).Run()
}

// sessionAdapter makes *ui.App satisfy daemon.Stateful.
//
// ui.App.State returns ui.RuntimeState, a type internal/ui owns so that
// package never has to import internal/daemon — the two are independent on
// purpose, App is built and tested before internal/daemon exists.
// daemon.Stateful requires daemon.State by name, and Go's interface
// satisfaction is exact on return types: a *ui.App handed to the daemon
// directly does not satisfy Stateful, and session.(Stateful) fails silently
// at runtime. The daemon then treats every session as busy — indistinguishable
// from a broken build, since nothing refuses to compile. This adapter is the
// one place that conversion belongs: cmd/dv already imports both packages to
// wire them together.
//
// SetScreen, Run, Stop and SwitchTo are unaffected — SwitchTo's signature
// matches daemon.Switcher exactly, so embedding satisfies it without a method
// here.
type sessionAdapter struct{ *ui.App }

func (a sessionAdapter) State(ctx context.Context) (daemon.State, error) {
	s, err := a.App.State(ctx)
	return daemon.State{DataSource: s.DataSource, Busy: s.Busy}, err
}

// statefulSession is what closingSession wraps: sessionAdapter satisfies it,
// and so does any fake standing in for one in a test — a named interface
// rather than embedding sessionAdapter's concrete *ui.App is what makes
// closingSession testable without a terminal or a database behind it.
type statefulSession interface {
	daemon.Session
	daemon.Stateful
	daemon.Switcher
}

// closingSession closes the SQLite handles buildSession opened, once the
// session that used them actually stops running.
//
// openUI can defer Close inside the function that calls Run: Run there is
// synchronous, so the defers fire when the terminal exits. buildSession
// cannot do that — Run executes later, on a goroutine daemon.Server.admit
// starts, so a defer here would close the cache and history out from under
// a session that had not yet used them. Closing instead belongs to the one
// moment that means "this session stopped needing them": Run returning.
// Without this, the handles stayed open for the life of the dv server
// process, reclaimed only by exit.
type closingSession struct {
	statefulSession
	closers []io.Closer
}

func (s closingSession) Run() error {
	// Deferred rather than sequential: a panic out of the interface would
	// otherwise leave the SQLite handles open, and the server process running
	// this session outlives any one session.
	defer func() {
		for _, c := range s.closers {
			c.Close()
		}
	}()
	return s.statefulSession.Run()
}

// buildSession is what the server calls when the first client arrives.
//
// It is openUI's wiring with two differences. The warnings go back to the
// caller instead of to stderr, because in this process stderr is a log file
// nobody reads. And the password comes from the keychain and nowhere else:
// the terminal that could answer a prompt is in another process, which is the
// same reason a mid-session switch has never prompted.
func buildSession(ctx context.Context, h proto.Hello, cfg *config.Config, srv *daemon.Server) (daemon.Session, []string, error) {
	ds, err := lookupDataSource(cfg, h.DataSource)
	if err != nil {
		return nil, nil, err
	}

	password, err := secrets().Get(ds.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("no password for %q; run: dv auth %s — or set %s",
			ds.Name, ds.Name, secret.EnvVarName(ds.Name))
	}

	keys, err := keymap.FromConfig(cfg.Keymap.Preset, cfg.Keymap.Actions)
	if err != nil {
		return nil, nil, fmt.Errorf("keymap: %w", err)
	}

	var warnings []string

	// The schema cache is optional: a read-only home directory should cost
	// completion, not the whole session.
	var cache *catalog.Cache
	if path, err := catalog.DefaultCachePath(); err == nil {
		if opened, err := catalog.OpenCache(path); err == nil {
			cache = opened
		} else {
			warnings = append(warnings, fmt.Sprintf("completion disabled: %v", err))
		}
	}

	// History is optional for the same reason as the cache.
	var hist *history.Store
	if path, err := history.DefaultPath(); err == nil {
		if opened, err := history.Open(path); err == nil {
			hist = opened
		}
	}

	// The list of directories attached before, optional for the same reason.
	var recents *recent.List
	if path, err := recent.DefaultPath(); err == nil {
		if opened, err := recent.Open(path); err == nil {
			recents = opened
		}
	}

	// Whether the first-run card has been shown. Optional for the same reason:
	// a state directory that cannot be written costs the card being shown once
	// more, not the session.
	var introPath string
	if path, err := intro.DefaultPath(); err == nil {
		introPath = path
	}

	// The worktree is optional in the same way: a path that no longer exists —
	// a branch cleaned up since the command was last run — should cost the
	// file list, not the session the client is trying to start.
	var wt *worktree.Worktree
	if h.WorkDir != "" {
		opened, err := worktree.Open(h.WorkDir)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("no worktree attached: %v", err))
		} else {
			wt = opened
		}
	}

	// The context bounds the connection attempt only, the way openUI's does.
	// Without one an unreachable host would hang here forever, and this call
	// runs under the server's admit lock: every later attach, for any
	// datasource, would queue behind it until the process was stopped by hand.
	sess, err := session.Open(ctx, ds, password)
	if err != nil {
		return nil, nil, err
	}

	// cache and hist are each individually optional, exactly like openUI's —
	// only the ones that actually opened need closing.
	var closers []io.Closer
	if cache != nil {
		closers = append(closers, cache)
	}
	if hist != nil {
		closers = append(closers, hist)
	}

	return closingSession{
		statefulSession: sessionAdapter{ui.New(sess, cfg, ui.Deps{
			Keys:      keys,
			Cache:     cache,
			History:   hist,
			Worktree:  wt,
			Recent:    recents,
			IntroPath: introPath,
			Connect:   connectTo,
			Detach:    srv.Detach,
		})},
		closers: closers,
	}, warnings, nil
}

// lookupDataSource resolves the name a client asked for, defaulting to the
// first configured datasource when it asked for none.
func lookupDataSource(cfg *config.Config, name string) (*config.DataSource, error) {
	if len(cfg.DataSources) == 0 {
		return nil, errors.New("no datasources are configured; run: dv init")
	}
	if name == "" {
		return &cfg.DataSources[0], nil
	}
	for i := range cfg.DataSources {
		if cfg.DataSources[i].Name == name {
			return &cfg.DataSources[i], nil
		}
	}
	return nil, fmt.Errorf("no datasource named %q", name)
}

// secrets is where every password is read and written.
//
// One function rather than three constructions, so the environment layer
// cannot end up on the interface's reads but not the wizard's writes — which
// would show up as a datasource that works until you switch to it.
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

// runWizard writes the first configuration file.
//
// Everything it says goes to stderr, including what it managed to do: the
// wizard is a conversation, and a conversation in the middle of `dv init >
// somewhere` would end up in the file rather than in front of the user.
func runWizard(path string) int {
	w := &cli.Wizard{
		Path:         path,
		Out:          os.Stderr,
		Ask:          ask,
		Choose:       choose,
		ReadPassword: readPassword,
		Secrets:      secrets(),
		// The timeout bounds one attempt, not the wizard: a deadline over the
		// whole thing would expire while someone was still typing.
		Probe: func(ctx context.Context, ds *config.DataSource, password string) (string, error) {
			ctx, cancel := context.WithTimeout(ctx, cli.CheckTimeout)
			defer cancel()
			return probe(ctx, ds, password)
		},
	}

	if err := w.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "\n%v\n", err)
		return 1
	}
	return 0
}

// stdin is shared by every question. A fresh reader per prompt would discard
// whatever the last one had already buffered, which on a fast paste is the
// next answer.
//
// readPassword reads the descriptor directly rather than through this, and the
// two only coexist because of what a terminal in its ordinary line-editing mode
// does: a read returns at most one line, so this never buffers past the answer
// it was asked for, and there is nothing left in it for term.ReadPassword to
// lose. Anything else is not a terminal, and readPassword refuses those
// outright rather than reading a password from a pipe.
var stdin = bufio.NewReader(os.Stdin)

// ask reads one line, taking def when the answer is empty.
func ask(prompt, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", prompt, def)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
	}

	line, err := stdin.ReadString('\n')
	// A final line without a newline is still an answer; only an empty read is
	// the end of the input.
	if err != nil && line == "" {
		return "", err
	}
	if trimmed := strings.TrimSpace(line); trimmed != "" {
		return trimmed, nil
	}
	return def, nil
}

// choose numbers the options and reads one back.
//
// Numbers rather than arrow keys: this runs before the terminal interface
// exists, and putting a raw-mode menu in front of the setup wizard would make
// the first thing a new user meets the thing most likely to render wrongly.
func choose(prompt string, options []string, def int) (int, error) {
	fmt.Fprintf(os.Stderr, "\n%s\n", prompt)
	for i, option := range options {
		marker := " "
		if i == def {
			marker = "*"
		}
		fmt.Fprintf(os.Stderr, "  %s %d) %s\n", marker, i+1, option)
	}

	// A negative default is a question with no answer to fall back on: nothing
	// is marked, nothing is offered in brackets, and Enter on its own asks
	// again rather than resolving to something nobody picked.
	fallback := ""
	if def >= 0 {
		fallback = strconv.Itoa(def + 1)
	}

	for {
		answer, err := ask("choice", fallback)
		if err != nil {
			return 0, err
		}

		n, convErr := strconv.Atoi(answer)
		if convErr == nil && n >= 1 && n <= len(options) {
			return n - 1, nil
		}
		fmt.Fprintf(os.Stderr, "choose a number between 1 and %d.\n", len(options))
	}
}

// printGettingStarted is what a machine that cannot answer the wizard is told.
//
// It names the command rather than printing a configuration file: the file is
// what `dv init` is for, and an example someone retypes is an example they can
// get past the parser only by luck.
func printGettingStarted(path string) {
	fmt.Fprintf(os.Stderr, `no configuration found at %s

run this at a terminal and it will ask for what goes in it:

  dv init

`, path)
}
