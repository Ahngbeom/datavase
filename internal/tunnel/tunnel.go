// Package tunnel forwards a local port to a database through an SSH bastion.
//
// Authentication and host-key checking are supplied by the caller rather than
// constructed here, so the forwarding logic can be tested against an
// in-process SSH server. The production defaults live in AgentAuth and
// KnownHostsCallback.
package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// DialTimeout bounds the SSH handshake so an unreachable bastion fails
// promptly instead of hanging the interface.
const DialTimeout = 15 * time.Second

// Options describe one tunnel.
type Options struct {
	// Bastion is the SSH host to connect through.
	Bastion *config.Tunnel
	// Remote is the address to reach from the bastion, as host:port.
	Remote string
	// Auth lists authentication methods, in order of preference.
	Auth []ssh.AuthMethod
	// HostKeyCallback verifies the bastion's identity. It must not be nil:
	// an unverified bastion can read every credential that crosses it.
	HostKeyCallback ssh.HostKeyCallback
}

// Tunnel is a running local listener forwarding to Remote via the bastion.
type Tunnel struct {
	listener net.Listener
	client   *ssh.Client
	remote   string

	closeOnce sync.Once
	wg        sync.WaitGroup

	mu   sync.Mutex
	errs []error
}

// Open dials the bastion and starts a local listener.
//
// The listener binds to loopback only. Binding to all interfaces would let
// anything on the network reach the database through this process.
func Open(ctx context.Context, opt Options) (*Tunnel, error) {
	if opt.Bastion == nil {
		return nil, errors.New("no bastion configured")
	}
	if opt.HostKeyCallback == nil {
		return nil, errors.New("no host key callback: refusing to connect without verifying the bastion")
	}
	if len(opt.Auth) == 0 {
		return nil, errors.New("no SSH authentication methods available")
	}

	addr := net.JoinHostPort(opt.Bastion.Host, strconv.Itoa(opt.Bastion.Port))

	// A plain net.Dial honours the context; ssh.Dial does not.
	dialer := &net.Dialer{Timeout: DialTimeout}
	rawConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connecting to bastion %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(rawConn, addr, &ssh.ClientConfig{
		User:            opt.Bastion.User,
		Auth:            opt.Auth,
		HostKeyCallback: opt.HostKeyCallback,
		Timeout:         DialTimeout,
	})
	if err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("authenticating to bastion %s: %w", addr, err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("opening a local listener: %w", err)
	}

	t := &Tunnel{listener: listener, client: client, remote: opt.Remote}
	t.wg.Add(1)
	go t.accept()

	return t, nil
}

// LocalAddr is the address the database driver should dial.
func (t *Tunnel) LocalAddr() string {
	return t.listener.Addr().String()
}

// Err returns the first forwarding failure, if any. Forwarding errors happen
// on background goroutines, so this is how the interface learns that a
// tunnel has gone bad rather than merely idle.
func (t *Tunnel) Err() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.errs) == 0 {
		return nil
	}
	return t.errs[0]
}

// Close stops the listener and the SSH connection.
func (t *Tunnel) Close() error {
	t.closeOnce.Do(func() {
		t.listener.Close()
		t.client.Close()
		t.wg.Wait()
	})
	return nil
}

func (t *Tunnel) accept() {
	defer t.wg.Done()

	for {
		local, err := t.listener.Accept()
		if err != nil {
			// A closed listener is the normal shutdown path.
			return
		}

		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			t.forward(local)
		}()
	}
}

// forward joins one local connection to a channel opened on the bastion.
func (t *Tunnel) forward(local net.Conn) {
	defer local.Close()

	remote, err := t.client.Dial("tcp", t.remote)
	if err != nil {
		t.record(fmt.Errorf("reaching %s through the bastion: %w", t.remote, err))
		return
	}
	defer remote.Close()

	// Copy in both directions and stop as soon as either side ends, so a
	// half-closed connection cannot leave a goroutine parked forever.
	done := make(chan struct{}, 2)
	go func() { io.Copy(remote, local); done <- struct{}{} }()
	go func() { io.Copy(local, remote); done <- struct{}{} }()
	<-done
}

func (t *Tunnel) record(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errs = append(t.errs, err)
}

// AgentAuth returns the authentication methods offered by the running
// ssh-agent.
//
// Delegating to the agent means datavase never reads a private key, and
// passphrase-protected keys work without prompting.
func AgentAuth() ([]ssh.AuthMethod, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, errors.New("SSH_AUTH_SOCK is not set: start ssh-agent and add your key with ssh-add")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("connecting to ssh-agent: %w", err)
	}

	client := agent.NewClient(conn)
	keys, err := client.List()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("listing keys from ssh-agent: %w", err)
	}
	if len(keys) == 0 {
		conn.Close()
		return nil, errors.New("ssh-agent holds no keys: add one with ssh-add")
	}

	return []ssh.AuthMethod{ssh.PublicKeysCallback(client.Signers)}, nil
}

// KnownHostsCallback verifies bastions against ~/.ssh/known_hosts.
//
// There is deliberately no "skip verification" option. This tunnel carries
// production database credentials, and an unverified bastion is one an
// attacker can impersonate to read all of them.
func KnownHostsCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("locating home directory: %w", err)
	}

	path := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf(
			"%s is not readable; connect once with ssh to record the bastion's key: %w", path, err)
	}

	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if err := callback(hostname, remote, key); err != nil {
			return fmt.Errorf(
				"%w\n\nif this bastion is new, record its key first:\n  ssh-keyscan -H %s >> %s",
				err, hostname, path)
		}
		return nil
	}, nil
}
