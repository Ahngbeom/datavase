# Wave 2 Summary — Adoption Authority, Frequency, and Team Operations

> Interviews 06–10. Synthetic persona simulations, not customer evidence.

## Personas

| ID | Persona | Primary finding | Fit |
|---:|---|---|---|
| 06 | Junior Backend Engineer | Safety anxiety is real, but a senior or platform owner controls adoption | Medium as user; low as adopter |
| 07 | Senior Backend Engineer | Autonomy does not erase switching cost when the current workflow already works | Medium |
| 08 | Platform Engineer | Team rollout depends on configuration, diagnostics, versioning, and support load | Medium-high as rollout owner |
| 09 | Solo Developer | Easy installation does not create retention when the task occurs monthly | Low |
| 10 | Agency/Maintenance Developer | Multi-client context creates value and a new cross-customer isolation risk | High, conditional |

## Cross-persona findings

Wave 2 separates four decisions that an install count would otherwise collapse: discovering the tool, completing a first connection, choosing it again for real work, and recommending or standardizing it for others. The same person may have authority to install but no reason to switch, or may be a likely user without authority to approve the workflow.

The repeated need is broader than accidental-write prevention. Users must know which customer, environment, host, database, user, and actual session state they are operating against. A stored datasource creates value only when it reduces setup work without hiding or confusing that context.

Usage frequency is a stronger predictor than company size. A solo developer who checks production monthly has weak retention despite full autonomy. An agency developer who changes customer and environment repeatedly has a stronger workflow benefit, but requires strict separation of credentials, history, exports, and visible context.

## Hypothesis status

### Strengthened

- Retention should be measured by second and third real-work use, not installation or binary persistence.
- Persistent environment identity is a primary product value, not decorative UI.
- Existing datasource import or managed configuration can matter more than editor features.
- Reliability under SSH reconnect, tmux resize, idle sessions, and config upgrades is a switching requirement.
- Platform and senior reviewers are part of the adoption path even when junior developers are the daily users.

### Weakened

- Individual installation autonomy predicts adoption.
- Junior developers will self-adopt a safer client after seeing a demo.
- Single-binary distribution is sufficient for team rollout.
- Read-only alone is a strong hook when DB accounts already enforce SELECT-only access.
- A broad “solo developer” segment is attractive without measuring workflow frequency.

### New risks

- A config-derived safety indicator can be mistaken for server-confirmed session state.
- Centralizing many datasources increases wrong-customer and wrong-environment risk.
- Shared history, CSV files, and clipboard data can cross datasource or customer boundaries.
- Team adoption can increase Platform support burden even when individual UX improves.
- Automatic updates and config schema changes can reduce incident-time reproducibility.

## Product boundary

The interviews do not justify restoring IDE features removed in PR #82. They point instead to operational completeness around the existing workflow: visible connection identity, accurate session-state reporting, durable configuration, understandable connection failures, and explicit local-data handling.

These are not all immediate feature commitments. They are assumptions to test against the current product before expanding scope. In particular, `read_only` must remain an accidental-write guardrail layered on DB permissions, not a claim that customer boundaries, expensive reads, or data handling are safe.

## Measurement implications

| Funnel stage | Candidate event or measure |
|---|---|
| Discover | README demo viewed; installation instructions opened |
| Activate | datasource saved; first connection; first successful query |
| Retain | second real-work session; third real-work session; days between uses |
| Replace | task completed without `mysql` or GUI fallback; fallback reason |
| Expand | configuration shared; teammate activated; support request count |

Segment these measures by production-read frequency, terminal starting point, DB permission model, and whether the user configured the datasource or received a managed one.

## Next validation

1. Recruit users by recent terminal production-read frequency rather than title alone.
2. Observe first datasource setup and record documentation visits, errors, and abandonment.
3. Re-test after a 30-day gap to measure discoverability without remembered shortcuts.
4. Compare config intent with the host, database, user, and read-only state confirmed by the live session.
5. Pilot with five users and measure independent reuse, Platform support time, and fallback reasons.
6. Test whether history, clipboard, and CSV behavior preserve datasource and customer boundaries.

---
