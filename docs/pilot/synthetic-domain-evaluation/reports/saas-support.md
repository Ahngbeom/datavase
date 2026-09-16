# Synthetic B2B SaaS support-operations evaluation

## Classification

**INCONCLUSIVE / EVIDENCE STOP.** This is a synthetic-agent evaluation, not a real-user pilot. It contributes zero qualifying pilot days.

One real `dv open` PTY process reached `saas-support-eval @saas_support` and its sanitized transcript records three reads, full-row/sort interaction, CSV save, two session read-only values of `1`, and the visible refusal `write refused: read_only is set on this datasource` (`saas-support-007`). The UI did not expose a numeric database error code; the observed code is therefore `null`. Because the retained PTY is a reconstructed textual capture rather than raw terminal bytes, the read-only result is conservatively inconclusive and is the evidence stop. Later independent checks are retained only as corroboration and excluded from scored product-success conclusions.

## Evidence

| Area | Result | Evidence |
|---|---|---|
| Reachability | `dv check` reported MariaDB 11.4.12 reachable; it did not prove read-only state | `saas-support-006` |
| PTY workflow | One actual process; UI observations and malformed-path CSV creation are consolidated into it | `saas-support-007`; transcript |
| Save relocation | Product-created 459-byte CSV moved unchanged from the malformed path to the allocated path | `saas-support-007`, `saas-support-011` |
| Entitlement | Exact retained `mariadb -B` output: enterprise/active, missing entitlement, effective `0` | `saas-support-017` (post-stop) |
| Open tickets | Exact retained output has tickets 9001, 9003, 9006 with UTF-8 and escaped newline | `saas-support-018` (post-stop) |
| Aggregation | Exact retained output has trialing 2/8, active 6/485, past_due 1/20, cancelled 1/18 | `saas-support-019` (post-stop) |
| Post-write state | Exact retained count for ticket 9001 is `1` | `saas-support-020` (post-stop) |
| CSV | A recovery/normalization record preserves the prior parser result and verifies the current CSV digest and size; it is not a replayable Ruby execution | `saas-support-021` (post-stop) |
| Credential hygiene | Not scored: no reproducible scan command/output or retained exact generated credential survived | limitation |

## Timing and recovery limitations

- `saas-support-007` is the only PTY process event. Its 247-second interval is a reconstructed evidence-window bound, not measured application latency; individual actions were not timed.
- Commands with equal timestamps and `elapsed_ms: 0` were not instrumented. Each such event labels zero as an unmeasured placeholder and makes no latency claim.
- During final remediation, the prior 21-line JSONL was accidentally truncated by a failed mechanical transformation. No backup, open file descriptor, temporary copy, or reachable Git object was found. The current log was recovered from the surviving transcript, report, fixture, CSV, and reviewer-captured event output. It does not claim byte identity with the lost JSONL.
- Exact database stdout for `saas-support-017` through `saas-support-020` was recoverable. The full Ruby source for the former CSV comparison was not; `saas-support-021` is explicitly an evidence-normalization, non-command recovery record limited to the surviving prior result and the current CSV digest/size. These events remain post-stop corroboration only.
- The corrected fixture remains deterministic, but the original erroneous fixture bytes were never preserved. That earlier attempt is no longer represented after recovery.
- Schema-tree table/column expansion, clipboard, TLS, keychain, tunnels, remote networking, and real support workloads were not exercised.

## Conclusion

The surviving evidence supports local reachability, one real interactive workflow, exact independent result values, post-write row preservation, and CSV content/lineage. It does not support a passing read-only or credential-hygiene verdict. No qualifying pilot, adoption, retention, or human-usability claim is made.
