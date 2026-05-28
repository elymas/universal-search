# SPEC-SEC-001 Progress

## Implementation Status

### Phase 4 — SSRF Mitigation Generalization (COMPLETE)

- [x] `internal/security/ssrf/` package created
  - ssrf.go: ValidateScheme, ValidateHost, ValidateRedirect, Options struct
  - Dialer: PinnedIPDialer, DialContextWithPinnedIP
  - Hostname blocklist (GCP, Azure, EC2 metadata endpoints)
  - Case-insensitive + suffix matching
- [x] Coverage: 85.4% (target 85%)
- [x] Tests: 30 tests, all passing with -race
- [x] @MX:WARN tags on security-critical paths

### Phase 6 — Secrets Resolver Multi-Backend (COMPLETE)

- [x] `internal/security/secrets/` package created
  - Resolver interface with Get(ctx, key)
  - EnvResolver (os.Getenv)
  - K8sResolver (mounted file with path traversal prevention)
  - VaultResolver stub (ErrNotImplemented)
  - NewResolver factory function
- [x] Coverage: 90.9%
- [x] Tests: 8 tests, all passing with -race
- [x] Path traversal prevention in K8sResolver

### Phase 5 — Security Events + Merkle Chain (COMPLETE)

- [x] `internal/security/events/` package created
  - 7-type event logger (auth.failed, auth.success, ssrf.blocked, secret.scan.finding, ratelimit.exceeded, rbac.denied, prompt.sanitized)
  - Merkle hash chain (SHA-256 prev_hash linkage)
  - VerifyChain integrity verification
  - MetricsRecorder interface for metric emission
  - slog emission at appropriate severity levels
- [x] Coverage: 87.0%
- [x] Tests: 6 tests, all passing with -race

### Phase 9 — Rate Limit + Abuse Detection (COMPLETE)

- [x] `internal/security/ratelimit/` package created
  - Per-tenant token bucket using golang.org/x/time/rate
  - Default: 60 queries/min, burst 10
  - HTTP 429 response with Retry-After header
  - Chi-compatible middleware
  - Tenant classification (known/unknown) for cardinality
- [x] Coverage: 96.4%
- [x] Tests: 8 tests, all passing with -race
- [x] golang.org/x/time/rate dependency added

### Phase 10 — LLM Prompt-Injection Sanitization (COMPLETE)

- [x] `internal/security/prompt/` package created
  - Sanitize function with EVIDENCE block wrapping
  - 10 heuristic injection patterns (5 classes)
  - [SANITIZED:class] replacement markers
  - System prompt for LLM instruction separation
  - IsInjectionDetected quick check
- [x] Coverage: 100.0%
- [x] Tests: 11 tests + 9 table-driven subtests, all passing with -race

## Regression Check

- [x] `internal/access/` tests: all passing (no regression from SSRF extraction)

## Remaining Work (CI/Config/Docs)

- [ ] `.github/workflows/security.yml` (gitleaks + gosec + semgrep + Trivy)
- [ ] `.gitleaks.toml` (baseline allowlist)
- [ ] `.gosec.yml` (gosec configuration)
- [ ] `.semgrepignore` (exclusion patterns)
- [ ] `.moai/config/sections/security.yaml` (security config)
- [ ] `internal/obs/metrics/security.go` (security metric collectors)
- [ ] `ops/security/runbook.md` (incident response)
- [ ] `ops/security/owasp-asvs-checklist.md` (ASVS L1)
- [ ] `ops/security/threat-model.md` (STRIDE model)
- [ ] `SECURITY.md` (responsible disclosure)
- [ ] CACHE-001 refactor to use internal/security/ssrf/
- [ ] AUTH-001 refactor to use internal/security/ssrf/
