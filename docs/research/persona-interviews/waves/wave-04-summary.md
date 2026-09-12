# Wave 4 Summary — Approval, Distribution, and Retention Failure

> Interviews 16–20. Synthetic persona simulations, not customer evidence.

## Personas

| ID | Persona | Primary finding | Fit |
|---:|---|---|---|
| 16 | Engineering Manager | Team adoption needs workflow-level time savings and support-cost evidence | Gatekeeper, not primary user |
| 17 | Security Engineer | Honest trust boundaries and inspectable artifacts matter more than broad safety claims | Gatekeeper and validator |
| 18 | Developer Experience Engineer | A binary is an artifact; onboarding also needs configuration, version, diagnostics, and ownership | Rollout owner |
| 19 | Open-source CLI Power User | A narrow production-read role is more credible than replacing every existing CLI | Medium-high, role-specific |
| 20 | Churned Trial User | Happy-path activation can collapse at the first unclear reconnect or connection failure | High initial fit; failed retention |

## Cross-persona findings

Wave 4 completes the adoption system around the likely end user. The person running a query, the manager approving a pilot, the security reviewer defining trust conditions, and the DevEx or Platform owner distributing the binary make different decisions with different evidence. One positive user reaction cannot stand in for all four.

The power user and churned user also show that installation is cheap but a place in the workflow is expensive. datavase does not need to replace `mysql`, `mycli`, and DataGrip everywhere. It needs to become the predictable default for a narrow production-read task and preserve `mysql` as an understood fallback without causing frequent fallback.

The strongest retention risk is not the intentionally small feature set. It is failure in the small promised path: connection setup, environment identity, value accuracy, reconnect, session read-only state, clipboard, and export. A feature roadmap cannot compensate for uncertainty in those steps.

## Hypothesis status

### Strengthened

- The adoption unit is a workflow, not a binary installation or an individual preference.
- Production-read positioning can coexist with DataGrip and standard `mysql` rather than replacing them globally.
- Second and later real-work use are the central product signal.
- Support time, fallback rate, and first-failure recovery belong in the success model.
- Distribution trust includes source-to-artifact provenance, version pinning, rollback, and a security contact.

### Weakened

- A compelling demo predicts sustained use.
- Free and open-source means team adoption has negligible cost.
- A manager or security reviewer can validate product-market fit through approval alone.
- Power users require unlimited customization or a plugin system.
- Churn primarily indicates missing IDE features.

### Rejected or deferred

- Installed binary count should be treated as active-user count.
- Client-side history is automatically helpful in production contexts.
- datavase should implement PAM, DLP, a central audit system, or organization management before the core workflow retains users.
- Team-wide standardization should precede a narrow five-user pilot.

## Adoption roles

| Role | Decision | Evidence required |
|---|---|---|
| End user | Use it for the next production read | Faster completion with reliable context and output |
| Champion | Recommend it to peers | Repeated success and understandable fallback |
| Manager | Fund or endorse a pilot | Net workflow time saved after support cost |
| Security | Approve a bounded use | Honest claims, credential/data lifecycle, provenance |
| DevEx/Platform | Distribute and support it | Reproducible config, pinned versions, diagnostics, rollback |

## Pilot proposal

Run a four-week pilot with five terminal-using MySQL/MariaDB engineers who performed at least four production reads in the previous month. Limit the recommended workflow to read-only inspection and result copy or approved export.

Measure:

- time from starting access setup to useful result;
- first datasource and first query success;
- use on a second and third distinct workday;
- eligible tasks completed with datavase;
- `mysql` or GUI fallback moment and reason;
- connection, context, value, clipboard, CSV, and reconnect failures;
- Platform or peer support minutes;
- security or data-handling exceptions.

Stop the pilot for credential exposure, incorrect displayed or exported values, false read-only state, or persistent wrong-environment ambiguity. Do not expand scope merely to improve install or demo metrics.

## Next validation

1. Interview real retained, rejected, and churned users separately.
2. Publish the smallest supported workflow and compatibility matrix.
3. Test failure recovery as prominently as the happy-path demo.
4. Provide secret-free diagnostic information, artifact checksums, and a rollback path.
5. Compare total user time saved with support and rollout time spent.
6. Use pilot evidence—not simulated quotes—to decide feature, platform, and monetization priorities.

---
