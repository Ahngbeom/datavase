# Wave 3 Summary — Platform Boundaries and Compatibility Cost

> Interviews 11–15. Synthetic persona simulations, not customer evidence.

## Personas

| ID | Persona | Primary finding | Fit |
|---:|---|---|---|
| 11 | PostgreSQL-first Developer | PostgreSQL demand does not prove this product's workflow value or justify engine complexity | None currently; expansion signal |
| 12 | MariaDB Operator | A support claim needs version- and behavior-level compatibility evidence | Low as primary user; high as validator |
| 13 | Windows/WSL Developer | The WSL-to-Windows clipboard, credential, config, and file path is one end-to-end workflow | Medium, conditional |
| 14 | Kubernetes Pod Operator | Runtime and credential lifecycle dominate the small SQL interaction | Medium, conditional |
| 15 | Cloud Developer without Bastion | Terminal affinity alone does not create fit when proxy and short-lived auth are the main friction | Low |

## Cross-persona findings

Wave 3 tests whether broader platform coverage naturally expands the market. It does not. A product can preserve four visible tasks while multiplying its internal support surface across SQL dialects, server versions, authentication methods, operating-system boundaries, proxies, filesystems, and runtime lifecycles.

The useful unit of support is not a logo or a successful `SELECT 1`. It is an end-to-end workflow whose values, connection state, clipboard or export output, cancellation, reconnect behavior, and failure messages remain correct. A narrow, published compatibility contract can create more trust than a broad unsupported claim.

The `ssh → mysql` message remains a useful wedge, but it is a segment filter rather than a universal description. WSL and Kubernetes users may still begin inside a terminal, while cloud-proxy users can be terminal-native without using SSH at all. The product should not promise to own every connection lifecycle before repeat use of its core query workflow is proven.

## Hypothesis status

### Strengthened

- Terminal context and repeated production-read behavior matter more than OS or job title.
- Correct display and export of raw values is a release-level trust requirement.
- Support claims should name tested server, OS, authentication, and runtime combinations.
- Actual session state after reconnect matters more than datasource configuration intent.
- Narrow recommended paths are preferable to ambiguous broad compatibility.

### Weakened

- PostgreSQL support would automatically create a large adjacent user base.
- MySQL protocol compatibility is enough to claim MariaDB support.
- A static binary makes Windows, WSL, container, and cloud distribution equivalent.
- SSH support covers most terminal production-read workflows.
- Clipboard and CSV are simple platform-independent output features.

### New risks

- Silent type coercion or truncation can make a readable result confidently wrong.
- Windows native and WSL can split config, credentials, clipboard, and export paths.
- Ephemeral containers can leak secrets through environment, logs, history, or persistent volumes.
- Short-lived credentials can expire across idle sessions and reconnects.
- Platform expansion can consume the reliability work needed by the current MySQL ICP.

## Product boundary

These interviews do not support adding PostgreSQL, Kubernetes orchestration, or broad cloud credential integrations now. They support defining and testing the compatibility contract already implied by MySQL/MariaDB, supported terminals, SSH, clipboard, CSV, and session read-only.

Expansion should require behavioral evidence: the target segment repeatedly experiences the core problem, a limited prototype is selected again during real work, and supporting the new path does not materially reduce reliability for the existing one. Requests, stars, and market size are discovery inputs rather than sufficient evidence.

## Compatibility evidence hierarchy

| Evidence | What it establishes |
|---|---|
| Build succeeds | Artifact can be produced |
| Connection succeeds | Network, TLS, and authentication work for one case |
| Fixture matrix passes | Types, NULLs, encoding, copy, and CSV remain accurate |
| Failure-path tests pass | Reconnect, cancel, expiry, and errors are understandable |
| End-to-end environment test passes | Terminal, clipboard, filesystem, credential, and runtime boundaries work together |
| Repeated real-work use | The supported path creates enough value to retain users |

## Next validation

1. Publish an explicit MySQL/MariaDB version and platform test matrix before widening support claims.
2. Add fixtures for numeric precision, unsigned values, NULL, binary, encoding, multiline text, and timestamps across grid, copy, and CSV.
3. Test read-only state, cancellation, and reconnect against the actual server session.
4. Choose one Windows/WSL path and validate installation through Windows clipboard and file consumption end to end.
5. Test the existing binary in a non-root, read-only-filesystem toolbox container before considering Kubernetes features.
6. Interview cloud-proxy users to determine whether the query UI creates repeat use after their external connection setup remains unchanged.
7. Require at least two real-work reuses of a limited prototype before committing to a new database engine or credential lifecycle.

---
