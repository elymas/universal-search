# usearch Security Runbook

Operator-facing incident response procedures for universal-search (usearch),
per SPEC-SEC-001. Covers committed-secret rotation, SSRF block triage, CRITICAL
CVE response, cosign signature verification, and audit-chain-break recovery.

- **Source SPEC**: SPEC-SEC-001
- **Audience**: repository owners, on-call operators, security reviewers
- **Severity classification**: CRITICAL (CVSS >= 9.0), HIGH (7.0-8.9),
  MEDIUM (4.0-6.9), LOW (< 4.0)

---

## 1. Committed-secret rotation (REQ-SEC-005)

Triggered when gitleaks (pre-commit or CI) or a manual audit finds a real
secret committed to the repository or its history.

**Classification: CRITICAL regardless of credential type.**

### 1.1 Four-step response (REQ-SEC-005)

1. **Revoke at the provider immediately.** Invalidate the leaked credential at
   its issuer (LiteLLM master key rotation, Naver app secret regen, OIDC client
   secret rotation, cloud IAM key deletion). Do this FIRST — purging git history
   does not un-leak an already-exposed secret.
2. **Record the incident in the AUTH-003 audit log** with event type
   `secret.scan.finding`, severity `critical` (emitted via
   `internal/security/events`).
3. **Issue a replacement credential** and roll it out via the configured
   secrets backend (`.moai/config/sections/security.yaml` `secrets.backend`:
   env → update `.env.local`; k8s → update the mounted Secret; vault → post-V1).
4. **Complete a post-mortem within 24 hours** documenting root cause, blast
   radius, and the prevention follow-up.

### 1.2 Git-history rewrite — GUARDED destructive operation (REQ-SEC-005a)

If the secret must be purged from git history (`git filter-repo` + force-push
to a shared branch such as `main`), the rewrite is BLOCKED unless **ALL FIVE**
guards below are satisfied. **Absent any single guard, do NOT proceed.**

1. **Human approval gate** — a named approver (security lead or repo owner per
   `.github/CODEOWNERS`) authorizes the rewrite **in writing**.
2. **Backup before rewrite** — create AND verify both:
   - a snapshot ref: `git update-ref refs/backup/$(date +%F) HEAD`
   - a full mirror clone: `git clone --mirror <repo> usearch-mirror-$(date +%F).git`
3. **Staging validation** — run the rewrite first on a throwaway clone / staging
   branch and verify the result (secret absent, history otherwise intact)
   BEFORE touching `main`.
4. **Rollback procedure (tested)** — restore `main` from the backup ref/mirror:
   `git update-ref refs/heads/main refs/backup/<date> && git push --force-with-lease origin main`
   (or re-push from the mirror). Test this on staging.
5. **Team coordination notice** — notify ALL collaborators before the
   force-push. Every commit SHA changes; downstream clones MUST re-clone or
   `git reset --hard origin/main`.

> The agent/automation NEVER performs this rewrite. It is an explicitly
> human-gated operation.

---

## 2. SSRF block triage

When `usearch_security_ssrf_blocks_total` increments or an `ssrf.blocked`
audit event appears.

1. **Identify the reason label** — one of `scheme`, `private_ip`,
   `redirect_hop`, `dns_rebind`, `hostname_allowlist`.
2. **Identify the component** — `access` (adapter/cache fetch), `auth` (OIDC
   discovery), or `adapter`.
