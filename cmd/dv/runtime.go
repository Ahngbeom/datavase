package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/Ahngbeom/datavase/internal/attach"
	"github.com/Ahngbeom/datavase/internal/cli"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/daemon"
	"github.com/Ahngbeom/datavase/internal/proto"
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
func attachSession(_ context.Context, ds *config.DataSource, cfg *config.Config, opt cli.UIOptions) error {
	path, err := daemon.SocketPath()
	if err != nil {
		return fallback(ds, cfg, opt, err)
	}

	conn, err := daemon.Dial(path)
	if err != nil {
		if err := spawnServer(); err != nil {
			return fallback(ds, cfg, opt, err)
		}
		conn, err = waitForSocket(path)
		if err != nil {
			return fallback(ds, cfg, opt, err)
		}
	}

	return attach.Run(conn, attach.Options{
		Version:    version.String(),
		WorkDir:    opt.WorkDir,
		DataSource: ds.Name,
		Err:        os.Stderr,
	})
}

// fallback runs monolithically and says why, once.
func fallback(ds *config.DataSource, cfg *config.Config, opt cli.UIOptions, cause error) error {
	fmt.Fprintf(os.Stderr, "no session server (%v); this session ends with the terminal\n", cause)

	password, err := secrets().Get(ds.Name)
	if err != nil {
		return fmt.Errorf("no password for %q; run: dv auth %s", ds.Name, ds.Name)
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

func spawnServer() error {
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

	cmd := exec.Command(exe, "server")
	cmd.Stdout, cmd.Stderr = logFile, logFile
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

	var srv *daemon.Server
	srv = daemon.New(daemon.Options{
		Version: version.String(),
		Start: func(h proto.Hello) (daemon.Session, []string, error) {
			return buildSession(h, cfg, srv)
		},
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

// serverStatus is `dv status`.
func serverStatus() (string, error) {
	path, err := daemon.SocketPath()
	if err != nil {
		return "", err
	}
	conn, err := daemon.Dial(path)
	if err != nil {
		return "no dv server is running", nil
	}
	conn.Close()
	return fmt.Sprintf("a dv server is running (%s)", path), nil
}
