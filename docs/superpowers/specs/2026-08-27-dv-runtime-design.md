# Runtime Split: A Headless Server and an Attaching Client

**Status:** design approved, not yet implemented
**Date:** 2026-08-27

## Goal

`dv` becomes two halves of one binary: a headless server that owns the
session, and a thin client that owns the terminal. Closing the terminal, or
losing the SSH connection that carried it, stops being the same thing as
ending the session. A statement that is streaming keeps streaming; the editor
keeps its text; the result buffer keeps its rows. Running `dv` again attaches
to what is still there.

A second, read-only socket answers "what is this session doing" to anything
that asks — a status line, a script, an agent.

## Why

Today `dv` is one process. The interface, the connection, the SSH tunnel and
the statement in flight all die together when the terminal does. That is the
wrong bargain for the work this tool is for: the queries worth protecting are
exactly the long ones, and the connections worth protecting are exactly the
tunnelled ones that took a handshake to raise.

`herdr` solves the same problem for terminal workspaces, and its shape is the
one being borrowed: a headless server, a client over a unix socket, and a
separate socket for observation.

## Scope

**In.**

- A headless server holding the session, spawned on demand.
- A client that attaches, detaches, and re-attaches.
- One attached client at a time.
- A read-only observation socket, and `dv status` built on it.

**Out, deliberately.**

- Rewriting in Rust. `dv` is already a CGO-free single static binary; the
  language was never what was missing. The runtime shape was.
- Remote attach (`dv --remote`). `internal/tunnel` already carries SSH in the
  other direction — to the database — and attaching to a remote *runtime* is
  a separate feature with its own key handling and its own failure modes.
- Named multiple sessions.
- Self-update channels.
- A control API. The observation API is read-only by construction; see below.

## Approach

The server runs the real `ui.App` against a `tcell.Screen` whose terminal is
on the other side of a socket. The client owns a real screen, forwards key,
mouse and resize events up, and draws the cells that come back.

```
[terminal] --key/mouse/resize--> dv (client) --socket--> dv server
                                                            |
                                             App.SetScreen(proxy screen)
                                                            |
[terminal] <-----cell diff------ dv (client) <--socket------+
```

`internal/ui` is not modified. `App.SetScreen` already exists as production
API — the UI integration tests have driven the whole interface through it,
without a terminal, since they were written. The server is that seam used in
production.

### Rejected: semantic frames

`herdr` ships semantic frames (`render_encoding=SemanticFrame` in its server
log) and renders client-side. That requires the interface to be split across
the socket. `internal/ui` is 19,172 lines built the other way round, and its
integration harness would have to be rebuilt with it. The observation API
chosen here is read-only, so nothing else needs the semantic model either.

### Rejected: PTY relay

Running the TUI inside a PTY and relaying bytes (the tmux shape) is the
simplest to build and the least useful here: the observation socket would have
only bytes to read, and `dv` would be carrying a small terminal multiplexer it
did not ask for.

## Process lifecycle

Running `dv` is unchanged from the outside.

```
dv
 |- socket answers?        -> attach
 |- no socket / refused    -> spawn `dv server`, then attach
 `- spawn or socket fails  -> run monolithically, one line to stderr
