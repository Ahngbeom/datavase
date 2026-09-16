# Datavase ran real local workflows, but all three evaluations remain inconclusive

## Verdict

The evaluated `dv` binary made actual connections to three isolated local
MariaDB databases and executed representative SQL. The retained evidence shows
useful results for commerce incident response, SaaS support investigation, and
healthcare operations. It does **not** satisfy the predeclared controlled-write
pass contract in any domain: none of the retained product evidence includes the
raw numeric MariaDB error code `1792` alongside the normalized refusal and an
independent unchanged-row check.

All three scenarios are therefore **INCONCLUSIVE / EVIDENCE STOP**. This is an
evidence-quality outcome, not a confirmed product critical defect. The visible
refusal wording and later database state are consistent with writes being
blocked, but the missing raw numeric signal prevents a pass. Post-stop
independent reads are retained as corroboration only and do not restore scenario
completion.

This was a synthetic-agent evaluation. It produced zero qualifying pilot days
and no evidence about employment, real-user usability, adoption, retention,
demand, or team preference.

## Evidence boundary

Verified claims are limited to retained local command output, event records,
artifacts, and the distinctions each domain report makes between raw,
reconstructed, and post-stop evidence. Synthetic persona interpretations are
test scenarios, not observations of employed people. Reconstructed PTY text,
unavailable generated passwords, and post-stop checks are never promoted to
product-pass evidence.

## Environment

The run used the locally built macOS arm64 `dv (devel)` binary with SHA-256
`f91f1fd387ba3a8a6cff65b30f2ef31f829d17046db24a8609901404139c10c5`
and three isolated MariaDB 11.4 Docker targets on ports 13316–13318. All three
evaluation containers and the dedicated temporary config/state root were
removed after final inspection.

## Completion matrix

| Criterion | Commerce | SaaS support | Healthcare operations |
|---|---|---|---|
| Isolated database target | Observed; original combined setup event is not scored | Corrected deterministic fixture survives; original erroneous bytes do not | Fixture survives; original seed command/full seed verification not canonical |
| `dv check` connection | Pass: MariaDB 11.4.12 | Pass: MariaDB 11.4.12 | Historical exact stdout retained; timing unmeasured |
| Interactive TUI evidence | One raw terminal-control transcript | One sanitized textual reconstruction of an actual process | Sanitized reconstruction only; no original PTY process evidence |
| Three representative reads | Displayed before stop | Recorded in reconstructed transcript | Not independently established through the TUI |
| Exact independent values | Exact post-stop MariaDB outputs for all three reads | Exact post-stop MariaDB outputs for all three reads | Exact post-stop MariaDB outputs for three operational reads |
| Datasource/schema/read-only cue | Visible in raw PTY | Recorded in reconstructed transcript | Recorded only in reconstructed transcript |
| Session read-only variables | Both spellings displayed as `1` | Both spellings recorded as `1` in reconstruction | Both spellings recorded as `1` in reconstruction |
| Controlled write | Normalized refusal; row unchanged; raw `1792` absent | Normalized refusal in reconstruction; later row count unchanged; raw `1792` absent | Normalized refusal in reconstruction; later row count unchanged; raw `1792` absent |
| CSV | Partial; product save required harness rename | Partial; product save required harness `mv` | Integrity only; product lineage unverified |
| Credential check | Generic pattern scan only; exact password unavailable | Not scored; no surviving reproducible scan | Generic pattern scan only; exact password unavailable |
| Final disposition | **INCONCLUSIVE / STOP** | **INCONCLUSIVE / STOP** | **INCONCLUSIVE / STOP** |

The original completion criteria requiring three completed scenarios, canonical
read-only proof, and fully verified artifact boundaries were not met.

## Critical findings

1. **No scenario retained the complete read-only proof required for a pass.**
   The contract requires both session variables to equal `1`, the exact visible
   refusal, observed numeric error code `1792`, and an independent unchanged-row
   result. The numeric code is absent in all three domains. This is the common
   stop condition.
2. **No wrong result, wrong datasource/schema indicator, successful controlled
   write, or credential disclosure was confirmed.** The evidence gaps prevent a
   pass, but they do not establish a product defect.
3. **The shared STOP file was not created during the original work.** The gap was
   identified retrospectively while reviewing evidence. Each report therefore
   marks later comparisons as post-stop corroboration and excludes them from
   completion scoring.

## Cross-domain UX

The most consistent positive cue was the persistent environment identity. Where
the TUI evidence was strong enough to assess it, datasource, schema, endpoint,
and read-only state stayed visible while queries were inspected. The compact
grid and full-row view were useful for narrow operational questions, especially
when a result contained wide text or typed fields.

The clearest repeated friction was CSV filename editing. In both commerce and
SaaS support, the save prompt retained its generated default filename when the
evaluator attempted to replace it with the allocated path. Datavase wrote a
valid CSV to an unintended concatenated filename, after which the harness moved
or renamed the unchanged bytes. These are partial export outcomes, not clean
passes for saving directly to a prescribed path. Healthcare cannot strengthen
this finding because its export lineage is reconstructed and unverified.

Keyboard discovery was another weak point. Function-key fallbacks enabled the
commerce workflow, while the SaaS evaluator could not establish table/column
tree expansion. The healthcare transcript is too weak to score navigation.
These are synthetic observations, not human usability findings.

## Domain fitness

- **Commerce incident response:** The retained raw PTY supports interactive
  investigation, row inspection, sorting, and visible environment context.
  Exact scripted comparisons remained clearer in the MariaDB CLI. The scenario
  is promising for narrow incident lookup but unqualified because setup evidence
  is incomplete, CSV targeting was unreliable, and read-only proof stopped.
