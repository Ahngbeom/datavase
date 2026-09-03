// Package proto is what the interface and the terminal say to each other when
// they are in different processes.
//
// It knows nothing about sockets: everything here works over an io.Reader and
// an io.Writer, which is what lets net.Pipe test the whole of it.
//
// The payload types come from internal/screen rather than being redeclared
// here. The screen holds the concept and this package only carries it; a wire
// format that owned the cell type would drag the screen along every time the
// transport changed.
package proto

import (
	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/gdamore/tcell/v2"
)

// Kind names the payload. Messages are an envelope with a Kind and one
// pointer per payload rather than registered interfaces, so that the whole
// protocol reads in one file and there is no registration to forget.
type Kind uint8

const (
	KindNone Kind = iota

	// Client to server.
	KindHello
	KindKey
	KindMouse
	KindResize
	KindPaste
	KindFocus
	KindClipboardData
	KindDetach
	KindStop

	// Server to client.
	KindWelcome
	KindReject
	KindFrame
	KindSetClipboard
	KindRequestClipboard
	KindTitle
	KindBell
	KindBye
)

// Hello opens a connection.
type Hello struct {
	Version string
	Caps    screen.Caps
	// WorkDir is --dir, empty when the session starts unattached.
	WorkDir string
	// DataSource is the name the client was asked to open, empty when none
	// was named.
	DataSource string
	PID        int
	// BuildFingerprint is version.BuildFingerprint(): empty for a release
	// build, which reports a real Version and needs nothing more to be told
	// apart from another. It exists because every development build reports
	// the same Version, "(devel)", so that alone cannot tell a rebuilt local
	// binary from the one already running in the server it is attaching to.
	BuildFingerprint string
}

// Welcome accepts a Hello.
//
// Warnings are what the server would have written to stderr while starting
// the session — a schema cache it could not open, a worktree that no longer
// exists. They travel here because in the server they would otherwise land in
// a log nobody reads, and completion would simply look broken.
type Welcome struct {
	Version  string
	Warnings []string
}

// Reject refuses a Hello and says why in a sentence meant for a person.
type Reject struct {
	Reason string
}

// Bye ends a connection from the server's side. Replaced means another client
// attached; Quit means the session itself has ended.
type Bye struct {
	Reason string
}

const (
	ByeQuit     = "quit"
	ByeReplaced = "replaced"
)

// Key is tcell.EventKey flattened. The event type carries a timestamp and
// unexported state that mean nothing in another process.
type Key struct {
	Key  tcell.Key
	Rune rune
	Mods tcell.ModMask
}

// Mouse is tcell.EventMouse flattened, for the same reason.
type Mouse struct {
	X, Y    int
	Buttons tcell.ButtonMask
	Mods    tcell.ModMask
}

// Size is a resize from the client.
type Size struct {
	Width, Height int
}

// Paste brackets a paste. Start false is the end of one.
type Paste struct {
	Start bool
}

// Focus reports the terminal gaining or losing focus.
type Focus struct {
	Focused bool
}

// ToServer is anything the client says.
type ToServer struct {
	Kind Kind

	Hello     *Hello
	Key       *Key
	Mouse     *Mouse
	Resize    *Size
	Paste     *Paste
	Focus     *Focus
	Clipboard []byte
}

// ToClient is anything the server says.
type ToClient struct {
	Kind Kind

	Welcome   *Welcome
	Reject    *Reject
	Frame     *screen.Frame
	Clipboard []byte
	Title     string
	Bye       *Bye
}
