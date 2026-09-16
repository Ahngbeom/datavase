# Synthetic Domain Evaluation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce reproducible evidence from three synthetic professional personas using the current `dv` binary against three real, domain-specific local MariaDB databases.

**Architecture:** The coordinator owns the binary, shared manifest, validation, and synthesis. Three parallel evaluators own disjoint fixture, event, transcript, and report files plus isolated MariaDB containers; they perform real PTY workflows and compare every correctness claim with the MariaDB CLI.

**Tech Stack:** Go 1.26, `dv`, MariaDB 11.4 Docker image, POSIX shell tools, PTY terminal automation, JSONL, SQL, Markdown.

---

## File map and ownership

Coordinator-created files:

- `docs/pilot/synthetic-domain-evaluation/README.md` — environment, method, limitations, reproduction and cleanup.
- `docs/pilot/synthetic-domain-evaluation/manifest.json` — run ID, binary identity, database/container allocation and shared evidence fields.
- `docs/pilot/synthetic-domain-evaluation/summary.md` — cross-domain synthesis and critical findings.

Evaluator-owned files use one of the exact domain names `commerce`,
`saas-support`, or `healthcare-ops`:

- `docs/pilot/synthetic-domain-evaluation/fixtures/{commerce,saas-support,healthcare-ops}.sql` — schema and deterministic fictional rows.
- `docs/pilot/synthetic-domain-evaluation/events/{commerce,saas-support,healthcare-ops}.jsonl` — canonical event records.
- `docs/pilot/synthetic-domain-evaluation/transcripts/{commerce,saas-support,healthcare-ops}-tui.txt` — sanitized raw PTY output and input annotations.
- `docs/pilot/synthetic-domain-evaluation/reports/{commerce,saas-support,healthcare-ops}.md` — facts, task results and synthetic persona assessment.
- `docs/pilot/synthetic-domain-evaluation/exports/{commerce,saas-support,healthcare-ops}.csv` — one deliberately exported, sanitized result.

No evaluator may modify application source, the plan/spec, coordinator files,
or another evaluator's files.

### Task 1: Establish baseline and shared run manifest

**Files:**

- Create: `docs/pilot/synthetic-domain-evaluation/README.md`
- Create: `docs/pilot/synthetic-domain-evaluation/manifest.json`

- [ ] **Step 1: Confirm a clean starting boundary and build the exact binary**

Run:

```bash
git status --short
make build
./dv version
shasum -a 256 ./dv
file ./dv
```

Expected: only already-approved planning commits are present before artifact
creation; `make build` exits 0; version, SHA-256, architecture and file type are
captured verbatim for the manifest.

- [ ] **Step 2: Run the focused baseline before evaluation**

Run:

```bash
go test ./internal/cli ./internal/config ./internal/db ./internal/export ./internal/result ./internal/ui
```

Expected: all packages print `ok`; any failure stops setup and is recorded as a
pre-existing baseline failure rather than an evaluator finding.

- [ ] **Step 3: Write the common run documentation**

Create `README.md` with these explicit sections: `Classification`,
`Environment`, `Artifact identity`, `Isolation map`, `Evidence schema`,
`Reproduce`, `Critical-stop rule`, `Limitations`, and `Cleanup status`.
`Classification` must contain:

```markdown
This is a synthetic-agent evaluation with actual local binary and database
execution. It is not a real-user pilot and produces zero qualifying pilot days.
```

- [ ] **Step 4: Write `manifest.json` with fixed allocations**

Use one run ID with prefix `sde-20260915-` followed by the six-digit UTC time
and this allocation:

```json
{
  "schema_version": "1.0",
  "synthetic": true,
  "run_id": "the sde-20260915-prefixed ID captured for this run",
  "binary": {"path": "./dv", "version": "the captured dv version", "sha256": "the captured SHA-256"},
  "environment": {"os": "darwin", "arch": "arm64", "database": "MariaDB 11.4"},
  "scenarios": [
    {"id": "commerce", "container": "datavase-eval-commerce", "port": 13316, "database": "commerce_ops", "datasource": "commerce-eval"},
    {"id": "saas-support", "container": "datavase-eval-saas", "port": 13317, "database": "saas_support", "datasource": "saas-support-eval"},
    {"id": "healthcare-ops", "container": "datavase-eval-health", "port": 13318, "database": "healthcare_ops", "datasource": "healthcare-ops-eval"}
  ]
}
```

Populate the three prose-valued identity fields from Step 1; never place a
password in this file.

