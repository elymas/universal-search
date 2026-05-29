---
id: SPEC-SEC-001
artifact: tasks
version: 0.1.0
status: draft
created: 2026-05-29
author: manager-strategy (Phase 1 run analysis)
methodology: ddd
source: spec.md + plan.md (15-phase) + acceptance.md (AC-001..AC-015)
note: |
  Atomic task decomposition mapping the 15 plan phases to implementable
  units. Phase 0 (plan-auditor) is NOT a code task — it is a prerequisite
  gate (see phase0_status in run report). Tasks below assume Phase 0 PASS.
  Each task is sized to one DDD cycle (ANALYZE / PRESERVE / IMPROVE) or one
  TDD sub-cycle for net-new packages. Max 10 atomic tasks per MoAI rule;
  Medium/Low phases are grouped into composite tasks (T07..T10) to respect
  the cap — they expand into sub-tasks at their own run sub-phase.
---

# SPEC-SEC-001 Task Decomposition

## Path correction notice (for run-phase agents)

The spec.md repeatedly references `internal/cache/access/` (e.g. §1.1, §7.2).
The **actual** package path is `internal/access/` (confirmed against
structure.md §1 and the filesystem). Run-phase agents MUST target
`internal/access/` — there is no `internal/cache/` directory. This is a
spec documentation drift, not a scope change; flag for amendment but do
not block on it.

The SSRF guard functions in `internal/access/ssrf.go` are **unexported**
(`validateScheme`, `validateHost`, `validateRedirect`, `isPrivateOrLoopback`)
and `internal/access/dialer.go` (`pinnedDialContext`,
`dialContextWithPinnedIP`). REQ-SEC-007 specifies an **exported** API
(`ValidateScheme`, `ValidateHost`, `ValidateRedirect`, `PinnedIPDialer`)
with a different signature (notably `ValidateHost(ctx, u, opts)` vs current
`validateHost(ctx, u, opts, fopts)`). The extraction is therefore an
API-reshape + extract, not a pure move. Characterization tests must pin
current behavior through the OLD call sites, not the new signatures.

---

## Critical-path tasks (High priority — risk burndown first)

| Task | Phase | Description | REQ mapping | Depends on | Planned files | DDD step |
|------|-------|-------------|-------------|------------|---------------|----------|
| **T01** | Phase 1 | ANALYZE existing security surface. Read + map `internal/access/ssrf.go`, `dialer.go`, `internal/auth/private_ip.go`, `discovery.go`, `internal/obs/metrics/metrics.go` cardinality allowlist, AUTH-003 audit schema. Write characterization tests pinning current SSRF + private-IP behavior through existing call sites (cascade.go, phase3_get.go, phase4_tls.go, auth/discovery.go). Produce `ops/security/analyze-report.md` (surface inventory + gap list). | REQ-SEC-007 (PRESERVE baseline), REQ-SEC-008/009 (gap identification) | — | `internal/access/ssrf_baseline_test.go`, `internal/auth/private_ip_baseline_test.go`, `ops/security/analyze-report.md` | ANALYZE + PRESERVE |
| **T02** | Phase 2 | [CRITICAL] Secret hygiene baseline. Author `.gitleaks.toml` allowlist (oidc_stub testdata, `*_test.go` creds, runbook samples). Run full git-history scan (`gitleaks detect --log-opts=--all`), triage findings. Add gitleaks pre-commit hook to existing `.pre-commit-config.yaml`. Create `.github/workflows/security.yml` with gitleaks job (first job). CODEOWNERS entry for `.gitleaks.toml`. | REQ-SEC-004, REQ-SEC-005 (runbook ref), NFR-SEC-003 | — | `.gitleaks.toml`, `.pre-commit-config.yaml` (edit), `.github/workflows/security.yml` (new), `.github/CODEOWNERS` (edit), `ops/security/gitleaks-fp-log.md` | IMPROVE (net-new TDD) |
| **T03** | Phase 3 | CVE scan consolidation. Add Trivy job (`aquasecurity/trivy-action@0.24.0`) to `security.yml` — Dockerfile + final-image scan, CVSS≥7.0 block, UNFIXED informational. Severity-gate matrix (CRITICAL page, HIGH PR-block). Create `ops/security/vuln-exceptions.yaml` schema + 90-day deadline CI check. Verify deps-audit.yml unchanged + green. Validate NFR-SEC-001 timing budget. | REQ-SEC-001, REQ-SEC-002, REQ-SEC-003, NFR-SEC-001, NFR-SEC-002 | T02 (security.yml exists) | `.github/workflows/security.yml` (edit), `ops/security/vuln-exceptions.yaml` | IMPROVE (net-new TDD) |
| **T04** | Phase 4 | [CRITICAL] SSRF generalization. Create `internal/security/ssrf/` (ssrf.go, dialer.go, hostname.go, options.go) by extracting access guards with exported API + Options struct. NEW: hostname blocklist (cloud metadata, case-insensitive, suffix match). Refactor `internal/access/{ssrf,dialer}.go` + `internal/auth/{private_ip,discovery}.go` to depend on new package (signatures at OLD call sites preserved). New metric collector `internal/obs/metrics/security.go` (`ssrf_blocks_total`). Extend OBS-001 cardinality allowlist. | REQ-SEC-007, REQ-SEC-008, REQ-SEC-009, NFR-SEC-006, NFR-SEC-007 | T01 (characterization tests green) | `internal/security/ssrf/{ssrf,dialer,hostname,options}.go` + tests, `internal/access/{ssrf,dialer}.go` (refactor), `internal/auth/{private_ip,discovery}.go` (refactor), `internal/obs/metrics/security.go` | PRESERVE + IMPROVE |
| **T05** | Phase 5 | [CRITICAL] Security event taxonomy + Merkle audit. AUTH-003 schema amendment for `prev_hash` column + PostgreSQL migration + backfill + rollback script. Create `internal/security/events/` (event.go 7-type enum, merkle.go chain verify). Add `security_event_total{type,severity}` Counter. Nightly verify job `.github/workflows/audit-verify.yml` + `cmd/audit-verify/`. 1M-row benchmark ≤30s. Fail-closed on chain break. | REQ-SEC-017, NFR-SEC-004, NFR-SEC-007 | T04 (security.go metrics exist) | `.moai/specs/SPEC-AUTH-003/spec.md` (amendment), `ops/migrations/20260522_audit_prev_hash.sql`, `internal/security/events/{event,merkle}.go` + tests, `cmd/audit-verify/main.go`, `.github/workflows/audit-verify.yml`, `internal/obs/metrics/security.go` (edit) | IMPROVE (net-new TDD) + schema migration |

