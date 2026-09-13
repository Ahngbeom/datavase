# Final Synthesis — 20 Synthetic Persona Interviews

> This document synthesizes simulations, not customer interviews. It is a hypothesis map for real research and must not be cited as evidence of demand, adoption, willingness to pay, or product-market fit.

## Research question

Who would replace a terminal `mysql` workflow with datavase, why would they change, and what would prevent initial adoption or continued use?

## Provisional answer

The best initial candidate is an engineer who already performs short, repeated MySQL or MariaDB production reads from a terminal and has enough autonomy—or a nearby champion—to adopt a small tool. The most promising contexts are Backend, Platform, SRE, or multi-client maintenance work where environment confusion, wide results, repeated connection setup, result copying, or accidental-write anxiety creates visible cost.

The product should compete for a bounded role:

> The predictable terminal workflow for production MySQL reads that would otherwise use `mysql`.

It should not position itself as a terminal IDE, a DataGrip replacement, an access-control system, or a universal database client before repeated use supports those expansions.

## Segment map

| Segment | Provisional fit | Why |
|---|---|---|
| Frequent terminal production reader in a small team | High | Repeated task, local choice, direct `mysql` alternative |
| Multi-client maintenance engineer | High, conditional | Datasource reuse is valuable; customer isolation must be trustworthy |
| CLI power user with a distinct production-read profile | Medium-high | Will trial and contribute if the narrow role repeatedly wins |
| Junior backend engineer | Medium as user | Safety anxiety exists, but adoption usually needs senior or Platform approval |
| Senior backend engineer with mature scripts | Medium | High autonomy but high switching threshold and strong fallback |
| Platform/DevEx owner | Medium-high as enabler | Can create team retention if rollout support cost stays low |
| Windows/WSL or Kubernetes terminal user | Medium, conditional | Terminal fit exists, but end-to-end environment boundaries add cost |
| Regulated enterprise user | Medium-low | Organizational approval, data handling, and server controls dominate |
| Low-frequency solo or cloud-proxy user | Low | Setup and connection lifecycle outweigh occasional grid value |
| DBA | Low as primary; high as validator | Current scope cannot replace expert diagnostics but exposes trust gaps |
| CS or non-developer analyst | Very low | Access, SQL semantics, and terminal learning remain unsolved |
| PostgreSQL-only user | None currently | No usable product; engine request alone is not expansion evidence |

## What creates initial interest

- a short demo that maps `ssh → mysql` to `ssh → dv`;
- persistent datasource context;
- session-level read-only as an accidental-write guardrail;
- readable wide results and low-friction cell or row copy;
- CSV export when policy permits it;
- a single artifact with a simple installation path.

These are acquisition hypotheses. They do not establish retention.

## What determines retention

1. The first real datasource connects without configuration ambiguity.
2. Customer, environment, host, database, user, and actual session state stay visible.
3. Values remain exact across grid, copy, and CSV.
4. SSH, network interruption, idle expiry, and reconnect fail predictably.
5. Read-only is re-applied and verified against the live session.
6. The tool works in the user's real terminal, resize, clipboard, and filesystem path.
7. A failed task explains the next action quickly enough to avoid permanent fallback.
8. A second and third workday use feels easier than returning to `mysql`.

## Product principles suggested by the simulations

### Keep the functional surface narrow

Do not restore IDE features merely to satisfy DBA, DataGrip, PostgreSQL, or power-user comparisons. The narrow workflow is a positioning advantage if it is complete and reliable.

### Expand reliability before breadth

Treat connection identity, actual session state, exact value handling, reconnect, cancellation, clipboard, CSV, configuration migration, and actionable errors as the existing product—not ancillary polish.

### Make narrow claims

Session read-only is an accidental-write guardrail, not a replacement for DB privileges. Auto LIMIT does not bound server work. Client cancellation does not automatically prove server termination. Local history, clipboard, and CSV are data-lifecycle surfaces.

### Publish a compatibility contract

List tested database versions, platforms, terminals, authentication paths, SSH combinations, and known limitations. A successful build or `SELECT 1` is weaker evidence than fixture, failure-path, and end-to-end tests.

### Separate adoption roles

Design evidence for users, champions, managers, security reviewers, and rollout owners. Do not infer team readiness from individual installation or approval from individual preference.

## Measurement framework

| Stage | Primary measure | Misleading proxy |
|---|---|---|
| Discover | qualified README/demo visit | total repository impressions |
| Activate | first successful real datasource query | binary download or local demo query |
| Retain | use on second and third distinct workday | binary still installed |
| Replace | eligible task completed without fallback | session opened |
| Trust | no critical context, value, credential, or read-only mismatch | self-reported feeling of safety |
| Expand | teammate independently activates with stable support cost | link shared or manager approval |

Segment every measure by task frequency, terminal starting point, server permission model, configuration source, and connection path.

## Immediate recommendation

Do not use these interviews to justify a large feature build. Use them to run a real five-user pilot and close the highest-risk assumptions in the current promise.

### Before the pilot

1. Define the supported MySQL/MariaDB, OS, terminal, and SSH matrix.
2. Verify exact grid, copy, and CSV values with edge-case fixtures.
3. Verify read-only and connection identity on every connection and reconnect.
4. Document credential, config, history, clipboard, export, and telemetry lifecycles.
5. Provide checksums, version reporting, config validation, rollback, and a security contact.
6. Instrument only privacy-preserving funnel events or use a structured pilot diary.

### During the pilot

Recruit five users with at least weekly terminal production reads. Observe first setup, then measure second and third use over four weeks. Record the exact fallback moment and total workflow time, including Platform support.

### After the pilot

- Continue the narrow direction if repeated-use and fallback results improve.
- Fix reliability if acquisition is healthy but first-failure churn dominates.
- Revisit positioning if users retain for a different workflow than production reads.
- Consider a new platform or database only after a limited prototype earns repeat real-work use.
- Stop expanding if the eligible task is too infrequent to overcome maintenance cost.

## Decisions not supported by this research

The simulations do not support claims about market size, willingness to pay, conversion rate, customer demand, or actual safety outcomes. They also do not justify PostgreSQL support, a plugin system, enterprise administration, PAM, DLP, centralized audit, or DataGrip-class features.

Those decisions require real behavior, operational tests, and—for commercial questions—actual commitment or payment evidence.

---
