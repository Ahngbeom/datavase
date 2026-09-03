# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`dv` — a terminal MySQL/MariaDB client (a DataGrip/DBeaver alternative), built
as a CGO-free single static binary. `README.md` documents it from the user's
side; read it before changing user-visible behaviour, and update it when that
behaviour changes.

## Commands

```sh
make build              # CGO_ENABLED=0 go build -o dv ./cmd/dv
make test               # unit tests, no database needed
make db-up              # start MariaDB 11.4 in Docker on port 13306
make test-integration   # every test, including those needing that database
make lint               # go vet ./... && gofmt -l .
make db-down            # remove the test container
make db-shell           # MariaDB shell against the test database
```

Running one test, or one package:

```sh
go test ./internal/vim/ -run TestNormalModeSequences
go test -tags integration ./internal/ui/ -run TestVimOperators -v
go test -race -tags integration ./internal/ui/
```

Releases are cut by pushing a `v*` tag, which runs goreleaser
(`.goreleaser.yaml`) via `.github/workflows/release.yml`. Validate a change to
that config **before** tagging, because a broken one only surfaces at tag time:

```sh
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```

The version string comes from `internal/version`: goreleaser injects the tag
via ldflags, and a `go install`ed binary falls back to the module version from
`debug.ReadBuildInfo`. `cli.HandleVersion` runs in `main` before flag parsing
and before configuration is read, so `dv version` works on a machine that has
never been set up.

