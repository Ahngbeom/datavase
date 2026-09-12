# datavase Persona Interview Research

> Status: 20/20 complete
>
> Product baseline: `Ahngbeom/datavase` through PR #89
>
> Started: 2026-09-12

## Important limitation

These are synthetic persona simulations, not interviews with real users and not evidence of demand. They are used to expose assumptions, compare plausible adoption contexts, and design better real-user research. No simulated quote should be presented as customer testimony.

## Research question

Who would replace a terminal `mysql` workflow with datavase, why would they change, and what would prevent initial adoption or continued use?

## Interview index

| ID | Persona | Primary hypothesis | Fit | Wave | Status |
|---:|---|---|---|---:|---|
| 01 | Backend Engineer | `ssh → mysql` replacement | High | 1 | Complete |
| 02 | CS Operations Analyst | CSV demand expands the ICP | Very low | 1 | Complete |
| 03 | Regulated SRE | Enterprise production adoption | Medium-low | 1 | Complete |
| 04 | Startup CTO | Small-team recommended workflow | Very high | 1 | Complete |
| 05 | MySQL DBA | Expert primary-user fit | Low; reviewer fit high | 1 | Complete |
| 06 | Junior Backend Engineer | Learning cost and safety anxiety | Medium; adopter fit low | 2 | Complete |
| 07 | Senior Backend Engineer | Autonomy and switching threshold | Medium | 2 | Complete |
| 08 | Platform Engineer | Standardization and support burden | Medium-high as rollout owner | 2 | Complete |
| 09 | Solo Developer | Low-frequency individual adoption | Low | 2 | Complete |
| 10 | Agency/Maintenance Developer | Multi-client datasource workflow | High, conditional on isolation | 2 | Complete |
| 11 | PostgreSQL-first Developer | MySQL-only product boundary | None currently; expansion signal | 3 | Complete |
| 12 | MariaDB Operator | Compatibility depth | Low as primary; high as validator | 3 | Complete |
| 13 | Windows/WSL Developer | Terminal and distribution friction | Medium, conditional | 3 | Complete |
| 14 | Kubernetes Pod Operator | Ephemeral runtime workflow | Medium, conditional | 3 | Complete |
| 15 | Cloud Developer without Bastion | Local terminal use without SSH wedge | Low | 3 | Complete |
| 16 | Engineering Manager | Purchase and rollout decision | Gatekeeper, not primary user | 4 | Complete |
| 17 | Security Engineer | Trust boundary and approval | Gatekeeper and validator | 4 | Complete |
| 18 | Developer Experience Engineer | Onboarding and team distribution | Rollout owner | 4 | Complete |
| 19 | Open-source CLI Power User | Competitive replacement threshold | Medium-high, role-specific | 4 | Complete |
| 20 | Churned Trial User | Retention failure | High initial fit; failed retention | 4 | Complete |

## Files

- `interviews/`: one complete record per persona
- `waves/`: cross-persona findings for each group of five
- `final-synthesis.md`: conclusions across all 20 simulations

## Common method

1. Begin with a recent concrete event, not a feature description.
2. Ask about current behavior, constraints, and alternatives before showing datavase.
3. Do not ask whether a desired feature is “good.”
4. Separate personal preference, permission to install, team adoption, and retention.
5. Record evidence that weakens the product hypothesis as prominently as positive reactions.
6. End with hypothesis changes and questions for real-user validation.

## Current provisional hypothesis

The strongest candidate is a Backend/Platform/SRE or multi-client maintenance engineer who repeatedly reads MySQL/MariaDB production data from a terminal. Frequency, environment clarity, reliable reconnection, and the ability to adopt or distribute trusted datasource configuration predict fit better than job title alone. Platform breadth should follow measured repeat use: every added database, OS boundary, proxy, or runtime multiplies compatibility obligations even when the visible feature set stays small.
