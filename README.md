# datavase

[![ci](https://github.com/Ahngbeom/datavase/actions/workflows/ci.yml/badge.svg)](https://github.com/Ahngbeom/datavase/actions/workflows/ci.yml)

A terminal MySQL/MariaDB client. Single static binary, no runtime, no IDE.

![dv opens a read-only datasource, runs a query, saves the result as a CSV file, and refuses an UPDATE](docs/demo.gif)

Four things, and no more:

- **Datasources.** Keep as many as you like; connect, add, edit and delete them
  from inside.
- **Look at a table.** Double-click one in the tree, or Enter on one in the
  tables tab, for its first hundred rows.
- **Run a statement** with `⌘↩` (`Ctrl+↩`, or `F5`).
- **Read the answer** in a grid, and copy what you need out of it: the cell
  with `⌘C`, the row with `⌘⇧R`, the whole result as Markdown or JSON with
  `⌘⇧C` — or as a CSV file on disk, from the same key.

Also there, because the four need them: an SSH tunnel to a bastion, table and
column completion, a searchable history of what you ran, passwords in the OS
keychain, TLS.

Not there, on purpose: a modal editor, a command palette, a production guard,
an explain plan, the process list, sessions that outlive the terminal. v0.8.x
is the last release with those.

## Install

macOS or Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/Ahngbeom/datavase/main/install.sh | sh
```

Homebrew:

```sh
brew install Ahngbeom/tap/dv
```

Linux packages (`.deb`, `.rpm`, `.apk`) and a Windows `.zip` are on the
[latest release](https://github.com/Ahngbeom/datavase/releases/latest).

Check it with `dv version`, then run `dv`.

Nothing else is needed, and nothing else is wanted: no runtime, no drivers,
no companion tools. It is one binary.

Upgrading an existing setup: [CHANGELOG.md](CHANGELOG.md) says what changed
and whether anything needs doing to your configuration first.

<details>
<summary>Other ways in</summary>

With a Go toolchain, which builds it rather than downloading it:

```sh
go install github.com/Ahngbeom/datavase/cmd/dv@latest
```

From a clone:

```sh
make build      # produces ./dv — CGO-free, single static binary
```

The install script takes `DV_VERSION` to pin a release and `DV_INSTALL_DIR` to
choose where the binary goes; it defaults to `/usr/local/bin`, or
`~/.local/bin` when that needs root.

**Downloading an archive with a browser on macOS** gets the file quarantined,
and this binary is not notarized, so macOS will refuse to run it. The script
and Homebrew both avoid that. If you took the archive by hand:

```sh
xattr -dr com.apple.quarantine ./dv
```

</details>

## First run

`dv` with no configuration opens the datasource list. `a` adds one; the form
asks for host, user, password and the rest, `Test` tries it against the server,
and `Save` writes `~/.config/datavase/config.yaml` and puts the password in the
keychain. `↩` on an entry connects.

The same list is `⌘⇧D` (`Ctrl+Shift+D`, or `F11`) once you are inside, where
`e` edits an entry and `d` deletes it along with its stored password.

With exactly one datasource configured, `dv` opens it directly.

## Configure

`~/.config/datavase/config.yaml` (or `$XDG_CONFIG_HOME/datavase/config.yaml`):

```yaml
datasources:
  - name: local
    host: 127.0.0.1
    port: 3306
    user: root
    database: app_db

  - name: app
    host: db.internal        # as named from the bastion
    user: readonly
    read_only: true          # the server refuses every write
    tls: verify-identity
    tunnel:
      host: bastion.example.com
      port: 22
      user: bahn

defaults:
  auto_limit: 1000        # LIMIT added to unbounded SELECTs
  fetch_chunk: 500        # rows per batch while streaming
  buffer_max: 50000       # rows held in memory before truncating
  mouse: true             # false turns off clicks; see "The screen"
  history: true           # false stops statements reaching disk at all
```

Every value above is the default, so a key left out behaves as the line
shows it.

The datasource dialog writes this file, so comments in it do not survive a
save. Editing it by hand still works; unknown keys are rejected rather than
ignored, so a typo like `hots:` fails immediately instead of surfacing later as
a confusing error.

An absent `tls:` means `preferred`. If you are coming from 0.8 with `env: prod`
and no `tls:`, that datasource still gets `required` — write `tls: required`
down now, because `env` stops being read in a later release.

**`read_only: true`** has the server refuse every write on that datasource,
on every connection, so nothing the client failed to recognise as a write
can get through. A refused statement says which setting refused it.

**The server is asked to confirm it, and the datasource does not open until
it does.** `read-only` on the top line is the server's answer about the
session your next statement will run on, not a repetition of what this file
asked for — a marker that only read the configuration would keep saying
"read-only" over a session that had stopped being one. If the server will
not confirm, `dv` says so and connects to nothing, because a protection that
is believed and absent is worse than one that was never claimed.

It is still a guard against a slip rather than a boundary:
`SET SESSION TRANSACTION READ WRITE` lifts it for the session, and whoever
types that has decided to. The next statement puts it back.

Passwords never go in this file. Store them in the OS keychain — macOS
Keychain, Windows Credential Manager, or Secret Service on Linux:

```sh
dv auth app               # prompts, echo off
dv auth -rm app           # remove
```

**On a machine with no keychain** — a headless Linux server runs no D-Bus
Secret Service, so there is nothing for `dv auth` to write to, and a plain
WSL2 shell is in the same position unless it has a desktop session to reach
one — pass the password in the environment instead:

```sh
export DATAVASE_PASSWORD_APP=...
```

The variable is `DATAVASE_PASSWORD_` followed by the datasource name, upper
cased, with anything that cannot appear in a variable name turned into `_`.
It is also how you supply a password in CI or a container. When it is set it
takes precedence over the keychain, so the same command does the same thing on
every machine.

## Use

```sh
dv                    # the datasource list, unless exactly one is configured
dv open app           # open a named datasource
dv ls                 # list datasources and whether a password is stored
dv auth app           # store a password in the keychain
dv check app          # verify reachability, then exit
dv version            # print the version
dv help               # the same list, from the binary
```

**`dv check` names what to look at.** A connection that did not happen is
reported the same way by every hop — a wrong host, a closed port, a VPN that
is down, a certificate, a password, a database that is not there, a database
this account is not granted, a server that turns this machine away — and
which one it was decides who can fix it. Where `dv` can tell them apart it says so
on the line after the server's own words, naming the setting or the thing
outside this program that decides it. The same sentence appears on screen
when connecting or reconnecting fails.

`-c path/to/config.yaml` points any of them at another configuration file.

### Keys

**`⌘` and `Ctrl` do the same thing** — use whichever your hands reach for.
Inside tmux the interface shows the `Ctrl` spelling even on a Mac, since tmux
does not forward `⌘` to the program it hosts.

| Action | Keys | Mouse |
|---|---|---|
| Run the statement under the cursor | `⌘↩` · `Ctrl+↩` · `F5` | |
| Run everything | `⌘⇧↩` · `Ctrl+Shift+↩` · `Shift+F5` | |
| Cancel the running statement | `⌘F2`, or `⌘C` while one is running | |
| Preview a table (`LIMIT 100`) | `↩` in the tables tab | double-click it in the tree, click it in the tables tab |
| Copy the selection or the cell | `⌘C` | |
| Copy the selected row, tab separated | `⌘⇧R` · `F8` | |
| Copy the whole result as Markdown or JSON, or save it as a CSV file | `⌘⇧C` · `F3` | `copy` on the result header |
| Datasource list | `⌘⇧D` · `F11` | the datasource name in the top bar |
| Choose the schema | `⌘⇧N` · `F7` | the schema name in the top bar |
| Reload the schema tree, or reconnect a dropped session | `⌘R` | |
| Hide or show the schema tree | `⌘B` | |
| Move between panes | `⇥` / `⇧⇥` | a pane's name |
| Switch tab in the focused pane | `Ctrl+⇥` · `F6` | a tab |
| Complete the word at the cursor | `^Space` | |
| Find in the editor or the results | `⌘F` | |
| Next / previous match | `⌘G` / `⌘⇧G` | |
| Search the query history | `⌘⇧F` · `F9` | |
| Show the selected result row in full | `⌘I` · `F4` | double-click a row |
| Sort the results by a column | `⌘⇧S` · `F12` | a column header |
| Key reference | `F1` | `F1 keys` in the top bar |
| Quit | `⌘Q` · `F10` | |

The editor's own keys are the ones every editor already uses:

| Action | macOS | Windows / Linux |
|---|---|---|
| Cut / paste | `⌘X` `⌘V` | `Ctrl+X` `Ctrl+V` |
| Select all | `⌘A` | `Ctrl+A` |
| Comment or uncomment | `⌘/` | `Ctrl+/` |
| Duplicate the line | `⌘D` | `Ctrl+D` |
| Delete the line | `⌘Y` | `Ctrl+Y` |
| Move one word | `⌥←` `⌥→` | `Ctrl+←` `Ctrl+→` |
| Extend the selection one word | `⌥⇧←` `⌥⇧→` | `Ctrl+Shift+←/→` |
| Start / end of line | `⌘←` `⌘→` | `Home` `End` |
| Select to start / end of line | `⌘⇧←` `⌘⇧→` | `Shift+Home` `Shift+End` |
| Delete the word before the cursor | `⌥⌫` | `Ctrl+⌫` |
| Delete to the start of the line | `⌘⌫` | — |

Cursor movement is the one place `⌘` and `Ctrl` are **not** interchangeable,
because macOS uses them for different things: `⌘←` goes to the start of the
line while `⌥←` and `Ctrl+←` move by word.

A word here is a SQL identifier, so `user_id` moves as one — not as `user`,
`_` and `id`, which is how a generic text editor would split it.

Undo is `Ctrl+Z`. It belongs to the text widget rather than to the key map,
which is why it has no `⌘` spelling.

**A function key carries every chord that a terminal might not deliver.**
Modified keys such as `⌘↩` need the extended keyboard protocol, which not every
terminal speaks and tmux does not turn on by default; `⌘` bindings additionally
need a terminal willing to forward `⌘` rather than keep it for its own menus.
`F5` always runs, whatever the terminal does with the rest.

Two combinations are impossible rather than merely awkward: `⌘⇥` is the macOS
application switcher, and `Ctrl+I` is byte 0x09 — the same as `⇥`.

`F1` prints the key reference from the map in force, so it cannot drift out of
step with what the keys actually do.

### The screen

```
█ app @app_db  root@db.internal:3306  read-only           F1 keys
█────────────────────────────────────────────────────────────────
█▌
█ SELECT id, email FROM users
█   WHERE id = 1;
█────────────────────────────────────────────────────────────────
█ ▸results                                             ⌘⇧C copy
█ id      │email
█ 1       │ada@example.com
█────────────────────────────────────────────────────────────────
█ 1 row  ·  6ms
```

**The top line is where you are; the bottom line is what just happened.** The
datasource, the schema an unqualified statement will reach, the account and
server behind them, and whether writes are refused all stay put up top. The
datasource name was chosen by whoever wrote the configuration file and two of
them can be one letter apart; `user@host:port` is the fact, and the port is
part of it because two tunnels forwarded to two ports on localhost are
otherwise the same string. The schema pane names the server too, until `⌘B`
puts the pane away. On a terminal too narrow for all of it the account and
server give way, and the datasource, the schema and `read-only` do not.
Row counts, timings, warnings and the SQL a table preview ran go below. The
datasource is filled rather than merely coloured, so which server this is
survives a terminal of any width; the column down the left is the same fill
continued, and it is the only thing on screen that ignores your terminal theme.

Regions are separated by a single rule rather than boxed. Each names itself
once, on its own header line, and the one holding the keyboard is marked `▌`.

**The schema pane** is open when a session starts, and `⌘B` puts it away
for that session. It has two tabs. **tree**
expands a schema for its tables and a table for its columns, marks the schema
unqualified names resolve against with `●`, and previews a table on a
double-click; `↩` on a column node puts its name in the editor. **tables** is a
flat, filterable list of the current schema's tables with row estimates, read
from the local cache so it fills instantly, and `↩` there previews a table too.
A preview never touches the editor — the text in it is yours.

**The schema tree** is there when a session opens, on the left, with a tables
tab beside it. `⌘B` puts it away when the width is wanted for the result, for
that session. The wheel moves the selection through it, so wherever you stop
is where a click lands.

**Each pane names its own keys** in its header, while the keyboard is in it —
running and completion in the editor, the preview in the tree, copy and sort
and row in the result. Move between panes with `⇥` to see the rest; `F1` has
all of it at once.

**When the connection goes** — an idle timeout, a dropped network, a laptop
that slept — the statement that discovers it says the connection is gone and
names the key that opens a new session. `⌘R` is that key; on a live session
it still reloads the tree, and there is nothing to reload on a dead one.
Reconnecting keeps the schema you had chosen, because dropping back to the
datasource's default without a word is how the right statement comes to be
run against the wrong schema, and it confirms `read_only` again on the new
session. Nothing is re-run: the statement in the editor is yours to send
again or not.

**The editor** is an ordinary one: typing types, and there is no mode to leave
first. **The grid** streams the result as it arrives, sorts on a column and
opens one row down the page rather than across it.

**Clicks mean something.** The datasource, the schema or `keys` in the top bar
does what pressing its key does; a tab or a region name moves focus there; a
column header sorts by it; `copy` on the result header copies the whole result;
double-clicking a result row opens it in full, and double-clicking a table in
the tree previews it.

**Three sizes of copy.** `⌘C` in the results takes the cell under the cursor;
`⌘⇧R` takes that whole row, tab separated, so it lands in a spreadsheet as
columns; `⌘⇧C` takes the entire result as Markdown or JSON, or writes it to a
CSV file, which is where fifty thousand rows belong rather than on a
clipboard. It shows a name made of the datasource and the moment, which `↩`
takes and anything you type replaces, and refuses
a path that is already a file. `⌘C` in the editor takes the selection, as it
does anywhere else.

**Copying** goes two ways at once. The terminal is asked to take the text —
the only route that reaches your own clipboard when `dv` is running over SSH —
and a local session also hands it to `pbcopy` on macOS, `clip` on Windows, or
`wl-copy`/`xclip` on Linux, whichever of those is actually installed; without
one, only the terminal route is tried. That second route is there because the
first is a request the terminal may refuse: Ghostty asks before allowing it,
iTerm2 keeps it off until "Applications in terminal may access clipboard" is
ticked, tmux drops it without `set -g set-clipboard on`, and Terminal.app has
never implemented it.

WSL runs the Linux binary, so it takes the Linux route above — `wl-copy` or
`xclip`, which need a Wayland or X11 session to find. A plain WSL2 shell has
neither unless WSLg (GUI app support) is set up, which leaves only the
terminal route; check whether the terminal on the Windows side honours OSC 52
before relying on copy inside one.

Over SSH only the terminal route is used. Running a helper on the far end would
put the text on a clipboard nobody is sitting at, so if the terminal refuses
there, the copy has nowhere to go — that is the one case where the setting
above has to be found.

Set `mouse: false` under `defaults` to turn all of that off, for anyone who
selects text by dragging: mouse reporting in a terminal disables its own native
selection, so a click that means something is a regression rather than a
feature for them. **This makes `dv` ignore the mouse; it does not hand the
terminal its selection back.** Mouse reporting is turned on by the client, for
its own terminal, independently of this setting. Everything the mouse can reach
is on the key table above.

## What this program leaves on the machine

| What | Where |
|---|---|
| Datasources | `$XDG_CONFIG_HOME/datavase/config.yaml`, or `~/.config/datavase/config.yaml` |
| Statement history | `$XDG_STATE_HOME/datavase/history.db`, or `~/.local/state/datavase/history.db` |
| Schema cache | the same directory, `datavase.db` |
| Passwords | the OS keychain, or `DATAVASE_PASSWORD_<NAME>` in the environment |

These paths are the same on every platform dv builds for, including native
Windows: home directory plus `.config`/`.local/state`, never `%APPDATA%`.
There is no separate Windows convention to look for.

The two databases live in a directory created `0700`, which is what protects
them; the files inside it are `0644` — Windows has no such permission bits,
so this protection is macOS/Linux/WSL only. Deleting either is safe at any
time — the cache is rebuilt on the next reload, costing completion and the
tables tab until then, and the history is simply gone.

Statements are written unless you say otherwise: **`history: false` under
`defaults` stops them being written at all,** and the file is never created.
Reach for it where the statements themselves are the sensitive thing.
Nothing else is recorded: there is no telemetry, no update check and no
crash reporting, so the only things `dv` connects to are your database and
the bastion in front of it.

Going back a version is downloading an older one; no release writes state an
earlier one cannot read. The install script takes `DV_VERSION` for that, and
[SECURITY.md](SECURITY.md) has the rest of the trust surface: what is and is
not a boundary, what reaches the network, and how to report something.

## An exported file is what was on screen

Two settings decide what a result holds before it is ever exported, and both
say so at the bottom of the screen while the result is there:

- **`auto_limit`** adds a `LIMIT` to a `SELECT` that does not limit itself,
  so the result — and the file — stops there. The status line says
  `LIMIT 1000 added` when it happened. It bounds what is fetched, not what
  the server does: a statement that scans the whole table still scans it.
- **`buffer_max`** caps the rows held in memory. Past it the result is
  truncated, the status line says so, and the export says
  `truncated at buffer_max` as it writes.

So a CSV is exactly the result you were looking at, which is not always the
whole table. Raise either setting, or narrow the statement, before exporting
something that has to be complete.

## What is actually tested

A client is easy to claim compatibility for and hard to be compatible: a
successful `SELECT 1` says the protocol matched, not that a `DECIMAL` kept
its scale or that `information_schema` answered the same question. This is
what runs on every change, and what does not.

| Checked every change | What that covers |
|---|---|
| MariaDB 11.4 | the whole suite: streaming, cancellation through `KILL QUERY`, catalog reads, session read-only, reconnection, the interface itself, and that values keep their scale, sign, fraction, encoding and emptiness from the server through the grid to the exported file |
| MySQL 8.4 | the same suite, against the same assertions |
| linux/amd64 | where the suite runs |
| macOS, Linux and Windows, amd64 and arm64 | the release is built and packaged for each, on every change rather than at tag time |

**Not checked, so not claimed.** Other server versions are likely to work and
nothing here says they do. The interface is exercised against a simulated
screen, so what a particular terminal does with `⌘`, the mouse or the
clipboard is not covered — the fallbacks exist because of it. The macOS and
Windows binaries are built on every change and run by nobody.

**One thing CSV cannot say.** An empty field is both a `NULL` and an empty
string, and inventing a spelling for one of them would be a convention this
file made up. Where the difference matters, the grid shows `NULL` for absence
and nothing for emptiness, and a JSON export writes `null` and `""`.

## Checking where a download came from

Every released archive and package is signed by the workflow that built it. To
check one:

```sh
gh attestation verify <the archive you downloaded> --repo Ahngbeom/datavase
```

That answers a different question from the checksum the install script
already verifies. A checksum says the file arrived intact; this says it was
built by this repository's release workflow and not swapped for something
else — which the checksum cannot tell you, because it is downloaded from the
same place as the file it describes.

## Development

```sh
make test              # unit tests, no database needed
make db-up             # start MariaDB on port 13306
make test-integration  # everything, including tests against the real server
make db-down
```

Unit tests cover the tokenizer, config, formatting, export and the keychain
contract. Integration tests cover streaming, cancellation, catalog reads and
the interface itself — the TUI runs against a tcell simulation screen, so
assertions like "a table preview leaves the editor alone" are automated rather
than trusted.

## License

MIT — see [LICENSE](LICENSE).
