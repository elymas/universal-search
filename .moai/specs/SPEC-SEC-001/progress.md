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

## 2026-05-29 — Run Phase T06–T10 (plan Phases 6–13) — manager-ddd (DDD)

DDD cycle for the Medium/Low composite tasks. Build clean; full `go test ./...`
GREEN (55 packages, 0 FAIL). Characterization preserved on every refactor.

### ENVIRONMENT BLOCKER (partial completion of T06/T08-prompt/T09)
New-directory creation under `internal/security/` is DENIED in this run
environment (Write + Bash mkdir both refused; writes to EXISTING dirs and repo
root succeed). The three net-new packages — `internal/security/secrets`
(Phase 6), `internal/security/ratelimit` (Phase 9), `internal/security/prompt`
(Phase 10) — therefore COULD NOT be created. Relocating them was rejected: it
would break the SPEC pinned paths + the DEPLOY-001 (`secrets.backend`) and
SYN-002 wiring contracts. These three packages are BLOCKED pending the
orchestrator/user creating the empty dirs (e.g. `mkdir -p
internal/security/{secrets,ratelimit,prompt}` or a `.gitkeep` commit), after
which the code lands mechanically.

### Phase 6 (T06) — secrets resolver — PARTIAL
- DONE: `.moai/config/sections/security.yaml` extended with `secrets.backend`
  (env|k8s|vault, default env), `secrets.k8s_mount_path`, plus `ratelimit` and
  `ssrf` blocks (DEPLOY-001 consumes secrets.backend).
- DONE: REQ-SEC-018 — `scripts/check-no-secret-logs.sh` (python3, precise
  secret-value-in-sink detection; excludes redactKey + env-name string
  literals); baseline PASS (verified locally, exit 0). Wired as `secret-grep`
  job in security.yml. Confirmed `internal/llm/client.go` already redacts
  MasterKey via redactKey — codebase already REQ-SEC-018-compliant at runtime.
- BLOCKED: `internal/security/secrets/{resolver,env,k8s,vault,factory}.go` +
  tests (new dir). Verified real os.Getenv secret-read sites for the future
  refactor: `internal/llm/config/config.go:46` (LITELLM_MASTER_KEY) and
  `internal/adapters/naver/naver.go:115,123` (NAVER_CLIENT_ID/SECRET). NOTE:
  Meili/Qdrant keys are STRUCT FIELDS (not os.Getenv); OIDC client secret has
  NO os.Getenv site — the plan's assumed call sites are narrower than stated.

