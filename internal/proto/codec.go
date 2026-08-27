package proto

import (
	"encoding/gob"
	"io"
)

// Encoder writes messages to an io.Writer.
//
// gob rather than JSON because a frame carries runes, combining runes and a
// tcell.Style, and because its stream encoder sends type information once per
// stream instead of once per frame. Both ends are dv, so gob being a Go
// format costs nothing.
type Encoder struct {
	enc *gob.Encoder
}

func NewEncoder(w io.Writer) *Encoder { return &Encoder{enc: gob.NewEncoder(w)} }

func (e *Encoder) ToServer(m ToServer) error { return e.enc.Encode(m) }
func (e *Encoder) ToClient(m ToClient) error { return e.enc.Encode(m) }

// Decoder reads messages from an io.Reader.
//
// gob will allocate whatever a length prefix asks for, and nothing here can
// bound that: this package cannot see who is at the other end of the reader.
// Whoever supplies it is what has to decide who can reach it.
type Decoder struct {
	dec *gob.Decoder
}

func NewDecoder(r io.Reader) *Decoder { return &Decoder{dec: gob.NewDecoder(r)} }

func (d *Decoder) ToServer() (ToServer, error) {
	var m ToServer
	err := d.dec.Decode(&m)
	return m, err
}

func (d *Decoder) ToClient() (ToClient, error) {
	var m ToClient
	err := d.dec.Decode(&m)
	return m, err
}