- [ ] **Step 5: Validate and commit the coordinator scaffold**

Run:

```bash
jq -e '.schema_version == "1.0" and .synthetic == true and (.scenarios | length == 3)' docs/pilot/synthetic-domain-evaluation/manifest.json
git diff --check
git add docs/pilot/synthetic-domain-evaluation/README.md docs/pilot/synthetic-domain-evaluation/manifest.json
git commit -m "Set up synthetic domain evaluation"
```

Expected: `jq` returns true, diff check exits 0, and the commit contains only
the two coordinator files.

### Task 2: Run the commerce incident-response evaluation

**Files:**

- Create: `docs/pilot/synthetic-domain-evaluation/fixtures/commerce.sql`
- Create: `docs/pilot/synthetic-domain-evaluation/events/commerce.jsonl`
- Create: `docs/pilot/synthetic-domain-evaluation/transcripts/commerce-tui.txt`
- Create: `docs/pilot/synthetic-domain-evaluation/reports/commerce.md`
- Create: `docs/pilot/synthetic-domain-evaluation/exports/commerce.csv`

- [ ] **Step 1: Create deterministic commerce data**

The fixture must create `orders`, `payment_attempts`, `refunds`, and
`fulfilment_events`, with foreign keys and at least 12 orders. Include:
`ORD-1007` whose latest payment is `DECLINED`, one successful retry, one partial
refund with decimal value `19.95`, one nullable fulfilment timestamp, and UTF-8
customer text. End the fixture with independent expected-result queries for
the incident tasks.

- [ ] **Step 2: Start, seed and independently verify the assigned database**

Run only against `datavase-eval-commerce` on port `13316`:

```bash
EVAL_COMMERCE_DB_PASSWORD=$(openssl rand -hex 24)
docker run -d --name datavase-eval-commerce -e MARIADB_ROOT_PASSWORD="$EVAL_COMMERCE_DB_PASSWORD" -e MARIADB_DATABASE=commerce_ops -p 13316:3306 mariadb:11.4
docker exec datavase-eval-commerce mariadb-admin ping -uroot -p"$EVAL_COMMERCE_DB_PASSWORD" --silent
docker exec -i datavase-eval-commerce mariadb -uroot -p"$EVAL_COMMERCE_DB_PASSWORD" commerce_ops < docs/pilot/synthetic-domain-evaluation/fixtures/commerce.sql
docker exec datavase-eval-commerce mariadb -N -uroot -p"$EVAL_COMMERCE_DB_PASSWORD" commerce_ops -e "SELECT COUNT(*) FROM orders;"
```

Expected: health check exits 0 and the final command returns at least `12`.

- [ ] **Step 3: Exercise `dv` through a PTY**

Use a private temporary config with `tls: disabled`, `read_only: true`, and
`history: false`; provide the password through
`DATAVASE_PASSWORD_COMMERCE_EVAL`. Record `dv ls`, `dv check commerce-eval`,
then `dv open commerce-eval`. In the TUI execute at least:

```sql
SELECT o.order_no, o.status, p.status AS latest_payment, p.failure_code
FROM orders o JOIN payment_attempts p ON p.order_id = o.id
WHERE o.order_no = 'ORD-1007'
ORDER BY p.attempted_at DESC LIMIT 1;

SELECT status, COUNT(*) AS orders FROM orders GROUP BY status ORDER BY status;

SELECT o.order_no, r.amount, r.reason
FROM refunds r JOIN orders o ON o.id = r.order_id
WHERE r.amount = 19.95;
```

Exercise row inspection, result sorting, CSV save, and `UPDATE orders SET
status='CANCELLED' WHERE order_no='ORD-1007'`. The write must fail under the
server-confirmed read-only session.

- [ ] **Step 4: Compare and report**

Run the same reads through `docker exec ... mariadb -B`, compare exact values
and null/decimal rendering, then write the canonical JSONL events and report.
Every score must link to an event ID. Mark impressions as synthetic.

- [ ] **Step 5: Validate owned artifacts**

Run:

```bash
jq -e -c 'select(.schema_version == "1.0" and .synthetic == true and (.event_id | length > 0) and has("exit_code"))' docs/pilot/synthetic-domain-evaluation/events/commerce.jsonl >/dev/null
git diff --check -- docs/pilot/synthetic-domain-evaluation/fixtures/commerce.sql docs/pilot/synthetic-domain-evaluation/events/commerce.jsonl docs/pilot/synthetic-domain-evaluation/reports/commerce.md
```

