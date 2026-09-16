# Healthcare operations evaluation

## Classification and outcome

**INCONCLUSIVE — critical-stop contract not satisfied.** This is a
synthetic-agent evaluation by a fictional healthcare SRE/database operator. It
is not a real-user pilot, contributes zero qualifying pilot days, and uses only
fictional data.

The retained evidence supports a historical `dv check` response and a later
set of exact independent MariaDB reads. It does not establish completion of the
TUI workflow. The only TUI transcript is a sanitized reconstruction created
after the session; raw PTY bytes, per-action boundaries, duration, and the
numeric database error code were not retained. The reconstructed session says
the write was refused with the expected normalized wording, and a later
independent query found zero target rows, but `1792` was never observed in raw
evidence. The README requires all three signals. The controlled-write result
is therefore inconclusive and is the retrospective evaluation stop.

The original run did not create the shared STOP signal because this evidence
gap was recognized only during review. Events `healthcare-ops-003` through
`healthcare-ops-009` are retained as post-stop corroboration or artifact
validation and are excluded from scenario-completion scoring.

## Facts and evidence

| Criterion | Result | Evidence |
|---|---|---|
| Fixture | defined, not completion evidence | `fixtures/healthcare-ops.sql` defines 3 clinics, 15 appointments, 4 appointment events, 3 job runs, and 6 audit events. The original seed command and independent full seed verification were not retained as canonical events. |
| Reachability | observed with timing limitation | `healthcare-ops-001` preserves the exact `dv check` stdout for `11.4.12-MariaDB-ubu2404`. Its original duration was not retained; the normalized integer `0` is not a latency measurement. `dv check` does not prove read-only state. |
| Failed PTY attempt | preserved extraction, not original process event | `healthcare-ops-009` is an exact `sed` extraction of transcript lines 8–10 and records the `/dev/tty` failure separately. The original attempt lacked its own event and timing. |
| TUI context and reads | not independently established | `healthcare-ops-002` is explicitly a non-command normalization summary pointing to the reconstructed transcript. It is not raw product output and carries no observed connection or query claim. |
| Failed-job query | post-stop corroboration | `healthcare-ops-003` contains exact MariaDB batch stdout for `JOB-2042 / failed / DST_WINDOW_COLLISION`. It corroborates fixture state but does not validate the reconstructed TUI display. |
| Similar clinic names and times | post-stop corroboration | `healthcare-ops-004` contains the exact 11-row MariaDB batch output. It does not establish TUI navigation or rendering. |
| Audit boundary | post-stop corroboration | `healthcare-ops-005` contains five action/type/time rows and does not select `details`. It does not establish the earlier TUI result. |
| Read-only guard | inconclusive / stop | The reconstructed transcript records both session variables as `1` and the normalized refusal wording; `healthcare-ops-006` later reports zero `JOB-2099` rows. No raw observed `1792` exists, so the canonical refusal contract does not pass. |
| CSV artifact | integrity only; lineage unverified | `healthcare-ops-007` is an exact rerunnable `shasum` command and confirms the current 126-byte file digest. Because creation is supported only by the reconstructed transcript, the event classifies the file as `pre_existing` and makes no product-created or independent-equality claim. |
| Generic credential scan | partial | `healthcare-ops-008` records the exact generic-pattern `rg` invocation over the five healthcare repository artifacts; exit `1` with empty stdout means that pattern found no match. The original generated password is unavailable, so exact-value absence is unverified. |

## Synthetic persona interpretation

The reconstruction suggests that persistent datasource/schema/account/port and
read-only context could be useful to an operator, and that explicit local/UTC
columns would help diagnose the scheduling case. Those are synthetic
interpretations of an unscored reconstruction, not verified usability
findings. Table/column navigation, raw NULL/UTF-8 rendering, clipboard delivery,
and a trustworthy product-created export lineage were not established.

## Evidence and reproducibility limitations

- The reconstructed transcript is not raw PTY evidence. Its datasource bar,
  query values, NULL/UTF-8 rendering, refusal wording, export notice, and exit
  status cannot be independently authenticated from the retained bytes.
- `healthcare-ops-002.elapsed_ms` is the zero-duration normalization record,
  not the original TUI duration. `healthcare-ops-001.elapsed_ms` is a legacy
  integer placeholder because the historical check duration was not retained.
  Neither value is suitable for latency analysis.
- The evaluator datasource password is unavailable. A fresh authenticated
  `dv open`, raw 1792 capture, and exact password-value artifact scan could not
  be performed. Reproduction requires a new password, datasource, and seed.
- The generic scan is only a pattern scan. A separate final workspace check
  found the deliberate canary only in fixture line 90, but no canonical event
  proves an exact scan against the unavailable generated password.
- Events `healthcare-ops-003` through `healthcare-ops-006` are exact measured
  independent-client observations made after the retrospective stop. They may
  corroborate fixture state but cannot rescue completion or erase the stop.
- One local macOS arm64 binary and one MariaDB 11.4 container only; no tunnels,
  VPN, remote latency, keychain, production permissions, or real sensitive
  data were exercised.

## Artifact inventory

| Artifact | SHA-256 | Bytes | Creator / status |
|---|---|---:|---|
| `fixtures/healthcare-ops.sql` | `5bbd705e0ab2f09cc92e5237656d00ec29bdd0aa0d5f6f98d7039df1749d9343` | 4383 | harness |
| `events/healthcare-ops.jsonl` | `223b1a72dc984a9c0d7d70e1e7a24096335a3f947a2c2cbc4df987a236d975bf` | 17352 | harness |
| `transcripts/healthcare-ops-tui.txt` | `0d63b1d080fa8fd0bad8f0df8b7c6c99902acc2335977205f322325b70d7569e` | 2839 | harness; sanitized reconstruction |
| `exports/healthcare-ops.csv` | `ec5ab4cf7cced696ce1ffe69069e844f4f946e1d1986eac678facfdb2c24ee53` | 126 | lineage unverified; pre-existing at final validation |

The report is omitted from its own embedded digest because that would be
recursive. The coordinator can inventory the final report externally.
