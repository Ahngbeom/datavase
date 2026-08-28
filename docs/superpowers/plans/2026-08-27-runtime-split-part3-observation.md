# Runtime Split, Part 3: Observation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A read-only socket that answers "what is this session doing" —
`dv api snapshot` for a script or an agent, `dv status` for a person — without
ever being able to change anything or to hand out a row of data.

**Architecture:** `internal/snapshot` builds the answer and encodes it as JSON;
it holds two functions and never a session, so a control path cannot be added
to it by accident. The server tier is answered from the server process and
always succeeds; the session tier goes through the interface's own goroutine
and may time out, which is why `dv status` cannot hang. `internal/daemon`
serves it on a second socket.

**Tech Stack:** Go 1.26.4, `encoding/json` and `net` from the standard
library. No new third-party dependency.

**Spec:** `docs/superpowers/specs/2026-08-27-dv-runtime-design.md`

**Depends on:** Parts 1 and 2, complete and merged.

## Global Constraints

- Repository `Ahngbeom/datavase`, default branch `main`. Work on a branch and
  open a PR; never push to `main`.
- `CGO_ENABLED=0` stays viable. No third-party dependency is added.
- `make lint` clean and `make test` passing at every commit.
- **The API is read-only by construction.** `snapshot.Source` holds two
  functions and no session. Do not add a field, a method, or a request kind
  that can change anything. A control API needs a `guard` policy designed with
  it, and that decision stays available only by not opening the door now.
- **No row data leaves this package.** Column names and a count, never a cell.
  `result.Buffer` holds production data that is written to no log and no
  history; `internal/export` is the path a person chooses on purpose.
- **No editor text.** Line count and a modified flag. The running statement is
  a fact about what the database is doing; the editor buffer is an unrun
  draft.
- The API socket is `$XDG_STATE_HOME/datavase/dv-api.sock`, mode `0600`.
- **Field names are an interface other things depend on.** A golden JSON test
  makes a rename loud. Adding a field updates the golden; renaming or removing
  one must be a decision, not a side effect.
- **Comments say why. Tests state the consequence. TDD throughout.**

## File Structure

| File | Responsibility |
| --- | --- |
| `internal/snapshot/snapshot.go` | The types, `Source`, `Take`, `Handle`. |
| `internal/snapshot/snapshot_test.go` | Tiers, timeout, secrecy, golden JSON. |
| `internal/snapshot/testdata/snapshot.json` | The golden. |
| `internal/ui/app.go` | `App.Snapshot`, and the running statement's text. |
| `internal/daemon/socket.go` | `APISocketPath`, `ServeAPI`. |
| `cmd/dv/runtime.go` | `dv api snapshot`, and `dv status` from the session. |
| `internal/cli/cli.go` | The `api` command and its usage lines. |

---

### Task 1: The answer, and what it will not say

**Files:**
- Create: `internal/snapshot/snapshot.go`, `internal/snapshot/snapshot_test.go`, `internal/snapshot/testdata/snapshot.json`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Snapshot, Server, Session, DataSource, Statement, Result, Batch, Worktree, Editor struct`
  - `type Source struct { Server func() Server; Session func(context.Context) (*Session, error) }`
  - `func (Source) Take(ctx context.Context) Snapshot`
  - `func Handle(w io.Writer, src Source, ctx context.Context) error`
  - `const SessionTimeout = 2 * time.Second`

- [ ] **Step 1: Write the failing test**

Create `internal/snapshot/snapshot_test.go`:

```go
package snapshot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/snapshot"
)

func serverTier() snapshot.Server {
	return snapshot.Server{
		PID:            12345,
		StartedAt:      "2026-08-27T09:12:03Z",
		UptimeSeconds:  3600,
		ClientAttached: true,
	}
}

