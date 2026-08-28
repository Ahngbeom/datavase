package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Ahngbeom/datavase/internal/attach"
	"github.com/Ahngbeom/datavase/internal/cli"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/daemon"
	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/Ahngbeom/datavase/internal/snapshot"
	"github.com/Ahngbeom/datavase/internal/version"
)

// spawnWait bounds how long a freshly started server has to take its socket
// before the client gives up and runs on its own.
const spawnWait = 3 * time.Second

// attachSession reaches a running session, starting a server if there is
// none.
//
// Nothing here is allowed to cost the session. A socket that cannot be made,
// a server that will not start, a machine where none of this works at all —
// each of those costs persistence, says one line about it, and runs the way
// dv always did.
//
// The context is not used: it bounds a connection attempt, and the connection
// is opened in the server process, which bounds it there. Applying it here
// would put a deadline on the whole attached session instead.
func attachSession(_ context.Context, ds *config.DataSource, cfg *config.Config, opt cli.UIOptions, configPath string) error {
	path, err := daemon.SocketPath()
	if err != nil {
		return fallback(ds, cfg, opt, err)
	}

	conn, err := daemon.Dial(path)
	if err != nil {
		if err := spawnServer(configPath); err != nil {
			return fallback(ds, cfg, opt, err)
		}
		conn, err = waitForSocket(path)
		if err != nil {
			return fallback(ds, cfg, opt, err)
		}
	}

	return attach.Run(conn, attach.Options{
		Version:    version.String(),
		WorkDir:    absWorkDir(opt.WorkDir),
		DataSource: ds.Name,
		Err:        os.Stderr,
	})
}

// absWorkDir resolves --dir here, in the process that knows what it was
// relative to.
//
// The server opens the worktree it is sent, and a relative path there is
// resolved against the server's own working directory — inherited from
// whichever dv happened to start it and never changed since. So `dv --dir .`
// would silently attach that directory instead of this one, with no warning,
// because "." always exists.
//
// A working directory that cannot be read is not worth losing the session
// over: the worktree is optional, and the path goes on unresolved so the
// server's own "no worktree attached" warning is what the user sees.
func absWorkDir(dir string) string {
	if dir == "" {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

// fallback runs monolithically and says why, once.
func fallback(ds *config.DataSource, cfg *config.Config, opt cli.UIOptions, cause error) error {
	fmt.Fprintf(os.Stderr, "no session server (%v); this session ends with the terminal\n", cause)

	password, err := secrets().Get(ds.Name)
	if err != nil {
		// Both ways out, the same as every other path that reads a password:
		// a machine with no keychain is exactly the machine most likely to
		// have got here.
		return fmt.Errorf("no password for %q; run: dv auth %s — or set %s",
			ds.Name, ds.Name, secret.EnvVarName(ds.Name))
	}
	return openUI(context.Background(), ds, password, cfg, opt)
}

func waitForSocket(path string) (net.Conn, error) {
	deadline := time.Now().Add(spawnWait)
	for {
		conn, err := daemon.Dial(path)
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			logPath, _ := daemon.LogPath()
			return nil, fmt.Errorf("the server did not start; see %s", logPath)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func spawnServer(configPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	logPath, err := daemon.LogPath()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()

	// The config file is named rather than left to be resolved again: the
	// server must read the same one this client did, or `dv -c other.yaml`
	// starts a server that looks a datasource name up somewhere else.
	cmd := exec.Command(exe, "-c", configPath, "server")
	cmd.Stdout, cmd.Stderr = logFile, logFile
	// Stdin stays nil, which os/exec gives the null device — this process
	// must never be able to read from the terminal that started it.
	detach(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// runServer is `dv server`: it holds the session until the session ends.
func runServer(cfg *config.Config) error {
	path, err := daemon.SocketPath()
	if err != nil {
		return err
	}

	ln, err := daemon.Listen(path)
	if err != nil {
		if errors.Is(err, daemon.ErrAlreadyRunning) {
			return errors.New("a dv server is already running")
		}
		return err
	}
	defer os.Remove(path)
	defer ln.Close()

	apiPath, err := daemon.APISocketPath()
	if err != nil {
		return err
	}
	apiLn, err := daemon.Listen(apiPath)
	if err != nil {
		return err
	}
	defer os.Remove(apiPath)
	defer apiLn.Close()

	var srv *daemon.Server
	srv = daemon.New(daemon.Options{
		Version: version.String(),
		Start: func(ctx context.Context, h proto.Hello) (daemon.Session, []string, error) {
			return buildSession(ctx, h, cfg, srv)
		},
	})

	started := time.Now()
	go daemon.ServeAPI(apiLn, snapshot.Source{
		Server:  func() snapshot.Server { return srv.Info(version.String(), started, time.Now()) },
		Session: sessionSnapshot,
	})

	go srv.Accept(ln)
	return srv.Wait()
}

// stopServer is `dv server stop`.
func stopServer() error {
	path, err := daemon.SocketPath()
	if err != nil {
		return err
	}
	conn, err := daemon.Dial(path)
	if err != nil {
		return errors.New("no dv server is running")
	}
	defer conn.Close()

	return proto.NewEncoder(conn).ToServer(proto.ToServer{Kind: proto.KindStop})
}

// apiSnapshot asks the running server what it is doing.
func apiSnapshot() ([]byte, error) {
	path, err := daemon.APISocketPath()
	if err != nil {
		return nil, err
	}
	conn, err := daemon.Dial(path)
	if err != nil {
		return nil, errors.New("no dv server is running")
	}
	defer conn.Close()

	return io.ReadAll(conn)
}

// serverStatus is dv status: one paragraph for a person, from the same
// snapshot the API serves.
func serverStatus() (string, error) {
	raw, err := apiSnapshot()
	if err != nil {
		return "no dv server is running", nil
	}

	var s snapshot.Snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("the server answered with something unreadable: %w", err)
	}

	if s.Session == nil {
		return fmt.Sprintf("a dv server is running (pid %d), but the session did not answer: %s",
			s.Server.PID, s.SessionError), nil
	}

	line := fmt.Sprintf("%s on %s (%s), up %ds",
		s.Session.DataSource.Name, s.Session.DataSource.Host,
		s.Session.DataSource.Env, s.Server.UptimeSeconds)
	if !s.Server.ClientAttached {
		line += ", detached"
	}
	if s.Session.Statement.Running {
		line += fmt.Sprintf("\nrunning for %dms: %s",
			s.Session.Statement.ElapsedMS, s.Session.Statement.SQL)
	}
	return line, nil
}

// sessionSnapshot reaches the *ui.App the server built, for the observation
// socket to ask.
func sessionSnapshot(ctx context.Context) (*snapshot.Session, error) {
	liveMu.Lock()
	app := live
	liveMu.Unlock()

	if app == nil {
		return nil, nil
	}
	return app.Snapshot(ctx)
}
