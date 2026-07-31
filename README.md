# datavase

[![ci](https://github.com/Ahngbeom/datavase/actions/workflows/ci.yml/badge.svg)](https://github.com/Ahngbeom/datavase/actions/workflows/ci.yml)

A terminal MySQL/MariaDB client. Single static binary, no runtime, no IDE.

Built for the workflow where you are already in a terminal and want to run a
query without launching something heavy — while making it structurally hard
to damage a production database by accident.

## Status

Usable, and specific about its edges.

Built: connect (directly or through an SSH bastion), browse the schema, edit
and run SQL with schema-aware completion, stream large results, cancel a
runaway query, search the query history, export to CSV or JSON, and the
production guard.

Not built, and worth knowing before you lean on it:

- **No transactions.** Each statement runs on its own connection out of a
  pool, so `BEGIN` cannot reach the statement after it. Rather than let a
  `ROLLBACK` report success having undone nothing, transaction control and
  session statements are [refused with that explanation](#what-is-refused-for-a-different-reason).
- **A write does not say how many rows it changed.** The count on the status
  bar describes a result set, and a write has none.
- **The result grid is plain.** No vertical view for a wide row, no way to
  open a truncated value in full, no sorting.

Those are tracked in the issues, roughly in that order.

## Install

```sh
go install github.com/Ahngbeom/datavase/cmd/dv@latest
```

Or take a prebuilt binary for macOS, Linux or Windows from
[Releases](https://github.com/Ahngbeom/datavase/releases) — put `dv` on your
PATH and check it with `dv version`.

From a clone:

```sh
make build      # produces ./dv — CGO-free, single static binary
```

## Configure

`~/.config/datavase/config.yaml` (or `$XDG_CONFIG_HOME/datavase/config.yaml`):

```yaml
datasources:
  - name: local
    env: dev              # prod | stage | dev — this drives the guard
    host: 127.0.0.1
    port: 3306
    user: root
    database: app_db

  - name: prod-app
    env: prod
    host: db.internal        # as named from the bastion
    user: readonly
    tunnel:
      host: bastion.example.com
      port: 22
      user: bahn

defaults:
  auto_limit: 1000        # LIMIT added to unbounded SELECTs
  fetch_chunk: 500        # rows per batch while streaming
  buffer_max: 50000       # rows held in memory before truncating
```

Passwords never go in this file. Store them in the OS keychain:

```sh
dv auth prod-app          # prompts, echo off
dv auth -rm prod-app      # remove
```

Unknown keys are rejected rather than ignored, so a typo like `hots:` fails
immediately instead of surfacing later as a confusing error.

## Use

```sh
dv                    # opens the only configured datasource
dv open prod-app      # opens a named one
dv ls                 # list datasources and whether a password is stored
dv check prod-app     # verify reachability, then exit
```

### Keys

**The editor is modal by default** — it starts in vim's normal mode, so press
`i` before you type. The status bar always says which mode you are in, and
`F1` lists the vim keys and how to leave them. To use an ordinary typing
editor instead, see [Changing the keys](#changing-the-keys).

Everything outside the editor is the same on every keyboard, and follows
DataGrip. Where DataGrip and VS Code agree, that key is used; where they
differ, DataGrip wins. **`⌘` and `Ctrl` do the same thing** — use whichever
your hands reach for.

| Action | macOS | Windows / Linux |
|---|---|---|
| Run the statement under the cursor | `⌘↩` | `Ctrl+↩` |
| Run everything | `⌘⇧↩` | `Ctrl+Shift+↩` |
| Cancel the running statement | `⌘F2` | `Ctrl+F2` |
| Copy the selection, or cancel if nothing is selected | `⌘C` | `Ctrl+C` |
| Cut / paste | `⌘X` `⌘V` | `Ctrl+X` `Ctrl+V` |
| Undo / redo | `⌘Z` `⌘⇧Z` | `Ctrl+Z` `Ctrl+Shift+Z` |
| Select all | `⌘A` | `Ctrl+A` |
| Move one word | `⌥←` `⌥→` | `Ctrl+←` `Ctrl+→` |
| Extend the selection one word | `⌥⇧←` `⌥⇧→` | `Ctrl+Shift+←/→` |
| Start / end of line | `⌘←` `⌘→` | `Home` `End` |
| Select to start / end of line | `⌘⇧←` `⌘⇧→` | `Shift+Home` `Shift+End` |
| Delete the word before the cursor | `⌥⌫` | `Ctrl+⌫` |
| Delete to the start of the line | `⌘⌫` | — |
| Comment or uncomment | `⌘/` | `Ctrl+/` |
| Duplicate the line | `⌘D` | `Ctrl+D` |
| Delete the line | `⌘Y` | `Ctrl+Y` |
| Move between panes | `⇥` / `⇧⇥` | `Tab` / `Shift+Tab` |
| Show or hide the schema tree | `⌘B` | `Ctrl+B` |
| Reload the schema tree | `⌘R` | `Ctrl+R` |
| Choose the schema | `⌘⇧N` | `Ctrl+Shift+N` |
| Complete the word at the cursor | `^Space` | `Ctrl+Space` |
| Search the query history | `⌘F` | `Ctrl+F` |
| Jump to a table | `⌘N` | `Ctrl+N` |
| Command palette | `⌘⇧A` | `Ctrl+Shift+A` |
| Show the selected table's definition | `⌘I` | `Ctrl+Shift+I` |
| Switch tab in the focused pane | `Ctrl+⇥` | `Ctrl+⇥` |
| Key reference | `F1` | `F1` |
| Quit | `⌘Q` | `Ctrl+Q` |

`F5` runs, `F4` shows a definition, `F6` switches tab, `F7` chooses the schema
and `F10` quits everywhere, as fallbacks. Two combinations are impossible
rather than merely awkward: `⌘⇥` is the macOS application switcher, and
`Ctrl+I` is byte 0x09 — the same as `⇥`.

Run `dv keys` for the current map, which reflects the preset and any
customisation.

Cursor movement is the one place `⌘` and `Ctrl` are **not** interchangeable,
because macOS uses them for different things: `⌘←` goes to the start of the
line while `⌥←` and `Ctrl+←` move by word.

A word here is a SQL identifier, so `user_id` moves as one — not as `user`,
`_` and `id`, which is how a generic text editor would split it.

### Making your terminal deliver them

Two things can get in the way, and `dv keys` tells you when they do.

**Modified keys such as `Ctrl+Enter`** need a terminal that speaks the
extended keyboard protocol. Most do; tmux does not by default, because its
`TERM` is `screen-256color`:

```sh
dv keys --tmux >> ~/.tmux.conf    # then restart tmux
```

**`⌘` bindings** are kept by macOS for the terminal's own menus, so the
terminal has to forward them:

```sh
dv keys --ghostty >> ~/.config/ghostty/config   # then restart Ghostty
dv keys --iterm2                                # iTerm2 is configured in its UI
```

Neither is required — the `Ctrl` bindings and `F5` work regardless. And if a
key still misbehaves, `dv keys --debug` shows exactly what the terminal sent,
which separates a terminal problem from an application one.

### Changing the keys

Pick a keyboard with `preset`, then disagree with it where you like:

```yaml
# ~/.config/datavase/config.yaml
keymap:
  preset: vim          # vim · datagrip · vscode
  actions:
    run:    ["ctrl+enter", "cmd+enter", "f5"]
    cancel: ["ctrl+f2", "cmd+f2"]
```

The bare mapping form is still accepted, so an existing file keeps working:

```yaml
keymap:
  run: ["f5"]
```

| Preset | What it is |
| --- | --- |
| `vim` | **the default** — modal editing (normal, insert, visual) on DataGrip's application keys |
| `datagrip` | DataGrip's SQL-tool keyboard |
| `vscode` | VS Code where the two disagree: `⌘⇧K` deletes a line, `⌘⇧P` is the palette, `⌘P` finds a table |

Presets can also be switched mid-session from the command palette
(`keymap vim`), which is the fastest way to try one.

Listed bindings replace an action's defaults rather than adding to them. An
unknown preset, action name or binding is refused at startup, and the help
screen is generated from the map in force — so it cannot drift out of step
with what the keys actually do.

## The production guard

Each datasource carries an `env` label, and it decides what may run.

| | prod | stage / dev |
|---|---|---|
| `SELECT`, `SHOW`, `EXPLAIN` | run | run |
| `INSERT` / `UPDATE` / `DELETE` | refused until [unlocked](#unlocking-writes-to-production), then confirm | confirm |
| `UPDATE` / `DELETE` with no top-level `WHERE` | refused, and the unlock does not lift it | type the phrase |
| `DROP` / `TRUNCATE` | refused, and the unlock does not lift it | type the phrase |
| `CREATE` / `ALTER` | refused, and the unlock does not lift it | confirm |
| Anything unrecognised | refused, and the unlock does not lift it | confirm |
| `SELECT` with no `LIMIT` | `LIMIT` added, and disclosed | same |

Three properties are worth calling out, because they are what make the guard
more than decoration:

- **It is fail-closed.** The tokenizer is not a full MySQL parser. Anything
  it cannot classify is refused against production rather than assumed safe.
- **It reads what the server reads.** MySQL executes the contents of
  `/*! ... */` version-hint comments. The tokenizer unwraps them, so
  `/*!40001 DELETE FROM users */` is judged as the DELETE it really is.
- **It knows where a clause belongs.** Only a `WHERE` at parenthesis depth
  zero counts, so `DELETE ... WHERE id IN (SELECT ...)` passes while
  `UPDATE t SET x = (SELECT ... WHERE ...)` does not.

The refusal dialog has no "run anyway" button. A dialog people can dismiss is
a dialog people learn to dismiss.

### Unlocking writes to production

Writing to production is possible, and it is deliberately not a button on the
refusal. Leave the dialog, open the command palette and run `write`;
`readonly` locks it again, and so does closing the session. The status bar
carries `writes on` for as long as it lasts.

What that unlocks is narrow, and the narrowness is the point:

- it turns a refusal into a **confirmation**, never into a silent run
- it applies only to an `INSERT`, or an `UPDATE`/`DELETE` that already carries
  a top-level `WHERE`
- it does **not** apply to an unbounded `UPDATE`/`DELETE`, to `DROP` or
  `TRUNCATE`, to `CREATE`/`ALTER`, or to anything the tokenizer could not
  classify — those are refused with writes on or off

So the escape hatch exists for the statement you meant to write, and not for
the one that would rewrite a table.

### What is refused for a different reason

`BEGIN`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`, `SET` and `LOCK TABLES` are
refused in **every** environment, dev included. This is not about danger.

Statements run on a connection borrowed from a pool and handed back when they
finish, so none of that state reaches the next statement: `BEGIN` would open a
transaction nothing could commit, `SET SESSION sql_mode = …` would be accepted
and thrown away, and `ROLLBACK` would report success having undone nothing.
Every one of them would look like it worked. Refusing them and saying why is
the only honest answer until a transaction can hold one connection for its
whole life.

`USE` is refused too, and points at the schema picker instead — that choice
does travel with every statement, which is exactly what `USE` was reached for.

### Running more than one statement

`⌘⇧↩` runs the whole buffer, one statement at a time, in order. Each goes
through the guard on its own: a confirmation stops the queue until it is
answered, and a refusal — or a declined confirmation, or a failure, or `^C` —
stops the rest.

The status bar then says how many of them ran: `5 statements · 2 ran ·
refused at statement 3`. That count is reported even when everything
succeeded, because a batch that stopped part-way has left the database in a
state nothing on screen describes, and there is no transaction to unwind it.

Statements after the one that stopped are not attempted. They were written to
follow it.

## Cancellation

`Ctrl-C` sends `KILL QUERY` over a second connection held open for exactly
this purpose. Cancelling the context alone would only detach the client while
the server kept working — the integration tests assert the statement actually
disappears from `information_schema.PROCESSLIST`.

That second connection is also why the schema tree stays usable while a large
result is still streaming: MySQL will not accept another statement on a
connection until the current result set has been read to the end.

## SSH tunnels

Give a datasource a `tunnel:` block and the connection is forwarded through
that bastion. The `host` stays whatever the database is called *from the
bastion* — the local end of the tunnel is an implementation detail.

Authentication goes through `ssh-agent`, so datavase never reads a private
key and passphrase-protected keys work without prompting:

```sh
ssh-add ~/.ssh/id_ed25519
```

Bastions are verified against `~/.ssh/known_hosts`, and there is no option to
skip that. This tunnel carries production database credentials; an unverified
bastion is one an attacker can impersonate to read all of them. For a host
you have not connected to before:

```sh
ssh-keyscan -H bastion.example.com >> ~/.ssh/known_hosts
```

## The schema pane

The tree root is the **server**, and its children are that server's schemas —
all at the same level. The root carries the host so it cannot be mistaken for
a schema, which matters when a datasource is named after its main schema. The
schema an unqualified query will hit is marked with `●`.

The pane has two tabs:

- **tree** — expand a schema for its tables, a table for its columns. Choosing
  a schema also points completion and the tables tab at it.
- **tables** — a flat, filterable list of the current schema's tables with row
  estimates. It reads from the local cache, so it fills instantly and filters
  as you type.

The result pane has two tabs as well: **results** and **DDL**. `⌘I` on a
selected table runs `SHOW CREATE TABLE` (or `SHOW CREATE VIEW`) and switches
to the DDL tab; `⌘C` there copies the whole definition. Running a query
switches back to results.

The lookup is deliberate rather than automatic — `SHOW CREATE` is a server
round trip, and issuing one every time the tree selection moved would make
browsing expensive.

### Switching schema

`⌘⇧N` (`Ctrl+Shift+N`, or `F7`) opens a filterable list of the server's
schemas; `use schema` in the command palette does the same. Choosing one in
the schema tree has the same effect.

The choice reaches the server, not just the status bar: the schema travels
with every statement, so `SELECT * FROM users` runs against the schema the bar
names. Statements that qualify their tables are unaffected.

## Completion

`Ctrl+Space` completes against a local snapshot of the schema, not against
the server. `information_schema` takes hundreds of milliseconds on a large
database — far too slow to run on a keystroke — so the schema is copied into
a local SQLite file when you connect and refreshed in the background. Cached
lookups run about thirty times faster than asking the server.

Completion understands the statement it is in:

```sql
SELECT i.        -- offers invoices' columns
FROM customers c
JOIN invoices i ON i.customer_id = c.id
```

The alias is defined *after* the caret, so the whole statement is parsed
rather than only the text before it.

`⌘N` searches every table by name, which is the faster route once a database
has more tables than a tree is pleasant to browse. It writes a starter query
into the editor rather than running it — the guard and you still decide when
anything executes.

## History and export

Every statement you run is recorded, and `⌘F` searches them. Choosing an entry
puts it back in the editor.

Statements that embed a password — `CREATE USER … IDENTIFIED BY …`,
`SET PASSWORD …` — are deliberately not recorded. Writing them down would put
a plaintext credential on disk, which is what keeping the real ones in the
keychain avoids.

The command palette (`⌘⇧A`) exports the current result:

- **CSV** — values beginning with `=`, `+`, `-` or `@` are prefixed with an
  apostrophe. Spreadsheets execute such fields as formulas, and exporting
  query results into a spreadsheet is an everyday workflow; without this a
  database row becomes code running on someone's machine.
- **JSON** — numbers stay numbers, `NULL` becomes `null`, binary becomes
  base64, and duplicate column names are made distinct rather than dropped.

Exports are written to the working directory with a timestamped name, so one
never silently overwrites another. If the result was truncated, the
confirmation says so.

## Development

```sh
make test              # unit tests, no database needed
make db-up             # start MariaDB on port 13306
make test-integration  # everything, including tests against the real server
make db-down
```

Unit tests cover the parser, the guard, config, formatting and the keychain
contract. Integration tests cover streaming, cancellation, catalog reads and
the interface itself — the TUI runs against a tcell simulation screen, so
assertions like "a production DELETE shows a refusal dialog" are automated
rather than trusted.

## License

MIT — see [LICENSE](LICENSE).
