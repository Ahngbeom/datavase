//go:build integration

package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/db"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/Ahngbeom/datavase/internal/testssh"
	"golang.org/x/crypto/ssh"
)

// testOptions supply credentials for the in-process bastion, leaving the
// rest of Open's behaviour exactly as it runs in production.
func testOptions(t *testing.T, srv *testssh.Server) Options {
	t.Helper()

	return Options{
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(testssh.Signer(t))},
		HostKeyCallback: ssh.FixedHostKey(srv.HostKey),
	}
}

func TestOpenWithoutATunnel(t *testing.T) {
	ds, password := testmysql.DataSource(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := Open(ctx, ds, password)
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	defer sess.Close()

	if sess.Conn.ServerVersion() == "" {
		t.Error("ServerVersion() is empty")
	}
	if err := sess.TunnelErr(); err != nil {
		t.Errorf("TunnelErr() = %v, want nil when no tunnel is configured", err)
	}
}

// The tunnel and the driver are each tested on their own; this is the proof
// that a real MySQL session survives being carried over a forwarded channel.
func TestOpenThroughATunnelReachesTheDatabase(t *testing.T) {
	srv := testssh.Start(t)
	ds, password := testmysql.DataSource(t)
	ds.Tunnel = srv.Bastion(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sess, err := OpenWith(ctx, ds, password, testOptions(t, srv))
	if err != nil {
		t.Fatalf("OpenWith() error = %v, want nil", err)
	}
	defer sess.Close()

	// A streamed query exercises the forwarded connection well past the
	// handshake, which is where a half-wired tunnel would break.
	stream := sess.Conn.Query(ctx, "SELECT 1 AS n UNION ALL SELECT 2", db.Options{})
	defer stream.Close()

	rows := 0
	for ev := range stream.Events {
		if ev.Kind == db.EventRows {
			rows += len(ev.Rows)
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("streaming through the tunnel: %v", err)
	}
	if rows != 2 {
		t.Errorf("streamed %d rows through the tunnel, want 2", rows)
	}
}

// Cancellation has to survive the tunnel too: KILL QUERY travels on the
// control connection, which is forwarded over the same bastion.
func TestCancellationWorksThroughATunnel(t *testing.T) {
	srv := testssh.Start(t)
	ds, password := testmysql.DataSource(t)
	ds.Tunnel = srv.Bastion(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sess, err := OpenWith(ctx, ds, password, testOptions(t, srv))
	if err != nil {
		t.Fatalf("OpenWith() error = %v, want nil", err)
	}
	defer sess.Close()

	stream := sess.Conn.Query(context.Background(), "SELECT SLEEP(30)", db.Options{})
	defer stream.Close()

	if _, err := stream.WaitConnectionID(ctx); err != nil {
		t.Fatalf("WaitConnectionID() error = %v", err)
	}
	if err := stream.Cancel(); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}

	done := make(chan struct{})
	go func() {
		for range stream.Events {
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the stream did not end after Cancel(); the query is still running")
	}

	if stream.Err() == nil {
		t.Error("Err() = nil after Cancel(), want a cancellation error")
	}
}

// A bastion that refuses to forward must produce an error naming the
// bastion, not the driver's bare "connection reset".
func TestOpenThroughABrokenTunnelExplainsTheBastion(t *testing.T) {
	srv := testssh.StartRejecting(t)
	ds, password := testmysql.DataSource(t)
	ds.Tunnel = srv.Bastion(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := OpenWith(ctx, ds, password, testOptions(t, srv))
	if err == nil {
		t.Fatal("OpenWith() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), ds.Tunnel.Host) {
		t.Errorf("error = %v, want it to name the bastion", err)
	}
}

// Without an agent the error must say what to do; this is the first thing a
// new user hits when a datasource has a tunnel.
func TestOpenWithoutAnAgentExplainsHowToFix(t *testing.T) {
	srv := testssh.Start(t)
	ds, _ := testmysql.DataSource(t)
	ds.Tunnel = srv.Bastion(t)

	t.Setenv("SSH_AUTH_SOCK", "")

	_, err := Open(context.Background(), ds, "pw")
	if err == nil {
		t.Fatal("Open() error = nil, want an error about the missing agent")
	}
	if !strings.Contains(err.Error(), "ssh-add") {
		t.Errorf("error = %v, want it to suggest ssh-add", err)
	}
	if !strings.Contains(err.Error(), ds.Name) {
		t.Errorf("error = %v, want it to name the datasource", err)
	}
}

// Verification must never be skippable through the session layer either.
func TestOpenRefusesAnUnexpectedBastionKey(t *testing.T) {
	srv := testssh.Start(t)
	ds, password := testmysql.DataSource(t)
	ds.Tunnel = srv.Bastion(t)

	opt := testOptions(t, srv)
	opt.HostKeyCallback = ssh.FixedHostKey(testssh.Signer(t).PublicKey())

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := OpenWith(ctx, ds, password, opt); err == nil {
		t.Fatal("OpenWith() error = nil, want a host key mismatch")
	}
}

// A bastion that goes away mid-session is the failure the driver cannot
// describe: what it reports is a socket that has gone, which reads as the
// database being in trouble. TunnelErr is documented as how that becomes
// visible, so it has to actually become visible.
func TestABastionThatGoesAwayIsVisibleThroughTunnelErr(t *testing.T) {
	srv := testssh.Start(t)
	ds, password := testmysql.DataSource(t)
	ds.Tunnel = srv.Bastion(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sess, err := OpenWith(ctx, ds, password, testOptions(t, srv))
	if err != nil {
		t.Fatalf("OpenWith() error = %v, want nil", err)
	}
	defer sess.Close()

	// Working first, so that what follows is a session that broke rather than
	// one that never worked.
	if err := runOne(ctx, sess.Conn); err != nil {
		t.Fatalf("a statement failed before the bastion went: %v", err)
	}
	if err := sess.TunnelErr(); err != nil {
		t.Fatalf("TunnelErr() = %v while the bastion was up", err)
	}

	srv.Stop()

	// Statements now fail, and the driver will try a fresh connection first —
	// which is the attempt the tunnel records, because there is no longer a
	// bastion to forward it.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if runOne(ctx, sess.Conn) != nil && sess.TunnelErr() != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("the bastion is gone and TunnelErr() is still %v", sess.TunnelErr())
}

// runOne sends the smallest statement there is, and reports whether it landed.
func runOne(ctx context.Context, conn *db.Conn) error {
	stream := conn.Query(ctx, "SELECT 1", db.Options{})
	defer stream.Close()

	for range stream.Events {
	}
	return stream.Err()
}
