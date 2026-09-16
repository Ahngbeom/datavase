# Synthetic domain evaluation

## Classification

This is a synthetic-agent evaluation with actual local binary and database
execution. It is not a real-user pilot and produces zero qualifying pilot days.

All personas, databases, and rows in this evaluation are fictional. Results may
establish what the recorded binary did in the recorded local environment, but
they are not evidence of employment, adoption, retention, or human preference.

## Environment

- Host: macOS 26.6.2 (build 25G83), arm64
- Go toolchain: `go1.26.4 darwin/arm64`
- Database target: MariaDB 11.4 in isolated local Docker containers
- Run ID: `sde-20260915-005610`
- Baseline build command: `make build` (exit 0)
- Focused baseline: `go test ./internal/cli ./internal/config ./internal/db
  ./internal/export ./internal/result ./internal/ui` (all six packages `ok`)

The environment facts above were captured on 2026-09-15. A later reproduction
must record its own environment and binary identity instead of assuming they
remain unchanged.

## Artifact identity

The evaluated repository binary is:

| Property | Captured value |
|---|---|
| Path | `./dv` |
| `dv version` | `dv (devel)` |
| SHA-256 | `f91f1fd387ba3a8a6cff65b30f2ef31f829d17046db24a8609901404139c10c5` |
| File identity | `Mach-O 64-bit executable arm64` |

Each event repeats the version and digest so evidence cannot silently become
detached from this build. The shared allocations and machine-readable identity
are in [manifest.json](manifest.json).

## Isolation map

| Scenario | Persona | Container | Port | Database | Datasource | Password environment variable |
|---|---|---|---:|---|---|---|
| `commerce` | Commerce incident-response backend engineer | `datavase-eval-commerce` | 13316 | `commerce_ops` | `commerce-eval` | `DATAVASE_PASSWORD_COMMERCE_EVAL` |
| `saas-support` | B2B SaaS support operations analyst | `datavase-eval-saas` | 13317 | `saas_support` | `saas-support-eval` | `DATAVASE_PASSWORD_SAAS_SUPPORT_EVAL` |
| `healthcare-ops` | Healthcare SRE/database operator | `datavase-eval-health` | 13318 | `healthcare_ops` | `healthcare-ops-eval` | `DATAVASE_PASSWORD_HEALTHCARE_OPS_EVAL` |

Every scenario uses its own generated database password, temporary config and
state directories, SQL fixture, event log, PTY transcript, report, and export.
Passwords are supplied only through the scenario's documented environment
variable and must not appear in repository artifacts. The exact config/state
directories and all five artifact paths are fixed per scenario in the manifest;
evaluators must not substitute another path.

## Evidence schema

Each manifest `artifacts.events` file contains one compact JSON object per
attempted action, including failed or retried actions. The following complete
record illustrates the canonical shape; it is a schema example, not run
evidence:

```json
{
  "schema_version": "1.0",
  "event_id": "commerce-004",
  "run_id": "sde-20260915-005610",
  "scenario_id": "commerce",
  "synthetic": true,
  "started_at": "2026-09-15T01:02:03Z",
  "ended_at": "2026-09-15T01:02:04Z",
  "elapsed_ms": 1000,
  "binary": {
    "version": "dv (devel)",
    "sha256": "f91f1fd387ba3a8a6cff65b30f2ef31f829d17046db24a8609901404139c10c5"
  },
  "invocation": {
    "command": "./dv",
    "args": ["open", "commerce-eval"],
    "password_source": "env"
  },
  "exit_code": 0,
  "stdout": "",
  "stderr": "",
  "transcript": {
    "applicable": true,
    "path": "docs/research/synthetic-domain-evaluation/transcripts/commerce-tui.txt",
    "sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
    "size_bytes": 3
  },
  "expected": {
    "connection": {
      "applicable": true,
      "datasource": "commerce-eval",
      "database": "commerce_ops",
      "server_version": "MariaDB 11.4",
      "read_only": true
    },
    "query": {
      "applicable": true,
      "sql": "UPDATE orders SET status='CANCELLED' WHERE order_no='ORD-1007'",
      "kind": "write",
      "row_count": null,
      "write_refused": true,
      "error_code": 1792,
      "error_message": "write refused: read_only is set on this datasource",
      "result_ref": null
    },
    "artifacts": [
      {
        "path": "docs/research/synthetic-domain-evaluation/transcripts/commerce-tui.txt",
        "kind": "file",
        "creator": "harness_created",
        "state": "present",
        "sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
        "size_bytes": 3
      }
    ]
  },
  "observed": {
    "connection": {
      "applicable": true,
      "datasource": "commerce-eval",
      "database": "commerce_ops",
      "server_version": "11.4.8-MariaDB",
      "read_only": true
    },
    "query": {
      "applicable": true,
      "sql": "UPDATE orders SET status='CANCELLED' WHERE order_no='ORD-1007'",
      "kind": "write",
      "row_count": null,
      "write_refused": true,
      "error_code": 1792,
      "error_message": "write refused: read_only is set on this datasource",
      "result_ref": "docs/research/synthetic-domain-evaluation/transcripts/commerce-tui.txt"
    },
    "artifacts": [
      {
        "path": "docs/research/synthetic-domain-evaluation/transcripts/commerce-tui.txt",
        "kind": "file",
        "creator": "harness_created",
        "state": "present",
        "sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
        "size_bytes": 3
      }
    ]
  },
  "artifacts": {
    "before": [
      {
        "path": "docs/research/synthetic-domain-evaluation/transcripts/commerce-tui.txt",
        "kind": "file",
        "creator": "harness_created",
        "state": "absent",
        "sha256": null,
        "size_bytes": null
      }
    ],
    "after": [
      {
        "path": "docs/research/synthetic-domain-evaluation/transcripts/commerce-tui.txt",
        "kind": "file",
        "creator": "harness_created",
        "state": "present",
        "sha256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
        "size_bytes": 3
      }
    ]
  },
  "redactions": []
}
```