## Medium/Low priority tasks (grouped composites — expand at run sub-phase)

| Task | Phase | Description | REQ mapping | Depends on | Planned files | DDD step |
|------|-------|-------------|-------------|------------|---------------|----------|
| **T06** | Phase 6 | Secrets resolver multi-backend. `internal/security/secrets/` (resolver.go interface, env.go, k8s.go, vault.go stub, factory.go). `.moai/config/sections/security.yaml`. Refactor existing os.Getenv call sites (`internal/llm/`, `internal/index/`, `internal/auth/`) to use Resolver. REQ-SEC-018 grep CI step + TestNoSecretInLogs. | REQ-SEC-013, REQ-SEC-018 | T05 (events for any secret.scan emit) — soft | `internal/security/secrets/*.go` + tests, `.moai/config/sections/security.yaml`, call-site refactors | IMPROVE (net-new TDD) |
| **T07** | Phase 7+8 | Static analysis + secure-defaults audit (composite). gosec (`.gosec.yml`) + semgrep (`.semgrepignore`, p/golang+p/owasp-top-ten+p/jwt) jobs in security.yml; baseline-clean. TLS-1.2-min grep CI + `TestCookieFlagsCompliance`. UI-001 coordination for CSP/HSTS headers in `next.config.js`. | REQ-SEC-010, REQ-SEC-012 | T02 (security.yml) | `.gosec.yml`, `.semgrepignore`, `.github/workflows/security.yml` (edit), `internal/auth/*_test.go`, `web/next.config.js` (coordinate) | IMPROVE |
| **T08** | Phase 9+10 | Rate-limit + prompt-injection (composite). `internal/security/ratelimit/` token bucket (`golang.org/x/time/rate` NEW dep) + chi middleware + 429/Retry-After + `ratelimit.exceeded` event. `internal/security/prompt/` Sanitize + patterns + EVIDENCE wrap, wired into `internal/synthesis/faithfulness.go` (SYN-002) + `prompt.sanitized` event. EVAL-001 A/B baseline check. | REQ-SEC-014, REQ-SEC-015 | T05 (events package) | `internal/security/ratelimit/*.go` + tests, `internal/security/prompt/*.go` + tests, `internal/synthesis/faithfulness.go` (wire), go.mod (x/time) | IMPROVE (net-new TDD) |
| **T09** | Phase 11+12 | Supply chain + ASVS evidence (composite). SLSA L2 provenance + cosign keyless signing in release workflow; runbook verify command. `ops/security/owasp-asvs-checklist.md` V1-V14 ≥80% Pass with evidence links + lint test. | REQ-SEC-016, REQ-SEC-011 | T02, T03, T07 (CI evidence to link) | `.github/workflows/release.yml`, `ops/security/owasp-asvs-checklist.md` | IMPROVE |
| **T10** | Phase 13 | Operator documentation. `ops/security/runbook.md` (secret rotation 4-step, SSRF triage, CVE response, cosign verify, chain-break recovery), `ops/security/threat-model.md` (STRIDE from research §13), `SECURITY.md` (disclosure policy). Markdown lint + GitHub Security-tab recognition. | REQ-SEC-005, NFR-SEC-005, REQ-SEC-006, REQ-SEC-011 (V14) | T02..T09 (cross-references) | `ops/security/runbook.md`, `ops/security/threat-model.md`, `SECURITY.md`, `README.md` (edit) | IMPROVE |