3. **Assess intent**:
   - `hostname_allowlist` / `private_ip` to a cloud metadata endpoint (the
     link-local 169.254.x.x range, `metadata.google.internal`) → likely an SSRF
     probe. Investigate the originating tenant/query.
   - `scheme` (file://, gopher://) → injection attempt; block is working.
   - `redirect_hop` exhaustion → redirect loop; usually benign misconfiguration.
4. **No action needed if the block held** — the guard prevented the request.
   Record the source for abuse-pattern correlation.
5. **False positive?** A legitimate internal host wrongly blocked → the guard
   default is deny-by-default. Do NOT set `allow_private_networks: true` in
   production; instead scope the specific need with the security owner.

---

## 3. CRITICAL CVE response

When `security.yml` (Trivy) or `deps-audit.yml` (govulncheck/pip-audit/pnpm)
reports a CRITICAL (CVSS >= 9.0) or HIGH (7.0-8.9) dependency vulnerability.

1. **CRITICAL** → CI fails + alert. Target MTTR <= 7 calendar days (NFR-SEC-002).
2. **HIGH** → CI fails the PR. Target MTTR <= 30 days.
3. **Remediate**: bump the affected dependency to a fixed version; re-run the
   scanner to confirm.
4. **If no upstream fix (UNFIXED)**: add an entry to
   `ops/security/vuln-exceptions.yaml` with `cve_id`, `dependency`, `severity`,
   `rationale`, `discovered_at`, `review_deadline` (<= discovered_at + 90 days),
   and `owner`. CI (`scripts/check-vuln-exceptions.sh`) fails once the deadline
   passes without renewal.
5. **MEDIUM/LOW** → informational; track but do not block.

---

## 4. cosign signature verification (REQ-SEC-016)

Release artifacts and images are signed keyless via GitHub Actions OIDC
(`.github/workflows/release.yml`, `sign` job). Verify before deploying.

### 4.1 Verify a container image

```bash
cosign verify \
  --certificate-identity-regexp \
    "https://github.com/elymas/universal-search/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  <image>:<tag>
```

### 4.2 Verify a signed blob (binary)

```bash
cosign verify-blob \
  --certificate usearch.pem \
  --signature usearch.sig \
  --certificate-identity-regexp \
    "https://github.com/elymas/universal-search/.github/workflows/release.yml@.*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  usearch
```

A non-zero exit means the artifact is unsigned, tampered, or signed by an
unexpected identity — **do not deploy it.**

> CROSS-SPEC (SPEC-REL-001): REL-001 owns the release build (goreleaser, image
> push). The `release.yml` SLSA + cosign jobs are the SEC-001 security layer;
> REL-001 feeds artifact digests into them rather than re-implementing signing.

---

## 5. Audit-chain-break recovery (REQ-SEC-017 / NFR-SEC-004)

The audit hash chain is the EXISTING AUTH-003 chain (`internal/audit`:
`ComputeThisHash`, `VerifyChain`, per-tenant `AcquireAdvisoryLock`), verified by
the daily `audit.chain_verify` job. SEC-001 emits its 7-type taxonomy into this
chain; it adds no new chain or verify job.

When `usearch_audit_chain_violations_total` increments (chain break detected):

1. **Alert-first, lock-later.** V1 default is `audit.hash_chain.fail_closed:
false` — a break raises a CRITICAL alert but does NOT lock the audit
   subsystem (prevents lockout from a botched migration or transient race).
2. **Investigate the break point** — run `VerifyChain` for the affected
   `(tenant_id, event_type)` to find the first non-matching `this_hash`.
3. **Distinguish causes**:
   - Botched migration / backfill → re-run the backfill; the chain should
     re-verify.
   - Genuine tampering → treat as a CRITICAL security incident; preserve the
     current DB state for forensics, notify the security owner.
4. **Enabling fail-closed lockdown** (staged, opt-in) requires: (a) AUTH-003
   owner sign-off, (b) a successful post-backfill chain verify on the target
   environment BEFORE enabling, and (c) this documented operator unlock
   procedure. Only then set `audit.hash_chain.fail_closed: true`.
5. **Unlock procedure** (if fail-closed is enabled and locks): correct the
   chain, run a successful `VerifyChain`, then clear the lock flag per the
   AUTH-003 owner's procedure.

---

## 6. Sample tokens (allowlisted, NOT real secrets)

The `.gitleaks.toml` allowlist covers documentation placeholders and test
fixtures (the canonical AWS example access-key ID from AWS docs, OIDC stub keys
in `internal/auth/testdata/oidc_stub/`, and `*_test.go` embedded credentials).
These are inert placeholders, never live credentials. New allowlist entries
require CODEOWNERS approval.