```

The last branch matters. **The server is an optional convenience of the same
grade as the schema cache, the query history and the first-run marker.** A
home directory that cannot hold a socket costs persistence, never the session.
That is the bargain those three already make, and the reason it is stated in
their package comments.

The fallback prints one line to stderr, in the shape `openUI` already uses for
a cache it could not open.

### Commands

Existing commands are unchanged. Added:

| Command | Effect |
| --- | --- |
| `dv --no-session` | Run monolithically. The escape hatch, and what integration tests use. |
| `dv server` | Run the headless server in the foreground. Diagnostic. |
| `dv server stop` | Stop a running server. |
| `dv status` | Whether a server is up, on what datasource, and whether a statement is running. |
| `dv api snapshot` | The full observation snapshot as JSON. |

### Detach is not quit

`ActionQuit` keeps its meaning exactly: it ends the session, closes the
connection and the tunnel, and the server process exits with it.

`ActionDetach` is new. The client leaves; the server keeps the session.

Default bindings follow the existing pattern in `keymap/map.go` — a
`Ctrl`/`Cmd` chord with a function-key fallback, because modified keys need
the extended keyboard protocol: **`Ctrl`/`Cmd`+`\` and `Shift+F10`**.

`F1` through `F12` are all bound already, so the fallback has to share. It
shares with `ActionQuit` on `F10`, which is the relationship `map.go` has
encoded once before: `ActionRun` is `F5` and `ActionRunAll` is `Shift+F5`, the
only `Shift`+function key in the map. Quit and detach are siblings in exactly
that sense — both leave, differing in how much they leave behind.

`Ctrl+D` is not used; it reads as a neighbour of `Ctrl+Shift+D`, which already
switches datasource. `\` is unbound: the map binds only `space / a b c d e f g
i l n o q r s u v w x y`.

Adding the action touches five places, and that is the design working rather
than a cost: `keymap/action.go` (constant, both name maps, the full list),
`keymap/map.go` (binding), `ui/dialog.go` (the `Application` help group),
`ui/palette.go` (command palette), `ui/app.go` (dispatch). `dialog_test.go`
fails if the help entry is missing.

### Naming a different datasource against a live session

`dv open prod` while a session on `local` is up. Attaching silently would let
someone run a statement believing they are somewhere they are not — the
failure `internal/guard` exists to prevent.

- **No statement running:** attach, then perform the in-session datasource
  switch. This is what `ActionSwitchDataSource` already does, tunnel closing
  included, and the spine and top bar repaint the environment immediately.
- **A statement is running:** refuse, and say what the options are. Switching
  closes the session it leaves, and closing the session kills the streaming
  statement this whole design exists to protect.

```
a statement is running on "local".

  dv              attach to it
  dv server stop  end it and start again
```

### Sockets on disk

Both live in `$XDG_STATE_HOME/datavase/`, beside the schema cache and the
history — they are runtime state, not configuration. Mode `0600`.

- `dv.sock` — the client protocol.
- `dv-api.sock` — the observation API.

**A stale socket file is not a running server.** The client connects and reads
the answer; `ECONNREFUSED` means remove the file and spawn. Existence is never
the test.

On macOS `sun_path` is 104 bytes. An unusually long `XDG_STATE_HOME` makes the
socket unopenable, which is the optional-convenience fallback above.

### Warnings the server must not swallow

`openUI` writes warnings to stderr today — `completion disabled: …`, `no
worktree attached: …`. In the server those would land in a log the user never
reads, and completion would simply appear broken.

The attach handshake carries a `Warnings []string`, which the client prints to
its own stderr. It is populated once per session, not once per attach.

## The proxy screen — `internal/screen`

A `tcell.Screen` whose terminal is elsewhere. It knows nothing about sockets,
tview or `dv`: it holds cells, produces frames, and accepts events. The
transport is supplied by the caller.

### Built on `tcell.CellBuffer`, not `SimulationScreen`

`tcell.SimulationScreen` is the reference implementation to read, not a base
to embed. Three things make it wrong to build on:

- `Colors()` returns a hardcoded 256 and `HasMouse()` returns false. `dv`
  spends 752 lines on its palette and enables the mouse; both lies show up as
  rendering.
- Its `draw()` advances `x += width - 1`, so the cell behind a wide rune keeps
  whatever was there before. A grid of Korean text is where that surfaces.

`tcell.CellBuffer` is exported and gives what is actually needed:
`GetContent(x, y)` returns the width alongside the rune and style, and the
buffer already tracks a dirty flag per cell — the diff engine tcell's own
terminal backend uses.

### The 46 methods

| Group | Count | Handling |
| --- | --- | --- |
| Cell buffer | 9 | Delegate to the `CellBuffer`. |
| Capability | 6 | **Report the client's values, from the handshake.** |
| Forward to client | 6 | Cursor, title, clipboard, bell. |
| Events | 5 | A channel fed by the transport. |
| Frames | 2 | `Show` (diff), `Sync` (whole screen). |
| No-op / meaningless | 18 | `Suspend`, `Resume`, `Tty`, region locks, rune fallbacks. |

Capabilities are the rule worth stating twice: the client is the only source
of truth for colour depth, character set and mouse support. A screen that
misreports colours makes the palette degrade itself, and that cannot be
repaired on the far side of the socket.

### Frames

`Show` walks the buffer, collects dirty cells, clears their flags, and emits
them:

```go
x += max(width, 1)   // the cell behind a wide rune does not exist
```

That single line is the wide-rune answer. The trailing half is never put in a
frame; the client replays the cells it receives with `SetContent`, and the
real tcell screen computes width and handles the trailing cell itself. The
width travels with each cell for diagnosis, not for the client to predict
coordinates — every cell carries its own.

`Sync` calls `Invalidate()` and takes the same path. A whole frame is needed
in exactly two places, re-attach and resize, and tview already calls `Sync` at
both.

### While detached

With no transport, `Show` updates the buffer, clears dirty, and drops the
frame. The application never learns it is unobserved: the streaming statement
keeps calling `QueueUpdateDraw`, the result buffer keeps filling, the screen
keeps being drawn.

**This is why nothing has to be serialised.** There is no state to preserve
across a detach, because nothing is torn down. Re-attach is `Invalidate()` and
one whole frame.

Order matters when the size changed: `SetSize` → buffer resize →
`PostEvent(NewEventResize)` → tview relayouts and calls `Sync`. Posting the
resize before the size is updated relayouts against the old geometry.

Drawing while detached is wasted work of a few milliseconds per redraw. It is
kept: bypassing `QueueUpdateDraw` to skip it would put this code in
disagreement with tview's own state, which is a worse trade than the waste.

### Clipboard is two-way

`App` holds the screen for exactly one reason — OSC 52. Both directions are
needed:

- `SetClipboard(data)` travels to the client, whose real screen writes OSC 52.
- `GetClipboard()` asks the client; a reply arrives as `EventClipboard`.
  **Most terminals refuse, so no reply is the normal case.** `App` already
  keeps a session-local copy for pasting, which is the fallback — no timeout
  needs inventing.

## The protocol — `internal/proto`

Messages and framing over an `io.Reader`/`io.Writer`. It knows nothing about
sockets or screens, so `net.Pipe` tests all of it.

Encoding is `encoding/gob`: cell frames carry `rune`, `[]rune` and
`tcell.Style` directly, it is standard library (no CGO), and its stream
encoder sends type information once per connection. Both ends are `dv`, so
gob's Go-specificity costs nothing.

Messages are an envelope struct with a `Kind` and one pointer per payload,
rather than registered interfaces: the whole protocol reads in one file, and
there is no registration to forget.

### Handshake and version

`Hello` carries the client's capabilities, its version, `WorkDir`, the
datasource it was asked for, and its PID. The server replies `Welcome` (with
`Warnings`) or `Reject`.

**A version mismatch is refused outright.** With self-update out of scope, a
newly installed `dv` meeting a server from the previous release is a real
situation, and negotiating protocol compatibility buys nothing today.
`internal/version` already knows the answer exactly.

```
the running dv server is 0.6.3; this dv is 0.7.0.

  dv server stop   end that session and start again
