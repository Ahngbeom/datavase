package daemon

import (
	"fmt"
	"io"
	"sync"

	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/gdamore/tcell/v2"
)

// conn is one attached client.
type conn struct {
	rwc io.ReadWriteCloser
	enc *proto.Encoder

	frames *proto.FrameQueue
	msgs   chan proto.ToClient

	// writes serialises the two goroutines that write: frames come off the
	// queue, everything else off msgs, and they are independent of each
	// other.
	writes sync.Mutex

	closeOnce sync.Once
	done      chan struct{}
}

func newConn(rwc io.ReadWriteCloser) *conn {
	return &conn{
		rwc:    rwc,
		enc:    proto.NewEncoder(rwc),
		frames: proto.NewFrameQueue(),
		msgs:   make(chan proto.ToClient, 16),
		done:   make(chan struct{}),
	}
}

func (c *conn) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.frames.Close()
		_ = c.rwc.Close()
	})
}

func (c *conn) send(m proto.ToClient) {
	c.writes.Lock()
	defer c.writes.Unlock()
	if err := c.enc.ToClient(m); err != nil {
		// A write that fails means the terminal is gone. That is a detach,
		// not a shutdown: the reader will see the same thing and unwind.
		c.close()
	}
}

// Sink implementation. Nothing here blocks: the goroutine calling it is the
// one drawing the interface, and it is also the one reading rows off the
// database connection.

func (c *conn) Frame(f screen.Frame) { c.frames.Put(f) }

func (c *conn) post(m proto.ToClient) {
	select {
	case c.msgs <- m:
	case <-c.done:
	default:
		// Sixteen unread titles or bells means the terminal is not keeping
		// up with anything, and these are cosmetic. Frames are what must not
		// be lost, and they are not queued here.
	}
}

func (c *conn) SetTitle(t string) { c.post(proto.ToClient{Kind: proto.KindTitle, Title: t}) }
func (c *conn) SetClipboard(b []byte) {
	c.post(proto.ToClient{Kind: proto.KindSetClipboard, Clipboard: b})
}
func (c *conn) RequestClipboard() { c.post(proto.ToClient{Kind: proto.KindRequestClipboard}) }
func (c *conn) Bell()             { c.post(proto.ToClient{Kind: proto.KindBell}) }

var _ screen.Sink = (*conn)(nil)

func (c *conn) writeFrames() {
	for {
		f, ok := c.frames.Take(contextDone(c.done))
		if !ok {
			return
		}
		frame := f
		c.send(proto.ToClient{Kind: proto.KindFrame, Frame: &frame})
	}
}

func (c *conn) writeMessages() {
	for {
		select {
		case m := <-c.msgs:
			c.send(m)
		case <-c.done:
			return
		}
	}
}

// Serve runs one client to completion and returns when it goes away.
func (s *Server) Serve(rwc io.ReadWriteCloser) {
	dec := proto.NewDecoder(rwc)

	first, err := dec.ToServer()
	if err != nil {
		_ = rwc.Close()
		return
	}
	if first.Kind != proto.KindHello || first.Hello == nil {
		reject(rwc, "the first message was not a hello")
		return
	}
	h := *first.Hello

	if h.Version != s.opts.Version {
		reject(rwc, fmt.Sprintf(
			"the running dv server is %s; this dv is %s.\n\n  dv server stop   end that session and start again",
			s.opts.Version, h.Version))
		return
	}

	c, warnings, err := s.admit(h, rwc)
	if err != nil {
		reject(rwc, err.Error())
		return
	}

	c.send(proto.ToClient{
		Kind:    proto.KindWelcome,
		Welcome: &proto.Welcome{Version: s.opts.Version, Warnings: warnings},
	})

	go c.writeFrames()
	go c.writeMessages()

	s.mu.Lock()
	scr := s.screen
	s.mu.Unlock()

	scr.SetSize(h.Caps.Width, h.Caps.Height)
	scr.Attach(c)

	s.read(dec, scr, c)
}