### Phase 7 (T07a) — static analysis — DONE
- `.gosec.yml` (exclude testdata/vendor/web; HIGH gate; #nosec audit) +
  `.semgrepignore` (tests/fixtures/sidecar-tests/vendor/generated). gosec +
  semgrep jobs added to security.yml (p/golang + p/owasp-top-ten + p/jwt).
- Baseline: gosec/semgrep binaries NOT installable locally (gosec v2.21.4 pulls
  x/tools incompatible with Go 1.26; no semgrep). Baseline runs in CI
  (authoritative) — residual.

### Phase 8 (T07b) — TLS + cookie + CSP — DONE
- CI grep gate `tls-grep` job (no tls.VersionTLS10/11 in non-test Go). Verified:
  `internal/access/phase4_tls.go` already enforces MinVersion TLS12; zero legacy
  literals present.
- `internal/auth/cookie.go` (NewSessionCookie, Secure+HttpOnly+SameSite=Lax) +
  `cookie_test.go` (TestCookieFlagsCompliance, TestSessionCookieMaxAge). Auth
  suite GREEN -race, coverage 90.7%. NOTE: no session cookie is set anywhere yet
  (OIDC callback returns 501 in V1) — this is the tested contract factory for
  when the session flow lands.
- CROSS-SPEC (UI-001): `web/next.config.mjs` extended with additive `headers()`
  — CSP (strict-dynamic + hash, NOT nonce), HSTS, X-Frame-Options DENY,
  X-Content-Type-Options nosniff, Referrer-Policy, Permissions-Policy. File was
  minimal (reactStrictMode only); change is non-conflicting; `node` load check
  passes. Flag for UI-001 owner review.

### Phase 9 (T09) — rate limit — BLOCKED (code) / config DONE
- BLOCKED: `internal/security/ratelimit/limiter.go` + tests (new dir). Design
  recorded: per-tenant token bucket (golang.org/x/time/rate — NOT yet in
  go.mod), stdlib net/http middleware (NOT chi — project has NO web framework;
  mirror `internal/deepagent/costguard/middleware.go` 429+Retry-After pattern),
  V1 alert-only default (reject_on_exceed:false in security.yaml), tenant_id_class
  (known/unknown) metric label only.

### Phase 10 (T08-prompt) — prompt-injection sanitization — BLOCKED (code)
- BLOCKED: `internal/security/prompt/{sanitize,patterns}.go` + tests (new dir).
  SYN-002 wiring point identified precisely: `internal/deepagent/agents.go:254`
  `VerifierWithChecker(ctx, cfg, draft, docs []deepreport.NormalizedDocPayload,
  checkFn)` — Sanitize must wrap each doc body in an `<EVIDENCE>` block before it
  reaches `CheckFaithfulnessFn` → `synthesis.CheckFaithfulness(...docs)`
  (`internal/synthesis/faithfulness.go:40`). Python sidecar unchanged.

### Phase 11 (T10a) — SLSA + cosign — DONE
- `.github/workflows/release.yml` NEW: SLSA L2 provenance
  (slsa-github-generator generator_generic_slsa3.yml@v2.0.0) + cosign keyless
  sign (cosign-installer@v3.7.0). `build` job is an explicit REL-001 PLACEHOLDER
  with a documented cross-SPEC boundary (REL-001 owns goreleaser/build; SEC-001
  owns the SLSA+cosign layer — no duplication). cosign verify command in runbook.

### Phase 12 (T10b) — OWASP ASVS L1 — DONE
- `ops/security/owasp-asvs-checklist.md`: V1–V14, 38 rows (3 N/A, 35 applicable),
  33 Pass / 2 Deferred / 0 Fail → 94.3% Pass (>= 80% target met). Every Pass row
  has an evidence link (lint-clean). The 2 Deferred (V5.2.5 prompt-injection,
  V11.1.4 rate-limit) map to the blocked packages.

### Phase 13 (T10c) — operator docs — DONE
- `ops/security/runbook.md` (secret rotation 4-step + REQ-SEC-005a 5 guards,
  SSRF triage, CRITICAL CVE response, cosign verify, audit chain-break recovery),
  `ops/security/threat-model.md` (STRIDE S/T/R/I/D/E from research §13, with T1
  corrected to the EXISTING AUTH-003 chain — no "new Merkle"), `SECURITY.md`
  (disclosure email, response SLA, V1 no-bounty). last-reviewed-at headers set
  for NFR-SEC-005.

### Acceptance criteria progress (this run)
Newly met: AC for REQ-SEC-010 (gosec/semgrep CI), REQ-SEC-012 (TLS grep +
cookie test), REQ-SEC-016 (SLSA+cosign), REQ-SEC-011 (ASVS >= 80%), REQ-SEC-005
(+005a) runbook, NFR-SEC-005 (threat model), REQ-SEC-018 grep gate.
Partial/blocked: REQ-SEC-013 (secrets — config done, code blocked), REQ-SEC-014
(ratelimit — config done, code blocked), REQ-SEC-015 (prompt — wiring point
identified, code blocked). Error count delta: 0 (no new test failures).

### Blockers / gates for orchestrator (T06–T10)
- HARD: create `internal/security/{secrets,ratelimit,prompt}/` dirs (env denied
  agent dir-creation) so the three net-new packages can land. Code is designed;
  only the directory creation is blocked.
- CI-authoritative: gosec/semgrep/gitleaks/trivy/cosign baselines run in CI (no
  local binaries; gosec uninstallable under Go 1.26).
- CROSS-SPEC carry-forward: UI-001 (next.config.mjs CSP review), REL-001
  (release.yml SLSA/cosign boundary), AUTH-003 (4 new EventType consts sign-off
  from T05).

## 2026-05-29 — Run Phase T06 + T08 UNBLOCKED (Phases 6/9/10) — manager-ddd (DDD)

The three net-new packages that were dir-creation-blocked last run are now
landed (orchestrator pre-created the dirs + doc.go stubs). The package formerly
named `secrets/` is now `secretstore/` (spec v0.2.1 rename — `secrets/` collided
with the repo-root `./secrets/**` deny rule). Config key UNCHANGED:
`secrets.backend`. Build clean (`go build ./...` exit 0); `go vet` clean on all
touched packages; `go mod tidy` done.

### Phase 6 (T06) — secret resolver — COMPLETE
- `internal/security/secretstore/`: resolver.go (Resolver iface + ErrNotImplemented
  sentinel, @MX:ANCHOR+REASON on the Resolver.Get contract), env.go (EnvResolver =
  os.LookupEnv wrapper, error on unset), k8s.go (K8sResolver reads
  <mount>/<key>, trims, path-traversal guard), vault.go (stub → ErrNotImplemented),
  factory.go (NewResolver env|k8s|vault, "" defaults to env, unknown=error).
  Coverage 96.0%.
- PRESERVE refactor (characterization-first): `internal/llm/config/config.go`
  LITELLM_MASTER_KEY and `internal/adapters/naver/naver.go` NAVER_CLIENT_ID/SECRET
  now resolve via a package-level `secretEnv secretstore.Resolver = NewEnvResolver()`.
  EnvResolver has os.Getenv semantics (error iff empty), so the existing empty→
  fallback/"not set" paths are byte-identical. Evidence: llm + naver suites GREEN
  both BEFORE (cached ok) and AFTER (llm 89.9%, llm/config 94.7%, naver 94.7%).
  `os` import dropped from naver.go (no other use). Scope kept minimal — Meili/
  Qdrant struct-field keys and the non-existent OIDC Getenv site untouched.
- REQ-SEC-018: relies on the existing CI grep (`scripts/check-no-secret-logs.sh`)
  — secretstore never logs a resolved value, so there is no natural unit-test
  home for TestNoSecretInLogs (noted, not invented).

### Phase 9 (T08-ratelimit) — per-tenant rate limit — COMPLETE
- `internal/security/ratelimit/`: limiter.go (per-tenant `golang.org/x/time/rate`
  token bucket, default 60/min + burst 60, lazy per-tenant creation under mutex;
  @MX:WARN+REASON on hot-path Allow), middleware.go (stdlib net/http — NO chi;
  alert-only by DEFAULT, 429+Retry-After only when MiddlewareConfig.RejectOnExceed
  is true, modeled on costguard CapCheckMiddleware). On breach it ALWAYS emits
  `ratelimit.exceeded` (events.SeverityMedium) + increments the metric, then
  serves the request unless enforcement is on.
- New metric `usearch_security_ratelimit_exceeded_total{tenant_id_class}` with
  class ∈ {known, unknown} ONLY — raw tenant_id is never a label. Added to ALL
  THREE cardinality allowlists (metrics.go `labelNames` slice, metrics_test.go
  map, router_test.go switch — the inverse-check site that initially failed).
  `golang.org/x/time` promoted to a direct dep (v0.15.0). Coverage 100%.

### Phase 10 (T08-prompt) — prompt-injection sanitization — COMPLETE
- `internal/security/prompt/`: patterns.go (5 heuristic regex classes —
  override_attempt, role_injection, tag_break, persona_swap, format_break; NO LLM
  classifier), sanitize.go (Sanitize wraps content in <EVIDENCE>…</EVIDENCE> and
  neutralizes breakout markers incl. literal </EVIDENCE>; SanitizeAndEmit fires a
  low-severity `prompt.sanitized` event on detection; @MX:NOTE marks the SYN-002
  integration point). Coverage 100%.
- SYN-002 wiring: `internal/deepagent/agents.go` VerifierWithChecker docs loop now
  passes each `d.Body` through `prompt.Sanitize(...).Sanitized` before
  CheckFaithfulness. deepagent suite GREEN (74.7% pkg coverage unchanged; mockFns
  ignore doc-body content so wrapping is behavior-safe). Python sidecar
  (services/researcher) UNCHANGED.

### Verification (this run)
- `go build ./...` exit 0; `go vet` clean (security/llm/naver/deepagent/obs-metrics).
- `go test -race -cover ./internal/security/... ./internal/llm/... ./internal/adapters/naver/... ./internal/deepagent/...`
  ALL GREEN. New-package coverage: secretstore 96.0%, ratelimit 100.0%, prompt 100.0%
  (all >= 85% target). obs/metrics GREEN with the new collector + 3 allowlist updates.

### Acceptance criteria progress (this run)
Newly met: REQ-SEC-013 (secretstore 3 backends + env-resolver call-site refactor),
REQ-SEC-014 (ratelimit alert-only + config-gated 429 + bounded metric),
REQ-SEC-015 (prompt Sanitize + EVIDENCE wrap wired into SYN-002 verifier flow).
Error count delta: 0 (no new test failures). The 2 previously-Deferred ASVS rows
(V5.2.5 prompt-injection, V11.1.4 rate-limit) are now implemented.

### Residual / cross-SPEC carry-forward
- SPEC path amendment applied (spec.md + plan.md `secrets/`→`secretstore/`,
  v0.2.1 HISTORY note, version bumped 0.2.0→0.2.1, status stays approved).
- Orchestrator note: the task brief labeled these "T06/T09/T10"; the accurate
  task IDs are T06 (secretstore) and T08 (ratelimit+prompt). T09/T10 (supply-chain
  + operator docs) were already completed in the prior run and are untouched.
- ratelimit middleware is wired as a constructor (MiddlewareConfig) but not yet
  mounted on a live route — mounting + a TenantExtractor backed by costguard's
  TenantIDFromContext is a future integration step (no live HTTP route currently
  rate-limited; same posture as the cookie factory from Phase 8).

---

## DDD Cycle — evaluator-active FAIL remediation (Security 65→ closing)

ANALYZE-PRESERVE-IMPROVE applied to the 3 must-fix findings from the
evaluator-active FAIL. Characterization-first: `go test -race ./internal/deepagent/...`
green before and after; behavior preserved except EVIDENCE wrapping.

### Finding 1 [HIGH] — prompt-injection sanitization fully wired on the main LLM path
Root cause: `prompt.Sanitize` (REQ-SEC-015) was applied only in
`VerifierWithChecker`; the LLM-facing agents (Researcher, Reviewer, Writer) sent
RAW document bodies to the model.

Fix (`internal/deepagent/agents.go`):
- Added shared helper `sanitizeDocBodies([]NormalizedDocPayload) []NormalizedDocPayload`
  (@MX:ANCHOR) — returns a copy with each `.Body` EVIDENCE-wrapped via
  `prompt.Sanitize`; never mutates input. The canonical `ResearcherOutput.Evidence`
  stays RAW so the Verifier path sanitizes it exactly once (no double-wrap).
- Wired at every document-body→LLM serialization site:
  - Researcher `json.Marshal(sanitizeDocBodies(payloads))` (agents.go:~63)
  - Reviewer `json.Marshal(sanitizeDocBodies(research.Evidence))` (agents.go:~117)
  - Writer `json.Marshal(sanitizeDocBodies(research.Evidence))` (agents.go:~157)
  - Verifier `prompt.Sanitize(d.Body)` (agents.go:~298, pre-existing, unchanged)
- Plain `Sanitize` (no emitter) at all sites — matches the existing Verifier
  pattern; no emitter is threaded into these functions, so no signature change.

Contract tests (`internal/deepagent/agents_test.go`):
- `TestResearcherSanitizesDocBodiesBeforeLLM` — asserts the EVIDENCE fence reaches
  the LLM mock AND that returned Evidence stays raw.
- `TestReviewerSanitizesEvidenceBeforeLLM`, `TestWriterSanitizesEvidenceBeforeLLM`.
- `TestVerifierCallsCheckFaithfulnessExactlyOnce` — augmented to assert the doc
  text reaching `checkFn` is EVIDENCE-wrapped (closes the gap the evaluator noted).
- `assertContainsEvidence` helper derives the JSON-escaped marker at runtime
  (json.Marshal escapes `<`→`<`), so assertions match the real LLM-facing bytes.

deepagent: 75.0% coverage, race-clean, all green before+after.

### Finding 2 [MEDIUM] — K8sResolver path-traversal guard hardened
`internal/security/secretstore/k8s.go`: the old `strings.ContainsAny(key,"/\\")`
let bare `".."` through. Hardened to reject `""`, `"."`, `".."`, separator-bearing
keys, AND added a canonical-prefix check
`strings.HasPrefix(filepath.Clean(join), filepath.Clean(mountPath)+os.PathSeparator)`.
Test `TestK8sResolverRejectsPathTraversal` extended with `".."` and `"."` cases.
secretstore: 92.6% coverage, green.

### Finding 3 [LOW] — ASVS checklist refreshed
`ops/security/owasp-asvs-checklist.md`: V5.2.5 and V11.1.4 Deferred→Pass with
evidence links to real test files. Summary: Pass 33→35, Deferred 2→0,
pass rate 33/35 (94.3%) → 35/35 (100%).

### Verification
- `go build ./...` exit 0; `go vet ./internal/deepagent/... ./internal/security/...` exit 0.
- `go test -race -cover ./internal/deepagent/... ./internal/security/...` all ok.
- Security self-check — every document-body→LLM path is now sanitized:
  Researcher (docsJSON), Reviewer (evidenceJSON), Writer (evidenceJSON),
  Verifier (docTexts). No raw `.Body` reaches any synthesis LLM.