- **B2B SaaS support operations:** The surviving evidence supports local
  reachability, one real interactive workflow, exact independent entitlement,
  ticket, aggregation, and post-write values, plus intact CSV bytes. Schema
  discovery was not established, the PTY is a textual reconstruction, and the
  read-only verdict remains inconclusive.
- **Healthcare operations:** Exact independent reads show the fixture can support
  failed-job, similar-clinic/time, audit-boundary, and post-write checks. The TUI
  transcript is reconstructed rather than authenticated raw product evidence,
  so interactive rendering, navigation, NULL/UTF-8 behavior, and export lineage
  are not scored.

## Security and filesystem review

All databases and rows are fictional. Credentials were configured through the
scenario-specific environment-variable names in the manifest and no report
contains a credential value. That does not amount to a complete secret-clearance
claim:

- commerce and healthcare retained reproducible generic-pattern scans with no
  match, but their generated password values were unavailable for exact-value
  scans;
- SaaS support has no surviving reproducible credential scan and is not scored;
- the healthcare canary is deliberate fixture content, and no canonical event
  proves a complete exact-value boundary scan;
- temporary config/state directories and all three containers remain present.

The retained CSV provenance differs by domain. Commerce records the final file
as harness-created after a product save to an unintended filename. SaaS records
the product-created bytes at the malformed path and a separate harness `mv` to
the allocated path with the same digest. Healthcare records only current-file
integrity and classifies lineage as unverified.

## Evidence quality

Evidence strength is not uniform:

- Commerce has the strongest TUI artifact: raw terminal control bytes bound to
  one session. Its original container/seed/verification commands were combined
  by the harness and are deliberately not scored.
- SaaS has a sanitized textual reconstruction of one actual PTY process. During
  final remediation, its untracked 21-line JSONL was accidentally truncated by
  a failed mechanical transformation. No backup or reachable Git object was
  found. The current eight-event log was conservatively reconstructed from the
  surviving transcript, report, fixture, CSV, and reviewer-captured outputs; it
  does not claim byte identity with the lost log. The former CSV parser source
  was not recoverable, so that result is an evidence-normalization record rather
  than a replayable Ruby invocation.
- Healthcare has no retained original PTY process artifact. Its transcript is a
  later sanitized reconstruction, and its exact MariaDB results are post-stop
  corroboration rather than proof of the TUI workflow.

Zero-duration or reconstructed event intervals are evidence bookkeeping, not
latency measurements. Exact result claims are limited to events retaining the
actual SQL and raw stdout. Reconstructed transcript statements are never treated
as equivalent to raw product output.

## Evidence index

Shared contract and allocation:

- `docs/pilot/synthetic-domain-evaluation/README.md`
- `docs/pilot/synthetic-domain-evaluation/manifest.json`
- `docs/superpowers/specs/2026-09-15-synthetic-domain-evaluation-design.md`
- `docs/superpowers/plans/2026-09-15-synthetic-domain-evaluation.md`

Commerce:

- Fixture: `docs/pilot/synthetic-domain-evaluation/fixtures/commerce.sql`
- Events: `docs/pilot/synthetic-domain-evaluation/events/commerce.jsonl`
  (`commerce-001` through `commerce-018`)
- Transcript: `docs/pilot/synthetic-domain-evaluation/transcripts/commerce-tui.txt`
- Export: `docs/pilot/synthetic-domain-evaluation/exports/commerce.csv`
- Report: `docs/pilot/synthetic-domain-evaluation/reports/commerce.md`

SaaS support:

- Fixture: `docs/pilot/synthetic-domain-evaluation/fixtures/saas-support.sql`
- Events: `docs/pilot/synthetic-domain-evaluation/events/saas-support.jsonl`
  (`saas-support-006`, `saas-support-007`, `saas-support-011`, and
  `saas-support-017` through `saas-support-021`)
- Transcript: `docs/pilot/synthetic-domain-evaluation/transcripts/saas-support-tui.txt`
- Export: `docs/pilot/synthetic-domain-evaluation/exports/saas-support.csv`
- Report: `docs/pilot/synthetic-domain-evaluation/reports/saas-support.md`

Healthcare operations:

- Fixture: `docs/pilot/synthetic-domain-evaluation/fixtures/healthcare-ops.sql`
- Events: `docs/pilot/synthetic-domain-evaluation/events/healthcare-ops.jsonl`
  (`healthcare-ops-001` through `healthcare-ops-009`)
- Transcript: `docs/pilot/synthetic-domain-evaluation/transcripts/healthcare-ops-tui.txt`
- Export: `docs/pilot/synthetic-domain-evaluation/exports/healthcare-ops.csv`
- Report: `docs/pilot/synthetic-domain-evaluation/reports/healthcare-ops.md`

## Limitations

This evaluation covers one macOS arm64 build, local Docker networking, MariaDB
11.4, deterministic fictional data, and synthetic personas. It does not cover
real users, production data, remote networks, SSH tunnels, VPNs, cloud
authentication, keychains, Windows/WSL, Linux, large workloads, reconnect
behavior, longitudinal use, or production permissions. Clipboard coverage is
incomplete. No performance conclusion is supported by reconstructed timings or
UI status notices.

## Conclusion

Datavase demonstrably connected to local MariaDB targets and supported useful
read workflows. The run cannot support a release-quality read-only safety pass
or a completed synthetic-domain evaluation because the predeclared numeric
error-code evidence was not retained. A future rerun should preserve raw PTY
bytes, exact action timestamps, independent seed checks, the raw `1792` signal,
exact generated-password scans, and direct product export paths from the start.
