# Datavase ran real local workflows, but all three evaluations remain inconclusive

## Verdict

The evaluated `dv` binary made actual connections to three isolated local
MariaDB databases and executed representative SQL. The retained evidence shows
useful results for commerce incident response, SaaS support investigation, and
healthcare operations.

All three scenarios are **INCONCLUSIVE**, for two different reasons that were
first reported as one.

**The controlled-write criterion was mis-specified and cannot be met through
the interface.** It asked for the raw numeric MariaDB error code `1792` in
retained product evidence. On a `read_only` datasource the product never emits
it: `readOnlyRefusal` in `internal/ui/transport.go` replaces the server's
sentence with `write refused: read_only is set on this datasource`, which the
contract in `README.md` describes correctly and then contradicts by demanding
the number the replacement removed. What each scenario did retain — the exact
refusal wording, both session variables at `1`, and an independent read showing
the row unchanged — is what the contract's own text says that wording proves.
This is a defect in the criterion, found by running it, and not a product
defect or a loss of evidence.

The raw code is already pinned where it can be: `TestIsReadOnlyRefusalKnows
TheServersNumber` in `internal/db` asserts the number, and the read-only
integration test asserts the server's own `READ ONLY` refusal. A future rerun
that wants the raw signal has to take it from a channel the product does not
normalize — the MariaDB CLI on the same session, or a datasource that is not
marked `read_only`.

**The scenarios remain inconclusive on their own evidence**, which is the
reason the disposition does not change. Two of three transcripts are
reconstructions rather than raw product output, one event log was rebuilt
after the original was truncated, healthcare retained no PTY process artifact
at all, and CSV lineage is unverified in every domain. Post-stop independent
reads are retained as corroboration only and do not restore scenario
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
| Controlled write | Normalized refusal; row unchanged; criterion's raw `1792` unobtainable through the interface | Normalized refusal in reconstruction; later row count unchanged; same criterion defect | Normalized refusal in reconstruction; later row count unchanged; same criterion defect |
| CSV | Partial; product save required harness rename | Partial; product save required harness `mv` | Integrity only; product lineage unverified |
| Credential check | Generic pattern scan only; exact password unavailable | Not scored; no surviving reproducible scan | Generic pattern scan only; exact password unavailable |
| Final disposition | **INCONCLUSIVE / STOP** | **INCONCLUSIVE / STOP** | **INCONCLUSIVE / STOP** |

The original completion criteria requiring three completed scenarios, canonical
read-only proof, and fully verified artifact boundaries were not met.

## Critical findings

1. **The read-only criterion asks the interface for something it is built to
   remove.** The contract requires both session variables to equal `1`, the
   exact visible refusal, the numeric error code `1792`, and an independent
   unchanged-row result. The first, second and fourth were obtained in every
   domain. The third cannot be: the product replaces the server's sentence,
   which the contract states and then asks to see through anyway. Rewrite the
   criterion before the next run rather than treating this as a finding about
   the product.
2. **One product defect was confirmed: the CSV save prompt appends what is
   typed to the name it suggested.** Two of three scenarios saved to a
   concatenated filename nobody chose and had to rename the file afterwards.
   An absolute path fails loudly; a relative one succeeds quietly at the wrong
   name. Filed as issue #105. It appears again under cross-domain UX below.
3. **No wrong result, wrong datasource/schema indicator, successful controlled
   write, or credential disclosure was confirmed.** The remaining evidence gaps
   prevent a pass, but they do not establish a further product defect.
4. **The shared STOP file was not created during the original work.** The gap was
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
- the temporary config/state root and all three containers were removed after
  the final inspection, which an earlier draft of this section denied while the
  environment section above recorded it correctly.

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

- `docs/research/synthetic-domain-evaluation/README.md`
- `docs/research/synthetic-domain-evaluation/manifest.json`
- `docs/superpowers/specs/2026-09-15-synthetic-domain-evaluation-design.md`
- `docs/superpowers/plans/2026-09-15-synthetic-domain-evaluation.md`

Commerce:

- Fixture: `docs/research/synthetic-domain-evaluation/fixtures/commerce.sql`
- Events: `docs/research/synthetic-domain-evaluation/events/commerce.jsonl`
  (`commerce-001` through `commerce-018`)
- Transcript: `docs/research/synthetic-domain-evaluation/transcripts/commerce-tui.txt`
- Export: `docs/research/synthetic-domain-evaluation/exports/commerce.csv`
- Report: `docs/research/synthetic-domain-evaluation/reports/commerce.md`

SaaS support:

- Fixture: `docs/research/synthetic-domain-evaluation/fixtures/saas-support.sql`
- Events: `docs/research/synthetic-domain-evaluation/events/saas-support.jsonl`
  (`saas-support-006`, `saas-support-007`, `saas-support-011`, and
  `saas-support-017` through `saas-support-021`)
- Transcript: `docs/research/synthetic-domain-evaluation/transcripts/saas-support-tui.txt`
- Export: `docs/research/synthetic-domain-evaluation/exports/saas-support.csv`
- Report: `docs/research/synthetic-domain-evaluation/reports/saas-support.md`

Healthcare operations:

- Fixture: `docs/research/synthetic-domain-evaluation/fixtures/healthcare-ops.sql`
- Events: `docs/research/synthetic-domain-evaluation/events/healthcare-ops.jsonl`
  (`healthcare-ops-001` through `healthcare-ops-009`)
- Transcript: `docs/research/synthetic-domain-evaluation/transcripts/healthcare-ops-tui.txt`
- Export: `docs/research/synthetic-domain-evaluation/exports/healthcare-ops.csv`
- Report: `docs/research/synthetic-domain-evaluation/reports/healthcare-ops.md`

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
read workflows. It cannot support a completed synthetic-domain evaluation,
because two of three scenarios rest on reconstructed transcripts and every
domain's export lineage is unverified. It found one real defect, in CSV export,
and one defect in its own contract.

A future rerun should preserve raw PTY bytes, exact action timestamps,
independent seed checks, exact generated-password scans, and direct product
export paths from the start — and should take the raw `1792` from the MariaDB
CLI rather than asking the interface for a number it exists to replace.