Field rules:

- `event_id` is unique across the run and follows `<scenario>-NNN` in action
  order. `run_id` and `scenario_id` must match the manifest.
- Timestamps are RFC 3339 UTC values. `elapsed_ms` is a non-negative integer
  derived from the same action interval.
- `invocation.command` and `invocation.args` are sanitized, separate values;
  shell-expanded secrets are never recorded. `password_source` is exactly
  `none`, `env`, or `keychain`, and never contains a password value.
- `exit_code` is an integer containing the real process exit code. `stdout` and
  `stderr` are strings retaining raw, sanitized process output.
- `transcript` is always an object with exactly `applicable` (boolean), `path`
  (string), `sha256` (string or null), and `size_bytes` (non-negative integer or
  null). A PTY event sets `applicable: true`, uses the manifest transcript path,
  and requires the SHA-256 and byte size of the sanitized transcript. Its
  matching `artifacts.after` item must repeat the same digest and size. A
  non-PTY event uses `false`, an empty path, and null digest and size.
  Evaluators finalize and sanitize the transcript before writing JSONL, then
  reference that final digest and byte size from every event covered by the
  same PTY session.
- Both `expected.connection` and `observed.connection` are objects with exactly
  `applicable` (boolean), `datasource` (string), `database` (string),
  `server_version` (string), and `read_only` (boolean or null). When not
  applicable, use `false`, empty strings, and null rather than omitting keys.
- Both `expected.query` and `observed.query` are objects with exactly
  `applicable` (boolean), `sql` (string), `kind` (`read`, `write`, or `none`),
  `row_count` (non-negative integer or null), `write_refused` (boolean or null),
  `error_code` (non-negative integer or null), `error_message` (string or
  null), and `result_ref` (string or null). Large values belong in the
  referenced fixture, transcript, export, or report. A non-query event uses
  `false`, an empty SQL string, `none`, and null for the final five fields.
- Every entry in `expected.artifacts`, `observed.artifacts`,
  `artifacts.before`, and `artifacts.after` has exactly `path` (string), `kind`
  (`file` or `directory`), `creator` (`harness_created`, `product_created`, or
  `pre_existing`), `state` (`present`, `absent`, or `deleted`), `sha256`
  (string or null), and `size_bytes` (non-negative integer or null). A present
  file requires its digest and size. A present directory, absent path, or
  deleted path uses null for both; `absent` means it did not exist at inventory
  time and `deleted` means it existed in the preceding inventory and was
  removed by the recorded action.
- `redactions` is an array of objects with `field`, `reason`, and
  `replacement`. Allowed reasons are `credential` and
  `sensitive_fixture_value`. Redacted content uses an explicit marker such as
  `[REDACTED:CREDENTIAL]`; it is never silently summarized.
- A failed action remains an event. A retry gets a new event ID and does not
  replace the original record. Reports link every scored claim to event IDs and
  label persona impressions as synthetic observations.

### Controlled read-only write

Every evaluator uses the domain-specific harmless write from the execution
plan. Immediately before it, the evaluator runs both
`SELECT @@session.transaction_read_only` and the MariaDB-compatible alias
`SELECT @@session.tx_read_only` through the TUI and requires the one-row value
`1` from each. The first spelling is the one Datavase itself uses when opening
and confirming every session; MariaDB 10.3+ retains the second spelling as an
alias. The top bar must also show the assigned datasource, database, and
`read-only`.

The write is a pass only when all criteria agree: the TUI reports exactly
`write refused: read_only is set on this datasource`; `write_refused` is true;
and an independent MariaDB read confirms the target row count/value did not
change.

