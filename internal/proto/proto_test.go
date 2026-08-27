package proto_test

import (
	"net"
	"testing"

	"github.com/Ahngbeom/datavase/internal/proto"
	"github.com/Ahngbeom/datavase/internal/screen"
	"github.com/gdamore/tcell/v2"
)

// A style or a combining rune lost on the wire is a character drawn wrongly
// with nothing to say it was ever right.
func TestFrameSurvivesTheWire(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	want := proto.ToClient{
		Kind: proto.KindFrame,
		Frame: &screen.Frame{
			Cells: []screen.Cell{
				{
					X: 3, Y: 4, Main: '한',
					Comb:  []rune{0x0301},
					Style: screen.StyleFrom(tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true)),
					Width: 2,
				},
			},
			Cursor: screen.Cursor{X: 3, Y: 4, Visible: true},
		},
	}

	errc := make(chan error, 1)
	go func() { errc <- proto.NewEncoder(server).ToClient(want) }()

	got, err := proto.NewDecoder(client).ToClient()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("encode: %v", err)
	}

	if got.Kind != proto.KindFrame {
		t.Fatalf("Kind = %v, want KindFrame", got.Kind)
	}
	if got.Frame == nil {
		t.Fatal("Frame is nil")
	}
	if len(got.Frame.Cells) != 1 {
		t.Fatalf("got %d cells, want 1", len(got.Frame.Cells))
	}
	c := got.Frame.Cells[0]
	if c.Main != '한' || c.Width != 2 {
		t.Errorf("cell = %q width %d, want '한' width 2", c.Main, c.Width)
	}
	if len(c.Comb) != 1 || c.Comb[0] != 0x0301 {
		t.Errorf("combining = %v, want [0x0301]", c.Comb)
	}
	if c.Style != want.Frame.Cells[0].Style {
		t.Error("style did not survive the wire")
	}
	if fg, _, _ := c.Style.Tcell().Decompose(); fg != tcell.ColorRed {
		t.Errorf("foreground came back as %v, want red", fg)
	}
	if got.Frame.Cursor != want.Frame.Cursor {
		t.Errorf("Cursor = %+v, want %+v", got.Frame.Cursor, want.Frame.Cursor)
	}
}

// A key that changes on the way up is a key the user did not press.
func TestKeyEventSurvivesTheWire(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	want := proto.ToServer{
		Kind: proto.KindKey,
		Key:  &proto.Key{Key: tcell.KeyRune, Rune: '가', Mods: tcell.ModAlt},
	}

	errc := make(chan error, 1)
	go func() { errc <- proto.NewEncoder(client).ToServer(want) }()

	got, err := proto.NewDecoder(server).ToServer()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("encode: %v", err)
	}

	if got.Kind != proto.KindKey || got.Key == nil {
		t.Fatalf("got %+v, want a key", got)
	}
	if *got.Key != *want.Key {
		t.Errorf("Key = %+v, want %+v", *got.Key, *want.Key)
	}
}

// The handshake carries what the screen will report as its capabilities. A
// colour depth mangled here is a palette degraded for the whole session.
func TestHelloCarriesCaps(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	want := proto.ToServer{
		Kind: proto.KindHello,
		Hello: &proto.Hello{
			Version: "0.7.0",
			Caps: screen.Caps{
				Width: 162, Height: 45,
				Colors: 16777216, CharacterSet: "UTF-8", HasMouse: true,
			},
			WorkDir:    "/home/x/reports",
			DataSource: "prod-ro",
			PID:        4242,
		},
	}

	errc := make(chan error, 1)
	go func() { errc <- proto.NewEncoder(client).ToServer(want) }()

	got, err := proto.NewDecoder(server).ToServer()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("encode: %v", err)
	}

	if got.Hello == nil {
		t.Fatal("Hello is nil")
	}
	if *got.Hello != *want.Hello {
		t.Errorf("Hello = %+v, want %+v", *got.Hello, *want.Hello)
	}
}

// Several messages share one connection; gob sends its type information once
// and a second message must still arrive whole.
func TestSecondMessageOnTheSameConnection(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	enc := proto.NewEncoder(client)
	go func() {
		_ = enc.ToServer(proto.ToServer{Kind: proto.KindKey, Key: &proto.Key{Key: tcell.KeyRune, Rune: 'a'}})
		_ = enc.ToServer(proto.ToServer{Kind: proto.KindDetach})
	}()

	dec := proto.NewDecoder(server)
	if first, err := dec.ToServer(); err != nil || first.Kind != proto.KindKey {
		t.Fatalf("first = %+v, err = %v", first, err)
	}
	second, err := dec.ToServer()
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Kind != proto.KindDetach {
		t.Errorf("second Kind = %v, want KindDetach", second.Kind)
	}
}
