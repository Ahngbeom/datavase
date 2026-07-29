// Package testssh runs an in-process SSH server for tests.
//
// It honours direct-tcpip channels, which is the mechanism an SSH port
// forward actually uses, so tests can prove a tunnel moves bytes rather than
// merely that it dials.
package testssh

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net"
	"strconv"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"golang.org/x/crypto/ssh"
)

// Server is a running test SSH server.
type Server struct {
	Addr    string
	HostKey ssh.PublicKey

	rejectForward bool
}

// Start launches a server that forwards direct-tcpip channels.
func Start(t *testing.T) *Server {
	return start(t, false)
}

// StartRejecting launches a server that refuses every forwarding attempt,
// for exercising the failure path.
func StartRejecting(t *testing.T) *Server {
	return start(t, true)
}

func start(t *testing.T, rejectForward bool) *Server {
	t.Helper()

	signer := Signer(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	s := &Server{
		Addr:          listener.Addr().String(),
		HostKey:       signer.PublicKey(),
		rejectForward: rejectForward,
	}

	cfg := &ssh.ServerConfig{
		// Any key is accepted: these tests are about forwarding, not about
		// the authentication path.
		PublicKeyCallback: func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	cfg.AddHostKey(signer)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go s.handle(conn, cfg)
		}
	}()

	return s
}

// Bastion describes this server as a datasource tunnel configuration.
func (s *Server) Bastion(t *testing.T) *config.Tunnel {
	t.Helper()

	host, portText, err := net.SplitHostPort(s.Addr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q) error = %v", s.Addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("Atoi(%q) error = %v", portText, err)
	}
	return &config.Tunnel{Host: host, Port: port, User: "tester"}
}

func (s *Server) handle(raw net.Conn, cfg *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(raw, cfg)
	if err != nil {
		raw.Close()
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for ch := range chans {
		if ch.ChannelType() != "direct-tcpip" {
			ch.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		if s.rejectForward {
			ch.Reject(ssh.ConnectionFailed, "forwarding refused")
			continue
		}
		go forward(ch)
	}
}

func forward(newChannel ssh.NewChannel) {
	var payload struct {
		DestAddr string
		DestPort uint32
		OrigAddr string
		OrigPort uint32
	}
	if err := ssh.Unmarshal(newChannel.ExtraData(), &payload); err != nil {
		newChannel.Reject(ssh.ConnectionFailed, "bad payload")
		return
	}

	target, err := net.Dial("tcp", net.JoinHostPort(payload.DestAddr, strconv.Itoa(int(payload.DestPort))))
	if err != nil {
		newChannel.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	defer target.Close()

	channel, requests, err := newChannel.Accept()
	if err != nil {
		return
	}
	defer channel.Close()

	go ssh.DiscardRequests(requests)

	done := make(chan struct{}, 2)
	go func() { io.Copy(target, channel); done <- struct{}{} }()
	go func() { io.Copy(channel, target); done <- struct{}{} }()
	<-done
}

// Signer returns a fresh SSH key for tests.
func Signer(t *testing.T) ssh.Signer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("NewSignerFromKey() error = %v", err)
	}
	return signer
}

// Echo starts a TCP echo server standing in for a remote service.
func Echo(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				io.Copy(conn, conn)
			}()
		}
	}()

	return listener.Addr().String()
}
