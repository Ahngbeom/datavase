# datavase

[![ci](https://github.com/Ahngbeom/datavase/actions/workflows/ci.yml/badge.svg)](https://github.com/Ahngbeom/datavase/actions/workflows/ci.yml)

A terminal MySQL/MariaDB client. Single static binary, no runtime, no IDE.

Built for the workflow where you are already in a terminal and want to run a
query without launching something heavy — while making it structurally hard
to damage a production database by accident.

## Status

Usable, and specific about its edges.

Built: connect (directly, over TLS, or through an SSH bastion), browse the
schema, edit and run SQL with schema-aware completion, run a whole file a
statement at a time, stream large results, cancel a runaway query, search the
query history, export to CSV or JSON, attach a worktree and open, run and
save the `.sql` files in it, read SQL out of a GitLab merge request, and the
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

Nothing else is needed to run a query. One optional feature has an outside
dependency: [reading SQL out of GitLab](#gitlab) borrows its credential from
[glab](https://gitlab.com/gitlab-org/cli), so that half of the tool is absent
until glab is installed. Everything else is in the one binary.

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
    tls: verify-identity     # optional; see below for what prod defaults to
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

dv open local --dir ~/work/migrations   # with a worktree of SQL attached
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
| Find in the editor or the results | `⌘F` | `Ctrl+F` |
| Next / previous match | `⌘G` `⌘⇧G` | `Ctrl+G` `Ctrl+Shift+G` |
| Search the query history | `⌘⇧F` or `F9` | `Ctrl+Shift+F` or `F9` |
| Jump to a table | `⌘N` | `Ctrl+N` |
| Open a file from the worktree | `⌘⇧O` | `Ctrl+Shift+O` |
| Save the open file | `⌘S` | `Ctrl+S` |
| Command palette | `⌘⇧A` or `F3` | `Ctrl+Shift+A` or `F3` |
| Show the selected table's definition | `⌘I` | `Ctrl+Shift+I` |
| Switch tab in the focused pane | `Ctrl+⇥` | `Ctrl+⇥` |
| Key reference | `F1` | `F1` |
| Quit | `⌘Q` | `Ctrl+Q` |

`F5` runs, `F2` saves, `F4` shows a definition, `F6` switches tab, `F7` chooses
the schema, `F8` opens a file, `F9` searches the history and `F10` quits
everywhere, as fallbacks. Two
combinations are impossible
rather than merely awkward: `⌘⇥` is the macOS application switcher, and
`Ctrl+I` is byte 0x09 — the same as `⇥`.

Run `dv keys` for the current map, which reflects the preset and any
customisation.

Cursor movement is the one place `⌘` and `Ctrl` are **not** interchangeable,
because macOS uses them for different things: `⌘←` goes to the start of the
line while `⌥←` and `Ctrl+←` move by word.

A word here is a SQL identifier, so `user_id` moves as one — not as `user`,
`_` and `id`, which is how a generic text editor would split it.

### When something else has taken the key

A terminal that keeps `⌘` for its own menus, a window manager with its own
shortcuts, a multiplexer whose prefix is `Ctrl+B` — any of them takes the key
before datavase sees it, and there is nothing datavase can do about that from
the inside.

So nothing is only reachable by a chord. **`F3` opens the command palette, and
every command is in there by name** — including `schema tree`, `go to table`,
`comment`, `duplicate line` and the rest. A test refuses to let an action exist
that has neither a plain key nor a palette entry.

Two things are outside that promise, deliberately:

- **Copy, cut and paste.** Whatever took those keys is doing the same job with
  them, and routing a paste through a palette is worse than not having one.
- **Undo and redo.** They belong to the text widget rather than to datavase's
  key map, so there is nothing here to offer.

And on the **vim** preset — the default — editing needs no modifiers in the
first place: `dd`, `yy`, `p`, `gg` are all unclaimable.

You can also just move the key. Listed bindings replace an action's defaults:

```yaml
keymap:
  actions:
    toggle-sidebar: ["f12"]
```

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
refusal. Leave the dialog, open the command palette (`⌘⇧A`, `F3`) and run
`write`; `readonly` locks it again, and so does closing the session. The
status bar carries `writes on` for as long as it lasts.

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

## TLS

```yaml
datasources:
  - name: prod-app
    env: prod
    tls: verify-identity
    tls_ca: /etc/ssl/rds-combined-ca-bundle.pem   # optional; a private CA
```

| `tls:` | what the server has to prove |
|---|---|
| `disabled` | nothing; everything crosses the wire in clear text |
| `preferred` | encrypted when the server offers it, plain when it does not |
| `required` | encrypted or the connection fails — but who answered is unchecked |
| `verify-ca` | the certificate chains to a trusted authority |
| `verify-identity` | and the name on it matches the host dialled |

The names are MySQL's own `ssl-mode`, so what is configured here can be
checked against what the server was started with.

**The default follows `env`:** `required` for a `prod` datasource,
`preferred` everywhere else. Production is where a credential in clear text
costs the most, and it is also where the managed databases that refuse plain
connections outright live — AWS RDS and Aurora, Azure Database for MySQL,
Cloud SQL — so `required` is usually the working setting as well as the safer
one. Elsewhere the cost of being wrong is a laptop that cannot connect, which
is why those get `preferred`.

A production database that genuinely cannot speak TLS stays reachable with
`tls: disabled`. Saying so in the file is the point: it is visible, and it is
a decision rather than a default.

`tls_ca` replaces the system trust store rather than adding to it — an
instance behind a private authority should not also be satisfied by a public
one. It is only accepted under `verify-ca` and `verify-identity`; naming one
under a mode that verifies nothing is refused rather than read and ignored. A
file that cannot be read, or that holds no certificate, stops the connection
instead of quietly falling back to the system roots.

`verify-ca` exists for the case `verify-identity` cannot serve: a database
reached by an address its certificate was never issued for — the local end of
an SSH tunnel is `127.0.0.1`, a failover alias is not the instance's name —
where the authority still says who answered.

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

## The screen

```
█ PROD  prod-app@app_db  ·  feature/add-index                       F1 keys
█──────────────────────────────────────────────────────────────────────────
█▌ 002_add_index.sql *
█ ALTER TABLE users
█   ADD INDEX idx_email (email);
█──────────────────────────────────────────────────────────────────────────
█  ▸results ddl
█ id      email
█ 1       ada@example.com
█──────────────────────────────────────────────────────────────────────────
█ 1 row  ·  6ms
```

**The column down the left is the environment.** Red for production, amber for
stage, and a dark slate for dev, which is where you are most of the time. It is
a column and not a badge because a badge is a field, and a field is something a
narrow terminal can drop — this cannot be squeezed out, and it is the only
colour in datavase that ignores your terminal theme.

**The top line is where you are; the bottom line is what just happened.** The
environment, the datasource, the schema an unqualified statement will reach and
the attached branch stay put up top. Row counts, timings, warnings and messages
go below.

Regions are separated by a single rule rather than boxed. Each names itself
once, on its own header line, and the one holding the keyboard is marked `▌`.
The editor's header carries the open file and a `*` while it differs from disk;
it does not say "editor", because the caret already does.

## The schema pane

**It starts hidden.** `⌘B` (`Ctrl+B`) brings it. Finding a table by name is
`⌘N`, a schema is `⌘⇧N` and a file is `⌘⇧O` — all of them searchable lists that
answer faster than scrolling a tree does, so the tree is there for browsing
rather than in the way of everything else.

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

The result pane has two tabs as well: **results** and **ddl**. `⌘I` on a
selected table runs `SHOW CREATE TABLE` (or `SHOW CREATE VIEW`) and switches
to the ddl tab; `⌘C` there copies the whole definition. Running a query
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

## Finding text

`/` searches whatever has focus — the SQL in the editor, or the values in the
results. `?` searches backwards, and `n` and `N` step through the matches. The
prompt sits on the bottom row rather than over the screen, so you can see what
is being searched while you type it, and the match moves as the pattern grows.
`Esc` puts the cursor back where it started.

On a non-modal keyboard the same thing is `⌘F`, with `⌘G` and `⌘⇧G` for the
next and previous match. In the results `/` works whichever keyboard you are
on, because a grid has nothing for an unmodified letter to collide with.

Matching is literal, not a regular expression, and **smart about case**: a
lower-case pattern matches anything, while one capital in it means you meant
that capital. So `/id` finds every `id` and `/ID` finds only the column
actually spelled that way.

Two things worth knowing:

- **Searching the results reads the data, not what is on screen.** The grid
  truncates long values and doubles their brackets for the markup parser;
  searching that copy would mean a `[` in your data could never be found, and
  neither could anything past the two-hundredth character.
- **Only the current match is highlighted.** The text widget has no way to
  colour arbitrary ranges, so there is no equivalent of vim's `hlsearch`.

While rows are still arriving, "no match" means "not in the rows so far", and
says so.

## The worktree

Most SQL worth keeping already lives in a directory — a branch's migrations, a
folder of verification queries. Point datavase at one and you can open those
files, run them and save them back, without leaving for an editor.

```sh
dv open local --dir ~/work/migrations
```

Or from the command palette (`⌘⇧A`) mid-session: `attach directory`. Pressing
`⌘⇧O` with nothing attached opens the same prompt, so the feature is one key
away whether or not you have found the palette. One directory is attached at a
time, and `detach directory` forgets it.

The prompt opens on the directories you have attached before, and each row says
whether it is a git repository or a plain folder. **`⇥` completes the
highlighted path back into the field**, so a deep path is walked into a segment
at a time rather than typed out in full. Typing a segment beginning with `.`
brings the hidden directories into view.

`F1` lists the palette's commands as well as the keys — nothing is reachable
only by knowing it exists.

`⌘⇧O` (`Ctrl+Shift+O`, or `F8`) lists the `.sql` files, filtered as you type:

```
 files · feature/add-index * 
 file: 002
 M migrations/002_add_index.sql    modified
 ? migrations/scratch/probe.sql    untracked
```

The listing comes from git rather than from a directory walk, so a file
`.gitignore` excludes — a dump directory, a build artefact — is never offered.
The marker says what git makes of each file: `M` changed, `A` new to this
branch, `?` never added. The title carries the branch, and a `*` when the tree
has uncommitted changes.

**git is optional.** Attach a plain directory, or run on a machine with no git,
and you still get the file list; only the branch and the markers go missing.

Choosing a file loads it into the editor, and `⌘S` (`Ctrl+S`, or `F2`) writes
it back. The status bar carries the branch and the file, with a `*` while the
buffer differs from what is on disk.

Four things it will not do quietly:

- **Overwrite someone else's edit.** If the file changed on disk since it was
  opened — a rebase, an editor in the next window — saving asks first.
- **Leave a half-written file.** The save goes through a temporary file and a
  rename, so an interrupted one leaves the previous version intact.
- **Lose an unsaved buffer.** Opening another file or quitting asks first.
- **Open something it cannot show.** A file over 4 MiB is refused rather than
  handed to a text widget that would stop drawing.

The guard is unchanged by any of this. A `DELETE` loaded from a file is the
same statement it would be if you had typed it, and meets production the same
way.

## GitLab

The migration you want to look at is often the one nobody has merged yet.
datavase can read the `.sql` files out of an open merge request, or out of a
snippet, without checking the branch out.

**Set-up is one command, and you have probably already run it:**

```sh
glab auth login --hostname gitlab.example.com
```

datavase stores no GitLab token of its own. It borrows the one
[glab](https://gitlab.com/gitlab-org/cli) already holds — anyone reaching a
self-managed instance has it logged in, often through SSO with a credential
that rotates on its own, and a second personal access token would be a second
thing to create, rotate and revoke for nothing. Nothing is read until you first
open the dialog.

Then `open from gitlab` in the palette. It lists the open merge requests and
the snippets; choosing one offers its `.sql` files, and choosing a file loads
it into the editor.

**With a checkout attached there is nothing to configure.** The instance and
the project both come from its `origin`. Configuration is for narrowing that
down, or for a session with nothing attached:

```yaml
gitlab:
  host: gitlab.example.com   # optional; restricts the feature to this instance
  project: group/project     # optional; for a session with nothing attached
```

Naming a host means an `origin` on any other one is not this instance's
project, and datavase says so rather than asking GitLab about a repository it
has never heard of.

What comes back is **read-only**. There is no branch here to commit to, and a
database client that could push would be a worse git than git; `⌘S` refuses and
says so rather than looking for a worktree that would not help. Files are read
at the merge request's head commit, so a branch pushed to while the list is
open cannot hand back a file from a different change.

Nothing here happens at startup, and no failure reaches the session: the
credential is looked up and the request made when you open the dialog, and
anything that goes wrong is a line on the status bar. A missing glab and a
missing login get different sentences, because they have different fixes.

One caveat for an instance behind an internal certificate authority: datavase
makes the HTTPS request itself, so it trusts what the operating system trusts.
An instance whose certificate does not chain to a system root is not reachable
yet.

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
