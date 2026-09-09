# datavase

[![ci](https://github.com/Ahngbeom/datavase/actions/workflows/ci.yml/badge.svg)](https://github.com/Ahngbeom/datavase/actions/workflows/ci.yml)

A terminal MySQL/MariaDB client. Single static binary, no runtime, no IDE.

Four things, and no more:

- **Datasources.** Keep as many as you like; connect, add, edit and delete them
  from inside.
- **Look at a table.** Double-click one in the tree, or Enter on one in the
  tables tab, for its first hundred rows.
- **Run a statement** with `⌘↩` (`Ctrl+↩`, or `F5`).
- **Read the answer** in a grid, and copy what you need out of it: the cell
  with `⌘C`, the row with `⌘⇧R`, the whole result as Markdown or JSON with
  `⌘⇧C`.

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
```

The datasource dialog writes this file, so comments in it do not survive a
save. Editing it by hand still works; unknown keys are rejected rather than
ignored, so a typo like `hots:` fails immediately instead of surfacing later as
a confusing error.

An absent `tls:` means `preferred`. If you are coming from 0.8 with `env: prod`
and no `tls:`, that datasource still gets `required` — write `tls: required`
down now, because `env` stops being read in a later release.

Passwords never go in this file. Store them in the OS keychain:

```sh
dv auth app               # prompts, echo off
dv auth -rm app           # remove
```

**On a machine with no keychain** — a headless Linux server runs no D-Bus
Secret Service, so there is nothing for `dv auth` to write to — pass the
password in the environment instead:

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
| Copy the whole result as Markdown or JSON | `⌘⇧C` · `F3` | `copy` on the result header |
| Datasource list | `⌘⇧D` · `F11` | the datasource name in the top bar |
| Choose the schema | `⌘⇧N` · `F7` | the schema name in the top bar |
| Reload the schema tree | `⌘R` | |
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
█ app @app_db                                             F1 keys
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
datasource and the schema an unqualified statement will reach stay put up top.
Row counts, timings, warnings and the SQL a table preview ran go below. The
datasource is filled rather than merely coloured, so which server this is
survives a terminal of any width; the column down the left is the same fill
continued, and it is the only thing on screen that ignores your terminal theme.

Regions are separated by a single rule rather than boxed. Each names itself
once, on its own header line, and the one holding the keyboard is marked `▌`.

**The schema pane** starts hidden; `⌘B` brings it. It has two tabs. **tree**
expands a schema for its tables and a table for its columns, marks the schema
unqualified names resolve against with `●`, and previews a table on a
double-click; `↩` on a column node puts its name in the editor. **tables** is a
flat, filterable list of the current schema's tables with row estimates, read
from the local cache so it fills instantly, and `↩` there previews a table too.
A preview never touches the editor — the text in it is yours.

**The schema tree** is there when a session opens, on the left, with a tables
tab beside it. `⌘B` puts it away when the width is wanted for the result, for
that session.

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
columns; `⌘⇧C` takes the entire result as Markdown or JSON. `⌘C` in the editor
takes the selection, as it does anywhere else.

**Copying** goes two ways at once. The terminal is asked to take the text —
the only route that reaches your own clipboard when `dv` is running over SSH —
and a local session also hands it to `pbcopy`, `wl-copy` or `xclip`, whichever
is installed. That second route is there because the first is a request the
terminal may refuse: Ghostty asks before allowing it, iTerm2 keeps it off until
"Applications in terminal may access clipboard" is ticked, tmux drops it
without `set -g set-clipboard on`, and Terminal.app has never implemented it.

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