```

A second client does not get a `Reject`: **the new client wins** and the old
one receives `Bye{replaced}` and exits cleanly.

### Backpressure: a one-slot frame queue

The UI goroutine must never block on the socket. If it did, a slow terminal
would stall reading rows from the database — and because MySQL will not accept
another statement until the result set is drained, that stalls cancellation
and schema browsing with it.

`Show` puts the frame in a one-deep slot and returns. A writer goroutine
drains it.

**If the slot is occupied, the frames are merged: for each coordinate, the
later cell wins. Cursor and title take the newest.** Cells present in the
waiting frame and absent from the new one survive.

Merging is sound because both frames are differences against the same
baseline — the screen the client is currently showing, having applied neither.
Applying two such differences in order and applying their merge give the same
screen.

What this costs is the chance to observe an intermediate value of a cell that
changed twice. What it buys is a bounded queue — one screen — and a UI
goroutine that cannot be stalled by a client that is slow, suspended or gone.
The final frame always has nothing after it to merge with, so the last state
is always delivered.

Discarding the waiting frame instead of merging would be the bug: a cell it
carried and the new frame does not would stay stale on screen forever.

### Disconnection

`EPIPE` on the writer or `EOF` on the reader puts the server into the detached
state. A client killed with `SIGKILL`, or an SSH connection that dropped, is
the same thing. **It is a detach, not a shutdown** — surviving that is the
point.

When the application ends on its own, the server sends `Bye{quit}`, removes
its sockets and exits.

## The observation API — `internal/snapshot`

JSON lines over `dv-api.sock`, one response per request, many concurrent
readers. `snapshot` is the only command; a `watch` event stream can be added
later without changing it.

### Two tiers, so `dv status` cannot hang

`App` state belongs to the UI goroutine. Reading it from anywhere else is a
data race — the UI test harness solved this already, evaluating conditions on
the interface's own goroutine through `QueueUpdate`. The API takes the same
route, and inherits the same risk: `QueueUpdate` waits when the UI goroutine
is busy.

So the snapshot is split:

| Tier | Source | Needs the UI goroutine | Can fail |
| --- | --- | --- | --- |
| `server` | the server process | no | no |
| `session` | `ui.App` | yes | yes — 2s timeout |

**`dv status` therefore always answers.** Whatever has gone wrong inside the
session, the server tier still reports that a server is up, its PID, and
whether a client is attached. A diagnostic that dies with the thing it is
diagnosing is worse than none.

On timeout, `session` is null and a reason is given. Nothing waits.

### What the snapshot carries

An allow-list, not a serialised struct, so a field added to `App` later cannot
leak through.

```json
{
  "version": 1,
  "dv": "0.7.0",
  "server": {
    "pid": 12345,
    "started_at": "2026-08-27T09:12:03Z",
    "uptime_seconds": 3600,
    "client_attached": true
  },
  "session": {
    "datasource": {
      "name": "prod-ro", "env": "production",
      "host": "db.internal", "port": 3306, "user": "reader",
      "database": "app", "tunnel": true,
      "server_version": "8.0.36"
    },
    "schema": "app",
    "statement": {
      "running": true,
      "started_at": "2026-08-27T10:31:00Z",
      "elapsed_ms": 4210,
      "kind": "select",
      "sql": "SELECT ...",
      "guard": { "verdict": "allow", "injected_limit": 1000 }
    },
    "result": {
      "columns": ["id", "customer", "total"],
      "row_count": 4213,
      "truncated": false,
      "error": null
    },
    "batch": { "running": false, "completed": 0, "total": 0 },
    "worktree": {
      "path": "/home/…/reports", "branch": "main",
      "open_file": "monthly.sql", "modified": true
    },
    "editor": { "lines": 42, "modified": true },
    "mode": "normal"
  }
}
```

### What it does not carry, and why

**No row data.** `result` gives column names and a count. `result.Buffer`
holds `rows [][]any` — production data that is written to no log and no
history, and the only class of thing here that is not recorded anywhere else.
`internal/export` exists for data, and a person chooses it explicitly. An
observation API must not become a quiet exfiltration path.

**No editor text.** Line count and a modified flag only. The running statement
is a fact about what the database is doing; the editor buffer is an unrun
draft, and the two are different kinds of thing.

**The running statement's SQL is included.** It is the point of observing, and
`internal/history` already writes every executed statement's full text to
`query_history.sql_text` on disk in plain text. This creates no new class of
exposure, over a socket narrower than that file.

**Passwords are structurally unreachable.** `config.DataSource` has no
password field — secrets live only in `internal/secret` — and the session tier
reads datasource facts from that type alone.

### Read-only by construction

The API server is not given `*ui.App`. It is given two functions:

```go
type Server struct {
    Server  func() ServerInfo
    Session func(context.Context) (*SessionInfo, error)
}
```

Holding nothing that can mutate makes a control path impossible to add by
accident. When control is wanted, it arrives as a separate type designed
together with a `guard` policy for it — and not opening the door today is what
keeps that decision available.

### Field names are pinned by a golden test

The API is an interface other things depend on; a renamed field breaks every
script silently. A golden JSON test in `internal/snapshot` serialises a known
snapshot and compares it to a fixed string. Adding a field updates the golden;
renaming or removing one is made loud.

This is the same rule the repository already applies twice — every action
exactly once in `helpGroups` (`ui/dialog_test.go`), every sequence in
`vim.Reference()` typed into a real state machine (`vim/vim_test.go`).

## Package boundaries

| Package | What it does | What it deliberately does not know |
| --- | --- | --- |
| `internal/screen` | a `tcell.Screen` whose terminal is elsewhere | sockets, tview, `dv` |
| `internal/proto` | wire messages, codec, the frame slot | sockets, screens |
| `internal/snapshot` | builds and serves the observation snapshot | `ui.App`, sockets |
| `internal/daemon` | the headless server: owns `App`, sockets, lifecycle | terminals |
| `internal/attach` | the client: owns the real terminal | **`ui`, `db`, `config`** |

The last row is the one to hold. `attach` does not know that `dv` is a
database client. It draws cells and sends keys. As long as that holds, the
client is unaffected by anything the server chooses to draw.

## Error handling

| Situation | Behaviour | What the user sees |
| --- | --- | --- |
| Socket cannot be created | run monolithically | one line on stderr |
| Server spawn fails | run monolithically | one line on stderr |
| Stale socket file | remove, spawn | nothing |
| Version mismatch | `Reject` | how to stop the old server |
| Statement running, other datasource named | `Reject` | the two options |
| Second client attaches | first gets `Bye{replaced}` | the old terminal exits cleanly |
| Client killed / SSH dropped | **detach**, not shutdown | the next `dv` resumes |
| User quits | server exits, sockets removed | back at the shell |
| `session` snapshot times out | `server` tier still answered | `dv status` still works |

## Testing

The existing 21,741 lines of tests stay valid. `internal/ui`'s integration
tests drive `App` monolithically through a `SimulationScreen`; this work adds
a layer beneath them, so they remain the regression net.

### Layer 1 — pure units

`screen`, `proto`, `snapshot`, each without a terminal, a socket or a
database.

- Drawing the same content twice produces an empty second frame.
- One Korean character produces one cell with `Width == 2`.
- `Sync` produces the whole screen.
- `Colors()` reports the handshake value; returning 256 fails.
- Merging two frames and applying the result gives the same screen as
  applying both in order. **The claim is about the resulting screen, not the
  cell list.**
- A `Hello` with a different version is rejected.
- `Show` called a hundred times with a blocked writer does not stall the
  caller.
- The snapshot JSON contains no row array, and no `password` or `secret`
  string.
- Golden JSON match.

### Layer 2 — round trip over `net.Pipe`

`daemon` and `attach` joined in one process, the server driving a real
`ui.App` through a proxy screen, the client drawing onto a `SimulationScreen`.

**The assertion: the server's cell buffer and the client's screen match.**

- A key press changes both alike.
- They still match after a resize — a wrong order between `SetSize`,
  `EventResize` and `Sync` breaks this.
- **Changing the screen while detached and then re-attaching delivers the
  whole screen.** This pins the missing-`Invalidate()` bug: `Show` clears
  dirty flags while detached, so without it the client receives an empty
  frame.
- A grid containing Korean text matches on both sides.

The harness wraps `internal/ui`'s, keeping its `h.do(action)` and
`h.waitFor(...)` discipline. "Settle N times, then read" is a guess that fails
a few runs in twenty, and a socket makes it worse.

### Layer 3 — real sockets under `t.TempDir()`

- A stale socket file is removed and replaced.
- The socket is mode `0600`.
- A mismatched version is rejected.
- A second client displaces the first.
- **A socket that cannot be created falls back to monolithic and prints one
  line to stderr.** Without this, a read-only home directory costs the whole
  session rather than one convenience.

### Layer 4 — a real database (`//go:build integration`)

