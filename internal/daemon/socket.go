package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"github.com/Ahngbeom/datavase/internal/snapshot"
)

// ErrAlreadyRunning reports that something answered on the socket already.
var ErrAlreadyRunning = errors.New("a dv server is already listening")

// SocketPath is where a client looks for a server.
//
// Beside the schema cache and the query history rather than beside the
// configuration file: this is runtime state, and it means nothing once the
// process behind it is gone.
func SocketPath() (string, error) { return statePath("dv.sock") }

// LogPath is where a spawned server writes what it could not say to anyone.
//
// A server that dies while starting has no client to tell, and without this
// the failure is invisible.
func LogPath() (string, error) { return statePath("server.log") }

func statePath(name string) (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "datavase", name), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "datavase", name), nil
}

// Dial connects to a running server.
func Dial(path string) (net.Conn, error) { return net.Dial("unix", path) }

// Listen takes the socket, replacing one that nothing answers on.
func Listen(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}

	// A socket file is not a running server. A crash leaves one behind, and
	// connecting is the only thing that can tell the difference — so connect,
	// and take a refusal as permission to replace it. Testing for the file
	// instead would mean one crash costs every later session.
	if c, err := Dial(path); err == nil {
		_ = c.Close()
		return nil, ErrAlreadyRunning
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	// The umask decides the mode a socket is created with, so it is set
	// afterwards rather than hoped for. This is a way into a session that can
	// read production databases.
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}
	return ln, nil
}

// Accept serves clients until the listener closes.
func (s *Server) Accept(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go s.Serve(c)
	}
}

// APISocketPath is where the observation socket lives.
//
// A second socket rather than a second message on the first one: the client
// protocol is private, binary and high-frequency, and this is public,
// textual and rare. One socket carrying both would make each worse, and only
// this one may have several readers at a time.
func APISocketPath() (string, error) { return statePath("dv-api.sock") }

// ServeAPI answers snapshot requests until the listener closes.
//
// One response per connection: there is one question, and a connection that
// asked it has nothing more to say. Readers are unlimited because none of
// them can change anything.
func ServeAPI(ln net.Listener, src snapshot.Source) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			ctx, cancel := context.WithTimeout(context.Background(), snapshot.SessionTimeout)
			defer cancel()
			_ = snapshot.Handle(c, src, ctx)
		}(c)
	}
}
