# Synthetic domain evaluation design

## Goal

Run three independent, evidence-producing evaluations of the current `dv`
binary against real local MariaDB databases. Each evaluator acts from a
different working-domain perspective, creates a domain-specific schema and
fixture, and completes representative read tasks through `dv`.

This is a synthetic product evaluation. It can establish local execution,
database connection, SQL execution, result correctness, and exercised UX
paths. It cannot establish real employment, human usability, adoption,
retention, or a qualifying pilot day under `docs/pilot/README.md`.

## Scope

The evaluation covers the current repository build on this macOS arm64
workspace and MariaDB 11.4 in local Docker containers. It covers:

- build identity and checksum;
- datasource configuration and environment-supplied credentials;
- successful connection to an isolated database;
- schema/table discovery and representative read queries;
- result comparison against an independent MariaDB client;
- read-only enforcement;
- result copy or CSV save where the local environment permits it;
- terminal interaction observations, error messages, and fallback moments;
- filesystem artifacts created by the harness and by the product.

It does not cover remote networking, SSH tunnels, VPNs, cloud authentication,
Windows/WSL, Linux keychains, production systems, multi-day use, or real-user
preference.

## Evaluation architecture

The coordinator builds `dv`, records its version and SHA-256 digest, defines a
shared evidence contract, and assigns one isolated MariaDB container and one
artifact directory to each evaluator. Evaluators may read product source and
documentation but must independently perform their assigned workflow.

The three domains are:

1. **Commerce incident response — backend engineer.** Orders, payment
   attempts, refunds, and fulfilment events. Tasks trace a failed checkout,
   distinguish payment state from order state, inspect a wide row, aggregate
   failures, export a sanitized incident slice, and verify writes are refused.
2. **B2B SaaS customer investigation — support operations analyst.** Tenants,
   users, subscriptions, feature entitlements, and support tickets. Tasks find
   why a customer lacks access, join business entities without relying on
   internal IDs alone, sort/filter a result, save an escalation extract, and
   assess whether schema discovery is understandable to a SQL-literate
   non-developer.
3. **Healthcare operations — SRE/database operator.** Clinics, appointments,
   job runs, and pseudonymized audit events. Tasks diagnose a scheduling job,
   switch between similarly named schemas/tables safely, verify current
   datasource/schema and read-only truth, inspect null and timestamp handling,
   and check that no credential or sensitive fixture value leaks outside the
   expected result/export path.

All fixture data is fictional. Healthcare identifiers are synthetic and carry
no real personal or medical information.

## Isolation and ownership

Each evaluator receives a unique container name, host port, database name,
datasource name, config directory, state directory, environment variable name,
and artifact directory. No evaluator edits application source or another
evaluator's files. Shared repository files are written only by the coordinator
after evaluators finish.

Containers are disposable test infrastructure. Their names and ports are
resolved before creation, and only those exact containers may be removed.
Credentials are fixed synthetic test values, delivered through the appropriate
`DATAVASE_PASSWORD_<NAME>` environment variable, never embedded in an event
record or Markdown report.

## Evidence contract

Artifacts live under `docs/pilot/synthetic-domain-evaluation/`:

- `README.md`: method, limitations, environment, and reproduction steps;
- `fixtures/<domain>.sql`: reviewed schema and deterministic seed data;
- `events/<domain>.jsonl`: one JSON object per event with a single schema;
- `reports/<domain>.md`: persona-specific task outcomes and observations;
- `summary.md`: cross-domain findings, severity, and evidence links.

Every event contains:

- `schema_version`, `event_id`, `run_id`, `scenario_id`, and `synthetic: true`;
- start/end timestamps and elapsed milliseconds;
- binary version and digest;
- sanitized command and arguments;
- password source enum (`none`, `env`, or `keychain`) without its value;
- exit code and raw stdout/stderr or a PTY transcript reference;
- expected and observed connection/query claims;
- before/after artifact inventory with creator classification
  (`harness_created`, `product_created`, or `pre_existing`).

Raw output is retained unless it contains a credential or deliberately marked
sensitive fixture value. Redaction is represented explicitly rather than
silently summarized. Queries and expected results are stored in the fixture or
report so result claims can be reproduced.

## Workflow

The coordinator first checks the working tree, builds the current binary, runs
focused baseline tests, records artifact identity, starts three isolated
MariaDB containers, and waits for health checks. Each database is seeded using
its committed fixture and verified independently with the MariaDB CLI.

Each evaluator then:

1. records its assumed role, normal workflow, and task success criteria;
2. inspects only the public README before first use;
3. configures its assigned datasource without logging credentials;
4. runs `dv ls` and `dv check` and opens the real TUI in a PTY;
5. completes the domain tasks through visible product controls;
6. compares displayed/saved results with independent MariaDB queries;
7. attempts a controlled write on a read-only datasource and confirms both
   server state and UI wording;
8. records friction, discoverability, trust concerns, and fallback decisions;
9. inventories artifacts and submits its raw evidence and report.

The coordinator validates JSONL parsing and required fields, checks for secret
patterns, reviews all result comparisons, and produces the synthesis. A broad
test suite is run if evaluation uncovers behavior that appears inconsistent
with existing automated coverage. Product fixes are out of scope for this run;
suspected defects are reported with a minimal reproduction.

## Evaluation rubric

Each domain report separates facts from persona interpretation and scores:

- environment/setup: installation, config, credential, and connectivity;
- task completion: completion and fallback for every assigned task;
- correctness: displayed and exported values versus the independent client;
- safety/trust: datasource/schema identity, read-only truth, secret handling;
- usability: discovery, navigation, wide/null/time data, copy/export feedback;
- fitness: credible advantages and blockers for the represented workflow.

Scores use `pass`, `partial`, `fail`, or `not exercised`, always linked to an
event or transcript. Persona impressions are labelled synthetic observations,
not user findings.

## Critical-stop and error handling

The entire evaluation pauses if an evaluator observes a wrong value, wrong
datasource or schema indicator, false read-only state, or credential exposure.
The coordinator preserves sanitized evidence, independently reproduces the
observation, and classifies it as confirmed product defect, not reproduced,
external cause, or inconclusive. No later successful task erases a critical
finding.

Infrastructure failures are retried only after being identified as harness or
environment failures. The original failed event remains in the log. Missing
clipboard support is reported as an environment boundary; CSV save remains the
portable export criterion.

## Verification and completion criteria

The evaluation is complete only when:

- all three databases were created and independently seed-verified;
- the current `dv` binary successfully connected to each database;
- every domain executed at least three successful representative SQL reads;
- expected and observed values were compared for every scored correctness task;
- read-only enforcement was exercised in every domain;
- all JSONL files parse and satisfy the common required-field contract;
- credential-pattern and artifact-boundary checks completed;
- each domain report and the synthesis distinguish verified facts, synthetic
  interpretation, limitations, and untested behavior;
- container cleanup status and any skipped checks are recorded.

No qualifying pilot days, participant counts, retention conclusions, or
real-user claims are produced.