What the work exists for, against the MariaDB container on port 13306:

- Start a slow statement, detach, let it stream, re-attach: the rows are
  there.
- With a statement running, `dv open <other datasource>` is refused.

## Build order

Each step is complete and tested without the next. TDD applies throughout:
the failing test first, failing for the right reason.

1. `internal/screen` — layer 1. Nothing uses it yet.
2. `internal/proto` — layer 1.
3. `internal/daemon` + `internal/attach` over `net.Pipe` — layer 2. **The
   first point at which a screen crosses a socket, and the peak of the risk.**
4. Real sockets and lifecycle — spawn, stale sockets, versions, fallback.
   Layer 3.
5. `ActionDetach` — the five places. `dialog_test.go` catches an omission.
6. `internal/snapshot` and `dv status` — independent of the above.
7. Layer 4 integration tests.

Nothing before step 3 changes existing behaviour. After step 3, the rest is
wiring.

## Size

| Package | Production | Tests |
| --- | --- | --- |
| `screen` | ~400 | ~350 |
| `proto` | ~400 | ~400 |
| `snapshot` | ~300 | ~300 |
| `daemon` | ~500 | ~450 |
| `attach` | ~350 | ~250 |
| `cli`/`main` wiring, `keymap`/`ui` edits | ~350 | ~200 |
| **Total** | **~2,300** | **~1,950** |

About 10% on top of the existing 42,000 lines. `internal/ui`'s 19,172 lines
are not touched.

## Risks and deferred decisions

**Keychain access from a daemon.** `secret.NewKeychain()` is called by the
server once it is a detached process rather than by a foreground command.
macOS keychain behaviour for a background process needs measuring before step
4 is called done. `secret.WithEnv` is the existing fallback, so this degrades
rather than breaks, but it could spoil a first run.

**Version negotiation.** Refusing on any mismatch is deliberately blunt. It
should be revisited if and when self-update lands, because that is when a
handoff between versions becomes routine rather than exceptional.

**Frame merging drops intermediate values.** Accepted. Row counters may skip;
spinners may stutter. The final state is always correct.

**Drawing while detached is wasted work.** Accepted, for the reason above.
