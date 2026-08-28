// Package snapshot says what a datavase session is doing, and nothing else.
//
// It never holds a session. A Source is two functions — one that answers from
// the server process and one that answers from inside the interface — so
// there is nothing here that could be made to run a statement. That matters
// because internal/guard is fail-closed by design and its dialogs live in the
// interface: an API that could execute would be a way past them, and the way
// to keep that decision available is not to open the door.
//
// It also never carries a row. The result buffer holds production data that
// is written to no log and no history, and internal/export is the path a
// person chooses deliberately.
//
// It knows nothing about sockets: Handle writes to an io.Writer.
package snapshot

import (
	"context"
	"encoding/json"
	"io"
	"time"
)

// Version is the shape of this document. Adding a field does not change it;
// renaming or removing one does.
const Version = 1

// SessionTimeout bounds the wait for the interface to answer. Something is
// sitting at a prompt for this long.
const SessionTimeout = 2 * time.Second

type Snapshot struct {
	Version int    `json:"version"`
	DV      string `json:"dv"`
	Server  Server `json:"server"`

	// Session is nil when the interface did not answer, and SessionError says
	// why. The server tier is still filled in, which is what makes dv status
	// useful precisely when something has gone wrong.
	Session      *Session `json:"session"`
	SessionError string   `json:"session_error,omitempty"`
}

type Server struct {
	PID            int    `json:"pid"`
	StartedAt      string `json:"started_at"`
	UptimeSeconds  int    `json:"uptime_seconds"`
	ClientAttached bool   `json:"client_attached"`
	DV             string `json:"-"`
}

type Session struct {
	DataSource    DataSource `json:"datasource"`
	Schema        string     `json:"schema"`
	Statement     Statement  `json:"statement"`
	Result        Result     `json:"result"`
	Batch         Batch      `json:"batch"`
	Worktree      *Worktree  `json:"worktree"`
	Editor        Editor     `json:"editor"`
	Mode          string     `json:"mode"`
	WritesEnabled bool       `json:"writes_enabled"`
	InTransaction bool       `json:"in_transaction"`
}

// DataSource is everything about the connection except how to authenticate
// it. config.DataSource has no password field, and this is read from that type
// alone, so there is no path by which one could appear here.
type DataSource struct {
	Name          string `json:"name"`
	Env           string `json:"env"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	User          string `json:"user"`
	Database      string `json:"database"`
	Tunnel        bool   `json:"tunnel"`
	ServerVersion string `json:"server_version"`
}

// Statement is what the database is being asked to do.
//
// The SQL is here because it is the point of observing, and because
// internal/history already writes every executed statement's full text to
// query_history.sql_text on disk in plain text. This is no new class of
// exposure, over a socket narrower than that file.
type Statement struct {
	Running       bool   `json:"running"`
	ElapsedMS     int64  `json:"elapsed_ms"`
	SQL           string `json:"sql"`
	InjectedLimit int    `json:"injected_limit"`
	Truncated     bool   `json:"truncated"`
	Error         string `json:"error,omitempty"`
}

// Result is the shape of what came back and never its contents.
type Result struct {
	Columns  []string `json:"columns"`
	RowCount int      `json:"row_count"`
}

type Batch struct {
	Running   bool `json:"running"`
	Completed int  `json:"completed"`
	Total     int  `json:"total"`
}

type Worktree struct {
	Path     string `json:"path"`
	Branch   string `json:"branch"`
	OpenFile string `json:"open_file"`
	Modified bool   `json:"modified"`
}

// Editor is the shape of the buffer and not a word of it. What is typed and
// not yet run is a draft; what is running is a fact about the database.
type Editor struct {
	Lines    int  `json:"lines"`
	Modified bool `json:"modified"`
}

// Source is where the two tiers come from.
//
// Nothing else belongs in this struct. See the package comment.
type Source struct {
	// Server answers from the server process and cannot fail.
	Server func() Server
	// Session answers from inside the interface, because that is who owns the
	// state it reads. It may time out, and a timeout is an answer.
	Session func(context.Context) (*Session, error)
}

// Take builds the document.
func (s Source) Take(ctx context.Context) Snapshot {
	server := s.Server()

	out := Snapshot{Version: Version, DV: server.DV, Server: server}

	if s.Session == nil {
		out.SessionError = "no session"
		return out
	}

	session, err := s.Session(ctx)
	if err != nil {
		out.SessionError = err.Error()
		return out
	}
	if session == nil {
		out.SessionError = "no session"
		return out
	}
	out.Session = session
	return out
}

// Handle writes one snapshot as one line of JSON.
func Handle(w io.Writer, src Source, ctx context.Context) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(src.Take(ctx))
}
