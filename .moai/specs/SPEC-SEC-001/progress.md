# SPEC-SEC-001 Progress Log

## 2026-05-29 — Phase 1 (Analysis & Planning) — manager-strategy

Run-phase Phase 1 analysis completed. No code written (analysis-only).

### Dependency verification
All 6 depends_on SPECs confirmed `status: implemented` (CACHE-001,
AUTH-001, AUTH-002, AUTH-003, BOOT-001, OBS-001). DEP-001 + SYN-002 also
implemented. All referenced code assets exist:
- internal/access/ssrf.go (3.7k), dialer.go (2.8k) — SSRF guards present
- internal/auth/private_ip.go (1.5k), discovery.go (3.5k) — present
- .github/workflows/deps-audit.yml (11k) — present
- internal/obs/metrics/metrics.go (14k) — present, cardinality allowlist
- internal/synthesis/faithfulness.go + citation/ — SYN-002 integration point

### Findings flagged for run phase
- PATH DRIFT: spec.md references `internal/cache/access/` — actual path is
  `internal/access/`. No internal/cache/ dir exists. Documentation drift,
  not scope change.
- API RESHAPE: access SSRF guards are unexported (validateScheme etc.);
  REQ-SEC-007 wants exported API with changed signature. Extraction is
  reshape+extract, not pure move.
- TEST COUNT: SPEC claims "9 REQ-CACHE-013 tests"; actual is 22 SSRF-related
  test funcs across ssrf_test.go/ssrf_redirect_test.go/dialer_test.go.
  Favorable for PRESERVE.
- NEW DEP: golang.org/x/time not yet in go.mod (expected for T08 ratelimit).
- prev_hash column absent from auth audit schema (expected — T05 migration).

### Phase 0 status
No prior plan-auditor report for SEC-001 in .moai/reports/plan-audit/.
Status is still `draft`. Phase 0 plan-auditor PASS is REQUIRED before
implementation (plan.md Phase 0 + thorough harness).

### Artifacts produced
- tasks.md: 10 atomic tasks (T01..T10), 5 critical-path + 5 composite.

### Acceptance criteria baseline
0 / 15 AC met (analysis phase). Error count delta: n/a.

## 2026-05-29 — Run Phase T01–T05 (critical path) — manager-ddd (DDD)

DDD cycle ANALYZE -> PRESERVE -> IMPROVE executed for the five critical-path
tasks. All affected test suites GREEN with `-race`; full `go test ./...` clean.

### T01 — ANALYZE + PRESERVE (completed)
- Read + mapped the full security surface (access SSRF, auth private-IP, audit
  chain/types/store, obs metrics cardinality allowlist, deps-audit.yml,
  pre-commit). Verified the 22 CACHE-001 SSRF tests pass on unchanged code.
- Wrote characterization baselines: `internal/access/ssrf_baseline_test.go`
  (8 tests, pins FetchError.Category + dual AllowPrivateNetworks override +
  hop-cap default-5 + IP classification) and
  `internal/auth/private_ip_baseline_test.go` (4 tests, pins HTTPS-only +
  private-block boundary). Both GREEN on unchanged code.
- Wrote `ops/security/analyze-report.md` (surface inventory + gap list).

### T02 — Secret hygiene (completed; gitleaks CI-only)
- `.gitleaks.toml` allowlist (oidc_stub testdata, *_test.go, runbook samples,
  placeholders). pre-commit gitleaks hook (v8.20.0) added.
  `.github/workflows/security.yml` created with gitleaks as FIRST job
  (fetch-depth: 0 for full history). `.github/CODEOWNERS` gates allowlist edits.
  `ops/security/gitleaks-fp-log.md` baseline.
- gitleaks binary NOT installed locally → authoritative full-history scan runs
  in CI. No history rewrite performed (REQ-SEC-005a human gate not invoked).

### T03 — Dependency CVE consolidation (completed)
- Trivy jobs added to `security.yml` (config scan + per-service image matrix +
  CycloneDX SBOM, CVSS>=7.0 blocking, UNFIXED informational).
  `ops/security/vuln-exceptions.yaml` schema + `scripts/check-vuln-exceptions.sh`
  (90-day deadline enforcement; verified PASS within window / FAIL past
  deadline). `.github/workflows/deps-audit.yml` left UNCHANGED (verified).

### T04 — SSRF generalization (completed; DDD PRESERVE strict)
- New `internal/security/ssrf/` package: options.go, ssrf.go (ValidateScheme/
  ValidateHost/ValidateRedirect/IsPrivateOrLoopback + typed Error+Reason),
  dialer.go (PinnedIPDialer/DialContextWithPinnedIP), hostname.go (cloud-metadata
  blocklist, REQ-SEC-008). Preserves fopts FetchOptions override + RedirectMaxHops.
- Refactored `internal/access/{ssrf,dialer}.go` to delegate (thin wrappers
  translate ssrf.Error -> *FetchError + record metric); `internal/auth/
  private_ip.go` deduped onto ssrf.IsPrivateOrLoopback.
- All 22 CACHE-001 SSRF tests + 8 characterization baselines PASS unchanged.
- New metric collector `internal/obs/metrics/security.go`
  (ssrf_blocks_total{reason,component} + security_event_total{type,severity});
  cardinality allowlist extended (component/type/severity); wired via cycle-free
  atomic hook in obs.Init. 5 hostname blocklist tests + metric tests GREEN.
- ssrf coverage 91.8%.

### T05 — Security event taxonomy (completed; INTEGRATE)
- Added 4 new EventType constants to `internal/audit/types.go` (const block +
  AllEventTypes enum lock) — marked @MX:NOTE for AUTH-003 owner sign-off.
- New `internal/security/events/` package: 7-type taxonomy -> audit.EventType
  mapping + Decision mapping + severity->slog level, emits via the EXISTING
  audit.Emitter. NO new chain/migration/verify job. events coverage 96.6%.

### Coverage (new packages)
- internal/security/ssrf: 91.8% | internal/security/events: 96.6%
- internal/obs/metrics: 94.5% | internal/audit: 84.7% (unchanged)

### Acceptance criteria progress
AC-004 (SSRF extraction preserves CACHE-001), AC-005 (cloud-metadata block +
metric), AC-014 (vuln-exception lifecycle), AC-013 partial (taxonomy->existing
chain) met. Error count delta: 0 (no new failures introduced).

### Blockers / gates for orchestrator
- gitleaks first CI run is the authoritative history baseline; if a TRUE-positive
  is found, REQ-SEC-005a human-approval gate (history rewrite) is required —
  agent did NOT rewrite history.
- AUTH-003 owner cross-SPEC sign-off required for the 4 new EventType constants
  + fail-closed lockdown activation (kept default OFF, staged).
