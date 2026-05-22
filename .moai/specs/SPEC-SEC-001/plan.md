# SPEC-SEC-001 Plan — phased implementation

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: **DDD** (ANALYZE-PRESERVE-IMPROVE per `.claude/rules/
moai/workflow/workflow-modes.md`). DDD-mode justification: 본 SPEC은
**기존 보안 자산을 consolidate + extract**하는 작업이 본질이며 (CACHE-
001 SSRF guards → `internal/security/ssrf/`, AUTH-001 private-IP
block 통합), 신규 보안 시스템 발명이 아니다. ANALYZE 단계에서 현 보안
surface inventory를 정확히 capture; PRESERVE 단계에서
characterization test로 behavior 동일성 보장; IMPROVE 단계에서 통합
controls + 신규 gap closure (Trivy, gitleaks, prompt-injection
sanitization 등) 적용. 신규 코드 (예: secrets resolver, ratelimit,
prompt sanitize)는 TDD 하위 cycle로 실행.

Coverage target: 85% (per spec.md frontmatter)
Harness: **thorough** (per `.moai/config/sections/harness.yaml` —
**P0 + security domain은 thorough 강제**; Sprint Contract MANDATORY
per `.claude/rules/moai/design/constitution.md` §11 "Sprint Contracts
are required when harness level is `thorough`")

본 plan은 SPEC-SEC-001 구현을 priority-ordered phases로 sequence
한다. `.claude/rules/moai/core/agent-common-protocol.md` 시간 예측
금지 — phase는 priority + ordering만 사용.

---

## 1. Implementation principle

본 SPEC의 plan philosophy 4축:

1. **Risk-burndown ordering** — 가장 위험한 gap (committed secrets in
   git history)부터 audit. dependency CVE → SSRF allowlist → static
   analysis baseline → OWASP checklist → 신규 controls 순.
2. **DDD characterization-first** — CACHE-001 / AUTH-001 / AUTH-003
   기존 코드에 손대기 전 characterization test로 현 behavior snapshot.
   refactor 후 동일 test가 unchanged passing 보장 (PRESERVE).
3. **CI 분리** — `.github/workflows/deps-audit.yml` (existing, D1
   rationale 따라 unchanged 유지) + `.github/workflows/security.yml`
   (new). 두 workflow가 같은 도구를 중복 실행하지 않도록 명확한
   ownership boundary.
4. **Non-rollbackable awareness** — 보안 부채는 release 후 patch가
   어렵다. 매 phase exit gate에 plan-auditor 또는 manual sign-off
   요구 (특히 secrets handling refactor 시).

---

## 2. Sprint Contract (REQUIRED per thorough harness)

Sprint Contract는 builder (manager-ddd) ↔ evaluator-active 사이
협상 결과로 매 GAN Loop iteration 시작 전 작성. 본 SPEC의 V1 Sprint
Contract draft (run phase에서 evaluator-active와 finalize):

### Acceptance checklist (testable per iteration)

- [ ] gitleaks pre-commit hook 설치 + CI security.yml 통합 + zero
      baseline finding
- [ ] Trivy container scan workflow operational + CRITICAL/HIGH
      blocking
- [ ] `internal/security/ssrf/` package extraction + 9 REQ-CACHE-013
      tests passing unchanged
- [ ] Cloud metadata hostname blocklist (REQ-SEC-008) + integration
      test
- [ ] OWASP ASVS L1 checklist (V1-V14) ≥ 80% Pass status
- [ ] gosec + semgrep CI integration + zero HIGH finding baseline
- [ ] Audit log Merkle hash chain (REQ-SEC-017) + 1M-row verification
      ≤ 30s
- [ ] LLM prompt-injection sanitization (REQ-SEC-015) + SYN-002
      integration test

### Priority dimension

**Security correctness** (Functionality + Security from evaluator-active
4-dimension scoring). Originality is N/A — 본 SPEC은 industry-standard
controls 적용; novelty 없음. Completeness는 ASVS L1 checklist coverage
로 측정.

### Test scenarios (Playwright/integration)

- §5.1 dependency CVE injection: 테스트 브랜치에 known CVE 도입 → CI
  fail 확인
- §5.2 secret commit detection: `AKIA...` 패턴 commit → pre-commit
  + CI block 확인
- §5.5 cloud metadata blocking: `http://169.254.169.254/...` fetch →
  blocked + metric snapshot 확인
- §5.11 prompt-injection sanitization: 적대적 indexed doc → sanitized
  + event recorded 확인

### Pass conditions (minimum score per criterion)

- Security correctness: ≥ 0.85 (must-pass; below threshold = sprint
  fail per §12 Mechanism 3 Must-Pass Firewall)
- Functionality: ≥ 0.75
- Craft: ≥ 0.70 (code quality; lower acceptable for security-first
  refactor)
- Consistency: ≥ 0.80 (기존 SPEC pattern 준수)

---

## 3. Phase ordering

Priority labels per MoAI rule (no time estimates). Phase ordering은
**risk-burndown** principle 따라 sequence.

### Phase 0 — Plan-auditor PASS (Priority High)

- Plan-auditor reviews spec.md + research.md + plan.md + acceptance.md
  (the latter authored alongside this plan).
- Address MAJOR / MINOR / NIT findings via amendment commits.
- Sprint Contract finalize with evaluator-active.
- Status transition: `draft → approved` on PASS.
- Block: no implementation work begins until Phase 0 completes.

### Phase 1 — ANALYZE existing security surface (Priority High)

DDD ANALYZE step. 기존 코드 inventory + characterization test
baseline.

Tasks:
1. `internal/access/ssrf.go` (124 lines) + `internal/access/dialer.go`
   (83 lines) 정독; REQ-CACHE-013 9개 test의 input/output 패턴 기록.
2. `internal/auth/private_ip.go` (59 lines) 정독; AUTH-001 D8 검증
   path와의 인터페이스 매핑.
3. `internal/auth/discovery.go` (OIDC discovery URL fetch) 정독;
   SSRF guard 호출 지점 확인.
4. `internal/obs/metrics/` 현재 cardinality allowlist (recent e5ea981
   commit의 `reason_class` pattern) 확인; 본 SPEC이 추가할 label
   value 사전 audit.
5. AUTH-003 audit log schema (`internal/auth/rbac/` 관련) 정독;
   `prev_hash` column 추가의 migration impact 분석.
6. characterization test 작성: `internal/access/ssrf_baseline_test.go`
   (REQ-CACHE-013 input/output snapshot — REFACTOR 후에도 동일하게
   passing해야 함).
7. characterization test 작성: `internal/auth/private_ip_baseline_
   test.go` (AUTH-001 D8 검증 path snapshot).
8. ANALYZE 보고서 작성: `ops/security/analyze-report.md` (현 surface
   inventory + identified gap 목록).

Exit criterion:
- characterization tests 작성 완료 + green (현 코드 unchanged).
- ANALYZE 보고서 review 완료.
- 이후 IMPROVE phase에서 refactor 시 characterization tests가 break
  되지 않는지 매 commit 검증 가능 상태.

### Phase 2 — Secret hygiene baseline (Priority High)

[CRITICAL] 가장 시급한 risk. gitleaks 도입 + 현 git history audit +
baseline 확립.

Tasks:
1. `.gitleaks.toml` 작성 — allowlist 항목: `internal/auth/testdata/
   oidc_stub/`, `*_test.go` testdata, `ops/security/runbook.md`
   sample tokens.
2. gitleaks v8.20.0+ 로컬 설치 + 현 git history 전체 scan:
   `gitleaks detect --source . --log-opts="--all"`
3. 발견된 finding 분류:
   - True-positive (실제 secret): provider revoke + history rewrite
     + AUTH-003 audit log + post-mortem
   - False-positive: `.gitleaks.toml` allowlist 추가 (review 필요)
4. baseline 0 finding 확립.
5. pre-commit hook 추가: `.pre-commit-config.yaml`에 `gitleaks/
   gitleaks-action` 항목.
6. CI workflow `.github/workflows/security.yml` 신설 — 첫 job으로
   gitleaks scan 추가.
7. CODEOWNERS update: `.gitleaks.toml` ownership을 security reviewer
   에게 할당 (allowlist 추가 시 review 강제).

Exit criterion:
- 현 git history scan에서 0 true-positive finding (또는 모든 true-
  positive 처리 완료).
- pre-commit hook 활성 + CI security.yml gitleaks job green.
- `.gitleaks.toml` baseline established.

### Phase 3 — Dependency CVE scan consolidation (Priority High)

D1 핵심: Trivy 신설 + 기존 deps-audit.yml unchanged 유지.

Tasks:
1. `.github/workflows/security.yml`에 Trivy job 추가:
   - `aquasecurity/trivy-action@0.24.0` 사용
   - Dockerfile scan (severity HIGH/CRITICAL block)
   - final built image scan (build → scan → push pipeline)
   - SBOM 생성 (CycloneDX format → release artifact 첨부 준비)
2. Severity gate 검증: 테스트 PR에 known CVE-bearing dep 추가하여
   blocking 확인.
3. UNFIXED 예외 처리 frame:
   - `ops/security/vuln-exceptions.yaml` 신설
   - 90-day review deadline enforce
   - CI job이 deadline-expired 항목 검출 시 fail
4. deps-audit.yml unchanged 검증 — Trivy 추가 후에도 govulncheck /
   pip-audit / pnpm-audit / hadolint / license-scan / searxng-digest
   모두 green 유지.
5. NFR-SEC-001 timing budget 검증: security.yml gitleaks + Trivy
   합쳐서 5분 이내 (PR median 기준 측정).

Exit criterion:
- Trivy CI job operational + 테스트 PR로 blocking 검증.
- vuln-exceptions.yaml schema + lifecycle 검증.
- 기존 deps-audit.yml 4-tool chain 모두 unchanged green.
- security.yml + deps-audit.yml 합쳐 NFR-SEC-001 15분 budget 내.

### Phase 4 — SSRF mitigation generalization (Priority High)

D3 핵심: CACHE-001 + AUTH-001 SSRF guards를 `internal/security/ssrf/`
로 통합. DDD PRESERVE strict 단계.

Tasks:
1. `internal/security/ssrf/` 패키지 생성:
   - `ssrf.go`: ValidateScheme, ValidateHost, ValidateRedirect
     (CACHE-001 코드 그대로 이식; Options struct 신설)
   - `dialer.go`: PinnedIPDialer (CACHE-001 dialer.go 이식)
   - `hostname.go`: hostname blocklist 검증 (NEW — REQ-SEC-008)
   - `options.go`: Options struct (AllowPrivateNetworks, MaxRedirects,
     HostnameBlocklist, SchemeAllowlist)
2. characterization test 작성: `ssrf_test.go` — CACHE-001 REQ-CACHE-
   013 9 tests의 input/output을 그대로 재현 (behavior 동일성 검증).
3. 신규 hostname blocklist 테스트: `hostname_test.go`
   (TestValidateHostBlocksGCPMetadata, TestValidateHostBlocksAWSMetadata,
   TestValidateHostBlocksAzureMetadata, TestValidateHostCaseInsensitive,
   TestValidateHostSuffixMatch).
4. observability 통합: `internal/obs/metrics/security.go` 신설 —
   `usearch_security_ssrf_blocks_total{reason, component}` Counter.
   OBS-001 cardinality allowlist 확장: `reason` ∈ {scheme,
   private_ip, redirect_hop, dns_rebind, hostname_allowlist},
   `component` ∈ {access, auth, adapter}.
5. CACHE-001 refactor: `internal/access/ssrf.go` + `internal/access/
   dialer.go`가 `internal/security/ssrf/` 사용하도록 변경. 기존 함수
   signature는 유지 (caller 변경 없음 — 내부 wrapping).
6. AUTH-001 refactor: `internal/auth/private_ip.go` + `internal/auth/
   discovery.go`가 같은 패키지 사용.
7. CACHE-001 REQ-CACHE-013 9 tests **unchanged passing 검증** (DDD
   PRESERVE 핵심). characterization test도 green 유지.
8. NFR-SEC-006 latency 검증: SSRF validation overhead ≤ 10ms p99
   (CACHE-001 Phase 3 benchmark baseline 대비).

Exit criterion:
- `internal/security/ssrf/` 패키지 컴파일 + 모든 test green.
- CACHE-001 + AUTH-001 refactor 완료 + 기존 모든 test unchanged
  passing.
- 신규 hostname blocklist 5개 test green.
- metric collector + cardinality allowlist 확장 완료.
- NFR-SEC-006 latency 검증 완료.

### Phase 5 — Security event taxonomy + Merkle audit (Priority High)

D9 핵심: 7-type event logger + Merkle hash chain. AUTH-003 schema
amendment 포함.

Tasks:
1. AUTH-003 audit log schema amendment:
   - `.moai/specs/SPEC-AUTH-003/spec.md`에 `prev_hash` column 추가
     amendment 작성
   - PostgreSQL migration script: `ops/migrations/20260522_audit_
     prev_hash.sql`
   - 기존 row backfill: 첫 row의 `prev_hash = SHA-256("")`, 이후 row
     는 순차적으로 prev row hash 계산
   - rollback script 준비
3. `internal/security/events/` 패키지 생성:
   - `event.go`: 7-type enum + Insert function
   - `merkle.go`: hash chain compute + verify
   - `event_test.go`: TestEventInsertWithPrevHash, TestMerkleChain
     Verification, TestChainBreakDetection
4. `usearch_security_event_total{type, severity}` Counter 추가
   (`internal/obs/metrics/security.go`).
5. Merkle chain verification job:
   - `.github/workflows/audit-verify.yml` (nightly 02:00 UTC)
   - Go binary `cmd/audit-verify/main.go` 신설
   - 1M-row benchmark: `BenchmarkMerkleVerify1M` → ≤ 30s (NFR-SEC-
     004)
6. Chain break 감지 시 fail-closed: 후속 audit write block until
   manual operator intervention.

Exit criterion:
- AUTH-003 schema amendment merged.
- migration script staging 환경 검증.
- `internal/security/events/` 패키지 + 모든 test green.
- 1M-row benchmark ≤ 30s.
- audit-verify.yml workflow operational.

### Phase 6 — Secrets resolver multi-backend (Priority Medium)

D5 핵심: 3-tier secrets (env / K8s / Vault stub).

Tasks:
1. `internal/security/secrets/` 패키지 생성:
   - `resolver.go`: Resolver interface (`Get(ctx, key) (string,
     error)`)
   - `env.go`: EnvResolver (os.Getenv wrapping)
   - `k8s.go`: K8sResolver (mounted file at `/var/run/secrets/
     <secret-name>`)
   - `vault.go`: VaultResolver stub (returns ErrNotImplemented)
   - `factory.go`: NewResolver(backend string) (Resolver, error)
2. `.moai/config/sections/security.yaml` 신설:
   - `secrets.backend: env|k8s|vault` (default env)
   - `secrets.k8s_mount_path: /var/run/secrets`
3. 테스트: TestEnvResolverReadsOSEnv, TestK8sResolverReadsMountedFile
   (temp dir 사용), TestVaultResolverReturnsErrNotImplemented.
4. existing wiring 검증:
   - `internal/llm/` LiteLLM API key resolution
   - `internal/index/` Meilisearch master key + Qdrant API key
   - `internal/auth/` OIDC client secret
   - 위 모든 곳에서 Resolver.Get 사용하도록 refactor (현재는 직접
     os.Getenv).
5. REQ-SEC-018 검증:
   - CI grep step: `grep -rn "fmt.*\$\(SECRET\|KEY\|TOKEN\|PASSWORD
     \)" internal/` → zero match in non-test files
   - TestNoSecretInLogs: 모든 패키지의 log fixture에서 known secret
     pattern 부재 검증

Exit criterion:
- Resolver 3 backend 모두 implementation + test green.
- existing wiring refactor 완료 + 기존 기능 unchanged.
- REQ-SEC-018 CI grep + TestNoSecretInLogs green.

### Phase 7 — Static analysis baseline (Priority Medium)

D4 일부: gosec + semgrep CI integration.

Tasks:
1. `.gosec.yml` 작성:
   - exclude `*_test.go`, `testdata/`
   - severity HIGH = fail, MEDIUM = informational
2. security.yml에 gosec job 추가 (`securego/gosec@v2.21.0` action).
3. `.semgrepignore` 작성:
   - 제외: `testdata/`, `services/*/tests/`, `internal/**/*_test.go`
4. security.yml에 semgrep job 추가 (`returntocorp/semgrep-action@v1`):
   - rule sets: `p/golang`, `p/owasp-top-ten`, `p/jwt`
5. baseline scan: 현 codebase에서 gosec + semgrep 실행 → HIGH
   finding 모두 해결 또는 `// #nosec`/`# nosemgrep` 주석 + reason.
6. REQ-SEC-010 검증 PR: `crypto/md5` 사용 도입 → gosec fail 확인;
   hardcoded JWT secret 도입 → semgrep `p/jwt` fail 확인.

Exit criterion:
- gosec + semgrep CI green at baseline.
- 검증 PR 두 건 모두 blocking 확인.

### Phase 8 — TLS + cookie + CSP secure defaults audit (Priority Medium)

D4 나머지: REQ-SEC-012 검증 + 보강.

Tasks:
1. CI grep step: `grep -rn "tls.VersionTLS1[01]" --include="*.go"
   --exclude="*_test.go" internal/` → zero match assertion.
2. CACHE-001 `internal/access/phase4_tls.go` 검증: TLS min version
   설정 확인.
3. `internal/auth/` session cookie 검증:
   - TestCookieFlagsCompliance: Secure: true, HttpOnly: true,
     SameSite: SameSiteLaxMode
4. SPEC-UI-001 (Next.js app) coordination:
   - `next.config.js` headers: strict CSP, HSTS, X-Frame-Options:
     DENY, X-Content-Type-Options: nosniff, Referrer-Policy:
     strict-origin-when-cross-origin
   - CSP: `default-src 'self'; script-src 'self' 'strict-dynamic'`
     (V1 hash 방식; nonce는 post-V1)
   - UI-001 owner와 PR coordination

Exit criterion:
- 모든 grep + cookie test green.
- UI-001 PR merged with security headers.

### Phase 9 — Rate limit + abuse detection (Priority Medium)

D6 핵심.

Tasks:
1. `internal/security/ratelimit/` 패키지:
   - `limiter.go`: per-tenant token bucket using `golang.org/x/time/
     rate`
   - default 60 queries/min per tenant
   - HTTP 429 response with Retry-After header
2. API server integration: chi middleware wrapping
3. REQ-SEC-014 검증: TestRateLimitExceededReturns429
4. event integration: `internal/security/events/` `ratelimit.
   exceeded` event emit
5. cardinality 검증: raw tenant_id never as metric label value;
   `tenant_id_class` (known/unknown) only

Exit criterion:
- rate limit middleware operational.
- 429 response + Retry-After 검증.
- event + metric cardinality 검증.

### Phase 10 — LLM prompt-injection sanitization (Priority Medium)

D7 핵심.

Tasks:
1. `internal/security/prompt/` 패키지:
   - `sanitize.go`: Sanitize function + EVIDENCE block wrapping
   - `patterns.go`: injection pattern enum (override_attempt,
     role_injection, tag_break, persona_swap, format_break)
   - `sanitize_test.go`: TestSanitizeDetectsIgnorePreviousPattern,
     TestSanitizeWrapsEvidenceBlock, TestSanitizeEmitsEvent
2. SPEC-SYN-002 integration:
   - Go-side wiring point: SYN-002의 citation faithfulness flow
     entry
   - services/researcher Python sidecar는 Go orchestrator가 prompt
     준비 시 sanitization 적용 — Python 측 변경 없음
3. event integration: `prompt.sanitized` event with severity low
4. EVAL-001 baseline 영향 측정: sanitization on/off A/B 테스트
   (legitimate content false-positive 검증)

Exit criterion:
- Sanitize 패키지 + 모든 test green.
- SYN-002 integration test green + citation enforce passing.
- EVAL-001 baseline 점수 ±0.02 이내 (acceptable noise).

### Phase 11 — SLSA + cosign release pipeline (Priority Medium)

D8 핵심. SPEC-REL-001 prep work.

Tasks:
1. `.github/workflows/release.yml` 신설 (또는 existing release
   workflow에 step 추가):
   - `slsa-framework/slsa-github-generator/.github/workflows/
     generator_generic_slsa3.yml@v2.0.0` 호출
   - `*.intoto.jsonl` 생성 + release artifact 첨부
2. cosign keyless signing:
   - `sigstore/cosign-installer@v3.7.0` action
   - `cosign sign --yes ${IMAGE}:${TAG}` (OIDC identity 자동)
   - signature → release artifact 첨부
3. ops/security/runbook.md에 verify 명령 documented:
   ```
   cosign verify \
     --certificate-identity-regexp "https://github.com/<org>/<repo>/.github/workflows/release.yml@.*" \
     --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
     <image>:<tag>
   ```
4. 테스트 release tag 생성 후 verify 명령 실행 검증.

Exit criterion:
- release workflow에 SLSA + cosign 통합.
- 테스트 tag로 verify 명령 success 검증.
- runbook documented.

### Phase 12 — OWASP ASVS L1 checklist authoring (Priority Medium)

D4 evidence collection.

Tasks:
1. `ops/security/owasp-asvs-checklist.md` 신설:
   - V1-V14 각 section table
   - 각 row: ASVS ID, Applicability, Verification (Automated/Manual),
     Evidence link, Status (Pass/Fail/Deferred)
2. Automated 항목은 본 SPEC의 CI workflow / test로 link.
3. Manual 항목은 review 수행 + status 결정:
   - V1 Architecture: threat model link (research §13 → ops/security/
     threat-model.md)
   - V14 Configuration: SECURITY.md link
4. ≥ 80% Pass status 달성 검증.
5. Fail / Deferred 항목은 명시적 rationale + post-V1 plan.

Exit criterion:
- checklist 모든 row populated.
- ≥ 80% Pass.
- lint test: no Pass entry without evidence link.

### Phase 13 — Operator documentation (Priority Low)

Tasks:
1. `ops/security/runbook.md` 작성:
   - committed-secret rotation (4-step REQ-SEC-005)
   - SSRF block triage
   - CRITICAL CVE response procedure
   - cosign verify procedure (Phase 11 link)
   - audit log chain break recovery
2. `ops/security/threat-model.md` 작성 — research §13 cut.
3. `ops/security/vuln-exceptions.yaml` schema 정의 + initial entry
   (Phase 3에서 시작).
4. `ops/security/gitleaks-fp-log.md` 시작 (Phase 2에서).
5. `SECURITY.md` (repo root):
   - 책임 있는 disclosure email
   - 응답 SLA (CRITICAL 24h, HIGH 72h, MEDIUM 7d)
   - 보상 정책 (V1: no bounty, public acknowledgment only)

Exit criterion:
- 모든 docs 작성 완료 + lint pass.
- SECURITY.md GitHub repo 인식 (GitHub UI에서 Security tab 표시).

### Phase 14 — Sprint Contract Cycle (Priority High, runs continuously)

Per `.claude/rules/moai/design/constitution.md` §11 GAN Loop:

Tasks (per iteration):
1. expert-frontend (실제는 expert-security via manager-ddd) implements
   per Sprint Contract acceptance checklist.
2. evaluator-active scores against Sprint Contract criteria only
   (not arbitrary standards per §11 [HARD]).
3. Pass threshold ≥ 0.85 must-pass (Security correctness).
4. Iteration on fail; escalation after 3 iterations per §11.
5. Contract evolution: passed criteria carry forward; failed criteria
   refined; new criteria added if gaps revealed.

Sprint Contract artifacts stored in `.moai/sprints/SPEC-SEC-001/`.

### Phase 15 — Sync phase (Priority Low)

Goal: documentation + PR + status transition.

Tasks:
1. `manager-docs` updates user-facing docs:
   - parent repo `README.md`: Security section with link to
     SECURITY.md + ops/security/
   - SPEC-DOC-001 user docs site (when published): security
     hardening page
2. CHANGELOG entry per phase deliverable.
3. `manager-git` opens PR per V1 release process.
4. Status transition: `approved → implemented` after merge + all
   acceptance criteria green.
5. M9 SPEC-REL-001 / SPEC-DEPLOY-001 unblock notification.

---

## 4. Test inventory (DDD characterization + new TDD tests)

per-phase test checkpoints:

### Phase 1 (ANALYZE) — characterization tests:
- `internal/access/ssrf_baseline_test.go` (REQ-CACHE-013 input/output
  snapshot)
- `internal/auth/private_ip_baseline_test.go` (AUTH-001 D8 snapshot)

### Phase 2 (Secret hygiene):
- gitleaks scan: zero finding at baseline
- `TestGitleaksAllowlistCovers OIDCStub`
- pre-commit hook smoke test

### Phase 3 (Dependency CVE):
- `TestTrivyBlocksCriticalCVE` (synthetic image with known CVE)
- `TestVulnExceptionsSchema`
- `TestVulnExceptionDeadlineExpiry`

### Phase 4 (SSRF generalization):
- `TestValidateSchemeRejectsFileScheme` (preserved from CACHE-001)
- `TestValidateHostBlocksPrivateIP` (preserved)
- `TestValidateHostBlocksGCPMetadata` (NEW)
- `TestValidateHostBlocksAWSMetadata` (NEW)
- `TestValidateHostBlocksAzureMetadata` (NEW)
- `TestValidateHostCaseInsensitive` (NEW)
- `TestValidateHostSuffixMatch` (NEW)
- `TestValidateRedirectHopCap` (preserved)
- `TestPinnedIPDialerPreventsRebind` (preserved)
- `TestSSRFBlockMetricIncrement` (NEW)
- `TestSSRFBlockAuditLogEntry` (NEW)
- `TestSSRFLatencyOverheadUnder10ms` (NFR-SEC-006)
- + CACHE-001 REQ-CACHE-013 9 tests unchanged passing 검증

### Phase 5 (Security events + Merkle):
- `TestEventInsertWithPrevHash`
- `TestMerkleChainVerification` (1M rows ≤ 30s)
- `TestChainBreakDetection`
- `TestAuditWriteLockOnChainBreak`
- `TestEventMetricCardinalityCap`

### Phase 6 (Secrets resolver):
- `TestEnvResolverReadsOSEnv`
- `TestK8sResolverReadsMountedFile`
- `TestVaultResolverReturnsErrNotImplemented`
- `TestResolverFactoryDispatch`
- `TestNoSecretInLogs` (전 패키지 log fixture review)
- CI grep step `grep -rn "fmt.*\$\(SECRET\|KEY\|TOKEN\|PASSWORD\)"
  internal/` zero match

### Phase 7 (Static analysis):
- gosec baseline scan green
- semgrep baseline scan green
- `TestGosecBlocksMD5ForPassword` (synthetic PR)
- `TestSemgrepBlocksHardcodedJWTSecret` (synthetic PR)

### Phase 8 (TLS + cookies):
- CI grep `tls.VersionTLS1[01]` zero match
- `TestCookieFlagsCompliance` (Secure / HttpOnly / SameSite)
- UI-001 CSP integration test

### Phase 9 (Rate limit):
- `TestRateLimitExceededReturns429`
- `TestRateLimitRetryAfterHeader`
- `TestRateLimitEventEmit`
- `TestRateLimitMetricCardinality`

### Phase 10 (Prompt sanitization):
- `TestSanitizeDetectsIgnorePreviousPattern`
- `TestSanitizeWrapsEvidenceBlock`
- `TestSanitizeEmitsEvent`
- `TestSanitizeAllPatternClasses` (table-driven)
- SYN-002 integration test green
- EVAL-001 baseline ±0.02 검증

### Phase 11 (SLSA + cosign):
- release workflow dry-run
- `cosign verify` against test tag

### Phase 12 (ASVS checklist):
- `TestASVSChecklistAllRowsHaveEvidence`
- `TestASVSPassRateAtLeast80Percent`

### Phase 13 (Docs):
- markdown lint
- SECURITY.md GitHub UI recognition

---

## 5. MX tag plan

기존 코드 refactor 시 @MX 업데이트:

| File | Tag | Action | Reason |
|------|-----|--------|--------|
| `internal/access/ssrf.go::validateScheme` | `@MX:ANCHOR` | Update `@MX:REASON` | Now delegates to `internal/security/ssrf/` — note dual usage |
| `internal/access/dialer.go::pinnedIPDialer` | `@MX:ANCHOR` | Update `@MX:REASON` | Same — extracted to shared package |
| `internal/auth/private_ip.go::isPrivateIP` | `@MX:ANCHOR` | Update or remove (if function merged into shared package) | Refactor consolidation |
| `internal/security/ssrf/ssrf.go::ValidateHost` | `@MX:ANCHOR` (NEW) | Add | High fan_in (called from access + auth + future adapters) |
| `internal/security/secrets/resolver.go::Get` | `@MX:ANCHOR` (NEW) | Add | High fan_in (called from llm + index + auth) |
| `internal/security/events/event.go::Insert` | `@MX:ANCHOR` (NEW) | Add | High fan_in (called from all security event sites) |
| `internal/security/ratelimit/limiter.go::Allow` | `@MX:NOTE` (NEW) | Add | Per-request hot path |
| `internal/security/prompt/sanitize.go::Sanitize` | `@MX:NOTE` (NEW) | Add | SYN-002 integration point |
| `internal/security/events/merkle.go::VerifyChain` | `@MX:WARN` (NEW) + `@MX:REASON` | Add | I/O-heavy, runs nightly; chain break is fail-closed (operational risk) |

---

## 6. Risk-driven sequencing notes

research.md §11 risks와 mitigation phase 매핑:

- R1 (gitleaks FP 폭주) → Phase 2 baseline 측정 + NFR-SEC-003 30%
  cap
- R2 (Trivy CRITICAL 빈번) → Phase 3 vuln-exceptions.yaml lifecycle
- R3 (도구 finding 중복) → Phase 7 dedup report
- R4 (CACHE-001 refactor behavior 변경) → Phase 1 characterization +
  Phase 4 strict PRESERVE
- R5 (AUTH-003 schema migration) → Phase 5 staging 검증 + rollback
  script
- R6 (sanitization false-positive) → Phase 10 A/B 측정
- R7 (SLSA generation time) → Phase 11 budget 측정
- R8 (cosign verify adoption) → Phase 11 runbook + Phase 13 docs
- R9 (rate limit default) → Phase 9 config override + alert-only V1
- R10 (Vault stub silent fail) → Phase 6 ErrNotImplemented + DEPLOY-
  001 schema validation
- R11 (Merkle chain 30s 초과) → Phase 5 incremental verification
- R12 (hostname allowlist 우회) → Phase 4 suffix match + IP cross-
  check
- R13 (Playwright JS-based SSRF) → documented residual; Phase 4 doc
  Chromium proxy option 검토
- R14-R20 → Phase 13 runbook + NFR-SEC-005 quarterly review

---

## 7. Sync-phase deliverables (Phase 15)

- parent repo `README.md`: Security section + SECURITY.md 링크
- `CHANGELOG.md` (parent repo): SPEC-SEC-001 entry under M8
- PR title: `feat(security): implement SPEC-SEC-001 — security
  hardening for V1 release (M8)`
- PR body: links to spec.md, research.md, acceptance.md;
  ASVS L1 checklist link; SLSA + cosign verify example
- Status transition: `approved → implemented` on merge + all Phase 1-14
  exit criteria green
- Notify M9: SPEC-REL-001 owner ("security pass clean" exit criterion
  satisfied); SPEC-DEPLOY-001 owner (Helm chart secrets backend
  schema 사용 가능)

---

## 8. Open factoring decisions deferred to run phase

다음 결정은 plan time이 아닌 run-phase agent가 implementation 시점에
결정:

1. **Vault VaultResolver minimal implementation vs stub-only** —
   stub으로 ship한 후 V1.1에서 minimal HTTP client 추가하는 incremental
   approach 권장. 단 DEPLOY-001 ship 일정에 의존.

2. **AUTH-003 `prev_hash` migration backfill timing** — fresh deploy
   에는 backfill 불필요; 기존 데이터 보유 staging/prod에서는 batch
   migration 필요. 일정 따라 결정.

3. **gitleaks project-specific rules** — baseline 30일 측정 후
   custom rule (usearch JWT format, internal API key 패턴) 추가
   여부 결정. V1 ship 시 기본 rule set만.

4. **Trivy scan target depth** — Dockerfile-only로 시작; final image
   scan은 NFR-SEC-001 timing budget 측정 후 추가 여부 결정.

5. **CSP `strict-dynamic` vs nonce** — UI-001 owner와 협의. V1 권장
   `strict-dynamic` + hash.

6. **K8s + Vault mTLS** — DEPLOY-001 scope; 본 SPEC은 Resolver
   interface만 정의.

7. **Cosign issuer regex pattern** — GitHub org/repo 확정 후
   runbook.md 업데이트.

8. **Rate-limit 기본 60/min 적정성** — Phase 9 implementation 후
   M6 5-user staging에서 측정; tuning은 config override로 처리.

9. **Per-adapter custom SSRF policy** — V1 global blocklist only;
   adapter-specific allowlist는 post-V1 별도 SPEC.

10. **Chromium `--proxy-server=` for Phase 5 JS-SSRF mitigation** —
    technical feasibility 검증 후 V1 또는 V1.1 결정.

이 항목들은 scope-bounded — SPEC contract 변경 없음; mechanical
implementation choice.

---

## 9. Plan-auditor checklist

Plan-auditor PASS 위한 self-check:

- [ ] DDD methodology justification 명시 (§ top + §1)
- [ ] Sprint Contract draft 포함 (§2) — thorough harness REQUIRED
- [ ] Phase ordering risk-burndown principle (§3)
- [ ] 모든 REQ가 phase에 mapped
- [ ] characterization test 전략 (Phase 1, Phase 4) — DDD PRESERVE
- [ ] schema migration 처리 (Phase 5 AUTH-003 amendment)
- [ ] CI/CD integration plan (Phase 2/3/5/7)
- [ ] documentation deliverables (Phase 13)
- [ ] MX tag plan (§5)
- [ ] risk → mitigation phase mapping (§6)
- [ ] open factoring (§8)
- [ ] no time estimates (per agent-common-protocol.md HARD rule)
- [ ] sync phase deliverables (§7)

---

*End of SPEC-SEC-001 plan v0.1.0 (draft).*
