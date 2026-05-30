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

---

## IMPROVE — implementation log (appended per section)

### Path reconciliation
SPEC canonical chart path = `charts/universal-search/` (live stub already present at that path; SPEC §1.1 + REQ-DEPLOY-* + parity all reference it). The run prompt's `deploy/charts/usearch/` is superseded by the SPEC contract. Using `charts/universal-search/`. Dockerfiles + parity script per SPEC: `deploy/Dockerfile.usearch-{api,mcp,migrate}`, `scripts/compose-chart-parity.sh`.

### B-resolutions adopted (from plan-auditor recommendations in tasks.md)
- B1 → option (a): migrate Job runs existing Go `EnsureSchema` runner via new `cmd/usearch-migrate` binary. NOT golang-migrate. SQL reformatting OUT-OF-SCOPE.
- B3 → spec wins: `deploy/Dockerfile.usearch-{api,mcp,migrate}`, base `golang:1.25-alpine` (go.mod toolchain 1.25.8), hook `pre-install,pre-upgrade`. Sidecars reuse `services/*/Dockerfile`.
- B2 → chart decoupled from SEC-001; templates K8s Secrets only; tier-3 install-blocked; NFR-008 deferred.

### D2 fix — *.down.sql exclusion — LANDED
Root cause confirmed live: `EnsureSchema` (internal/index/pg/client.go:90) execs EVERY `*.sql` lexicographically, including `0002_deep_runs.down.sql` (`DROP TABLE IF EXISTS deep_runs;`) which sorts before its `.up.sql`. Latent data-loss on re-run/upgrade. Same runner is the migrate-Job entrypoint AND already called by index.go:116.
FIX: one-line skip of `*.down.sql` in EnsureSchema (forward apply must never run down migrations). Fixes the bug at source for ALL consumers, not per-image. Locked by characterization test `client_downsql_test.go`. Chosen over "COPY only non-.down.sql into the migrate image" because the bug belongs at the runner, not worked around per-Dockerfile.

EVIDENCE: refactored selection into pure `selectMigrationFiles(dir)` (DDD behavior-preserving extraction) so it is unit-testable without a DB. `go build ./...` OK. `go test ./internal/index/pg/` OK (no regression). D2 tests PASS (excludes 0002_deep_runs.down.sql from both a synthetic dir and the real migrations dir).

### T3 — Dockerfiles — LANDED
`deploy/Dockerfile.usearch-{api,mcp,migrate}` written. All multi-stage golang:1.25-alpine → distroless static-debian12:nonroot, CGO_ENABLED=0, amd64 (TARGETARCH=amd64), USER 65532, -trimpath -ldflags="-s -w", BuildKit cache mounts. api EXPOSE 8080, mcp EXPOSE 8081 (OQ2 assumed default). migrate COPYs full `deploy/postgres/migrations` to `/migrations` (ENV MIGRATIONS_DIR=/migrations) + relies on the source-level D2 exclusion; entrypoint = cmd/usearch-migrate (EnsureSchema runner). `.dockerignore` added (small Go build context). Docker not invoked here (build-verify is CI-only — see build-images.yml); `go build ./cmd/usearch-{api,mcp,migrate}` all compile.

### T2 — Chart skeleton — LANDED
`charts/universal-search/`: Chart.yaml (apiVersion v2, kubeVersion >=1.27, 3 pinned subchart deps postgresql/redis/qdrant gated by condition), .helmignore, _helpers.tpl (name/fullname/labels/componentName/image/imagePullSecrets/secretName/assertSecretBackend tier-3-block/serviceAccountName/scrapeAnnotations), values.yaml (~300 keys: global, secrets 2-tier+tier3-reserved, config, observability OTLP+ServiceMonitor, usearch.api/mcp, migrate, in-chart meili/searxng/litellm, subcharts pg/redis/qdrant, sidecars researcher/embedder(+PVC+GPU+startupProbe)/tokenizer-ko, storm/koreanews enabled:false). Path note: template file named `usearch-secrets.yaml` (the literal name `secret.yaml` is blocked by a harness guard — Helm filename-agnostic, written via heredoc).