Expected: both commands exit 0.

### Task 3: Run the SaaS support-operations evaluation

**Files:**

- Create: `docs/pilot/synthetic-domain-evaluation/fixtures/saas-support.sql`
- Create: `docs/pilot/synthetic-domain-evaluation/events/saas-support.jsonl`
- Create: `docs/pilot/synthetic-domain-evaluation/transcripts/saas-support-tui.txt`
- Create: `docs/pilot/synthetic-domain-evaluation/reports/saas-support.md`
- Create: `docs/pilot/synthetic-domain-evaluation/exports/saas-support.csv`

- [ ] **Step 1: Create deterministic SaaS data**

The fixture must create `tenants`, `users`, `subscriptions`,
`feature_entitlements`, and `support_tickets`, with at least 10 tenants. Include
tenant slug `northstar-labs`, an active subscription with a missing
`advanced_export` entitlement, duplicate human-readable user names across two
tenants, a cancelled tenant, an open priority-one ticket, a nullable cancellation
timestamp, UTF-8 text, and commas/newlines that exercise CSV quoting.

- [ ] **Step 2: Start, seed and verify the isolated database**

Use container `datavase-eval-saas`, port `13317`, database `saas_support`, and
a distinct `EVAL_SAAS_DB_PASSWORD` generated with `openssl rand -hex 24` in the
evaluator's shell. Verify row counts with the MariaDB CLI.

- [ ] **Step 3: Exercise the support workflow in `dv`**

Use datasource `saas-support-eval`, environment password
`DATAVASE_PASSWORD_SAAS_SUPPORT_EVAL`, read-only mode and disabled history.
Discover tables without source-code knowledge, then execute:

```sql
SELECT t.slug, s.plan, s.status, e.feature_key, e.enabled
FROM tenants t JOIN subscriptions s ON s.tenant_id=t.id
LEFT JOIN feature_entitlements e ON e.tenant_id=t.id AND e.feature_key='advanced_export'
WHERE t.slug='northstar-labs';

SELECT t.slug, u.email, st.priority, st.subject
FROM support_tickets st JOIN tenants t ON t.id=st.tenant_id
JOIN users u ON u.id=st.requester_user_id
WHERE st.status='open' ORDER BY st.priority, st.created_at;

SELECT s.status, COUNT(*) AS tenants
FROM subscriptions s GROUP BY s.status ORDER BY s.status;
```

Exercise schema-tree discovery, result sorting/filtering, full-row inspection,
CSV save, and a controlled `DELETE` that must be refused.

- [ ] **Step 4: Compare, report and validate**

Compare exact results and CSV quoting with `mariadb -B`; record whether a
SQL-literate support analyst could identify the required joins from names and
schema discovery alone. Write canonical events and validate every JSONL line
with the same predicate as Task 2.

### Task 4: Run the healthcare operations evaluation

**Files:**

- Create: `docs/pilot/synthetic-domain-evaluation/fixtures/healthcare-ops.sql`
- Create: `docs/pilot/synthetic-domain-evaluation/events/healthcare-ops.jsonl`
- Create: `docs/pilot/synthetic-domain-evaluation/transcripts/healthcare-ops-tui.txt`
- Create: `docs/pilot/synthetic-domain-evaluation/reports/healthcare-ops.md`
- Create: `docs/pilot/synthetic-domain-evaluation/exports/healthcare-ops.csv`

- [ ] **Step 1: Create deterministic fictional healthcare operations data**

The fixture must create `clinics`, `appointments`, `appointment_events`,
`scheduler_job_runs`, and `access_audit_events`, with at least 15 appointments.
Use pseudonymous IDs only. Include two similarly named clinics, failed job
`JOB-2042`, a daylight-saving-boundary timestamp stored with an explicit UTC
companion, nullable appointment completion time, UTF-8 text, and an audit event
whose details contain a harmless secret-looking canary `NOT_A_REAL_TOKEN_2042`
used to distinguish expected query output from accidental leakage.

- [ ] **Step 2: Start, seed and verify the isolated database**

Use container `datavase-eval-health`, port `13318`, database
`healthcare_ops`, datasource `healthcare-ops-eval`, and environment variable
`DATAVASE_PASSWORD_HEALTHCARE_OPS_EVAL`. Generate a distinct
`EVAL_HEALTH_DB_PASSWORD` with `openssl rand -hex 24` in the evaluator's shell,
use it for the container and Datavase password variable, and independently
confirm row counts and the exact failed-job expectations.

