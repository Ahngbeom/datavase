package tunnel

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/testssh"
	"golang.org/x/crypto/ssh"
)

func bastionConfig(t *testing.T, addr string) *config.Tunnel {
	t.Helper()

	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q) error = %v", addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("Atoi(%q) error = %v", portText, err)
	}
	return &config.Tunnel{Host: host, Port: port, User: "tester"}
}

func openTunnel(t *testing.T, srv *testssh.Server, remote string) *Tunnel {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tun, err := Open(ctx, Options{
		Bastion:         bastionConfig(t, srv.Addr),
		Remote:          remote,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(testssh.Signer(t))},
		HostKeyCallback: ssh.FixedHostKey(srv.HostKey),
	})
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	t.Cleanup(func() { tun.Close() })
	return tun
}

func TestTunnelCarriesDataToTheRemoteService(t *testing.T) {
	srv := testssh.Start(t)
	echo := testssh.Echo(t)
	tun := openTunnel(t, srv, echo)

	conn, err := net.DialTimeout("tcp", tun.LocalAddr(), 5*time.Second)
	if err != nil {
		t.Fatalf("dialling the tunnel: %v", err)
	}
	defer conn.Close()

	const message = "SELECT 1\n"
	if _, err := conn.Write([]byte(message)); err != nil {
		t.Fatalf("writing through the tunnel: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	got := make([]byte, len(message))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("reading through the tunnel: %v", err)
	}
	if string(got) != message {
		t.Errorf("read %q through the tunnel, want %q", got, message)
	}
}

// The listener must be reachable only from this machine; binding to every
// interface would expose the database to the whole network.
func TestTunnelListensOnLoopbackOnly(t *testing.T) {
	srv := testssh.Start(t)
	tun := openTunnel(t, srv, testssh.Echo(t))

	host, _, err := net.SplitHostPort(tun.LocalAddr())
	if err != nil {
		t.Fatalf("SplitHostPort(%q) error = %v", tun.LocalAddr(), err)
	}
	if host != "127.0.0.1" {
		t.Errorf("tunnel listening on %q, want 127.0.0.1", host)
	}
}

func TestTunnelSupportsSeveralConnections(t *testing.T) {
	srv := testssh.Start(t)
	echo := testssh.Echo(t)
	tun := openTunnel(t, srv, echo)

	// The query pool opens more than one connection, so the tunnel has to
	// multiplex rather than serve a single session.
	for i := 0; i < 4; i++ {
		conn, err := net.DialTimeout("tcp", tun.LocalAddr(), 5*time.Second)
		if err != nil {
			t.Fatalf("connection %d: %v", i, err)
		}
		defer conn.Close()

		want := []byte{byte('a' + i)}
		if _, err := conn.Write(want); err != nil {
			t.Fatalf("connection %d write: %v", i, err)
		}
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		got := make([]byte, 1)
		if _, err := io.ReadFull(conn, got); err != nil {
			t.Fatalf("connection %d read: %v", i, err)
		}
		if got[0] != want[0] {
			t.Errorf("connection %d echoed %q, want %q", i, got, want)
		}
	}
}

// A forwarding failure happens on a background goroutine; without Err the
// interface would show an unexplained connection refusal.
func TestTunnelReportsForwardingFailures(t *testing.T) {
	srv := testssh.StartRejecting(t)
	tun := openTunnel(t, srv, testssh.Echo(t))

	conn, err := net.DialTimeout("tcp", tun.LocalAddr(), 5*time.Second)
	if err != nil {
		t.Fatalf("dialling the tunnel: %v", err)
	}
	defer conn.Close()

	// The local side accepts, then closes once forwarding fails.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	io.ReadAll(conn)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if tun.Err() != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("Err() = nil after a failed forward, want an error")
}

// A wrong host key means something is impersonating the bastion; the tunnel
// must refuse rather than carry credentials to it.
func TestTunnelRefusesAnUnexpectedHostKey(t *testing.T) {
	srv := testssh.Start(t)
	other := testssh.Signer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := Open(ctx, Options{
		Bastion:         bastionConfig(t, srv.Addr),
		Remote:          testssh.Echo(t),
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(testssh.Signer(t))},
		HostKeyCallback: ssh.FixedHostKey(other.PublicKey()),
	})
	if err == nil {
		t.Fatal("Open() error = nil, want a host key mismatch")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "host key") {
		t.Errorf("Open() error = %v, want it to name the host key mismatch", err)
	}
}

// Omitting verification must be impossible by accident.
func TestOpenRefusesWithoutAHostKeyCallback(t *testing.T) {
	srv := testssh.Start(t)

	_, err := Open(context.Background(), Options{
		Bastion: bastionConfig(t, srv.Addr),
		Remote:  testssh.Echo(t),
		Auth:    []ssh.AuthMethod{ssh.PublicKeys(testssh.Signer(t))},
	})
	if err == nil {
		t.Fatal("Open() error = nil, want a refusal to skip verification")
	}
	if !strings.Contains(err.Error(), "verif") {
		t.Errorf("Open() error = %v, want it to explain why verification is required", err)
	}
}

func TestOpenRequiresAuthentication(t *testing.T) {
	srv := testssh.Start(t)

	_, err := Open(context.Background(), Options{
		Bastion:         bastionConfig(t, srv.Addr),
		Remote:          testssh.Echo(t),
		HostKeyCallback: ssh.FixedHostKey(srv.HostKey),
	})
	if err == nil {
		t.Fatal("Open() error = nil, want an error about missing authentication")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	srv := testssh.Start(t)
	tun := openTunnel(t, srv, testssh.Echo(t))

	if err := tun.Close(); err != nil {
		t.Errorf("first Close() = %v, want nil", err)
	}
	if err := tun.Close(); err != nil {
		t.Errorf("second Close() = %v, want nil", err)
	}
}

func TestOpenFailsPromptlyForAnUnreachableBastion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := Open(ctx, Options{
		// Port 1 on loopback: nothing listens there.
		Bastion:         &config.Tunnel{Host: "127.0.0.1", Port: 1, User: "tester"},
		Remote:          "127.0.0.1:3306",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(testssh.Signer(t))},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err == nil {
		t.Fatal("Open() error = nil, want a connection error")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Error("Open() hit the context deadline instead of failing on connect")
	}
}
