// Package attach is the half of dv that owns a terminal.
//
// It does not know that dv talks to a database, what a datasource is, or what
// any of the cells it draws mean. It sends what the terminal did and draws
// what comes back, which is what keeps it unaffected by anything the session
// on the other side chooses to do.
package attach

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/gdamore/tcell/v2"
)

type Options struct {
	// Version must match the server's exactly.
	Version string

	// WorkDir and DataSource are what this invocation asked for, passed on so
	// the server can build or move the session.
	WorkDir    string
	DataSource string

	// Screen is the terminal. Nil makes a real one, which is the ordinary
	// case; a test supplies a simulation screen and keeps it afterwards.
	Screen tcell.Screen

	// Err is where the server's warnings are printed. Nil discards them.
	Err io.Writer
}

// Run attaches over rwc and returns when the session ends, the client is
// detached, or the server refuses.
func Run(rwc io.ReadWriteCloser, opt Options) error {
	scr := opt.Screen
	owned := false
	if scr == nil {
		made, err := tcell.NewScreen()
		if err != nil {
			return fmt.Errorf("terminal: %w", err)
		}
		if err := made.Init(); err != nil {
			return fmt.Errorf("terminal: %w", err)
		}
		scr, owned = made, true
	}
	if owned {
		defer scr.Fini()
	}

	scr.EnableMouse()
	scr.EnablePaste()

	enc, dec := proto.NewEncoder(rwc), proto.NewDecoder(rwc)

	w, h := scr.Size()
	err := enc.ToServer(proto.ToServer{
		Kind: proto.KindHello,
		Hello: &proto.Hello{
			Version: opt.Version,
			Caps: screen.Caps{
				Width: w, Height: h,
				Colors:       scr.Colors(),
				CharacterSet: scr.CharacterSet(),
				HasMouse:     scr.HasMouse(),
			},
			WorkDir:    opt.WorkDir,
			DataSource: opt.DataSource,
			PID:        os.Getpid(),
		},
	})
	if err != nil {
		return fmt.Errorf("hello: %w", err)
	}

	first, err := dec.ToClient()
	if err != nil {
		return fmt.Errorf("handshake: %w", err)
	}
	switch first.Kind {
	case proto.KindReject:
		if first.Reject == nil {
			return errors.New("the server refused without saying why")
		}
		return errors.New(first.Reject.Reason)
	case proto.KindWelcome:
	default:
		return fmt.Errorf("the server answered the handshake with %v", first.Kind)
	}

	if opt.Err != nil && first.Welcome != nil {
		for _, warning := range first.Welcome.Warnings {
			fmt.Fprintln(opt.Err, warning)
		}
	}

	go sendEvents(scr, enc, rwc)

	return receive(scr, dec)
}

// sendEvents forwards what the terminal did until the terminal is gone.
func sendEvents(scr tcell.Screen, enc *proto.Encoder, rwc io.Closer) {
	for {
		ev := scr.PollEvent()
		if ev == nil {
			// The screen has been finalised: this terminal is over. Saying so
			// lets the server drop the client now rather than when its next
			// write fails.
			_ = enc.ToServer(proto.ToServer{Kind: proto.KindDetach})
			_ = rwc.Close()
			return
		}

		var m proto.ToServer
		switch e := ev.(type) {
		case *tcell.EventKey:
			m = proto.ToServer{Kind: proto.KindKey, Key: &proto.Key{
				Key: e.Key(), Rune: e.Rune(), Mods: e.Modifiers(),
			}}
		case *tcell.EventMouse:
			x, y := e.Position()
			m = proto.ToServer{Kind: proto.KindMouse, Mouse: &proto.Mouse{
				X: x, Y: y, Buttons: e.Buttons(), Mods: e.Modifiers(),
			}}
		case *tcell.EventResize:
			width, height := e.Size()
			m = proto.ToServer{Kind: proto.KindResize, Resize: &proto.Size{Width: width, Height: height}}
		case *tcell.EventPaste:
			m = proto.ToServer{Kind: proto.KindPaste, Paste: &proto.Paste{Start: e.Start()}}
		case *tcell.EventFocus:
			m = proto.ToServer{Kind: proto.KindFocus, Focus: &proto.Focus{Focused: e.Focused}}
		case *tcell.EventClipboard:
			m = proto.ToServer{Kind: proto.KindClipboardData, Clipboard: e.Data()}
		default:
			continue
		}

		if err := enc.ToServer(m); err != nil {
			return
		}
	}
}

// receive draws what the server sends until it stops sending.
func receive(scr tcell.Screen, dec *proto.Decoder) error {
	for {
		m, err := dec.ToClient()
		if err != nil {
			// The server is gone, or this client was detached. Neither is a
			// failure of this process, and neither has anything more to say.
			return nil
		}

		switch m.Kind {
		case proto.KindFrame:
			if m.Frame != nil {
				draw(scr, *m.Frame)
			}
		case proto.KindTitle:
			scr.SetTitle(m.Title)
		case proto.KindSetClipboard:
			scr.SetClipboard(m.Clipboard)
		case proto.KindRequestClipboard:
			scr.GetClipboard()
		case proto.KindBell:
			_ = scr.Beep()
		case proto.KindBye:
			return nil
		}
	}
}

func draw(scr tcell.Screen, f screen.Frame) {
	for _, c := range f.Cells {
		scr.SetContent(c.X, c.Y, c.Main, c.Comb, c.Style.Tcell())
	}
	if f.Cursor.Visible {
		scr.ShowCursor(f.Cursor.X, f.Cursor.Y)
	} else {
		scr.HideCursor()
	}
	scr.Show()
}