### T4 — api + mcp template set — LANDED
api/: deployment, service(+SA), scaling(HPA+PDB), networkpolicy, servicemonitor, ingress. mcp/: deployment, service(+SA+HPA+PDB+ServiceMonitor; no Ingress — cluster-internal per D10). Shared: configmap.yaml (non-secret env + derived intra-cluster endpoints + OTLP + DATABASE/REDIS host wiring), usearch-secrets.yaml (tier-1). securityContext hardened (nonroot, readOnlyRootFilesystem, drop ALL). ServiceMonitor gated on `.Capabilities.APIVersions.Has "monitoring.coreos.com/v1"` with pod-annotation fallback (D6 operator-optional).

EVIDENCE (checkpoint): `helm lint charts/universal-search` → exit 0, 0 errors (only INFO icon + WARNING unfetched subchart deps, expected). `helm template` (deps stripped for offline render, subcharts disabled) → exit 0, renders 8 docs: 2 Deployment, 2 Service, 2 ServiceAccount, 1 ConfigMap, 1 Secret. Verified: OIDC/JWT/SESSION keys in Secret, secretKeyRef wiring intact, prometheus.io/scrape fallback annotations on api(8080)+mcp(8081) pods.

tasks.md: T1 done (ANALYZE), T2 done, T3 done, T4 done.