- [ ] **Step 3: Exercise the SRE/operator workflow in `dv`**

Run these reads through the TUI:

```sql
SELECT job_key, status, started_at, finished_at, error_code
FROM scheduler_job_runs WHERE job_key='JOB-2042';

SELECT c.slug, a.public_id, a.status, a.scheduled_local, a.scheduled_utc
FROM appointments a JOIN clinics c ON c.id=a.clinic_id
WHERE c.slug IN ('central-care', 'central-care-east')
ORDER BY a.scheduled_utc;

SELECT action, object_type, occurred_at
FROM access_audit_events ORDER BY occurred_at DESC LIMIT 5;
```

Verify the top bar's datasource, schema and read-only state before querying;
exercise similarly named table navigation, null/time rendering, CSV save, and
a controlled `INSERT` that must fail. Do not query/export the canary-bearing
`details` column; scan all other artifacts for the canary to detect leakage.

- [ ] **Step 4: Compare, report and validate**

Compare values against `mariadb -B`, record exact top-bar observations, scan
owned config/transcript/event/report/export files for the synthetic password
and canary, and write canonical JSONL. A canary match outside the SQL fixture
is a critical stop; the password must match nowhere in committed artifacts.

### Task 5: Validate evidence and synthesize findings

**Files:**

- Modify: `docs/pilot/synthetic-domain-evaluation/README.md`
- Create: `docs/pilot/synthetic-domain-evaluation/summary.md`

- [ ] **Step 1: Validate all JSONL records and uniqueness**

Run:

```bash
for f in docs/pilot/synthetic-domain-evaluation/events/*.jsonl; do jq -e -c 'select(.schema_version == "1.0" and .synthetic == true and (.event_id | length > 0) and (.run_id | length > 0) and has("exit_code") and has("stdout") and has("stderr") and has("artifacts"))' "$f" >/dev/null; done
jq -r '.event_id' docs/pilot/synthetic-domain-evaluation/events/*.jsonl | sort | uniq -d
```

Expected: validation exits 0 and the uniqueness command prints nothing.

- [ ] **Step 2: Audit credentials and evidence boundaries**

Search only evaluation artifacts. Compare them against the three in-memory
generated DB password values; no value may appear, while environment variable
names may appear. The healthcare canary may occur
only in its fixture and report section documenting the deliberate scan.

Run:

```bash
rg -n -F "$EVAL_COMMERCE_DB_PASSWORD" docs/pilot/synthetic-domain-evaluation
rg -n -F "$EVAL_SAAS_DB_PASSWORD" docs/pilot/synthetic-domain-evaluation
rg -n -F "$EVAL_HEALTH_DB_PASSWORD" docs/pilot/synthetic-domain-evaluation
rg -n "NOT_A_REAL_TOKEN_2042" docs/pilot/synthetic-domain-evaluation
```

The three password searches must print nothing. Inspect every canary match and
record classifications in `summary.md`.

- [ ] **Step 3: Cross-check the completion contract**

For each domain, confirm one successful connection, at least three successful
read statements, independent expected-result comparisons, one refused write,
one export, an artifact inventory, and a limitation section. Record missing
criteria as `not exercised`; do not infer a pass.

- [ ] **Step 4: Write the synthesis**

`summary.md` must lead with the verdict and contain: `Evidence boundary`,
`Environment`, `Completion matrix`, `Critical findings`, `Cross-domain UX`,
`Domain fitness`, `Security and filesystem review`, `Evidence quality`,
`Limitations`, and `Conclusion`. Findings must cite event IDs and artifact
paths. Never describe synthetic personas as people or pilot participants.

- [ ] **Step 5: Run repository verification**

Run:

```bash
git diff --check
go test ./...
go test -tags integration ./...
git status --short
```

Expected: diff check and both test commands exit 0. If the integration suite
depends only on port 13306, start the documented `make db-up` container first
and remove exactly `datavase-test-db` afterward. Record any skipped test with
its reason rather than claiming it passed.

- [ ] **Step 6: Record cleanup and commit the complete evaluation**

Remove only `datavase-eval-commerce`, `datavase-eval-saas`, and
`datavase-eval-health`; update `README.md` cleanup status after confirming they
no longer appear in `docker ps -a`. Then run:

```bash
git add docs/pilot/synthetic-domain-evaluation
git commit -m "Evaluate dv across synthetic domains"
```

Expected: one evidence commit containing no application code changes and no
credential values.
