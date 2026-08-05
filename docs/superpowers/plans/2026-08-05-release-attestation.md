# Release Attestation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every artifact a `v*` tag publishes carries a GitHub build provenance attestation, and both the README and the release page say how to check it.

**Architecture:** One `actions/attest-build-provenance@v4` step runs after goreleaser in the existing release job, taking `dist/checksums.txt` as `subject-checksums` so all twelve artifacts are covered by a single attestation call. The job gains the two permissions that step needs. No `.goreleaser.yaml` build changes — only its release footer copy.

**Tech Stack:** GitHub Actions, `actions/attest-build-provenance@v4`, `gh attestation verify`, goreleaser v2 (config only).

## Global Constraints

- Repository: `Ahngbeom/datavase`. Default branch `main`. Work on a branch, open a PR; never push to `main`.
- The release job is `jobs.goreleaser` in `.github/workflows/release.yml`. It already declares `environment: HOMEBREW_TAP`. Do not remove or reorder that.
- Action pin: `actions/attest-build-provenance@v4` (current major; latest release is `v4.1.1`).
- The attest step MUST come after the `goreleaser/goreleaser-action@v6` step. `dist/checksums.txt` does not exist until goreleaser has run.
- Do NOT attest `dist/digests.txt`. goreleaser writes it for container images; this project builds none, so it is empty.
- Attest-step failure MUST fail the workflow. Do not add `continue-on-error`. A release whose attestation silently vanished is one nobody finds out about.
- Comments in this repository say *why*, never *what*. Match the surrounding style — see the existing comments in `release.yml`.
- Verification command used verbatim in all user-facing copy:
  `gh attestation verify <file> --repo Ahngbeom/datavase`
- `make lint` (`go vet ./...` + `gofmt -l .`) and `shellcheck install.sh` must stay clean. No Go or shell files are touched by this plan.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `.github/workflows/release.yml` | Runs the release on a `v*` tag | Add 2 permissions + 1 step |
| `README.md` | The user-facing account of installing | Add a verification block to `## Install` |
| `.goreleaser.yaml` | Release config; its `release.footer` is the release page copy | Add the same command to the footer |
| `CHANGELOG.md` | The edited account per release | Add a `v0.6.2` section |

No source files change. No tests exist for workflow YAML, so verification is by config parse, by CI, and by a real tag — stated explicitly in Task 5.

---

### Task 1: Grant the release job the permissions attestation needs

**Files:**
- Modify: `.github/workflows/release.yml` (the top-level `permissions:` block, currently lines 7-8)

**Interfaces:**
- Consumes: nothing.
- Produces: the `id-token: write` and `attestations: write` permissions that Task 2's step requires. Without both, that step fails with a permissions error at tag time.

- [ ] **Step 1: Read the current permissions block**

Run: `sed -n '1,12p' .github/workflows/release.yml`

Expected to contain exactly:

```yaml
permissions:
  contents: write # goreleaser creates the GitHub release and uploads archives
```

- [ ] **Step 2: Replace that block**

Replace the two lines above with:

```yaml
permissions:
  contents: write # goreleaser creates the GitHub release and uploads archives
  id-token: write # the attestation is signed against this run's OIDC identity
  attestations: write # and stored on the repository
```

- [ ] **Step 3: Verify the YAML still parses and the keys landed**

Run:

```bash
python3 -c "
import yaml
d = yaml.safe_load(open('.github/workflows/release.yml'))
print(d['permissions'])
assert d['permissions']['id-token'] == 'write'
assert d['permissions']['attestations'] == 'write'
assert d['permissions']['contents'] == 'write'
print('OK')
"
```

Expected: prints the three permissions then `OK`.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "Let the release job prove who built the artifacts

