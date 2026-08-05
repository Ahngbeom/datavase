# Changelog

What changed between releases, and what to do about it before upgrading.

The generated release page lists every commit; this file is the shorter,
edited account — and the place anything that needs action is written down.

## Unreleased

**Nothing here needs doing before you upgrade.** No configuration file changes,
no keys moved, and no statement is judged differently — this is a pass over
what the interface says and how it is drawn.

### Fixed

**A result scrolled sideways said nothing about it, and took the row's identity
with it.** A `SELECT` wider than the terminal scrolls, and the columns that go
off the left are the ones that identify the row: a screenful beginning at
`total_cents` is indistinguishable from one whose query never selected an id.
Every other way a result on screen is not the whole answer already said so on
the bar — an injected `LIMIT`, a cut stream, a sort over rows that are still
arriving — and this one was silent.

The bar now carries the count, and the first column stays put while the rest
scroll past it. A column wider than the pane still cannot be scrolled past;
that is unchanged, and it is the layout rather than the report.

**The process list was folded to sixteen columns whatever the terminal's
width.** It was laid out when the text was composed, which is while the tab is
still hidden — a hidden tab has no width to give, so the answer was the floor.
An eighty-column terminal showed a statement as a stack of four-word lines, and
it stayed that shape after the window was resized. It is laid out when it is
drawn now, as the plan pane and both bars already were.

**Switching to the vim keyboard left the editor promising that typing would
work.** The empty-editor placeholder says to press `i` first, which is the only
thing on screen that explains why an ordinary letter does nothing. It was
written once when the interface opened, so arriving at the modal keyboard the
advertised way — the command palette, `keymap vim` — landed you in an editor
still describing the other one.

**A status line too long for the terminal stopped mid-word.** It now ends in an
ellipsis, which is what the same interface does one row above when it shortens
a file name.

The greeting was one sentence, so the bar could only halve it; it is a list of
clauses now, dropped whole and ordered by what you can act on. The clause about
what your terminal cannot deliver was the longest thing on the line and so the
first to go — on exactly the terminals that need it. It is shorter, and says
what to press instead of why; `dv keys` and the key reference still explain the
whole of it.

**Every finder opened by reporting a search nobody had run.** `go to table`
began with "no matching tables" sitting above "type part of a name" — a result
and an invitation contradicting each other on the screen you meet first — and
the datasource, file, connection, command and history pickers each had their
own wording of the same thing.

An empty place now says what it is and what would fill it; a search that found
nothing says so and repeats what it looked for, so a typo is visible in the
answer as well as in the field you have already stopped looking at.

**The result pane asked for a statement while one was running.** The bar said
"running… ^C cancels" and the pane two rows above said "run a statement to see
rows here". It also said it straight after a write, when the row count had just
been reported — an empty grid after an `UPDATE` reading as a write that went
nowhere. It says which of the three it is now.

**Two things worked and could not be found.** The row view steps between rows
with `j` and `k` without closing, which is most of what it is opened for, and
its title said only which row you were on. And the dot beside a schema in the
tree is the only thing that says which one an unqualified statement will reach.
Both are named on screen now, in the space each pane already had.

**The hairlines between regions met without joining, and a dialog was as tall
as it was allowed rather than as tall as its contents.** The rule under the top
bar ran straight through the column the schema pane's rule starts in, and the
row view drew three columns of detail in a box of thirty rows. Neither changed
what anything did; both read as something having failed to render.

### Changed

**The interface draws four colours and the environment's three, and no others.**
Two were nobody's choice: the empty editor's placeholder and every finder's
second line came out in the terminal library's green, the colour every other
program uses for success. And the key reference and the palette drew their
section headings in the colour reserved for a state you could forget you are
in — an injected `LIMIT`, a truncated result, unlocked writes — which left
that cue sharing its meaning with the word "Editing".

Headings carry weight instead of colour, so they survive a monochrome terminal
as well. Nothing about the environment spine or the guard's own colours moved.

## v0.6.2 — 2026-08-05

**Nothing to do before upgrading.** The binaries do the same thing as
`v0.6.1`'s; this adds something alongside them.

### Added

**Every released archive and package is signed by the workflow that built it.** To check one:

```sh
gh attestation verify dv_v0.6.2_darwin_arm64.tar.gz --repo Ahngbeom/datavase
```

The install script already verifies each download against `checksums.txt`,
which says the file arrived intact. It cannot say more than that, because it
is fetched from the same release as the file it describes. This answers the
other question: that the file was built here.

## v0.6.1 — 2026-08-05

**Nothing to do before upgrading, and no reason to unless you use Homebrew.**
The binaries in `v0.6.0` are unaffected — this fixes how one of them is
published, not what it does.

### Fixed

**`brew install Ahngbeom/tap/dv` installs something.** `v0.6.0` published
every archive, every Linux package and the checksums, and then failed before
writing the Homebrew cask, so the tap had nothing in it and the command the
release notes advertised did not work.

The tap token was written in a template form goreleaser accepts everywhere
except that one field, where it compares the string against a fixed shape
instead of rendering it. Neither `goreleaser check` nor a snapshot build sees
that, because the comparison happens as the cask is pushed — the last thing a
release does, after everything else is already uploaded.

## v0.6.0 — 2026-08-05

**Nothing here needs doing before you upgrade.** No configuration file
changes, and an existing keychain entry keeps working exactly as it did — the
new environment variable is consulted first, and nobody has one set.

If you installed with `go install`, the new install script and the Homebrew
cask are alternatives rather than replacements; upgrading the way you already
do still works.

### Added