Integration tests carry `//go:build integration` and are invisible without the
tag — `go test ./...` compiles but does not run them. They talk to a real
server on port 13306 (deliberately not 3306, so they can never reach a
developer's own MySQL). `internal/testmysql` holds the defaults and the
`DATAVASE_TEST_*` overrides.

## Architecture

### The dependency rule

Every package that holds interesting logic is pure and testable without a
terminal or a database, and the UI is the only thing that knows about tview.
Read the package doc comment before working in a package — each one states
what it deliberately does *not* know, and those boundaries are load-bearing:

- `sqlparse` — a tokenizer, not a parser. Answers only the questions `guard`
  and the editor ask: statement boundaries, statement kind, top-level `WHERE`.
  Also owns `QuoteIdentifier`, which both `catalog` and `db` need (`catalog`
  imports `db`, so it could not live in either).
- `guard` — `Evaluate(stmt, policy) Decision` is a pure function. **Fail-closed:**
  anything the tokenizer cannot classify is refused against production.
- `keymap` — key events → named `Action`s. The UI switches on actions, never
  on keys.
- `vim` — the modal state machine. Keys in, `Command`s out; no widgets.
- `complete`, `result`, `export`, `config` — same pattern.

`ui` is the largest package and is split by concern rather than by widget:
`editor.go`/`edit.go`/`motion.go` (text), `vimedit.go`/`vimmotion.go`/`vimnav.go`
(modal), `status.go`, `tree.go`/`tables.go`/`ddl.go` (schema pane),
`grid.go` (results), `searchbox.go`/`palette.go`/`goto.go`/`useschema.go`
(dialogs).

### Two connections per datasource

`db.Conn` holds a **query pool** and a separate **control connection**.
MySQL will not accept another statement until the current result set is fully
read, so without the split a long stream would block both cancellation and
schema browsing — the two things most needed while a long stream is running.

The control connection is reached only through `WithControl(ctx, fn)`, never
handed out. Concurrent use desynchronises the MySQL protocol ("commands out of
sync"), after which the driver discards the connection permanently and
cancellation plus every catalog read die with it. Do not add an accessor that
returns the `*sql.Conn`.

### Streaming and cancellation

`Conn.Query` returns a `*Stream` immediately; connecting, sending and reading
all happen in the background. The stream has **no terminal event** — it ends
when `Events` closes, and the reason is read from `Err()`/`Truncated()`
afterwards, following the `sql.Rows` pattern. Cancellation is real: the
connection id is read on the very connection that will run the statement, so
`KILL QUERY` can target it.

`Options.Schema` issues `USE` on the executing connection. Because pooled
connections are handed round, once *any* query has switched schema every later
query states its schema explicitly (`Conn.schemaEverSwitched`); until then the
round trip is skipped.

### Local schema cache

`catalog.Cache` is a WAL-mode SQLite database under `XDG_STATE_HOME`.
Completion and the tables tab read from it rather than `information_schema`,
which is far too slow to run on a keystroke. The cache and the history store
are both **optional** — a read-only home directory should cost completion, not
the session.

### Keyboard presets

`keymap.FromConfig(preset, overrides)` is the single place configuration
becomes a key map; both `cmd/dv` and `dv keys` go through it so the reference
cannot disagree with the interface. Presets share one base map and only rebind
where the tools genuinely differ.

**The default preset is `datagrip`,** so typing types. `vim` is one answer to
`dv init` or one palette command away, and everything below is what makes the
modal editor survivable *for the people who choose it* — none of it may be
dropped on the grounds that it is no longer the default:

- the status bar always shows the mode and any half-typed sequence
- normal mode consumes **every** key — one leaking through gets typed
- the empty-editor placeholder says how to start typing
- the help screen opens with the way out, above the vim reference rather than
  at the foot of it
- `dv init` asks which keyboard before writing a config, so nobody meets the
  modal editor without having chosen it

`a.keys` is consulted at event time, so swapping the map switches keyboards
mid-session with no rebinding.

### The first ten minutes

Three things exist only for someone who has not used this before, and each is
built so it cannot go stale:

- **`internal/cli.Wizard`** (`dv init`) is the config package's only writer. It
  probes before it writes, and its env and preset prose is checked against
  `config`'s and `keymap`'s own values.
- **`startHere`** (`internal/ui/dialog.go`) is the help screen's opening five,
  rendered from the live key map. It deliberately repeats entries from
  `helpGroups`, which keeps its own exactly-once rule.
- **`internal/intro`** is one bit — whether the first-run card has been shown —
  stored as a file's existence under `XDG_STATE_HOME`. Optional like the cache
  and the history: a marker that cannot be written costs the card being shown
  again, never the session. An empty `Deps.IntroPath` means never show it,
  which is what every test that is not about the card gets.

## Conventions

**TDD.** Write the failing test first and watch it fail for the right reason.
When a test goes from build-failure straight to green, mutate the
implementation to confirm the test actually bites.

**Comments say why, not what.** The existing comments explain the failure a
piece of code exists to prevent — a corrupted protocol, a destroyed undo
history, a silently partial result. Match that; do not add narration.

**Tests state the consequence.** Test names and comments describe the user-
visible failure being pinned down ("the injected LIMIT warning was dropped"),
not the function being called.

**Nothing exists in production code for a test's sake.** The UI test harness
wraps the application's input capture from the outside rather than adding a
counter to `App`.

### UI tests

`internal/ui` runs the real interface against a tcell `SimulationScreen`. The
harness is in `app_integration_test.go`.

- `h.do(action)` presses an action's real binding — tests name actions, not
  keys, so rebinding does not invalidate behavioural tests.
- `h.waitFor(what, cond)` / `h.inspect(cond)` evaluate on the interface's own
  goroutine. **Use these rather than `settle()` then read.** An injected key
  goes onto the screen's event queue while `QueueUpdateDraw` uses the
  application's, and tview interleaves the two; "settle N times, then read" is
  a guess that fails a few runs in twenty. Reading `App` state from the test
  goroutine is also a data race.
- The shared harness pins the **datagrip** preset: those tests predate the
  modal editor. Modal behaviour uses `newVimHarness` (`e3_integration_test.go`).
- `h.buffer(text, caret)` sets the caret in a separate tick from `SetText`.
  tview's `TextArea` rebuilds its row index lazily, so a `Select` issued in the
  same tick resolves against the text that was just discarded and lands on the
  wrong line.

### Things that are checked automatically

- Every bindable action must appear exactly once in `helpGroups`
  (`internal/ui/dialog_test.go`) — otherwise a key works with no way to
  discover it.
- Every sequence in `vim.Reference()` is typed into a real state machine
  (`internal/vim/vim_test.go`) — the help cannot advertise a key that stopped
  working.

### Impossible key combinations

`⌘⇥` is the macOS application switcher and can never reach a terminal.
`Ctrl+I` is byte 0x09, indistinguishable from `⇥`. Modified keys such as
`Ctrl+Enter` need the extended keyboard protocol, which is why every binding
has a function-key fallback.