actions/attest-build-provenance signs against the run's OIDC identity and
writes the result to the repository, and can do neither without being told
to. Both permissions are useless on their own, so they arrive together."
```

---

### Task 2: Attest every artifact after goreleaser publishes

**Files:**
- Modify: `.github/workflows/release.yml` (append a step to `jobs.goreleaser.steps`, after the existing `goreleaser/goreleaser-action@v6` step, which is the last one in the file)

**Interfaces:**
- Consumes: the permissions from Task 1; `dist/checksums.txt`, written by the goreleaser step.
- Produces: a build provenance attestation covering all twelve files listed in `checksums.txt`. Verified by `gh attestation verify <file> --repo Ahngbeom/datavase`, which Tasks 3 and 4 document.

- [ ] **Step 1: Confirm the goreleaser step is last**

Run: `tail -12 .github/workflows/release.yml`

Expected: the file ends with the `env:` block of the goreleaser step, whose last line is
`HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}`.

- [ ] **Step 2: Append the attest step**

Append to the end of `.github/workflows/release.yml`:

```yaml

      # After goreleaser, because dist/checksums.txt is written by it and the
      # release is already published by the time it exists — attesting is
      # necessarily the last thing that happens.
      #
      # subject-checksums takes the whole file, so one call covers all twelve
      # artifacts rather than needing a subject per archive and package.
      #
      # This is deliberately allowed to fail the run. The cask is skipped
      # quietly when its token is missing because the release is still whole
      # without it; a missing attestation is not like that. Nobody goes looking
      # for a signature that was never there, so the run has to be the thing
      # that says so.
      - name: Attest what this run built
        uses: actions/attest-build-provenance@v4
        with:
          subject-checksums: ./dist/checksums.txt
```

Note: `dist/digests.txt` is deliberately not attested — goreleaser writes it for container images and this project builds none, so it is empty.

- [ ] **Step 3: Verify the step parses and is positioned last**

Run:

```bash
python3 -c "
import yaml
d = yaml.safe_load(open('.github/workflows/release.yml'))
steps = d['jobs']['goreleaser']['steps']
last = steps[-1]
print('step count:', len(steps))
print('last uses:', last['uses'])
assert last['uses'] == 'actions/attest-build-provenance@v4'
assert last['with']['subject-checksums'] == './dist/checksums.txt'
gr = [i for i, s in enumerate(steps) if 'goreleaser-action' in s.get('uses', '')]
assert gr and gr[0] < len(steps) - 1, 'attest must come after goreleaser'
print('OK')
"
```

Expected: `step count: 5`, the action name, then `OK`.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "Attest what this run built

checksums.txt already lists every artifact, so one subject-checksums call
covers all twelve rather than a subject per archive and package.

It runs last because it has to: goreleaser writes dist/checksums.txt as part
of publishing, so there is nothing to attest until the release is already
out. If this step fails the run fails with it — a release whose signature was
never written is not something anyone goes looking for."
```

---

### Task 3: Say how to check it, in the README

**Files:**
- Modify: `README.md` (the `## Install` section; insert after the line `Check it with `dv version`, then run `dv init`.` and before `Nothing else is needed, and nothing else is wanted:`)

**Interfaces:**
- Consumes: the attestation produced by Task 2.
- Produces: the canonical wording, reused verbatim by Task 4's release footer.

- [ ] **Step 1: Find the anchor**

Run: `grep -n "Check it with \`dv version\`" README.md`

Expected: one match, in the `## Install` section.

- [ ] **Step 2: Insert the verification block**

Find this exact text in `README.md`:

```markdown
Check it with `dv version`, then run `dv init`.

Nothing else is needed, and nothing else is wanted: no runtime, no drivers,
no companion tools. It is one binary.
```

Replace it with:

```markdown
Check it with `dv version`, then run `dv init`.

Nothing else is needed, and nothing else is wanted: no runtime, no drivers,
no companion tools. It is one binary.

### Checking where a download came from

Every released file is signed by the workflow that built it. To check one:

```sh
gh attestation verify dv_v0.6.2_darwin_arm64.tar.gz --repo Ahngbeom/datavase
```

That answers a different question from the checksum the install script
already verifies. A checksum says the file arrived intact; this says it was
built by this repository's release workflow and not swapped for something
else — which the checksum cannot tell you, because it is downloaded from the
same place as the file it describes.
```