**`observed.query.error_code` is not among them, and an earlier version of
this contract was wrong to require it.** On a `read_only` datasource the
product replaces the server's sentence with the normalized one before the
interface shows it, so no run can observe the number through the TUI — which
is why every scenario in this evaluation recorded `null` and then failed
itself for it. Seeing the exact wording is what establishes the classification,
because the product emits it only after matching MariaDB error 1792, and the
product's own tests pin that mapping. A rerun that wants the raw number must
take it from a channel that does not normalize: the MariaDB CLI on the same
session, or a datasource that is not marked `read_only`.
Any successful write, other error, state value other than `1`, or unverifiable
post-state is a critical stop. Session read-only is an accidental-write guard,
not a substitute for database privileges.

## Where these files live

This evaluation sits under `docs/research/` beside the persona interviews,
because it is synthetic and produced no pilot day. `docs/pilot/` is for the
run with real people, and nothing here belongs to it.

It was carried out while the tree was under `docs/pilot/`, so the event logs,
the terminal transcripts and the dated execution plan still name that path.
Those files record what was run and are left as they were; the paths in this
file, in `manifest.json` and in `summary.md` are the current ones.

## Reproduce

From the repository root, first rebuild and verify the shared baseline:

```sh
make build
./dv version
shasum -a 256 ./dv
file ./dv
go test ./internal/cli ./internal/config ./internal/db ./internal/export ./internal/result ./internal/ui
jq -e '.schema_version == "1.0" and .synthetic == true and (.scenarios | length == 3)' docs/research/synthetic-domain-evaluation/manifest.json
```

Then, for one scenario at a time, use its exact allocation from the isolation
map: create only that named MariaDB 11.4 container, generate a fresh password,
load `fixtures/<scenario>.sql`, verify expected rows independently with the
MariaDB CLI, and run the documented `dv ls`, `dv check`, and PTY workflow using
a private temporary config. Supply the password using the exact `password_env`
from the manifest, keep `history: false` and `read_only: true`, and compare each
Datavase result with the independent client before scoring it. Before each
container, client, or Datavase action, check that the manifest `stop_signal`
path does not exist.

Reproduction details, exact SQL, input annotations, independent results, and
exports live in each scenario's fixture, transcript, event log, and report.
Never copy temporary configs or passwords into this directory.

## Critical-stop rule

Pause all scenarios immediately if Datavase shows or exports a wrong value,
shows the wrong datasource or schema, claims read-only for a session that is
not read-only, or exposes a credential. Preserve only sanitized minimum
evidence and notify the coordinator. The coordinator may later authorize a
separate independent reproduction and classify it as `confirmed product
defect`, `not reproduced`, `external cause`, or `inconclusive`. A later
successful action does not erase a critical finding.

The first evaluator observing a stop condition writes a credential-free signal
file at `/private/tmp/datavase-sde-20260915-005610/STOP` containing only the
run ID, scenario ID, event ID, UTC timestamp, and stop-condition category, then
immediately notifies the coordinator with that same metadata. All evaluators
poll this exact path before every external or product action. On observing it,
they interrupt active PTY input, cancel or terminate their own in-flight
Datavase/client process, and send an acknowledgement to the coordinator.

After the signal exists, no evaluator may start or seed a database, execute
another SQL statement, retry a product action, or continue scoring. Allowed
work is limited to sanitizing and hashing already captured evidence, recording
the interrupted event, notifying the coordinator, and cleanup of the exact
assigned container and private config/state directories. Independent
reproduction resumes only as a separately coordinated incident action after
all scenario evaluators have acknowledged the stop.

## Limitations

This run covers one current local macOS arm64 build, local Docker networking,
MariaDB 11.4, deterministic fictional data, and synthetic persona assessments.
It does not cover real users or production data; remote networks, SSH tunnels,
VPNs, cloud authentication, keychains, Windows/WSL or Linux; longitudinal use;
or claims about usability, demand, retention, and team adoption. Clipboard
behavior may be constrained by the terminal environment; a sanitized CSV save
is the portable export criterion.

## Cleanup status

Final evidence review classifies all three scenarios as **INCONCLUSIVE**,
because two of three transcripts are reconstructions rather than raw product
output and every domain's export lineage is unverified. The numeric error code
the contract above once required is not among the reasons: it was asking the
interface for something the product removes. See [summary.md](summary.md) for the completion matrix, cross-domain
findings, and evidence-quality limitations. In particular, the SaaS event log
was reconstructed after accidental truncation of the prior untracked JSONL and
does not claim byte identity with that lost file.

Cleanup completed on 2026-09-16 after the final evidence inventory:

- `datavase-eval-commerce`, `datavase-eval-saas`, and
  `datavase-eval-health` were removed;
- the dedicated temporary evaluation root
  `/private/tmp/datavase-sde-20260915-005610/`, including all scenario config
  and state directories, was removed;
- the shared `STOP` path was absent because the evidence gaps were recognized
  retrospectively rather than signaled during the original workflows; and
- repository evidence files were retained intentionally.