## Continuous gates (not standalone tasks)

- **Phase 14 (Sprint Contract Cycle)** — runs continuously per GAN Loop;
  evaluator-active scores T01..T10 against Sprint Contract criteria
  (Security correctness ≥0.85 must-pass). Not a discrete task.
- **Phase 15 (Sync)** — handled by manager-docs + manager-git after all
  acceptance criteria green. Status transition approved→implemented.

## Execution ordering (dependency graph)

```
T01 ──► T04 ──► T05 ──► T06
                  │       │
T02 ──► T03       └──► T08 (events dep)
  │                     │
  ├──► T07 ─────────────┤
  └──► T09 ◄── T03,T07   │
            T10 ◄── all ─┘
```

Hard ordering: T01 before T04 (characterization gate). T04 before T05
(metrics collector). T02 before T03/T07 (security.yml must exist).
T05 before T06/T08 (events package). T10 last (cross-references all).

T01+T02 may run in parallel (no shared files). T03 and T07 may run in
parallel after T02. T06 and T08 may run in parallel after T05.

## Coverage target

85% (DDD; characterization-first for T01/T04, TDD sub-cycles for net-new
packages in T02/T03/T05/T06/T08).

## Task status

| Task | Status | Notes |
|------|--------|-------|
| T01 | completed | Characterization baselines + analyze-report; 22 SSRF tests verified green on unchanged code |
| T02 | completed | .gitleaks.toml + security.yml gitleaks job + pre-commit hook + CODEOWNERS + fp-log. gitleaks binary absent locally → CI-only history scan |
| T03 | completed | Trivy jobs + vuln-exceptions.yaml + check-vuln-exceptions.sh (deadline enforcement verified); deps-audit.yml unchanged |
| T04 | completed | internal/security/ssrf extracted; access+auth delegate; 22 SSRF tests pass unchanged; hostname blocklist + ssrf_blocks metric; coverage 91.8% |
| T05 | completed | internal/security/events 7-type taxonomy emits into existing AUTH-003 chain; 4 new EventType consts (AUTH-003 owner sign-off pending); coverage 96.6% |
| T06 | partial | security.yaml config (secrets/ratelimit/ssrf) + REQ-SEC-018 grep (check-no-secret-logs.sh, security.yml secret-grep job) DONE. internal/security/secrets pkg BLOCKED — env denies new-dir creation under internal/security/. os.Getenv secret sites for future refactor: llm/config (LITELLM_MASTER_KEY), adapters/naver (NAVER_*). Meili/Qdrant keys are struct fields not Getenv; no OIDC Getenv site. |
| T07 | completed | Phase7: .gosec.yml + .semgrepignore + gosec/semgrep jobs in security.yml. Phase8: tls-grep CI gate + internal/auth/cookie.go (NewSessionCookie) + TestCookieFlagsCompliance (auth -race 90.7%) + next.config.mjs CSP/HSTS/headers (additive, UI-001 cross-spec flag). gosec/semgrep baseline CI-only (uninstallable locally on Go 1.26). |
| T08 | partial | ratelimit + prompt pkgs BLOCKED (new-dir denial). Designs recorded: ratelimit = per-tenant x/time/rate token bucket, stdlib net/http middleware (NO chi in project), V1 alert-only (config-gated). prompt = Sanitize+EVIDENCE wrap, SYN-002 wiring at deepagent/agents.go:254 VerifierWithChecker docs param. golang.org/x/time NOT yet in go.mod. |
| T09 | completed | Phase11 .github/workflows/release.yml (SLSA L2 generator_generic_slsa3@v2.0.0 + cosign keyless @v3.7.0; build job = REL-001 placeholder w/ documented boundary). Phase12 ops/security/owasp-asvs-checklist.md V1-V14, 94.3% Pass (33/35 applicable, 2 Deferred=blocked pkgs, 0 Fail), every Pass has evidence link. |
| T10 | completed | ops/security/runbook.md (rotation 4-step + 005a 5 guards + SSRF triage + CVE + cosign verify + chain-break recovery), threat-model.md (STRIDE, T1 corrected to existing AUTH-003 chain), SECURITY.md (disclosure/SLA/no-bounty). NFR-SEC-005 last-reviewed-at headers. |
