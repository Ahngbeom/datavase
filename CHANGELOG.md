# Changelog

What changed between releases, and what to do about it before upgrading.

The generated release page lists every commit; this file is the shorter,
edited account — and the place anything that needs action is written down.

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
