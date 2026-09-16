# Commerce incident-response evaluation

## Classification and persona

This is a **synthetic-agent evaluation**, not a real employee interview or real
user pilot. The persona models a backend engineer investigating fictional
checkout and payment incidents. Nothing here is evidence of employment,
adoption, retention, or human preference.

Overall result: **INCONCLUSIVE — evaluation stopped at the controlled-write
contract**. Connection, investigation, result inspection, sorting, and the two
session read-only checks produced usable evidence. The controlled write showed
the expected normalized refusal and the independent post-check found no data
change, but the captured product evidence did not expose MariaDB error code
`1792`. The current evidence contract requires that numeric code, so the write
cannot be scored as a pass. CSV export was also only partial because the
save-path field retained its generated default filename; the evaluator renamed
the product-created file to the manifest path afterward.

## Environment and evidence

- Run: `sde-20260915-005610`; scenario: `commerce`; `synthetic: true`.
- Binary: `dv (devel)`, SHA-256
  `f91f1fd387ba3a8a6cff65b30f2ef31f829d17046db24a8609901404139c10c5`.
- Database: isolated `datavase-eval-commerce`, host port `13316`, schema
  `commerce_ops`, server `11.4.12-MariaDB-ubu2404`.
- Datasource: `commerce-eval`, TLS disabled for this local-only fixture,
  `read_only: true`, history disabled.
- Raw PTY evidence: `commerce-004` through `commerce-013`, bound to transcript
  SHA-256 `a417adebfd50c0bd7321e22b09190a72e5b53a9ae17a10045eedf9f12e52b56f`
  and byte size `1083017`.
- `commerce-018` records a clean generic credential-pattern scan over the exact
  five commerce artifact paths with empty stdout. The original generated
  password is no longer available in this process, so an exact-value scan
  against that in-memory password cannot be reproduced and credential absence
  is not promoted to a fully verified claim.

## Results

| Capability | Result | Evidence | Observed fact |
|---|---|---|---|
| Initial setup and seed | not scored | `commerce-001` | The original harness combined container health, fixture loading, and verification into one summary. Raw command boundaries and timings were not retained; the event is now explicitly a non-command evidence-normalization record. |
| Datasource discovery | pass | `commerce-002` | `commerce-eval` resolved to `127.0.0.1:13316/commerce_ops`. |
| Connectivity and identity | pass | `commerce-003`, `commerce-004` | Check reported MariaDB 11.4.12; top bar showed `commerce-eval @commerce_ops`, port 13316, and `read-only`. |
| Latest failed payment investigation | observed before stop | `commerce-005`; later corroboration `commerce-015` | `ORD-1007 / PENDING / DECLINED / DO_NOT_HONOR` was visible. The later MariaDB output corroborates it but is excluded from completion scoring because it was captured after the retrospectively identified stop. |
| Status aggregation | observed before stop | `commerce-006`; later corroboration `commerce-016` | Six enum statuses and counts were displayed. The later raw output is retained as historical evidence but excluded from completion scoring. |
| Partial-refund lookup | observed before stop | `commerce-010`; later corroboration `commerce-017` | `ORD-1005 / 19.95 / duplicate order — synthetic` was displayed. The later raw output is retained but excluded from completion scoring. |
| Row inspection | pass | `commerce-007` | F4 showed row 1 vertically with typed fields (`status ENUM`, `orders BIGINT`). |
| Client-side sort | pass | `commerce-008` | F12 sorted status ascending and displayed the sort indicator and notice. |
| CSV export | partial | `commerce-009` | Six sorted rows were correctly encoded, but typed absolute path was prepended to the retained default filename. The evaluator renamed that output to `exports/commerce.csv`; this is therefore not a clean pass for direct product creation at the manifest path. |
| Server-confirmed read-only state | pass | `commerce-011`, `commerce-012` | Both `@@session.transaction_read_only` and `@@session.tx_read_only` returned `1`. |
| Controlled UPDATE refusal | inconclusive / stop | `commerce-013`, `commerce-014` | UI showed exactly `write refused: read_only is set on this datasource`, and the immediate independent post-check kept `ORD-1007` at `PENDING`. However, `observed.query.error_code` is necessarily `null`: no raw numeric `1792` evidence was captured and the current password is unavailable for a faithful rerun. This fails the canonical pass contract and stops completion scoring. |
| Credential-pattern scan | partial | `commerce-018` | Sanitized `rg` invocation lists the exact commerce fixture, event log, transcript, report, and export paths; exit 1 with empty stdout means the generic pattern found no matches. The unavailable original password prevents the stronger exact-value scan. |

Under the current contract, `commerce-013` is the evaluation stop: the three
required write-refusal signals do not all agree because the observed numeric
error code is unavailable. This is an **evidence-quality inconclusive**, not a
confirmed product defect—the visible refusal and post-state are consistent
with a correctly blocked write. The original run did not create the shared
STOP signal because this contract gap was recognized only during review.
Events `commerce-015` through `commerce-017` therefore remain in the log as
historical post-stop evidence and are not used to claim scenario completion.

## Synthetic persona interpretation

As a synthetic commerce incident responder, the compact grid was effective for
the narrow questions “what was the latest payment outcome?”, “is the incident
concentrated in a status?”, and “which order has the partial refund?”. Keeping
the datasource, schema, endpoint, and read-only state visible reduced the risk
of losing environment context during an incident. Row inspection was useful
when a record had more fields than the terminal could comfortably show.

The largest friction was discoverability and focus state. Function-key
fallbacks made PTY automation possible, but moving between editor/results and
knowing F4/F12/F3 required the key reference. The CSV prompt's prefilled path
was especially risky: the evaluator's select-all input did not replace it and
produced a syntactically valid but unintended filename. In a time-sensitive
incident, a visible overwrite/replace behavior or an empty path field would be
easier to trust.

Fallback: for scripted checks, exact comparisons, or exporting to a prescribed
path, the MariaDB CLI remained clearer and more deterministic. Datavase was
more legible for interactive single-result inspection and retained stronger
environment identity on screen.

## Limitations

- This is one synthetic persona, one local macOS arm64 binary, one MariaDB
  container, deterministic fictional data, and one terminal size. It is not a
  usability study or longitudinal pilot.
- The PTY transcript preserves raw terminal control output and covers one
  session from `2026-09-15T01:22:44Z` to `01:25:01Z`. Per-action TUI timing was
  not captured independently; TUI event elapsed values bind to the whole
  session interval and must not be interpreted as per-query latency. UI status
  notices reported query times between 2 ms and 7 ms.
- Non-PTY timestamps are reconstructed to action-order windows because the
  initial harness did not persist a monotonic action clock. Their elapsed
  values are evidence bookkeeping, not performance measurements.
- The original `commerce-001` raw Docker/MariaDB actions and timings are not
  recoverable. Its normalized record preserves only a harness setup summary
  and is deliberately excluded from command-level and deterministic-seed
  scoring.
- The generated commerce password was held in the original evaluator's memory
  and is unavailable now. The generic credential scan is reproducible, but the
  exact password-value scan is not; no stronger absence claim is made.
- Clipboard behavior, tunnels, TLS, reconnect behavior, large result sets,
  production privileges, remote latency, and persistence across sessions were
  not exercised.
- History was disabled as required; the private state directory contained only
  Datavase runtime state and no history database was observed.
- The assigned container was retained through coordinator inspection and was
  removed during final cleanup on 2026-09-16.
