# SPEC-DEPLOY-001 Progress

## 2026-05-31 — Phase 1 (Analysis & Planning, manager-strategy)

Recommendation: **needs-plan-auditor-first**. harness=thorough CONFIRMED (P0 + domain=migration + cross-SPEC). plan-auditor + Sprint Contract MANDATORY. tasks.md written (10 atomic tasks, DDD).

### Topology reality check (spec "13-service" vs actual)

Actual `deploy/docker-compose.yml` = **10 services**, not 13:
qdrant v1.16.3, meilisearch v1.42.1, postgres 16.13-alpine3.23, redis 7-alpine, searxng 2026.04.22-74f1ca203, litellm v1.83.7-stable.patch.1, researcher, embedder, prometheus v2.54.1, tokenizer-ko.

Stale refs:
- **storm, koreanews**: spec/plan template them as compose services (`enabled:false`); they are NOT in compose. `services/storm/Dockerfile` + `services/koreanews/Dockerfile` exist but no compose entry. (OQ1 already flags this — verified true.)
- **usearch-api, usearch-mcp**: NOT compose services — Go binaries run on host (prometheus scrapes `host.docker.internal`). Chart must author their k8s resources from scratch; no compose ground-truth for them.
- spec.md:46 "13 service topology" + frontmatter "13-service" = overcount (counts api/mcp/storm/koreanews as if present). Real chart-from-compose source = 10.
- Image tags in spec §1.1 / plan IMPROVE-5 all MATCH compose exactly (qdrant/meili/pg/redis/searxng/litellm/prometheus). ✓
- 5 Python sidecar Dockerfiles all present (`services/{researcher,embedder,tokenizer-ko,storm,koreanews}/Dockerfile`). ✓ GPU overlay (`docker-compose.gpu.yml`) present, embedder-only nvidia reservation. ✓

### Asset reality

- **Migrations**: 10 files in `deploy/postgres/migrations/` (spec says "9"). Applied by CUSTOM Go runner `internal/index/pg/client.go:87 EnsureSchema` (os.ReadDir, lexicographic, exec ALL *.sql). **NOT golang-migrate.** Files are not golang-migrate-shaped: duplicate version prefixes (two `0002_*`, two `0003_*`), mixed bare-`.sql` vs `.up.sql`, no `schema_migrations` table. → **BLOCKER B1** (spec D4 golang-migrate assumption is false against the actual files).
- **cosign / SLSA / syft release workflow**: NONE present. `.github/workflows/` = compose-check, deps-audit, go, pre-commit(x2), python, web. No signing workflow on main (SEC-001's would be on PR#42).
- **ServiceMonitor / OTLP**: OBS-001 implemented (`internal/obs`, `internal/observability`; `.env.example` has OTLP_ENDPOINT, USEARCH_ADMIN_PORT for /metrics). Chart adds ServiceMonitor + OTLP env refs only. ✓
- **Existing chart**: `charts/universal-search/` exists with ONLY an empty `templates/observability/` stub. Greenfield otherwise.

### SEC-001 forward-ref

`internal/security/` does NOT exist on `main` (SEC-001 entirely on PR#42, unmerged). Chart's 3-tier secret strategy only templates K8s Secrets — does NOT import SEC code — so chart ships decoupled (research §14.1). **B2 coordination flag**: NFR-DEPLOY-008 integration test deferred to post-PR#42 merge. Tier-3 ExternalSecrets (REQ-023) realistically blocked until SEC-001 lands.

### Spec ↔ acceptance contradictions (B3 — plan-auditor must resolve)

- Dockerfile location/name: spec REQ-002/003 = `deploy/Dockerfile.usearch-api`; acceptance AC-002/003 = `Dockerfile.api`/`Dockerfile.embedder` at REPO ROOT.
- Go base image: spec REQ-002 = `golang:1.24.x-alpine`; acceptance AC-002 = `golang:1.23-alpine`.
- Migration hook timing: spec REQ-006 + D4 = `pre-install,pre-upgrade`; acceptance AC-006 = `post-install,post-upgrade`.
- Sidecar Dockerfiles: spec says sidecar Dockerfiles already exist (not re-authored); acceptance AC-003 says SPEC ships `Dockerfile.embedder/tokenizer-ko/storm/koreanews`.

### .env.example gap (OQ3 — verified)

`.env.example` has NO OIDC/JWT/SESSION_SECRET vars at all (confirmed: only MEILI/PG/REDIS/LITELLM/SEARXNG/sidecar vars). spec REQ-008 lists OIDC_CLIENT_SECRET/JWT_SIGNING_KEY/SESSION_SECRET as secrets to template + parity-check. T10 must add these (the parity script will fail until synced).

### Config confirmations

- harness=thorough (harness.yaml: domain=migration + spec_priority==critical → thorough; evaluator per-sprint strict, plan_audit enabled, sprint_contract true).
- quality dev_mode=tdd globally; SPEC frontmatter methodology=ddd (legitimate per-SPEC override — extraction from existing compose).

### Top 3 risks

1. **Migration tool false assumption (B1)** — golang-migrate cannot consume these files; whole migrate Dockerfile/Job (REQ-003/006, T3/T7) blocked until tool decision. Recommend re-using existing Go `EnsureSchema` runner.
2. **compose→Helm fidelity for the REAL 10 services + 2 host binaries** — parity invariant (REQ-024) must be built against actual topology, not the spec's "13". storm/koreanews disabled-by-default but must be buildable if a user enables them (OQ1).
3. **SEC-001 / signing infra on PR#42** — secret tier-3 + any reuse of a SEC-001 signing workflow is unmerged; `<org>` placeholder in ghcr paths unresolved. Coordinate or defer cosign/SLSA.

### Phase 0 status

plan-auditor: **REQUIRED** (thorough harness, P0 release-blocker). Sprint Contract: **REQUIRED**.
Gate before IMPROVE: resolve B1 (migration tool), B3 (spec/acceptance contradictions), confirm B2 (SEC-001 decoupling + `<org>` registry).
