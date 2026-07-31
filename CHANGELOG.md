# Changelog

What changed between releases, and what to do about it before upgrading.

The generated release page lists every commit; this file is the shorter,
edited account — and the place anything that needs action is written down.

## Unreleased

Intended as **v0.2.0**. There is one breaking change; read it before upgrading
a production datasource.

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
