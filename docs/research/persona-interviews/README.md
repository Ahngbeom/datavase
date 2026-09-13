# datavase Persona Interview Research

> Status: 5/20 complete
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
| 06 | Junior Backend Engineer | Learning cost and safety anxiety | TBD | 2 | Planned |
| 07 | Senior Backend Engineer | Autonomy and switching threshold | TBD | 2 | Planned |
| 08 | Platform Engineer | Standardization and support burden | TBD | 2 | Planned |
| 09 | Solo Developer | Low-frequency individual adoption | TBD | 2 | Planned |
| 10 | Agency/Maintenance Developer | Multi-client datasource workflow | TBD | 2 | Planned |
| 11 | PostgreSQL-first Developer | MySQL-only product boundary | TBD | 3 | Planned |
| 12 | MariaDB Operator | Compatibility depth | TBD | 3 | Planned |
| 13 | Windows/WSL Developer | Terminal and distribution friction | TBD | 3 | Planned |
| 14 | Kubernetes Pod Operator | Ephemeral runtime workflow | TBD | 3 | Planned |
| 15 | Cloud Developer without Bastion | Local terminal use without SSH wedge | TBD | 3 | Planned |
| 16 | Engineering Manager | Purchase and rollout decision | TBD | 4 | Planned |
| 17 | Security Engineer | Trust boundary and approval | TBD | 4 | Planned |
| 18 | Developer Experience Engineer | Onboarding and team distribution | TBD | 4 | Planned |
| 19 | Open-source CLI Power User | Competitive replacement threshold | TBD | 4 | Planned |
| 20 | Churned Trial User | Retention failure | TBD | 4 | Planned |

## Files

- `interviews/`: one complete record per persona
- `waves/`: cross-persona findings for each group of five
- `final-synthesis.md`: conclusions after all 20 simulations (planned)

## Common method

1. Begin with a recent concrete event, not a feature description.
2. Ask about current behavior, constraints, and alternatives before showing datavase.
3. Do not ask whether a desired feature is “good.”
4. Separate personal preference, permission to install, team adoption, and retention.
5. Record evidence that weakens the product hypothesis as prominently as positive reactions.
6. End with hypothesis changes and questions for real-user validation.

## Current provisional hypothesis

The strongest candidate is a Backend/Platform/SRE engineer in a startup or small team who directly reads MySQL/MariaDB production data, already uses a terminal workflow, can install a tool without central approval, and experiences environment confusion or accidental-write anxiety.