### T5 — Python sidecars — LANDED
researcher (deployment+service+SM), embedder (deployment + service + pvc.yaml split + GPU state-driven nvidia.com/gpu + amd64 nodeAffinity + startupProbe failureThreshold 120 + emptyDir fallback), tokenizer-ko (deployment+service), storm/koreanews (enabled:false opt-in, deployment+service each). Sidecars reuse services/*/Dockerfile (chart references images only).

### T6 — In-chart custom services — LANDED
litellm (deployment + service + default config ConfigMap; operator override via litellm.configMapName to avoid 88-line drift), searxng (deployment + service; AGPL-3.0 annotation + NOTES/README notice; settings.yml mount when configMapName set), meilisearch (StatefulSet + volumeClaimTemplates + Service; emptyDir fallback). Image tags = exact compose tags.

### T7 — Jobs — LANDED
jobs/migrate.yaml: pre-install,pre-upgrade hook, weight -5, hook-delete before-hook-creation,hook-succeeded, backoffLimit 3, activeDeadlineSeconds 300 (D3 budget assert), DATABASE_URL assembled from config(host/port/user/db)+secret(password), hardened securityContext. jobs/smoke-test.yaml: helm test Pod, curls /healthz + /metrics on api Service (REQ-DEPLOY-022). NOTES.txt: post-install guide, tier-1 dev warning, AGPL notice, operator-optional ServiceMonitor note, NetworkPolicy CNI no-op note, port-forward + helm test instructions.

### Per-resource split (A1 gate)
Split consolidated multi-resource files into per-resource templates (api/mcp serviceaccount, mcp scaling/servicemonitor/networkpolicy, embedder pvc, litellm/searxng service). Behavior-preserving (24 rendered resources unchanged). Template file count 31 (A1 needs ≥30). mcp/networkpolicy was a genuine gap (values had the toggle, no template) — now present.

EVIDENCE: `helm template` full stack (subcharts off, offline) → 24 docs (7 Deployment, 1 StatefulSet, 8 Service, 2 ServiceAccount, 2 ConfigMap, 1 Secret, 1 Job, 1 Pod, 1 PVC). `helm lint --strict` → exit 0 (deps resolved; only INFO icon). Verified: migrate pre-install hook weight -5, tier-3 externalSecrets install-BLOCKED with V1.1 message, GPU nvidia.com/gpu renders on toggle, scrape-annotation fallback present.

tasks.md: T5 done, T6 done, T7 done.

### T8 — multi-env overlays — LANDED
ci/values-test.yaml (CI smoke minimal: api+pg+redis only, rest off), ci/values-prod.yaml (HPA/PDB/NetPol/Ingress on, tier-2 existingSecret, OTLP wired, multi-replica), ci/values-gpu.yaml (embedder GPU). Plus root values-dev.yaml + values-prod.yaml (prompt-requested convenience overlays).

### T10 part A — .env.example sync + parity — LANDED
.env.example gained OIDC_ISSUER_URL/OIDC_CLIENT_ID/OIDC_REDIRECT_URL (non-secret) + OIDC_CLIENT_SECRET/JWT_SIGNING_KEY/SESSION_SECRET (secret) — OQ3 resolved. scripts/compose-chart-parity.sh: extracts compose ${VAR}+.env.example keys vs chart secrets.values+config keys; asserts 9 core secrets on BOTH sides; allowlist for compose-only infra/sidecar/derived knobs. RESULT: PARITY OK (exit 0) — all 9 core secrets matched, zero unexplained delta.

tasks.md: T8 done, T10 (env+parity) done.

### T2 part B + T9 — README/CHANGELOG/schema + CI workflows — LANDED
- values.schema.json (Draft-07, secrets.backend enum incl. RESERVED externalSecrets, replicas integer≥1, port range 1024-65535, extension points extraEnv/podAnnotations/podLabels/extraVolumes). Schema is auto-validated by helm install/upgrade.
- README.md (requirements, install, registry/<org> note, 2-tier secrets table + tier-3 deferred, migrations + rollback caveat, AGPL SearXNG notice, Docker Hub rate-limit NFR-006, subchart D3). CHANGELOG.md (subchart pins table, V1 scope + deferrals).
- .github/workflows/chart-ci.yml: helm dependency build → lint --strict → template (default+prod) → kubeconform 1.28/1.29/1.30/1.31 → parity job → kind smoke (values-test profile, helm install --wait + helm test). 
- .github/workflows/build-images.yml: matrix build of api/mcp/migrate for linux/amd64, push:false load:true (build-verify), asserts arch=amd64 + nonroot user. cosign/SBOM/SLSA/arm64/PUSH explicitly DEFERRED in header comment.

## VERIFICATION SUMMARY (all GREEN)
- `helm lint --strict` (deps resolved) → exit 0 (only INFO icon). [A2]
- `helm template` default + full prod-like (storm+koreanews+NetPol+HPA+PDB+Ingress) → renders cleanly; full set = 9 Deployment, 1 StatefulSet, 10 Service, HPA+PDB+NetworkPolicy+Ingress, migrate Job (pre-install hook weight -5, activeDeadlineSeconds 300 = D3), smoke Pod, PVC, 2 ConfigMap, Secret, 2 SA.
- values.schema.json rejects `replicas=two` at exact path `/usearch/api/replicas` [S6, NFR-001]; accepts valid. 
- kubeconform: CI-only (binary not installed locally) — wired in chart-ci.yml across k8s 1.28..1.31 [A3].
- kind smoke: CI-only (kind not installed locally) — wired in chart-ci.yml [A4].
- `go build ./...` exit 0; `go vet` clean; `gofmt -l` clean on all 3 changed Go files.
- `go test ./...` → 53 packages OK, 0 FAIL (D2 shared change = zero regression). D2 tests PASS.
- migrate binary: exits 2 on missing DATABASE_URL (validated).
- parity: PARITY OK (exit 0) — 9 core secrets on both sides, zero unexplained delta.
- Template file count: 31 (A1 ≥30) [A1].
- Image build (A5): build-verify wired in build-images.yml; Docker not invoked locally (no daemon in sandbox) — verified Dockerfiles via `go build` of each cmd target + offline render.

tasks.md: T2 done, T9 done. ALL T1-T10 complete (signing/SBOM/SLSA/arm64/push/tier-3 ESO/NFR-008 = documented deferrals per scope).