func sessionTier() *snapshot.Session {
	return &snapshot.Session{
		DataSource: snapshot.DataSource{
			Name: "prod-ro", Env: "production",
			Host: "db.internal", Port: 3306, User: "reader",
			Database: "app", Tunnel: true, ServerVersion: "8.0.36",
		},
		Schema: "app",
		Statement: snapshot.Statement{
			Running:      true,
			ElapsedMS:    4210,
			SQL:          "SELECT id, customer, total FROM orders",
			InjectedLimit: 1000,
			Truncated:    false,
		},
		Result: snapshot.Result{
			Columns:  []string{"id", "customer", "total"},
			RowCount: 4213,
		},
		Batch:    snapshot.Batch{Running: false},
		Worktree: &snapshot.Worktree{Path: "/home/x/reports", Branch: "main", OpenFile: "monthly.sql", Modified: true},
		Editor:   snapshot.Editor{Lines: 42, Modified: true},
		Mode:     "NORMAL",
		WritesEnabled: false,
		InTransaction: false,
	}
}

// dv status exists to say whether a session is alive. A status command that
// hangs when the session is wedged is worse than none: it dies with the thing
// it was asked about.
func TestServerTierAnswersWhenTheSessionWillNot(t *testing.T) {
	src := snapshot.Source{
		Server: serverTier,
		Session: func(ctx context.Context) (*snapshot.Session, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	got := src.Take(ctx)

	if got.Server.PID != 12345 {
		t.Errorf("server tier is %+v; it must be answerable whatever the session is doing", got.Server)
	}
	if got.Session != nil {
		t.Error("a session that did not answer was reported as if it had")
	}
	if got.SessionError == "" {
		t.Error("the snapshot does not say why the session is missing")
	}
}

// The result buffer holds production data that is written to no log and no
// history. An observation API must not become a quiet way out for it.
func TestNoRowDataIsEverEncoded(t *testing.T) {
	src := snapshot.Source{
		Server:  serverTier,
		Session: func(context.Context) (*snapshot.Session, error) { return sessionTier(), nil },
	}

	var out bytes.Buffer
	if err := snapshot.Handle(&out, src, context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var generic map[string]any
	if err := json.Unmarshal(out.Bytes(), &generic); err != nil {
		t.Fatalf("the response is not JSON: %v", err)
	}

	for _, banned := range []string{"\"rows\"", "\"cells\"", "\"password\"", "\"secret\"", "\"text\""} {
		if strings.Contains(out.String(), banned) {
			t.Errorf("the snapshot contains %s", banned)
		}
	}
}

// The API is something other programs read. A renamed field breaks every one
// of them and nothing here would otherwise notice.
func TestSnapshotMatchesTheGolden(t *testing.T) {
	src := snapshot.Source{
		Server:  serverTier,
		Session: func(context.Context) (*snapshot.Session, error) { return sessionTier(), nil },
	}

	var out bytes.Buffer
	if err := snapshot.Handle(&out, src, context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	golden := "testdata/snapshot.json"
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, out.Bytes(), 0o644); err != nil {
			t.Fatalf("writing golden: %v", err)
		}
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	if out.String() != string(want) {
		t.Errorf("the snapshot changed shape.\n got: %s\nwant: %s\n\nIf this is deliberate, re-run with UPDATE_GOLDEN=1 and read the diff before committing it.", out.String(), want)
	}
}

// Nothing in the API may change the session. The type is the guard: it holds
// two functions and no session, so there is nothing to call.
func TestSourceHoldsNothingThatCanMutate(t *testing.T) {
	// This test is a statement of intent that the compiler enforces: if
	// Source ever gains a field that is not one of these two, this stops
	// building and whoever added it has to say why here.
	_ = snapshot.Source{
		Server:  serverTier,
		Session: func(context.Context) (*snapshot.Session, error) { return nil, nil },
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/snapshot/`
Expected: build failure — the package does not exist.

- [ ] **Step 3: Write the implementation**

Create `internal/snapshot/snapshot.go`:

```go
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
```

- [ ] **Step 4: Create the golden and run the tests**

```bash
mkdir -p internal/snapshot/testdata
UPDATE_GOLDEN=1 go test ./internal/snapshot/ -run TestSnapshotMatchesTheGolden
git diff --stat internal/snapshot/testdata/
go test ./internal/snapshot/ -v
```

Expected: all four PASS. **Read the golden file before committing it** — it is
the document other programs will be written against, and this is the one
moment its shape is chosen rather than inherited.

- [ ] **Step 5: Prove the secrecy test bites**

Add `Rows [][]string \`json:"rows"\`` to `Result` and populate it in
`sessionTier()`. Run `go test ./internal/snapshot/ -run TestNoRowData` and
confirm it FAILS with `the snapshot contains "rows"`. Remove both.

- [ ] **Step 6: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test ./internal/snapshot/
git add internal/snapshot/
git commit -m "Say what the session is doing, and nothing more

A Source is two functions and never a session, which is the whole of the
read-only guarantee: there is nothing here that could be made to run a
statement. guard is fail-closed and its dialogs are in the interface, so
an API that could execute would be a way past them — and the way to keep
that decision open is not to open the door.

The two tiers are why dv status cannot hang. The server tier answers
from the server process and always succeeds; the session tier goes
through the interface's own goroutine, which owns the state it reads,
and a timeout there is an answer rather than a wait. A diagnostic that
dies with the thing it is diagnosing is worse than none.

No row ever leaves: columns and a count. The buffer holds production
data written to no log and no history, and export is the path a person
chooses on purpose. The statement's SQL does travel, because it is the
point of observing and because history already writes every executed
statement to disk in plain text.

The golden test is there because the field names are an interface other
programs depend on, and a rename would otherwise be silent."
```

---

### Task 2: The interface answers for itself

**Files:**
- Modify: `internal/ui/app.go`
- Test: `internal/ui/snapshot_integration_test.go`

**Interfaces:**
- Consumes: `snapshot.Session` and friends from Task 1.
- Produces: `func (a *App) Snapshot(ctx context.Context) (*snapshot.Session, error)`

- [ ] **Step 1: Record the statement that is running**

`App` knows when a statement is in flight but not what it is. In
`internal/ui/app.go`, beside `running *db.Stream`:

```go
	// runningSQL is the statement a.running is executing. Held because the
	// stream does not carry it and something outside the interface has to be
	// able to say what the database is being asked to do.
	runningSQL string
```

Set it where `a.running` is assigned, and clear it where `a.running` is set to
nil. Both places are in `app.go`; find them with `grep -n 'a.running = '`.

- [ ] **Step 2: Write the failing test**

Create `internal/ui/snapshot_integration_test.go`:

```go
//go:build integration

package ui

import (
	"context"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
)

// Reading a.running or the connection from another goroutine is a race. This
// is the check that the answer comes from the interface's own goroutine and
// arrives whole.
func TestSnapshotReportsTheLiveSession(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	got, err := h.app.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if got.DataSource.Name == "" {
		t.Error("the snapshot does not name the datasource")
	}
	if got.DataSource.ServerVersion == "" {
		t.Error("the snapshot does not carry the server version")
	}
	if got.Statement.Running {
		t.Error("a fresh session reports a statement running")
	}
}

// A caller deciding whether to wait must be able to stop waiting.
func TestSnapshotHonoursItsDeadline(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()

	if _, err := h.app.Snapshot(ctx); err == nil {
		t.Error("Snapshot ignored an expired context")
	}
}
```

Check `newHarness`'s field for the application — `app_integration_test.go`
names it; use whatever it is rather than `h.app` if it differs.

- [ ] **Step 3: Run it and watch it fail**

Run: `make db-up && go test -tags integration ./internal/ui/ -run TestSnapshot`
Expected: build failure — `h.app.Snapshot undefined`.

- [ ] **Step 4: Write the implementation**

In `internal/ui/app.go`, beside `State` from Part 2:

```go
// Snapshot describes the session for something outside it.
//
// Like State, it runs on the interface's own goroutine: a.running, the
// connection, the buffer and the vim state all belong to it, and reading them
// from anywhere else is a race. The context bounds the wait, because whatever
// asked has to be able to give up and say so.
func (a *App) Snapshot(ctx context.Context) (*snapshot.Session, error) {
	out := make(chan *snapshot.Session, 1)

	go a.app.QueueUpdate(func() { out <- a.snapshot() })

	select {
	case s := <-out:
		return s, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// snapshot reads the session. Only the interface's goroutine may call it.
func (a *App) snapshot() *snapshot.Session {
	ds := a.conn.DataSource()

	s := &snapshot.Session{
		DataSource: snapshot.DataSource{
			Name:          ds.Name,
			Env:           string(ds.Env),
			Host:          ds.Host,
			Port:          ds.Port,
			User:          ds.User,
			Database:      ds.Database,
			Tunnel:        ds.Tunnel != nil,
			ServerVersion: a.conn.ServerVersion(),
		},
		Schema: a.selectedSchema,
		Statement: snapshot.Statement{
			Running:       a.running != nil,
			ElapsedMS:     a.status.elapsed.Milliseconds(),
			SQL:           a.runningSQL,
			InjectedLimit: a.status.limitInjected,
			Truncated:     a.status.truncated,
		},
		Result: snapshot.Result{
			Columns:  a.buf.Columns(),
			RowCount: a.buf.RowCount(),
		},
		Batch:         snapshot.Batch{},
		Editor:        snapshot.Editor{Lines: strings.Count(a.editor.GetText(), "\n") + 1, Modified: a.fileDirty()},
		Mode:          a.vim.Mode().String(),
		WritesEnabled: a.status.writesEnabled,
		InTransaction: a.conn.InTransaction(),
	}

	if a.status.err != nil {
		s.Statement.Error = a.status.err.Error()
	}

	if a.batch != nil {
		s.Batch = snapshot.Batch{Running: true, Completed: a.batch.ran, Total: len(a.batch.stmts)}
	}

	if a.wt != nil {
		s.Worktree = &snapshot.Worktree{
			Path:     a.wt.Path(),
			Branch:   a.wtSnap.Branch,
			OpenFile: a.openFile.rel,
			Modified: a.fileDirty(),
		}
	}

	return s
}
```

`a.wt.Path()` is whatever `worktree.Worktree` calls its directory — check
`internal/worktree` and use that. `a.editor.GetText()` is tview's `TextArea`
accessor; if it takes an argument in this version, pass what the rest of
`app.go` passes.

Note what is deliberately absent: the editor's text, and any cell of
`a.buf`. Both are named in this plan's constraints.

- [ ] **Step 5: Run the tests and watch them pass**

Run: `go test -tags integration ./internal/ui/ -run TestSnapshot -v`
Expected: both PASS.

- [ ] **Step 6: Prove the deadline test bites**

Replace the `select` in `Snapshot` with a plain `return <-out, nil`. Run
`TestSnapshotHonoursItsDeadline` and confirm it FAILS with `Snapshot ignored
an expired context`. Restore it.

- [ ] **Step 7: Lint and commit**

```bash
gofmt -l . && go vet ./... && go test ./internal/... && go test -tags integration ./internal/ui/ -run TestSnapshot
git add internal/ui/
git commit -m "Let the interface answer for itself, on its own goroutine

a.running, the connection, the result buffer and the vim state all
belong to the interface's goroutine. Reading them from a socket handler
is a race, and the test harness solved this years ago by evaluating on
that goroutine — this takes the same route.

The context is the part that matters. Whatever asked is sitting at a
prompt, and a snapshot that waits forever on a wedged interface makes
dv status die with the thing it was asked about.

The statement's text is now held beside the stream that is running it.
The stream does not carry it, and what the database is being asked to do
is the first thing anyone observing wants to know."
```

---

### Task 3: A second socket, and two commands on it

**Files:**
- Modify: `internal/daemon/socket.go`, `internal/cli/cli.go`, `cmd/dv/runtime.go`
- Test: `internal/daemon/api_test.go`

**Interfaces:**
- Consumes: `snapshot.Source`, `snapshot.Handle` (Task 1); `App.Snapshot` (Task 2).
- Produces:
  - `func APISocketPath() (string, error)`
  - `func ServeAPI(ln net.Listener, src snapshot.Source)`
  - `func (s *Server) Info(dv string, started, now time.Time) snapshot.Server`
  - `dv api snapshot`; `dv status` from the session

- [ ] **Step 1: Write the failing test**

Create `internal/daemon/api_test.go`:

```go
package daemon_test

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"

	"github.com/Ahngbeom/datavase/internal/daemon"
	"github.com/Ahngbeom/datavase/internal/snapshot"
)

// Reading what a session is doing must not need a turn: a status line, a
// script and a person can all be looking at once.
func TestSeveralReadersAtOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dv-api.sock")
	ln, err := daemon.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	src := snapshot.Source{
		Server:  func() snapshot.Server { return snapshot.Server{PID: 7, DV: "test"} },
		Session: func(context.Context) (*snapshot.Session, error) { return nil, nil },
	}
	go daemon.ServeAPI(ln, src)

	for i := 0; i < 3; i++ {
		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatalf("reader %d: %v", i, err)
		}

		var got snapshot.Snapshot
		if err := json.NewDecoder(conn).Decode(&got); err != nil {
			t.Fatalf("reader %d decode: %v", i, err)
		}
		conn.Close()

		if got.Server.PID != 7 {
			t.Errorf("reader %d got PID %d, want 7", i, got.Server.PID)
		}
	}
}

// The API socket is a way to read what a production session is doing.
func TestAPISocketIsPrivateToItsOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dv-api.sock")
	ln, err := daemon.Listen(path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %04o, want 0600", perm)
	}
}
```

Add the `os` import.

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/daemon/ -run 'SeveralReaders|APISocket'`
Expected: build failure — `daemon.ServeAPI undefined`.

- [ ] **Step 3: Write the implementation**

Append to `internal/daemon/socket.go`:

```go
// APISocketPath is where the observation socket lives.
//
// A second socket rather than a second message on the first one: the client
// protocol is private, binary and high-frequency, and this is public,
// textual and rare. One socket carrying both would make each worse, and only
// this one may have several readers at a time.
func APISocketPath() (string, error) { return statePath("dv-api.sock") }

// ServeAPI answers snapshot requests until the listener closes.
//
// One response per connection: there is one question, and a connection that
// asked it has nothing more to say. Readers are unlimited because none of
// them can change anything.
func ServeAPI(ln net.Listener, src snapshot.Source) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			ctx, cancel := context.WithTimeout(context.Background(), snapshot.SessionTimeout)
			defer cancel()
			_ = snapshot.Handle(c, src, ctx)
		}(c)
	}
}
```

Add `context`, `net` and the `snapshot` import.

In `internal/daemon/daemon.go`, add what only the server can answer:

```go
// Info is the tier of a snapshot that comes from this process. It cannot
// fail, which is what lets dv status answer whatever the session is doing.
func (s *Server) Info(dv string, started time.Time, now time.Time) snapshot.Server {
	s.mu.Lock()
	attached := s.client != nil
	s.mu.Unlock()

	return snapshot.Server{
		PID:            os.Getpid(),
		StartedAt:      started.UTC().Format(time.RFC3339),
		UptimeSeconds:  int(now.Sub(started).Seconds()),
		ClientAttached: attached,
		DV:             dv,
	}
}
```

- [ ] **Step 4: Wire the commands**

In `internal/cli/cli.go`, add the field:

```go
	// APISnapshot fetches the observation snapshot as JSON.
	APISnapshot func() ([]byte, error)
```

and the command, in `Run`'s switch:

```go
	case "api":
		return a.api(args[1:])
```

```go
func (a *App) api(args []string) int {
	if len(args) != 1 || args[0] != "snapshot" {
		fmt.Fprintln(a.Err, "usage: dv api snapshot")
		return exitUsage
	}
	if a.APISnapshot == nil {
		fmt.Fprintln(a.Err, "this build has no observation socket")
		return exitUsage
	}
	out, err := a.APISnapshot()
	if err != nil {
		fmt.Fprintf(a.Err, "%v\n", err)
		return exitFailure
	}
	a.Out.Write(out)
	return exitOK
}
```

Add to `usage`, beside `dv status`:

```
  dv api snapshot       print what the running session is doing, as JSON
```

In `cmd/dv/runtime.go`, replace the placeholder `serverStatus` from Part 2 and
add the fetcher:

```go
// apiSnapshot asks the running server what it is doing.
func apiSnapshot() ([]byte, error) {
	path, err := daemon.APISocketPath()
	if err != nil {
		return nil, err
	}
	conn, err := daemon.Dial(path)
	if err != nil {
		return nil, errors.New("no dv server is running")
	}
	defer conn.Close()

	return io.ReadAll(conn)
}

// serverStatus is dv status: one paragraph for a person, from the same
// snapshot the API serves.
func serverStatus() (string, error) {
	raw, err := apiSnapshot()
	if err != nil {
		return "no dv server is running", nil
	}

	var s snapshot.Snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("the server answered with something unreadable: %w", err)
	}

	if s.Session == nil {
		return fmt.Sprintf("a dv server is running (pid %d), but the session did not answer: %s",
			s.Server.PID, s.SessionError), nil
	}

	line := fmt.Sprintf("%s on %s (%s), up %ds",
		s.Session.DataSource.Name, s.Session.DataSource.Host,
		s.Session.DataSource.Env, s.Server.UptimeSeconds)
	if !s.Server.ClientAttached {
		line += ", detached"
	}
	if s.Session.Statement.Running {
		line += fmt.Sprintf("\nrunning for %dms: %s",
			s.Session.Statement.ElapsedMS, s.Session.Statement.SQL)
	}
	return line, nil
}
```

And in `runServer`, open the second socket beside the first:

```go
	apiPath, err := daemon.APISocketPath()
	if err != nil {
		return err
	}
	apiLn, err := daemon.Listen(apiPath)
	if err != nil {
		return err
	}
	defer os.Remove(apiPath)
	defer apiLn.Close()

	started := time.Now()
	go daemon.ServeAPI(apiLn, snapshot.Source{
		Server:  func() snapshot.Server { return srv.Info(version.String(), started, time.Now()) },
		Session: sessionSnapshot,
	})
```

`sessionSnapshot` reaches the `*ui.App` the server built. Hold it where
`buildSession` returns it:

```go
// live is the interface the server is holding, for the observation socket to
// ask. It is written once, when the first client arrives, and read by the API
// goroutine; the mutex is for that and nothing else.
var (
	liveMu sync.Mutex
	live   *ui.App
)

func sessionSnapshot(ctx context.Context) (*snapshot.Session, error) {
	liveMu.Lock()
	app := live
	liveMu.Unlock()

	if app == nil {
		return nil, nil
	}
	return app.Snapshot(ctx)
}
```

Set `live` in `buildSession` just before returning the app.

Finally, in `main.go`, add `APISnapshot: apiSnapshot` to the `cli.App`
literal.

- [ ] **Step 5: Run the tests and try it by hand**

```bash
gofmt -l . && go vet ./... && go test -race ./internal/... && make build
```

Then:

```bash
./dv                          # start a session, type something, run it
# detach with Shift+F10
./dv status                   # names the datasource, says detached
./dv api snapshot | jq .      # the whole document
./dv api snapshot | grep -i password   # nothing
```

- [ ] **Step 6: Update the README**

`README.md` documents `dv` from the user's side and `CLAUDE.md` says to update
it when user-visible behaviour changes. This changes it three times over:
sessions now survive the terminal, `Shift+F10` detaches, and `dv status` and
`dv api snapshot` exist. Add a short section covering those, in the voice the
rest of the file uses.

- [ ] **Step 7: Commit**

```bash
git add internal/daemon/ internal/cli/ cmd/dv/ README.md
git commit -m "Answer what the session is doing, on a socket of its own

A second socket rather than a second message on the first. The client
protocol is private, binary and sent on every keystroke; this is public,
textual and asked for occasionally, and only this one may have several
readers at a time — none of them can change anything, so none of them
needs a turn.

dv status and dv api snapshot are the same document read two ways. The
server tier answers from this process and cannot fail, so status still
says what is running when the session itself has stopped talking, which
is exactly when someone is asking."
```

---

## What this leaves

The runtime is done. `dv` survives its terminal, says what it is doing to
anything that asks, and can still be run the old way with `--no-session`.

What was deliberately left out, and what would have to be decided before
adding it: remote attach (`dv --remote`), named sessions, self-update
channels, and any API that can change the session — the last of which needs a
`guard` policy designed alongside it, which is why nothing here can be made to
execute a statement.