- [ ] **Step 3: Verify the block landed once**

Run:

```bash
grep -c "gh attestation verify" README.md
grep -n "### Checking where a download came from" README.md
```

Expected: `1`, and one line number inside the Install section (below line 45, above the `## Set up` heading — confirm with `grep -n "^## Set up" README.md`).

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "Say how to tell where a download came from

The install script verifies the checksum, which only says the file arrived
intact — and checksums.txt comes from the same release as the file it
describes, so it cannot answer the question anyone worried about a swap is
actually asking."
```

---

### Task 4: Put the same command on the release page

**Files:**
- Modify: `.goreleaser.yaml` (the `release.footer` block, at the end of the file)

**Interfaces:**
- Consumes: Task 3's wording.
- Produces: release-page copy. `{{ .Tag }}` is already used elsewhere in this footer, so it is known to resolve.

- [ ] **Step 1: Read the end of the footer**

Run: `tail -10 .goreleaser.yaml`

Expected to end with:

```yaml
    ```sh
    xattr -dr com.apple.quarantine ./dv
    ```
```

- [ ] **Step 2: Append the verification block to the footer**

Append to the end of `.goreleaser.yaml` (keeping the footer's four-space indentation):

```yaml

    Every file above is signed by the workflow that built it. To check one:

    ```sh
    gh attestation verify dv_{{ .Tag }}_darwin_arm64.tar.gz --repo Ahngbeom/datavase
    ```
```

- [ ] **Step 3: Verify the config still validates**

Run: `go run github.com/goreleaser/goreleaser/v2@latest check`

Expected: `1 configuration file(s) validated` and no deprecation warnings.

- [ ] **Step 4: Verify the footer renders with the tag substituted**

Run: `go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean 2>&1 | tail -3 && grep -A3 "attestation verify" dist/CHANGELOG.md 2>/dev/null || echo "(snapshot does not render the footer; checked by check above)"`

Expected: the snapshot succeeds. The footer is only rendered at publish time, so this step confirms nothing broke rather than confirming the copy — Task 5 is where the rendered page is read.

- [ ] **Step 5: Clean up and commit**

```bash
rm -rf dist
git add .goreleaser.yaml
git commit -m "Put the verification command where the download is

The release page is where someone lands with the archive in front of them,
which is the moment the command is worth knowing."
```

---

### Task 5: Release it, and prove the check actually bites

**Files:**
- Modify: `CHANGELOG.md` (insert a `## v0.6.2 — <today>` section above `## v0.6.1 — 2026-08-05`)

**Interfaces:**
- Consumes: Tasks 1-4.
- Produces: a published `v0.6.2` with attestation, and evidence that verification rejects a tampered file.

**Why this task exists:** neither `goreleaser check` nor the CI snapshot reaches the attest step — the snapshot never publishes, so `dist/checksums.txt` is never attested. This is the same blind spot that let a bad Homebrew token template through to `v0.6.0`. The only real test is a tag.

- [ ] **Step 1: Add the changelog section**

Find `## v0.6.1 — 2026-08-05` in `CHANGELOG.md` and insert above it:

```markdown
## v0.6.2 — 2026-08-05

**Nothing to do before upgrading.** The binaries are identical in behaviour to
`v0.6.1`; this adds something alongside them.

### Added

**Every released file is signed by the workflow that built it.** To check one:

```sh
gh attestation verify dv_v0.6.2_darwin_arm64.tar.gz --repo Ahngbeom/datavase
```

The install script already verifies each download against `checksums.txt`,
which says the file arrived intact. It cannot say more than that, because it
is fetched from the same release as the file it describes. This answers the
other question: that the file was built here.

```

