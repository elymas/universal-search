# Release Manual: v1.0.0 Ceremony

**Audience**: Maintainer (limbowl) + release engineer  
**Scope**: SPEC-REL-001 release ceremony operational procedures  
**Frozen-tag policy**: No force-push; tag is immutable after push  
**KST timezone**: Preferred tag push window 09:00–18:00 KST (UTC+9 = 00:00–09:00 UTC)  

---

## A. Pre-tag Verification Matrix (Manual Checklist)

Before creating the v1.0.0 tag, verify all 12 gates (G1–G12) locally. These gates will run automatically in the `release.yml` workflow on tag push, but it is your responsibility to confirm they are ready before committing to a tag.

### G1: Code Health

```bash
# Run go vet
go vet ./...

# Run tests with race detection
go test -race ./...

# Check coverage (target ≥ 85%)
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
```

**Expected outcome**: vet is clean, tests pass, coverage ≥ 85%.

### G2: Linting + Pre-commit

```bash
# Run golangci-lint (if configured)
golangci-lint run --timeout=10m ./... || echo "golangci-lint not configured (optional)"

# Run pre-commit on all files
pre-commit run --all-files
```

**Expected outcome**: No errors.

### G3: LSP Gate

```bash
# Check LSP status via moai (if available)
moai lsp check . 2>&1 | grep -E "^(zero|[0-9]+ error)" || echo "LSP check not available (CI-only)"
```

**Expected outcome**: zero errors, ≤ 10 warnings.

### G4: Dependency Audit

```bash
# Check latest deps-audit.yml run on main
gh run list --workflow=deps-audit.yml --branch=main --limit=1 --json conclusion --jq '.[0].conclusion'
```

**Expected outcome**: `success`.

### G5: Security Workflow

```bash
# Check latest security.yml run on main (SPEC-SEC-001)
gh run list --workflow=security.yml --branch=main --limit=1 --json conclusion --jq '.[0].conclusion' || echo "security.yml not on main yet (pre-merge)"
```

**Expected outcome**: `success` or "workflow not found yet" (if SEC-001 PR not merged).

### G6: EVAL Trio

```bash
# G6a: EVAL-001 faithfulness score
gh run list --workflow=eval-faithfulness.yml --branch=main --limit=1 --json conclusion --jq '.[0].conclusion' || echo "EVAL-001 not on main yet"

# G6b: EVAL-002 dashboard live (check within 24h)
curl -s https://<eval-002-dashboard-url>/api/health && echo "Dashboard live" || echo "Dashboard not accessible (check EVAL-002 deployment)"

# G6c: EVAL-003 manual sign-off
ls -la .moai/reports/eval-003-korean-benchmark-*.md && echo "Sign-off found" || echo "Sign-off not found"
```

**Expected outcome**: EVAL-001 success, EVAL-002 reachable, EVAL-003 sign-off file exists with maintainer name.

### G7: Helm Chart + Container Images

```bash
# Check chart-ci.yml latest run (SPEC-DEPLOY-001)
gh run list --workflow=chart-ci.yml --branch=main --limit=1 --json conclusion --jq '.[0].conclusion'

# Verify the actual app images exist with cosign signature
gh api repos/elymas/universal-search/releases --jq '.[] | select(.tag_name=="v1.0.0") | .assets[].name' | grep -E "usearch-api|usearch-mcp|usearch-migrate" || echo "Images will be verified post-tag"

# Verify chart appVersion matches tag (after DEPLOY-001 merge)
helm show chart oci://ghcr.io/elymas/charts/universal-search:1.0.0 2>&1 | grep appVersion || echo "Chart will be published post-merge"
```

**Expected outcome**: `chart-ci.yml` success; real images published with cosign signatures (post-merge).

### G8: Documentation Build

```bash
# Check latest docs.yml run (SPEC-DOC-001)
gh run list --workflow=docs.yml --branch=main --limit=1 --json conclusion --jq '.[0].conclusion'
```

**Expected outcome**: `success`.

### G9: Adapter Drift Detection

```bash
# Check adapter-reference-drift.yml or parity test (SPEC-DOC-002)
gh run list --workflow=adapter-reference-drift.yml --branch=main --limit=1 --json conclusion --jq '.[0].conclusion' || \
gh run list --workflow=chart-ci.yml --branch=main --limit=1 --json | jq '.[] | select(.name | contains("parity")) | .conclusion'
```

