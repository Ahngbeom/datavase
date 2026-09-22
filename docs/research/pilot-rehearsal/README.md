# Pilot rehearsal (v0.11.1)

## Classification

This is a synthetic-persona QA rehearsal with actual local binary and database
execution. It is not a real-user pilot and produced **zero qualifying pilot
days**. It does not affect the gate in
[issue #102](https://github.com/Ahngbeom/datavase/issues/102) or
`docs/pilot/README.md`. Every persona is a fictional voice a Claude subagent
adopted from one interview in `docs/research/persona-interviews/`; none of
this is a real participant's experience, preference, or demand.

## Why this exists

Before recruiting the five real people for issue #102, this checked whether
the pilot's own onboarding path — README-only install and configuration, a
first query, the weekly-diary format, the three stop-conditions — actually
works end to end, and whether it surfaces a critical-stop-class defect that
would otherwise cost the real pilot its four weeks.

## Environment

| | |
|---|---|
| Binary | `dv (devel)`, sha256 `894b7f526bcacecfb43a2e47bb4f879959b7d3d3c476d013d4a4dc57165194f9`, built via `make build` |
| Database | MariaDB 11.4 via `make db-up`, `shop` schema from `docs/demo/seed.sql` (`customers`, `orders`) |
| Account | `pilot_ro`, `GRANT SELECT ON shop.* TO pilot_ro@'%'` — chosen so persona 15's "already read-only account" condition applied uniformly, which also bounded the blast radius of five unsupervised agents against a shared database |
| Run date | 2026-09-22 |

Each persona ran in its own subagent with an isolated `XDG_CONFIG_HOME` /
`XDG_STATE_HOME`, was blind to `docs/pilot/README.md`, `internal/`, and every
other persona's interview, and could read only `README.md`, `SECURITY.md`,
and `CHANGELOG.md` — the same materials a real pilot participant gets.

## Personas

Selected to match the mix `docs/pilot/README.md` asks recruiters for: two who
reach through a bastion or SSH tunnel, one on Windows/WSL, one whose database
account is already read-only.

| Interview | Recruiting condition matched | Report |
|---|---|---|
| [01-backend-engineer](../persona-interviews/interviews/01-backend-engineer.md) | bastion + SSH tunnel | [reports/01-backend-engineer.md](reports/01-backend-engineer.md) |
| [07-senior-backend-engineer](../persona-interviews/interviews/07-senior-backend-engineer.md) | SSH tunnel + kubectl | [reports/07-senior-backend-engineer.md](reports/07-senior-backend-engineer.md) |
| [13-windows-wsl-developer](../persona-interviews/interviews/13-windows-wsl-developer.md) | Windows/WSL | [reports/13-windows-wsl-developer.md](reports/13-windows-wsl-developer.md) |
| [15-cloud-developer-without-bastion](../persona-interviews/interviews/15-cloud-developer-without-bastion.md) | database account already read-only | [reports/15-cloud-developer-without-bastion.md](reports/15-cloud-developer-without-bastion.md) |
| [05-mysql-dba](../persona-interviews/interviews/05-mysql-dba.md) | high-frequency daily user, deepest technical scrutiny | [reports/05-mysql-dba.md](reports/05-mysql-dba.md) |

## Stop-conditions

**Zero of five personas observed any of the three stop-conditions** (a wrong
value or export, a wrong read-only/datasource/schema display, an exposed
credential). Every session independently confirmed the top bar's datasource,
account, schema, and `read-only` badge against `mysql` CLI output or
`SELECT @@session.tx_read_only`, and found no plaintext password in
`config.yaml` or the state directory.

## Findings, ranked

1. **Stale error text survives on the status line as a "warning" on later,
   unrelated, successful statements** (05-mysql-dba). After one `UPDATE`
   was refused, four consecutive unrelated successful statements
   (`SET SESSION TRANSACTION READ WRITE`, three `SELECT`s) kept showing
   `1 warning: UPDATE command denied...` on the status line; it was only
   replaced when a genuinely new error occurred. Values were never wrong,
   but the one place that reports "what this statement just did" kept
   showing dead information — close enough to the spirit of stop-condition
   (a) that the persona ranked it first. Reproduced with an exact step
   sequence in the report; not yet independently verified against the
   source by this coordinator.
2. **~70x query latency versus the `mysql` CLI on the same container**
   (07-senior-backend-engineer). dv's own status line reported 3.02s–3.52s
   for four simple `SELECT`s against a six-row table; the same statements
   via `mysql` CLI took 43ms. Reproduced 4/4 in one session. This is the
   single finding most likely to defeat the pilot's actual question — repeat
   choice over `mysql` — and needs independent maintainer reproduction
   before the real pilot starts, since only one persona measured it and the
   rehearsal's blinding rules kept it from reading the Go source to explain
   it.
3. **`dv check`'s diagnostic text conflates credential rejection and
   schema/permission rejection** (15-cloud-developer-without-bastion). A
   nonexistent-database connection (server error 1044) and a wrong password
   (error 1045) both surface as "the server refused the credentials — check
   user, and the stored password with `dv auth`." The persona had explicitly
   asked, in interview, for these two causes to be told apart. Reproducible
   with `dv -c <config> check <name>` against a bad schema name. By
   contrast, port and DNS failures against this same persona produced
   diagnostics ("check ... whether the server or the proxy in front of it is
   running" / "whether the VPN or DNS it needs is up") that matched their
   workflow well — noted as a positive signal, not a finding.
4. **Unresolved by this rehearsal: whether the client `read_only: true`
   guard shows a distinct message from a server GRANT denial.** All five
   personas hit the same wall independently: the shared `pilot_ro` account
   is `SELECT`-only at the grant level, so every write attempt was refused
   with MySQL's own `Error 1142` before dv's session-level guard could show
   whatever text it shows. README's line — "a refused statement says which
   setting refused it" — was neither confirmed nor contradicted. This
   matches something interviews 05, 07, and 15 all raised independently
   before this rehearsal. **Re-run the write attempt with a write-privileged
   grant and `read_only: true` on the datasource before the real pilot**, to
   confirm the guard actually names itself.
5. **`dv ls`'s "(password stored)" does not distinguish a keychain entry
   from a password currently visible via `DATAVASE_PASSWORD_<NAME>`**
   (found independently by 01, 07, 15 — three of five). For anyone exporting
   a short-lived token per shell session, "stored" reads as more persistent
   than it is.
6. **The add-datasource form's `Test` button does not read
   `DATAVASE_PASSWORD_<NAME>`**, while `Save` followed by `dv ls`/`dv check`
   does (01-backend-engineer). Leaving the password field blank to rely on
   the environment variable makes `Test` fail with "Access denied" even
   though the saved datasource works. README does not document this
   asymmetry.
7. **CSV export writes to the process's current working directory with no
   absolute path shown**, before or after saving (13-windows-wsl-developer,
   05-mysql-dba). 13 found a stray, corrupted 683KB CSV apparently left in
   the repository root by a concurrently running persona rehearsal —
   concrete evidence that two `dv` processes sharing a working directory can
   collide on export filenames. Both the confirmation screen and the
   completed-export status line show only the filename.
8. **Windows/WSL is effectively undocumented** (13-windows-wsl-developer).
   "Windows" appears four times in README, one of them "The macOS and
   Windows binaries are built on every change and run by nobody."; "WSL"
   never appears. No `winget`/signing story beyond a bare `.zip`, no stated
   config path (`%APPDATA%` vs `$XDG_CONFIG_HOME`), no `clip.exe` in the
   clipboard helper list, no mention of Windows Credential Manager or
   Windows Terminal.
9. **Minor, cosmetic**: `LIMIT 1000 added` is shown even for a `FROM`-less
   `SELECT` that can only ever return one row (07, 15). The CSV-export
   dialog selects Markdown, not CSV, on a bare Enter (07). README does not
   say whether `history:` defaults to on or off when omitted (07). Inside
   tmux, the key-hint labels showed `Super+↩` rather than the "Ctrl
   spelling" README promises for tmux — functionality was unaffected, only
   the label (13; flagged as possibly specific to this sandbox's tmux and
   worth checking in a real terminal).
10. **Already-known, reconfirmed, not new**: no way to import datasources
    from an existing `mysql`/`~/.my.cnf` setup — the #1 ask from interview
    09 and raised again here by 07.

## Infrastructure note, not a product finding

`dv auth` / `Save` write to the real macOS login keychain regardless of
`XDG_CONFIG_HOME`/`XDG_STATE_HOME` (01-backend-engineer). Running personas in
parallel on one machine risks keychain-entry name collisions between them;
this run's entry (`service=datavase, account=prod-ro`) was deleted after the
session. A future rehearsal that runs personas concurrently should give each
one a distinct datasource name or accept this as a known limitation.

## Recommended before the real pilot starts

- Investigate findings 1–3 and decide fix-or-accept for each; if any is
  fixed, re-run this rehearsal against the patched binary.
- Re-run the write-refusal check (finding 4) with a write-privileged grant,
  since none of the five personas could confirm README's guard-naming
  promise.
- Given three personas independently hit finding 5 and two independently hit
  finding 7, both are worth fixing regardless of severity — independent
  rediscovery is itself a signal.

## Limitations

- No real bastion, SSH tunnel, VPN, or remote network latency — every
  persona connected directly to a local Docker container.
- No real WSL, Windows, or non-macOS terminal — the Windows/WSL persona's
  findings come from reading README/CHANGELOG critically, not from running
  on Windows.
- Every TUI session was driven by PTY/tmux automation standing in for a
  human, not a person's own hands; each report names where that plausibly
  changed the result (keychain prompts requiring a real TTY, timing
  granularity, this sandbox's non-standard tmux).
- One shared account, `SELECT`-only at the grant level, across all five
  personas — this is why finding 4 is unresolved, and why no persona could
  independently verify the client read-only guard's own wording.
- One short session per persona against a six-row/four-row dataset, not four
  weeks of real production use — this rehearsal cannot speak to repeat
  choice, the actual question the real pilot exists to answer.