(Replace the date with today's if it is no longer 2026-08-05.)

- [ ] **Step 2: Commit and open the PR**

```bash
git add CHANGELOG.md
git commit -m "Close the changelog for v0.6.2"
git push -u origin HEAD
gh pr create -R Ahngbeom/datavase --base main \
  --title "Attest what each release builds" \
  --body "Adds a build provenance attestation covering all twelve release artifacts, and documents \`gh attestation verify\` in the README and on the release page.

Neither \`goreleaser check\` nor the CI snapshot can reach the attest step — the snapshot never publishes, so there is no \`dist/checksums.txt\` to attest. The real test is the tag; see the verification notes below.

Unlike the Homebrew cask, this step is allowed to fail the run. A release missing its cask is still whole; a release missing its signature is one nobody goes looking for."
```

- [ ] **Step 3: Wait for CI, then merge**

Run: `gh pr checks <PR#> -R Ahngbeom/datavase`

Expected: `check`, `integration` and `release-config` all pass. Merging requires the human — the merge command is blocked for agents in this environment. Ask for it.

- [ ] **Step 4: Tag**

```bash
git fetch origin
git tag v0.6.2 origin/main
git push origin v0.6.2
```

- [ ] **Step 5: Confirm the attest step ran**

```bash
gh run list -R Ahngbeom/datavase --workflow=release.yml --limit 1
gh run view <run-id> -R Ahngbeom/datavase --log | grep -i "attest" | head -5
```

Expected: the run concludes `success` and the log shows the attest step.

- [ ] **Step 6: Verify a real artifact**

```bash
cd "$(mktemp -d)"
gh release download v0.6.2 -R Ahngbeom/datavase -p 'dv_v0.6.2_darwin_arm64.tar.gz'
gh attestation verify dv_v0.6.2_darwin_arm64.tar.gz --repo Ahngbeom/datavase
```

Expected: `Loaded digest sha256:... ` then a line confirming the attestation was verified against `Ahngbeom/datavase`.

- [ ] **Step 7: Prove the check bites**

A verification that only ever passes proves nothing. Tamper with the file and confirm it is rejected:

```bash
cp dv_v0.6.2_darwin_arm64.tar.gz tampered.tar.gz
printf 'x' >> tampered.tar.gz
gh attestation verify tampered.tar.gz --repo Ahngbeom/datavase; echo "exit=$?"
```

Expected: a non-zero exit and a failure message — the appended byte changes the digest, so no attestation matches it. If this passes, the verification is not checking what it appears to.

- [ ] **Step 8: Confirm the release page copy rendered**

```bash
gh release view v0.6.2 -R Ahngbeom/datavase --json body -q .body | grep -A3 "attestation verify"
```

Expected: the command with `v0.6.2` substituted for `{{ .Tag }}` — not the literal template.

---

## Self-Review

**Spec coverage:**
- Attestation on release → Tasks 1, 2.
- README verification docs → Task 3.
- Release footer copy → Task 4.
- `digests.txt` excluded → stated in Global Constraints and Task 2 Step 2.
- Attest failure fails the run → Global Constraints; no `continue-on-error` anywhere.
- Post-goreleaser ordering → Task 2 Steps 1 and 3 both assert it.
- "CI cannot catch this" limitation → Task 5 preamble.
- Verification must be shown to reject tampering → Task 5 Step 7.
- Version `v0.6.2` → Task 5.

**Placeholder scan:** No TBD/TODO. Every step has the literal text or command. The two `<PR#>`/`<run-id>` placeholders are values produced by the immediately preceding step, not unresolved decisions.

**Type consistency:** `actions/attest-build-provenance@v4` and `subject-checksums: ./dist/checksums.txt` are spelled identically in Task 2 Step 2 and its assertion in Step 3. The verify command is byte-identical in Global Constraints, Task 3, Task 4 and Task 5.

**Known gap:** Task 4 Step 4 cannot confirm the footer copy — goreleaser renders the footer only when publishing. Task 5 Step 8 closes it against the real release.