**Expected outcome**: `success` or "not found yet" (pre-merge).

### G10: 24h CI Green

```bash
# Check main branch CI health last 24h
gh run list --workflow=go.yml --branch=main --created-before=1day --limit=10 --json conclusion | jq 'map(select(.conclusion == "success")) | length >= 8'
```

**Expected outcome**: At least 8 out of last 10 runs are `success` (95%+ success rate).

### G11: GPG-Signed Tag Verification (optional / best-effort)

GPG tag signing is OPTIONAL and non-blocking. Artifact provenance is provided by
cosign keyless (OIDC) signing + SLSA provenance, so the G11 release gate never
fails on an unsigned or CI-unverifiable tag. If a GPG key is available, signing the
tag is still encouraged:

```bash
# Check GPG key is available (optional)
gpg --list-keys limbowl@elymas || echo "GPG key not found; signing is optional"

# Simulate tag creation (don't push yet)
git tag -a v1.0.0 -s -m "Release v1.0.0 — Universal Search" --dry-run || echo "Tag signing skipped; not required"
```

**Expected outcome**: GPG signing is best-effort. An unsigned tag does not block the release.

### G12: Version Consistency

```bash
# Build with ldflags and verify --version output
go build -ldflags \
  "-X github.com/elymas/universal-search/internal/version.Version=1.0.0 \
   -X github.com/elymas/universal-search/internal/version.Commit=$(git rev-parse --short HEAD) \
   -X github.com/elymas/universal-search/internal/version.BuildDate=$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
  ./cmd/usearch

./usearch --version | grep -q "usearch v1.0.0" && echo "Version matches tag" || echo "Version mismatch!"
```

**Expected outcome**: `Version matches tag`.

---

## B. Annotated GPG-Signed Tag Creation

Once all 12 gates are PASS (manually verified), create the tag:

```bash
# Create annotated, GPG-signed tag
git tag -a -s v1.0.0 \
  -m "Release v1.0.0 — Universal Search

[Paste first 30 lines of CHANGELOG.md [1.0.0] section here]

References:
- SPEC-REL-001: Release infrastructure
- SPEC-DOC-001: User guide site
- SPEC-DOC-002: Adapter reference
- SPEC-DEPLOY-001: Helm chart & container images
- SPEC-SEC-001: Security hardening
- SPEC-EVAL-001: Citation faithfulness
- SPEC-EVAL-002: Adapter dashboard
- SPEC-EVAL-003: Korean benchmark

Signed-off-by: limbowl <limbowl@elymas>" \

# Verify the tag is signed
git tag -v v1.0.0

# Push the tag to origin (this triggers release.yml)
git push origin v1.0.0
```

**Important**:
1. Replace `[Paste CHANGELOG...]` with the actual CHANGELOG.md `[1.0.0]` section first 30 lines.
2. The `-s` flag signs the tag with GPG. Ensure your GPG key is configured (`git config user.signingkey <KEY_ID>`).
3. Once pushed, the tag is immutable. Do NOT use `git push --force`.

---

## C. Emergency Rollback Procedure

If the release fails post-tag (e.g., goreleaser failed, GitHub Release creation failed), follow this procedure:

### Step C1: Delete the Git Tag (Local + Remote)

```bash
# Delete locally
git tag -d v1.0.0

# Delete on remote (requires push permission)
git push --delete origin v1.0.0
```

### Step C2: Retract GitHub Release (if published)

```bash
# Delete the GitHub Release
gh release delete v1.0.0 --yes
```

### Step C3: Unpublish Container Images + Chart (if published)

```bash
# Remove images from ghcr.io (requires container registry credentials)
# Example using docker/gh CLI:
gh api repos/elymas/universal-search/packages -X DELETE || echo "Images will be garbage-collected"

# Remove chart OCI artifact
helm chart pull oci://ghcr.io/elymas/charts/universal-search:1.0.0 || echo "Chart will be unpublished manually"
```

### Step C4: Create a Corrective Release

Once the issue is fixed:

1. **Increment the tag**: Use `v1.0.1` instead (not `v1.0.0` again).
2. **Document the incident**: Add an entry to `ops/release-incidents.md`:
   ```
   ## v1.0.0 Rollback — [ISO-8601 timestamp]
   
   **Issue**: [Description of failure]
   **Resolution**: [What was fixed]
   **New Tag**: v1.0.1
   **Reference PR**: [Link to fix PR]
   ```
3. Repeat the tag creation + push with the new version.

---

## D. Post-Release Tasks Checklist

After the GitHub Release is published (release.yml workflow completes successfully):

- [ ] Verify the GitHub Release page shows all artifacts (12 archives, SBOM, cosign signatures, provenance)
- [ ] Review and merge the auto-generated `.moai/project/roadmap.md` PR (marks M9 as shipped)
- [ ] Update SECURITY.md vulnerability reporting contact (per SPEC-SEC-001)
- [ ] Draft a release announcement (English + Korean summary) for GitHub Discussions / community channel (optional post-V1 channel)
- [ ] Run post-mortem if any release-day incident occurred (check obs dashboard + audit logs per SPEC-AUTH-003)
- [ ] Confirm downstream integrations (e.g., Skill Marketplace, partner systems) are aware of v1.0.0 availability

---

## E. Locale + Timing Protocol

### Tag Push Window

The tag MUST be pushed during **KST business hours** (09:00–18:00 KST, UTC+9 = 00:00–09:00 UTC) to ensure the maintainer (limbowl, KST timezone) can respond to any release-day incident within minutes.

**Never push the tag outside business hours without on-call confirmation.**

### Timezone Considerations

- **KST**: 09:00–18:00 (preferred push window)
- **UTC**: 00:00–09:00 (same as above, UTC equivalent)
- **US Pacific**: 17:00 previous day–01:00 same day
- **Europe CET**: 02:00–11:00

**Recommendation**: Push the tag at **09:30 KST** (early morning for full business day response).

### Release Notification

After successful publish, send a notification to:
- Internal team (if Slack webhook configured): Automated via release.yml `post-release-tasks`
- Public community (GitHub Discussions): Manual message with installation instructions + verification guide

---

## Appendix: Verification Commands for Operators

After downloading a binary from the v1.0.0 release, operators can verify authenticity:

### Verify Cosign Signature (Keyless OIDC)

```bash
# Download the binary, .sig, and .crt from the GitHub Release
curl -L https://github.com/elymas/universal-search/releases/download/v1.0.0/usearch_1.0.0_linux_amd64.tar.gz -O
curl -L https://github.com/elymas/universal-search/releases/download/v1.0.0/usearch_1.0.0_linux_amd64.tar.gz.sig -O
curl -L https://github.com/elymas/universal-search/releases/download/v1.0.0/usearch_1.0.0_linux_amd64.tar.gz.crt -O

# Verify the signature
cosign verify-blob \
  --certificate usearch_1.0.0_linux_amd64.tar.gz.crt \
  --signature usearch_1.0.0_linux_amd64.tar.gz.sig \
  --certificate-identity-regexp "https://github.com/elymas/universal-search/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  usearch_1.0.0_linux_amd64.tar.gz
```

**Expected output**: `Verified OK` (exit code 0).

### Verify SLSA Provenance

```bash
# Download the provenance file
curl -L https://github.com/elymas/universal-search/releases/download/v1.0.0/multiple.intoto.jsonl -O

# Verify with slsa-verifier
slsa-verifier verify-artifact \
  --provenance-path multiple.intoto.jsonl \
  --source-uri github.com/elymas/universal-search \
  --source-tag v1.0.0 \
  usearch_1.0.0_linux_amd64.tar.gz
```

**Expected output**: `SLSA verification succeeded. Builder ID: …` (exit code 0).

### Verify Git Tag

```bash
# Clone the repo and verify the tag signature
git clone https://github.com/elymas/universal-search.git
cd universal-search
git tag -v v1.0.0
```

**Expected output**: If the tag is GPG-signed, the signature is verified with limbowl's key (exit code 0). GPG tag signing is optional — artifact provenance is provided by cosign keyless signing + SLSA provenance.

---

**Last updated**: 2026-05-31  
**SPEC**: SPEC-REL-001 REQ-REL-005