// admit decides whether this client may have the session, starting one if
// there is none.
//
// admitMu is held for the whole call. Without it, two Serve calls arriving
// before any session exists can both pass the nil check before either has
// set s.session, and both call Start — opening two database connections and
// orphaning one that nothing will ever close. admit runs once per incoming
// connection, not per frame, so serialising all of it costs nothing against
// the throughput this package actually protects (frame delivery, which does
// not go through this lock).
func (s *Server) admit(h proto.Hello, rwc io.ReadWriteCloser) (*conn, []string, error) {
	s.admitMu.Lock()
	defer s.admitMu.Unlock()

	s.mu.Lock()

	if s.session == nil {
		start := s.opts.Start
		s.mu.Unlock()

		session, warnings, err := start(h)
		if err != nil {
			return nil, nil, err
		}

		scr := screen.New(h.Caps)
		session.SetScreen(scr)

		s.mu.Lock()
		s.session, s.screen = session, scr
		s.dataSource, s.warnings = h.DataSource, warnings
		c := newConn(rwc)
		s.client = c
		s.mu.Unlock()

		go func() { s.finish(session.Run()) }()
		return c, warnings, nil
	}

	session, known := s.session, s.dataSource
	s.mu.Unlock()

	if h.DataSource != "" && h.DataSource != known {
		st := s.state(session, known)
		if st.Busy {
			return nil, nil, fmt.Errorf(
				"a statement is running on %q.\n\n  dv              attach to it\n  dv server stop  end it and start again",
				st.DataSource)
		}
		if sw, ok := session.(Switcher); ok {
			sw.SwitchTo(h.DataSource)
			s.mu.Lock()
			s.dataSource = h.DataSource
			s.mu.Unlock()
		}
	}

	// The new client wins. The old terminal is told why so it can exit
	// cleanly rather than appear to have frozen.
	s.mu.Lock()
	old := s.client
	c := newConn(rwc)
	s.client = c
	s.mu.Unlock()

	if old != nil {
		// old.send is a write with no deadline; if the old terminal is alive
		// but has stopped draining its socket, that write blocks forever. It
		// must not do so on this goroutine, which the new client's whole
		// handshake is waiting on. Launch it detached and close old right
		// behind it, without waiting: closing old.rwc while the write is in
		// flight on the same conn value unblocks it with an error, for both
		// a real socket and net.Pipe (what this package's tests use).
		//
		// If some future io.ReadWriteCloser implementation does not unblock
		// a concurrent Write on Close, this goroutine leaks rather than
		// hanging the caller — the failure mode this fix removes moves from
		// "every future reconnect hangs" to "one goroutine per such client
		// never exits," which is the trade this package can make since it
		// only knows the rwc as an io.ReadWriteCloser, not as a socket.
		go old.send(proto.ToClient{Kind: proto.KindBye, Bye: &proto.Bye{Reason: proto.ByeReplaced}})
		old.close()
	}

	// Warnings are once per session, not once per attach.
	return c, nil, nil
}

// read pumps what the client sends into the screen until it stops.
func (s *Server) read(dec *proto.Decoder, scr *screen.Screen, c *conn) {
	defer func() {
		s.mu.Lock()
		mine := s.client == c
		if mine {
			s.client = nil
		}
		s.mu.Unlock()

		if mine {
			scr.Detach()
		}
		c.close()
	}()

	for {
		m, err := dec.ToServer()
		if err != nil {
			return
		}

		switch m.Kind {
		case proto.KindDetach:
			return
		case proto.KindKey:
			if m.Key != nil {
				scr.PostEventWait(tcell.NewEventKey(m.Key.Key, m.Key.Rune, m.Key.Mods))
			}
		case proto.KindMouse:
			if m.Mouse != nil {
				scr.PostEventWait(tcell.NewEventMouse(m.Mouse.X, m.Mouse.Y, m.Mouse.Buttons, m.Mouse.Mods))
			}
		case proto.KindResize:
			if m.Resize != nil {
				scr.SetSize(m.Resize.Width, m.Resize.Height)
				scr.Sync()
			}
		case proto.KindPaste:
			if m.Paste != nil {
				scr.PostEventWait(tcell.NewEventPaste(m.Paste.Start))
			}
		case proto.KindFocus:
			if m.Focus != nil {
				scr.PostEventWait(tcell.NewEventFocus(m.Focus.Focused))
			}
		case proto.KindClipboardData:
			scr.PostEventWait(tcell.NewEventClipboard(m.Clipboard))
		}
	}
}

func reject(rwc io.ReadWriteCloser, reason string) {
	_ = proto.NewEncoder(rwc).ToClient(proto.ToClient{
		Kind:   proto.KindReject,
		Reject: &proto.Reject{Reason: reason},
	})
	_ = rwc.Close()
}
