# Wave 1 Summary — Initial ICP and Product Boundary

> Interviews 01–05. Synthetic persona simulations, not customer evidence.

## Personas

| ID | Persona | Primary finding | Fit |
|---:|---|---|---|
| 01 | Backend Engineer | `ssh → mysql` replacement is understandable; read-only creates interest | High |
| 02 | CS Operations Analyst | CSV need does not overcome access, semantics, and terminal barriers | Very low |
| 03 | Regulated SRE | Personal interest does not imply organizational approval | Medium-low |
| 04 | Startup CTO | Frequent reads, a recent write incident, and decision authority align | Very high |
| 05 | MySQL DBA | Not a primary user; valuable reviewer of safety claims | Low as user, high as reviewer |

## Cross-persona findings

The strongest candidate is not “anyone who needs a CSV” or “any database professional.” It is a developer who already uses a terminal to inspect production MySQL, can choose a tool, and works in an organization where session-level accidental-write protection still reduces a real risk.

Read-only matters most when it changes the default path from a broad-privilege session to an intentionally constrained inspection session. It matters less when database permissions already enforce read-only access, and it does not solve lack of access, wrong datasource selection, expensive SELECTs, or data exfiltration.

## Hypothesis status

### Strengthened

- Compete with raw `mysql`, not DataGrip.
- Reliability matters more than feature breadth.
- Persistent environment context is part of the safety value.
- Small teams with local tool choice are more reachable than regulated enterprises.

### Weakened

- Read-only is a universal hook.
- Single binary alone removes organizational adoption barriers.
- CSV export is an unqualified positive.

### Rejected or deferred

- Non-developer data consumers are an immediate adjacent ICP.
- DBA features should be restored to win expert users.
- Client-side audit and policy controls can substitute for server-side enforcement.

## Product boundary

datavase should describe session read-only as an accidental-write guardrail, not enforced access control. Auto LIMIT, streaming, buffer truncation, and cancel should also be documented according to what each actually guarantees. A focused client gains trust by making narrow, testable promises.

## Next validation

- Observe 3–5 real backend/platform users for one to two weeks.
- Measure first successful connection and fallback-to-mysql reasons.
- Verify that environment and read-only status reflect the actual server session.
- Compare common row inspection and copy tasks against mysql CLI.