**Installing no longer means building.** On macOS or Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/Ahngbeom/datavase/main/install.sh | sh
```

or `brew install Ahngbeom/tap/dv`, or a `.deb`, `.rpm` or `.apk` from the
release page. Until now there were two ways in and both asked for something:
`go install` needs a Go toolchain, which is the build a single static binary
exists to avoid, and the archive on the release page did not run on macOS at
all. The binary is signed ad-hoc by the Go linker and notarized by nobody, and
a file a browser downloaded carries the quarantine attribute that macOS
enforces against exactly that — so the instruction to download it and put `dv`
on the PATH was one that could not be followed.

Nothing curl downloads is quarantined, and the Homebrew cask strips the
attribute on install, so both routes run the moment they finish. An archive
taken by hand still needs `xattr -dr com.apple.quarantine ./dv`, which the
release page now says instead of leaving it to be discovered.

The script verifies against `checksums.txt` before it installs anything, takes
`DV_VERSION` to pin a release and `DV_INSTALL_DIR` to choose where the binary
goes, and installs by rename so a `dv` that is running is never the
half-written file.

**A password can come from the environment.** `DATAVASE_PASSWORD_<NAME>` —
the datasource name upper-cased, with anything that cannot appear in a
variable name turned into `_`, so `prod-app` is `DATAVASE_PASSWORD_PROD_APP`.

A headless Linux server runs no D-Bus Secret Service, and that is what the
keychain needs. On the machine a terminal database client is most likely to be
installed on there was nowhere to put a password and no other way to supply
one: `dv init` proved the connection, wrote the configuration, then failed and
pointed at `dv auth`, which needs the same keychain that had just refused. The
wizard now finishes and names the variable to export.

It takes precedence over the keychain rather than filling in behind it, so the
same command does the same thing on every machine. It is also how a password
is supplied in CI or a container.

**`dv init` sets up the first datasource by asking.** Until now the first thing
a new user met was a YAML example printed to stderr, which asked someone who
had not run datavase yet to know what `env` changes and to get a hand-typed
file past a parser that rejects unknown keys. `dv` with no configuration now
asks the questions instead; `dv init` asks them on purpose.

**The connection is tested before the file is written.** A wrong host or a
wrong account is reported in the server's own words and asked again, with the
previous answer as the default. Nothing reaches disk until something answered,
so there is no wrong value to find and hand-edit later.

**`env` is the one question with no default.** Every other answer is offered
with something Enter will take; this one has to be chosen. It is the only thing
the wizard cannot work out for itself and the only one that decides whether the
guard does anything, and the guess that is wrong on a production database is
the guess that leaves it unguarded.

**It asks which keyboard you want.** The editor is still modal by default, but
that default is now a choice someone made rather than one they discover by
typing and watching nothing happen. The answer is written to `config.yaml`
alongside a comment saying what the other presets are.

The password goes to the keychain in the same pass, so `dv init` is the whole
of setup. An existing `config.yaml` is never touched — a second datasource is
added by editing the file.

Nothing changes for an existing configuration: `dv` finds it and opens it
exactly as before.

**The first session opens on a short card.** The few keys worth knowing, and —
the part nothing else says — what the guard will do to a statement that changes
data on this particular datasource. Against production it says they are refused
until writes are unlocked; anywhere else, that they ask first.

Any key puts it away, and a key it named then does what it named: pressing `F1`
on a card that says "F1 show this help" opens the reference rather than nothing.
`getting started` in the command palette brings it back, because making a dialog
go away is not the same as having read it.

It is recorded under `XDG_STATE_HOME`, alongside the schema cache and the query
history, and is optional in the same way: a state directory that cannot be
written costs the card being shown once more, never the session.

**The command palette is grouped when you browse it.** Opened with nothing
typed it now reads under headings — *Running*, *The result*, *Finding things*
and so on — instead of forty commands in one undifferentiated run. Type and it
goes back to a flat list ranked by what matches, so nothing about searching
changes.

The heading names are searchable too: `server` narrows to the connection
commands, `keyboard` to the presets, without either word appearing in a command
name.

**The key reference opens with the five keys worth knowing first**, and on the
modal keyboard the way out of it is in that same opening block. It used to be
at the foot of the vim reference — so the answer to "I cannot type into this"
sat behind the whole of what the reader was trying to leave.

### Fixed

**Enter on a freshly opened palette runs the first command again.** With the
headings in place the first row is a heading, and Enter took row zero outright
— on the one dialog whose entire purpose is to run something. The arrows step
over headings for the same reason: a highlighted row that Enter does nothing to
is the same dead end.

**And it is no longer `unlock writes`.** That command was the palette's first
entry, so opening the palette and pressing Enter — two keys, nothing read —
unlocked writes against production. The first command is now `cancel`, which
does nothing when nothing is running.

## v0.5.0 — 2026-08-04

**Upgrade for the guard fix.** Every release up to `v0.4.0` classified `ANALYZE`
as a read, and `ANALYZE FORMAT=JSON DELETE FROM orders` runs the delete. Against
a production datasource it went through with nothing asked. Nobody but you can
reach it — this is the guard being wrong about your own statement rather than a
hole anyone else can use — but it is the guard being wrong about exactly the
kind of statement it exists for. It is under **Fixed**.

Nothing here breaks a configuration file. Four new keys are bound — `⌘⇧E`,
`⌘⇧U`, `⌘⇧W` and `⌘⇧L` — so a configuration that rebinds another action onto
one of them takes the key from the new one; `dv keys` shows what your map
actually resolves to.

### Added

**See what is holding a lock.** `⌘⇧L` or `locks` in the palette draws who is
waiting on whom. The connection at the bottom waits on nothing and is the one to
deal with.

A blocker's current statement is usually not the one that took the lock — a
transaction keeps what it took until it ends — so the common case is a
connection holding a row and running nothing at all, which a list of statements
cannot show.

`Nothing is waiting on a lock` and `this server does not expose InnoDB lock
waits` are different answers and are never printed for each other.

**Stop another connection's statement.** `⌘⇧W` or `stop a statement` in the
palette; `stop a connection` ends the connection and rolls back what it held.

Against production both have to be typed — `KILL`, or `KILL CONNECTION` —
because stopping somebody else's work is the operation that most wants a
confirmation nobody can wave through. Elsewhere a button is enough.

**This session's own connections are not offered**, and are refused if reached
another way: killing the control connection takes cancellation and every
catalog read with it. Only the connections currently held count, since the
server reuses an id once a connection has gone.

**See what else is running on the server.** `⌘⇧U` (`Ctrl+Shift+U`) or `sessions`
in the palette lists the connections: who, from where, doing what, for how long.
Working connections first and longest first, with anything past a few seconds in
red; the ones merely holding a socket open follow.

**A user without the `PROCESS` privilege is told so.** The server does not
refuse the query — it answers with that user's own connections, so a list of one
means either "nothing else is running" or "you cannot see it", and those are
opposite answers.

Reading is explicit rather than on a timer: a list that reloads under the cursor
is how the wrong session gets acted on.

**`⌘⇧E` runs the statement and reports what actually happened.** Where `⌘E`
shows what the optimiser expected, this shows the estimate and the measurement
together — `rows 20084 → 20000`, `filtered 33.2 → 0` — and calls out an
estimate the run contradicted.

Only when it is worth acting on: a count has to be both wide of the mark and
large enough to matter, and a filter is measured in percentage points rather
than as a ratio, where one per cent against a tenth would otherwise read as
tenfold.

Because it runs the statement, it goes through the guard **as** that statement.
`⌘⇧E` on an unbounded `DELETE` against production is refused exactly as running
it would be. The buffer is not touched; the wrapping happens on the way to the
server.

### Fixed

**`ANALYZE` was classified as a read, and it runs the statement it wraps.**
`ANALYZE FORMAT=JSON DELETE FROM orders` empties the table — MariaDB's spelling
of what MySQL calls `EXPLAIN ANALYZE` — and the guard let it through against
production with nothing asked.

A wrapper that runs its statement is now judged as that statement, bounding
clause included, so the wrapped delete asks exactly what the bare one asks. A
wrapper around a verb the tokenizer does not recognise falls to the same
fail-closed default as anything else unrecognised.

`EXPLAIN` on its own executes nothing and stays free.

**A bastion dropping mid-session used to be invisible.** `session.TunnelErr` was
documented as how that becomes visible and had no caller but its own test, so
what you saw was whatever the driver says about a socket that has gone —
`invalid connection` — which reads as the database being in trouble.

A failure that could have happened anywhere between here and the server now
names the bastion when the tunnel has recorded a forwarding failure. A statement
the server refused keeps its own message: a numbered MySQL error is proof it
arrived and was answered, and `Tunnel.Err` never clears, so without that rule
one transient forward failure would make every later typo read as a dead
bastion for the rest of the session.

The connection is not re-established on its own; switching datasource opens a
fresh one.

## v0.4.0 — 2026-08-03

Everything the roadmap called for is now built. Nothing here breaks a
configuration file, but one command changed its name and two function keys are
now spoken for — both under **Changed**.

### Added

**A query plan you can read.** `⌘E` (`Ctrl+E`) or `explain` in the palette asks
the server how it would run the statement under the cursor and draws it as a
tree in its own tab. The buffer is not touched.

A full scan, a filesort and a temporary table are called out. A warning has to
be something you could act on, so the server's own `<union1,2>` and `<derived2>`
are not flagged for being scanned — there is no index to add to a table it has
just written.

The tree is laid out for the pane's width and again whenever that changes, so
nothing reaches sideways. Nothing about the plan's shape is assumed: MySQL and
MariaDB disagree about it and both change it between versions, so what is drawn
is whatever the server sent.

`EXPLAIN` does not run the statement, which is why it needs no confirmation
against production. `EXPLAIN ANALYZE` does, and is deliberately not on this key.

**Switching datasource without restarting.** `⌘⇧D` (`Ctrl+Shift+D`, `F11`) or
`switch datasource` in the palette. Comparing production against stage is
routine and used to mean quitting.

Everything that says where you are moves in one step: the environment spine and
its colour, the guard policy, the schema tree, completion, the chosen schema and
the results. **The unlock does not travel** — it is granted for one datasource,
and an unlock earned on stage is not one on production.

A running statement refuses the switch; an open transaction asks first, since
switching closes the connection under it. A switch connects before it lets go,
so a datasource that cannot be reached leaves the session exactly where it was.

Passwords come from the keychain, so a datasource without `dv auth` cannot be
switched to.

**Sorting a result by a column.** `⌘⇧S` (`Ctrl+Shift+S`, `F12`, or `s` with the
grid focused) orders by the column under the cursor; again reverses it, and a
third press restores the order the server sent — the answer to any `ORDER BY`
the statement carried. An arrow in the header says which column and which way.

The ordering follows the column's **declared type**. Values arrive as bytes, so
`9` and `10` look alike whether the column is a `BIGINT` or a `VARCHAR`, and
deciding from the bytes would sort a `VARCHAR` numerically and disagree with the
server about its own column. An unknown type sorts as text.

It sorts the rows this session is holding, and says so when those are not the
result: a result cut at its row limit, or one still arriving, reports how many
rows were sorted rather than claiming the column was.

**A `:` command line**, on the modal keyboard — the default one. `:w` saves,
`:q` quits and asks first if there is
unsaved work, `:q!` does not ask, `:wq` does both, and `:e path` opens a file
from the attached worktree. Beyond those it runs the palette's commands by
name, so there is one set of names rather than two.

An abbreviation runs only while it names one command. `:c` is refused with a
list of what it could have been — `cancel` and `commit` are opposite outcomes,
and a command line has already been committed to by the time Enter is pressed.
`Tab` completes as far as one name reaches.

`unlock writes` answers to nothing but its whole name.

### Changed

**The production write unlock is now called `unlock writes`, not `write`**, and
`readonly` is now `lock writes`. Nothing about what they do has changed, and
the guard's refusal names the new one, so the only thing to relearn is what to
type into the palette.

The reason is the `:` command line above. It resolves the same command names,
and to anyone with vim in their fingers `:write` saves a file. Leaving the
unlock called `write` would have put the most dangerous thing here behind the
most reflexive thing a vim user types.

**`F11` and `F12` are now bound**, to switching datasource and sorting a column.
If your configuration rebinds some other action onto either, that action keeps
the key and the new one is reachable by its `⌘`/`Ctrl` binding or the palette —
`dv keys` shows what your map actually resolves to.

### Fixed

**`cw` stops at the end of the word.** It used to travel on to the start of the
next one, so changing a word took the space after it and ran the two together.
In SQL that join is silent — `SELECT namefrom t` reads as a column and an
alias, so neither the editor nor the server ever reports the typo. `dw` and
`yw` still take the space, and from whitespace `cw` still changes the blanks
alone.

## v0.3.0 — 2026-08-03

Nothing breaking. One thing you will notice immediately: the schema tree no
longer starts on screen.

### Changed

**The schema pane starts hidden.** `⌘B` brings it. The application already
chose overlay finders as its way around — a table, a schema and a file each
have a key that opens a searchable list — and a permanent tree on top of them
spends a third of the screen saying what those already answer. The opening
message names the key, because a pane nobody knows about is worse than one in
the way.

**The screen is framed differently.** The environment moved from a status-bar
field to a column down the left. The bar sheds fields to fit, so on a narrow
terminal the one cue that mattered was the one that disappeared; a column of
the frame cannot be squeezed out. It is also the only colour here that ignores
your terminal theme — a production cue that goes pale because someone softened
their red is not a cue.

The top line now says where you are and the bottom what just happened.
Regions are separated by a hairline rather than boxed, and each names itself
once on its own header.

### Added

**Attach a directory of SQL and work in it.** `dv open local --dir ~/work/migrations`,
or `attach directory` in the palette. `⌘⇧O` lists the `.sql` files, `⌘S` saves.

The listing comes from git rather than a directory walk, so a file
`.gitignore` excludes is never offered, and each row says what git makes of
it. git stays optional — a plain directory still yields its files and only
loses the branch and the markers.

Four things it will not do quietly: overwrite an edit made since the file was
opened, leave a half-written file, lose an unsaved buffer, or open something
over 4 MiB that would stop the text widget drawing.

**Search what is on screen.** `/` searches whatever has focus, `?` backwards,
`n` and `N` step. The prompt sits on the bottom row rather than over the
screen, so you can see what is being searched while you type it. Matching is
literal and smart about case.

Searching the results reads the data rather than what is drawn: the grid
truncates long values and doubles their brackets for the markup parser, so a
`[` in your data would otherwise never be findable, and nor would anything
past the two-hundredth character.

**The palette offers the commands that had only chords**, and a test now
refuses to let an action exist that is reachable neither by a plain key nor by
name — which is not hypothetical: a terminal keeping `⌘` for its own menus
left the schema tree with no way in at all.

**vim: `.` repeats the last change** — including the text typed into it, so
`ciw` a name, then `.` on the next one. Insert-mode keys go straight to the
widget without the state machine seeing them, so the text is read back out of
the buffer when Escape ends the insert rather than accumulated keystroke by
keystroke: a backspace or a paste is recorded as what it left behind.

A count on the `.` replaces the one the change carried, as vim's does — `3.`
after a `dd` takes three lines.

**vim: text objects.** `ci(` replaces an `IN (…)` list, `ci'` a string
literal, `diw` the identifier under the cursor — the sequences a SQL editor is
reached for most, because the region worth changing is almost always delimited
rather than a number of words away. `i` takes what the delimiters hold, `a`
takes the delimiters too.

A pair is found by counting depth outwards, so a nested list reaches the one
the cursor is actually inside. Quotes are paired from the start of the line
instead, because both delimiters are the same character and searching outwards
cannot tell an opening quote from a closing one — between two literals is
outside both, and it says so rather than taking the operator sitting there.

**vim: counts and the find motions.** `3j`, `2dw` and `3dd` — a count in front
of any motion or operator, which is the first thing a vim user's hands reach
for and which pressing `d` three times is not. And `f`, `t`, `F`, `T`: `f,`
across a column list is the motion a SQL editor is used for most, and none of
them did anything at all before.

Counts multiply around an operator as vim's do, so `2d3w` is six words. `f`
and `t` are inclusive when an operator is using them, so `df,` takes the comma
with it rather than leaving the separator behind.

**A value can be copied out of the results.** With the grid focused the copy
key used to fall through to "nothing selected and nothing running", so lifting
one value into a ticket or the next query was impossible. `⌘C` now copies the
selected cell in full — as the server sent it, not the truncated and escaped
copy the grid draws — and `copy row` in the palette takes the whole row, tab
separated.

The precedence changed with it, and is now sayable in one sentence: **while a
statement is running the key cancels; otherwise it copies whatever has focus.**
It used to depend on which pane you were in, which meant a grid — where a cell
is always under the cursor — would have taken the key away from cancelling.

**Result columns name their type**, in the row view where there is room for it
rather than in the grid header where the values have too little width already.
The buffer has kept the types since the first release and nothing ever read
them.

**A row can be read down the page.** A grid is the wrong shape for a wide
table — forty columns share the terminal between them, and each value was cut
at two hundred characters besides, with no way to see the rest. `⌘I` on a
result row now turns it on its side: one column per line, names lined up,
values in full, `j`/`k` to step between rows.

It is the key that already showed a table's definition. `inspect` means "show
me this in full" and resolves by what has focus, like `⌘C` and `/` already do.

**Transactions.** `BEGIN` now opens one, and the connection it opens on is
held for its whole life — which is what makes `COMMIT` and `ROLLBACK` mean
anything. Before this they were refused, because a transaction opened on a
pooled connection was abandoned by the next statement.

```sql
BEGIN;
UPDATE orders SET status = 'X' WHERE id = 42;
SELECT * FROM orders WHERE id = 42;
ROLLBACK;
```

`commit` and `rollback` are palette commands too, the status bar carries `TX`
while one is open, and quitting with one open asks before rolling it back.

`SET SESSION …` is accepted inside a transaction, where the held connection
means it genuinely persists, and still refused outside one — the refusal now
names the transaction as the way to make it stick rather than being a dead
end. `LOCK TABLES` stays refused either way, and DDL inside a transaction says
that MySQL will commit the transaction to run it.

This closes the gap left by v0.2.0's refusals: they stopped the interface
claiming something untrue, and this makes the thing true.

## v0.2.0 — 2026-07-31

There is one breaking change; read it before upgrading a production
datasource.

### Breaking

**A `prod` datasource now requires TLS unless told otherwise.**

TLS was not supported at all before, so `dv` could not reach a server
enforcing `require_secure_transport=ON` — AWS RDS and Aurora, Azure Database
for MySQL and Cloud SQL as they are normally run. Adding it raised the
question of what an absent setting should mean, and the answer follows `env`
the way the guard already does: `required` for `prod`, `preferred` everywhere
else.

So a `prod` datasource pointed at a server that speaks no TLS **will stop
connecting**. It stays reachable by saying so in the file, which is the point
— it becomes a visible decision rather than a silent default:

```yaml
datasources:
  - name: prod-app
    env: prod
    tls: disabled          # was the effective behaviour before v0.2.0
```

The five modes are MySQL's own `ssl-mode` names — `disabled`, `preferred`,
`required`, `verify-ca`, `verify-identity` — so what is configured here can be
checked against what the server was started with. `tls_ca:` names a private
certificate authority and replaces the system trust store rather than adding
to it. See [TLS](README.md#tls).

### Changed

**Transaction control and session statements are now refused, in every
environment.** `BEGIN`, `START TRANSACTION`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`,
`SET`, `LOCK TABLES` and `USE`.

They were accepted before, and did nothing. Each statement runs on a
connection borrowed from the pool and handed back when it finishes, so none of
that state reached the next statement: `BEGIN` opened a transaction nothing
could commit, `SET SESSION sql_mode = …` was thrown away with the connection,
and `ROLLBACK` reported success having undone nothing. On stage and dev it was
worse than silent — the guard raised a confirmation, so people pressed a
button to authorise something that then did not happen.

Being refused with an explanation is the honest answer until a transaction can
hold one connection for its whole life, which is still open. `USE` is refused
separately and points at the schema picker, whose choice does travel with
every statement.

If a script relied on `SET SESSION …` appearing to work, it never did.

### Added

**A write says what it changed.** The row count came from the result buffer,
which a write never fills, so every `INSERT`/`UPDATE`/`DELETE` reported
`0 rows` whatever it did. The guard would stop you, explain the statement and
make you agree to it, and then not say what happened.

```
1 row affected  ·  4ms
4812 rows affected  ·  1.2s
```

The number is MySQL's own, counting rows **changed** rather than matched — an
`UPDATE` setting a column to the value it already held reports zero. An
`INSERT` given an id carries it too.

Writes stay cancellable. The obvious way to get the count was to send them
through the connection pool, which would have quietly cost server-side
cancellation, since `KILL QUERY` needs the id of the connection actually
running the statement. They are sent on that connection instead, so `^C` stops
a runaway `UPDATE` exactly as it stops a runaway `SELECT`.

**The server's warnings are read.** MySQL reports data truncation, implicit
conversion and several `ALTER` side effects only as warnings, and calls the
statement a success. Nothing asked before, so a value quietly cut down to fit
its column looked exactly like one that fitted.

```
1 row affected  ·  4ms  ·  1 warning: Data truncated for column 's' at row 1
```

The server's own sentence is carried rather than a count, and it is never
dropped to make the status line fit — the elapsed time goes first.

**`Run everything` works.** `⌘⇧↩` (`Ctrl+Shift+↩`, `⇧F5`) was bound in every
preset, listed in the README and described by `dv keys`, and reported "not
built yet" for any buffer holding more than one statement — which is nearly
every file of SQL worth opening.

It now runs the buffer a statement at a time, in order, each through the
guard on its own. A confirmation pauses the queue until it is answered; a
refusal, a declined confirmation, a failure or `^C` stops the rest. The
status bar then says how far it got:

```
5 statements · 2 ran · refused at statement 3
```

That count is reported even when everything succeeded, because a batch that
stopped part-way leaves the database in a state nothing on screen describes,
and there is no transaction to unwind it.

### Fixed

- **The guard no longer names a command that does not exist.** A blocked
  production write read `unlock with :write`; no preset has a `:` command
  line, and the unlock is the palette entry `write`. The refusal now names the
  palette and the key this terminal can actually deliver, and only when
  enabling writes would genuinely lift it.
- **The README no longer claims more than the code does.** The production
  write unlock is documented, including the refusals it deliberately does not
  lift; the guard table distinguishes "refused" from "refused until unlocked";
  and the status section states what is not built rather than saying
  everything is.

## v0.1.0 — 2026-07-29

First release. Connect directly or through an SSH bastion, browse the schema,
edit and run SQL with schema-aware completion, stream large results, cancel a
runaway query on the server, search the query history, export to CSV or JSON,
and the production guard.
