# SPEC-DEPLOY-001 Plan — phased implementation

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: **DDD** (ANALYZE-PRESERVE-IMPROVE per `.claude/rules/
moai/workflow/workflow-modes.md`). DDD-mode justification: 본 SPEC은
**기존 dev-compose surface (`deploy/docker-compose.yml` 265 lines +
GPU overlay 19 lines, 9 SQL migrations, 5 Python sidecar Dockerfiles)
를 parameterized Helm chart로 추출 + 운영 hardening 추가**하는 작업이
본질이며, 신규 deploy 패러다임 발명이 아니다. ANALYZE 단계에서 현
deploy surface inventory를 정확히 capture; PRESERVE 단계에서 dev workflow
(`make compose-up`) 동일 동작 유지 + compose ↔ chart parity invariant
test로 behavior 동일성 보장; IMPROVE 단계에서 chart-ization +
Dockerfile authoring + multi-arch + signing + observability wiring +
3-tier secret strategy 적용. 신규 코드 (Dockerfile, chart template,
CI workflow, helper script)는 모두 TDD 하위 cycle로 실행 (template
unit test = helm-unittest; CI script characterization test).

Coverage target: 85% (per spec.md frontmatter)
Harness: **thorough** (per `.moai/config/sections/harness.yaml` —
**P0 + release-blocking + cross-SPEC integration**은 thorough 강제;
Sprint Contract MANDATORY per `.claude/rules/moai/design/constitution.
md` §11 "Sprint Contracts are required when harness level is
`thorough`")

본 plan은 SPEC-DEPLOY-001 구현을 priority-ordered phases로 sequence
한다. `.claude/rules/moai/core/agent-common-protocol.md` 시간 예측
금지 — phase는 priority + ordering만 사용.

---

## 1. Implementation principle

본 SPEC의 plan philosophy 5축:

1. **Compose-fidelity first** — 본 chart는 dev-compose의 13-service
   topology를 **byte-fidelity equivalent**로 표현해야 한다. compose
   ↔ chart parity smoke-test (REQ-DEPLOY-024)가 매 CI에서 강제. chart
   가 compose에 없는 env-var를 추가하면 (또는 vice versa) build fail.
2. **DDD characterization-first** — Dockerfile / migration job / chart
   template 모두 dev-compose 동작을 reproduce하는 characterization test
   를 먼저 작성 (PRESERVE). chart-rendered manifest가 compose service
   와 동일한 환경 변수 + 동일한 image tag + 동일한 volume mount + 동일한
   port 노출하는지 verify.
3. **Multi-stage IMPROVE** — chart-only 변경 → image authoring → signing
   pipeline → CI gate 순으로 incremental improve. 각 stage 자체가
   independent PR로 ship 가능 (release-blocking이지만 incremental
   ship으로 risk 분산).
4. **3-tier secret roll-out** — V1.0.0 ship 시점 권장 default tier 2
   (existingSecret); tier 1 (values)은 dev/CI 전용, NOTES.txt 경고;
   tier 3 (ExternalSecrets)은 ESO pre-install 운영자 한정 opt-in.
   tier 3은 SEC-001 implementation 후 default-recommended로 promote
   (post-V1 chart minor version bump).
5. **Cross-SPEC integration deferral acknowledgment** — SEC-001
   implementation 지연 시 본 SPEC은 K8s Secret refs까지만 ship; SEC-001
   ship 후 integration verification (NFR-DEPLOY-008) 별도 PR로 close-
   out. 본 SPEC main acceptance는 SEC-001 implementation에 의존하지
   않음 (decoupled per research §9.4).

---

## 2. Sprint Contract (REQUIRED per thorough harness)

Sprint Contract는 builder (manager-ddd) ↔ evaluator-active 사이
협상 결과로 매 GAN Loop iteration 시작 전 작성. 본 SPEC의 V1 Sprint
Contract draft (run phase에서 evaluator-active와 finalize):

### Acceptance checklist (testable per iteration)

- [ ] `charts/universal-search/Chart.yaml` apiVersion v2 + 3 subchart
      dependencies (postgresql, redis, qdrant) pinned exact patch
      versions
- [ ] `charts/universal-search/values.yaml` ≥ 300 keys covering all
      services + observability + security + ingress sections
- [ ] `charts/universal-search/values.schema.json` strict mode
      (additionalProperties: false) with documented forward-compat
      extension points
- [ ] `templates/_helpers.tpl` 공통 helper (fullname, labels, image,
      secret-resolver) — 모든 sub-directory template이 helper 활용
- [ ] `templates/{api,mcp}/` 완전 (Deployment + Service + ConfigMap +
      Secret + HPA + PDB + NetworkPolicy + ServiceMonitor + Ingress +
      ServiceAccount) — 9 resource type 각각의 template
- [ ] `templates/{researcher,embedder,tokenizer-ko,storm,koreanews}/`
      sidecar 5개 (각각 Deployment + Service; embedder 추가 PVC)
- [ ] `templates/{litellm,searxng}/` in-chart custom (Deployment +
      Service + ConfigMap)
- [ ] `templates/jobs/migrate.yaml` pre-install Helm hook + golang-
      migrate v4.18 + 9 SQL files
- [ ] `templates/jobs/smoke-test.yaml` helm test hook (`/healthz` +
      `/metrics` curl)
- [ ] `deploy/Dockerfile.usearch-api` multi-stage + multi-arch +
      distroless + non-root USER
- [ ] `deploy/Dockerfile.usearch-mcp` 동상
- [ ] `deploy/Dockerfile.usearch-migrate` distroless + golang-migrate
      + 9 SQL files COPY
- [ ] `.github/workflows/build-images.yml` 7 image × multi-arch +
      cosign + SBOM + SLSA L2 (build-time + run-time 모두 CI에서 검증)
- [ ] `.github/workflows/chart-ci.yml` helm lint + helm template +
      kubeconform 1.28..1.31 + kind smoke-test + parity script
- [ ] `.github/workflows/chart-release.yml` chart package + cosign +
      OCI push on tag
- [ ] `scripts/compose-chart-parity.sh` compose vs chart env-var diff
      검증
- [ ] `ci/values-test.yaml` minimal smoke profile
- [ ] `ci/values-prod.yaml` production reference
- [ ] `ci/values-gpu.yaml` embedder GPU overlay
- [ ] `README.md` install + upgrade + uninstall + troubleshoot (DOC-
      001 cross-link)
- [ ] `NOTES.txt` post-install guidance (port-forward, OIDC URI, docs
      link)
- [ ] `CHANGELOG.md` chart-specific changelog
- [ ] Schema validation rejects all 12 test scenarios in spec.md §5
      where expected (S6 invalid values fail-fast)
- [ ] kind cluster smoke install completes within 5min (NFR-DEPLOY-003)
- [ ] Multi-arch image manifest list verified for amd64+arm64 (embedder
      amd64-only acknowledged)
- [ ] cosign verify PASS for every signed artifact

### Priority dimension

**Functionality + Operability** (Functionality + Craft from evaluator-
active 4-dimension scoring). Originality is N/A — 본 SPEC은 industry-
standard chart 패턴 적용; novelty 없음. Completeness는 24 EARS REQ +
8 NFR + 13 acceptance gate coverage로 측정. Security 차원은 SEC-001
domain; 본 SPEC은 secret backend abstraction + signing pipeline까지만.

### Test scenarios (verification per spec.md §5)

- S1 `helm lint` PASS (REQ-DEPLOY-001, -020)
- S2 `helm template` against ci/values-prod.yaml (REQ-DEPLOY-005, -008,
  -016)
- S3 `kubeconform` k8s 1.28..1.31 (REQ-DEPLOY-020)
- S4 kind cluster smoke install (REQ-DEPLOY-006, -020, NFR-DEPLOY-003)
- S5 `helm test` smoke (REQ-DEPLOY-022)
- S6 Schema rejects invalid values (REQ-DEPLOY-005, NFR-DEPLOY-001)
- S7 Migration Job idempotency (REQ-DEPLOY-006, research §12.5)
- S8 Helm rollback (NFR-DEPLOY-004)
- S9 Secret backend tier switch (REQ-DEPLOY-016)
- S10 NetworkPolicy enforcement (REQ-DEPLOY-010)
- S11 Multi-arch image manifest (REQ-DEPLOY-018, NFR-DEPLOY-007)
- S12 Cosign signature verification (REQ-DEPLOY-018, D7)

### Pass conditions (minimum score per criterion)

- Functionality: ≥ 0.85 (must-pass; chart는 deployable + helm test
  PASS 가능해야 함)
- Operability: ≥ 0.80 (DOC-001 cross-link + NOTES.txt 운영자 가이드
  품질)
- Craft: ≥ 0.75 (template DRY + helper 활용 + values.schema 적합성)
- Consistency: ≥ 0.80 (compose-chart parity invariant + 기존 SPEC
  pattern 준수)

---

## 3. DDD Phase ANALYZE (Phase 3.1)

**Purpose**: 현 deploy surface를 정확히 capture하고 chart 작업 전
ground truth로 fix. research.md §1..§5가 이 단계의 산출물 — 본 plan
phase는 research가 SPEC-spec.md REQ에 빠짐없이 반영되었는지 verify.

### 3.1.1 Inventory verification tasks

| Task ID | Task | Verification |
|---------|------|--------------|
| A1 | dev-compose 13-service 명세 vs spec.md §1.1 What ships 매핑 | research §1 표 ↔ spec §1.1 표 사이 service 누락 0건 |
| A2 | 9 SQL migration 파일 vs migration Job spec | research §2 표 ↔ migrate.yaml `COPY deploy/postgres/migrations/` 일치 |
| A3 | `.env.example` env-var ~50 vs ConfigMap + Secret spec | research §5 카테고리별 표 ↔ spec REQ-DEPLOY-007 (ConfigMap) + REQ-DEPLOY-008 (Secret) cross-ref |
| A4 | cmd/{usearch-api,usearch-mcp,usearch} binary inventory | research §3 표 ↔ Dockerfile target (usearch CLI 제외 확인) |
| A5 | services/ 5 Python sidecar Dockerfile 존재 확인 | research §4 표 ↔ chart templates/<sidecar>/ 생성 매핑 |
| A6 | SPEC dependency status — implemented vs draft 분류 | spec.md HISTORY ↔ git log 최신 commit 확인 (SEC-001, DOC-001/002는 draft; 나머지 implemented) |

ANALYZE 단계 산출물:
- `research.md` (이미 작성 완료)
- spec.md §1.1 표가 research §1.1..§1.6 inventory와 1:1 매핑됨을
  사후 verify (plan-auditor 점검 항목)

### 3.1.2 SEC-001 dependency surface inventory

SEC-001 D5 (secrets) + D9 (security events)와 본 chart 통합 surface:

| SEC-001 surface | chart-side counterpart | dependency state |
|----------------|----------------------|------------------|
| `internal/security/secrets/Resolver` interface | chart는 K8s Secret resource까지만; binary가 Resolver로 읽음 | SEC-001 draft, chart decoupled (research §14.1) |
| `internal/security/events/` 7 event types | chart는 binary `/metrics` ServiceMonitor scrape | OBS-001 implemented; chart 추가 작업 없음 |
| `.github/workflows/security.yml` (SEC-001 신규) | chart-ci.yml은 별도 workflow | SEC-001 draft, 별도 workflow 분리 (충돌 없음) |
| `ops/security/runbook.md` | chart README가 DOC-001 cross-link | DOC-001 draft, 본 SPEC ship 시점 cross-link as planned |

### 3.1.3 BOOT-001 / CORE-001 / OBS-001 / AUTH-001/002/003 / CACHE-001
/ IDX-001..005 implementation status confirmation

각 dependency SPEC의 implemented status를 run phase 시작 전 재확인.
roadmap.md §M1..§M7 status memo:
- M1 BOOT/DEP/OBS/LLM — implemented (M1 close commit hash 확인)
- M2 IR/ADP-001/ADP-002/SYN-001/CLI-001 — implemented
- M3 FAN-001/ADP-003..009/IDX-001/002/003/CACHE-001 — implemented
- M4 SYN-002/003/004 — implemented (commit "TDD parallel" memo)
- M5 DEEP-001/002/003/004 — implemented per roadmap M5 status update
- M6 AUTH-001/002/003/IDX-004/005 — implemented
- M7 MCP-001/CLI-002/SKILL-001/UI-001/UI-002 — drafted (M7 partial)

본 SPEC dependency 중 **drafted only**인 SPEC (MCP-001, SEC-001, DOC-
001, DOC-002):
- MCP-001 → chart `usearch.mcp.enabled: false` default. implemented
  되면 enabled 전환 권장으로 DOC-001 업데이트.
- SEC-001 → decoupled (research §14.1)
- DOC-001/002 → chart README + NOTES.txt에서 cross-link; 두 SPEC ship
  된 후 hyperlink 활성화

---

## 4. DDD Phase PRESERVE (Phase 3.2)

**Purpose**: 기존 dev workflow (`make compose-up`) 동작이 chart-ization
이후에도 byte-fidelity로 작동함을 보장. characterization test 작성.

### 4.1 Characterization test set

#### CT1 — compose-up smoke test (existing)

`make compose-up` → 모든 service Ready → `make compose-down`. 본
SPEC 시작 전 기존 BOOT-001 CI에서 매 PR마다 실행 중. PRESERVE
phase가 CT1를 break하지 않음을 verify (regression test).

#### CT2 — `.env.example` ↔ compose ↔ chart parity invariant

`scripts/compose-chart-parity.sh` (new) — 3-way diff:
- `deploy/docker-compose.yml`에 reference된 모든 `${VAR}` extract
- `.env.example`에 정의된 모든 KEY=VALUE extract
- `charts/universal-search/values.yaml` + `values.schema.json`에
  reference된 모든 env-var path extract
- 세 set의 symmetric difference에 documented allowlist 이외 항목이
  있으면 fail.

#### CT3 — Postgres migration replay test

기존 dev workflow에서 `make migrate-up` (또는 동등 명령)이 9 SQL을
순차 적용; PRESERVE에서 동일 sequence가 idempotent함을 verify
(두 번 실행해도 schema 동일). Chart의 migration Job container가
동일한 SQL을 동일한 순서로 적용함을 cross-check.

#### CT4 — Health endpoint contract

기존 binary의 `/healthz` + `/metrics` HTTP contract가 변경되지 않음을
verify. usearch-api는 dev에서 `go run ./cmd/usearch-api`로 실행 시
`/healthz` 200 응답; chart-rendered Pod에서도 동일.

#### CT5 — adapter API contract preservation

SPEC-ADP-001..009의 contract test가 dev (compose) 환경과 k8s
(chart) 환경에서 동일 결과 — adapter contract test는 binary 내부
test이므로 deploy surface 변경에 영향받지 않음을 verify (negative
test).

### 4.2 PRESERVE phase exit gate

- CT1..CT5 모두 PASS
- spec.md §1.1 What ships의 dev-compose source artifact 13 service
  중 chart에서 대응 template이 생성되지 않은 service 0건 (또는
  documented exclusion으로 chart에 일부러 미포함 — 예: prometheus를
  외부 의존으로 가정)
- chart manifest rendering이 compose-spec과 동등 service set 노출

---

## 5. DDD Phase IMPROVE (Phase 3.3, file-by-file)

**Purpose**: chart authoring + Dockerfile + CI workflow + helper script
의 incremental ship. file-by-file으로 분해하여 `[HARD] Multi-File
Decomposition` 준수 (Section 7 Rule 2). 각 sub-phase는 independent
PR로 ship 가능.

### 5.1 Sub-phase IMPROVE-1 — Chart skeleton + helpers

**Files**:
- `charts/universal-search/Chart.yaml` (NEW)
- `charts/universal-search/values.yaml` (NEW, ~300 keys)
- `charts/universal-search/values.schema.json` (NEW)
- `charts/universal-search/templates/_helpers.tpl` (NEW)
- `charts/universal-search/templates/NOTES.txt` (NEW)
- `charts/universal-search/README.md` (NEW)
- `charts/universal-search/CHANGELOG.md` (NEW)
- `charts/universal-search/.helmignore` (NEW)

**Verification**:
- `helm lint charts/universal-search` exits 0
- `helm template universal-search charts/universal-search` produces
  non-empty output with zero error
- values.schema.json validates against `values.yaml` (chart's own
  defaults must satisfy schema)

**REQs satisfied**: REQ-DEPLOY-001 (chart structure), REQ-DEPLOY-004
(sub-directory ground), REQ-DEPLOY-005 (schema), REQ-DEPLOY-014
(subchart deps), REQ-DEPLOY-016 (secret backend switch — declared
in helpers).

### 5.2 Sub-phase IMPROVE-2 — Dockerfiles (3 new)

**Files**:
- `deploy/Dockerfile.usearch-api` (NEW)
- `deploy/Dockerfile.usearch-mcp` (NEW)
- `deploy/Dockerfile.usearch-migrate` (NEW)
- `deploy/.dockerignore` (NEW or amend)

**Verification**:
- `docker buildx build --platform linux/amd64,linux/arm64 -f
  deploy/Dockerfile.usearch-api -t usearch-api:test .` succeeds for
  both architectures
- 동상 for usearch-mcp + usearch-migrate
- 결과 image가 non-root USER로 실행 + `/healthz` 응답 (api/mcp;
  migrate는 ENTRYPOINT가 migrate 명령이므로 별도 verify)
- image size < 100MB (distroless static-debian12 + Go binary)

**REQs satisfied**: REQ-DEPLOY-002 (api Dockerfile), REQ-DEPLOY-003
(mcp + migrate Dockerfile), NFR-DEPLOY-007 (multi-arch).

### 5.3 Sub-phase IMPROVE-3 — api + mcp template set

**Files**:
- `charts/universal-search/templates/api/deployment.yaml`
- `charts/universal-search/templates/api/service.yaml`
- `charts/universal-search/templates/api/serviceaccount.yaml`
- `charts/universal-search/templates/api/hpa.yaml`
- `charts/universal-search/templates/api/pdb.yaml`
- `charts/universal-search/templates/api/networkpolicy.yaml`
- `charts/universal-search/templates/api/configmap.yaml`
- `charts/universal-search/templates/api/secret.yaml`
- `charts/universal-search/templates/api/servicemonitor.yaml`
- `charts/universal-search/templates/api/ingress.yaml`
- `charts/universal-search/templates/api/externalsecret.yaml` (tier 3)
- `charts/universal-search/templates/mcp/deployment.yaml`
- `charts/universal-search/templates/mcp/service.yaml`
- `charts/universal-search/templates/mcp/serviceaccount.yaml`
- `charts/universal-search/templates/mcp/hpa.yaml`
- `charts/universal-search/templates/mcp/pdb.yaml`
- `charts/universal-search/templates/mcp/networkpolicy.yaml`
- `charts/universal-search/templates/mcp/configmap.yaml`
- `charts/universal-search/templates/mcp/secret.yaml`
- `charts/universal-search/templates/mcp/servicemonitor.yaml`
- `charts/universal-search/templates/mcp/externalsecret.yaml` (tier 3)

**Verification**:
- helm-unittest cases per template:
  - default values renders all enabled resources
  - `enabled: false` toggle omits the Deployment
  - `secrets.backend` 3 modes each produce expected resource set
  - `networkPolicy.enabled: false` omits NetworkPolicy
  - `hpa.enabled: false` omits HPA
  - `pdb.enabled: false` omits PDB
  - `ingress.enabled: true` produces Ingress with cert-manager
    annotation
- `helm template` + kubeconform PASS

**REQs satisfied**: REQ-DEPLOY-009 (per-service resources), REQ-DEPLOY-
010 (NetworkPolicy state-driven), REQ-DEPLOY-013 (probes), REQ-DEPLOY-
016 (secret backend tier expansion), REQ-DEPLOY-019 (ServiceMonitor),
REQ-DEPLOY-021 (Ingress optional), REQ-DEPLOY-023 (ExternalSecret
P2).

### 5.4 Sub-phase IMPROVE-4 — Python sidecar templates (5)

**Files**:
- `templates/researcher/{deployment,service,configmap}.yaml`
- `templates/embedder/{deployment,service,configmap,pvc}.yaml`
- `templates/tokenizer-ko/{deployment,service,configmap}.yaml`
- `templates/storm/{deployment,service,configmap}.yaml`
  (default disabled)
- `templates/koreanews/{deployment,service,configmap}.yaml`
  (default disabled — ADP-009 design 확인 후 DaemonSet/CronJob 가능성)

**Verification**:
- `embedder.gpu.enabled: true` 시 `resources.limits."nvidia.com/gpu"`
  + nodeSelector + tolerations 출현
- `embedder.gpu.enabled: false` 시 amd64 nodeAffinity만 (NFR-DEPLOY-
  007)
- PVC가 `embedder.persistence.size` 값으로 생성
- 5 sidecar 모두 ServiceMonitor 생성 (observability.serviceMonitor.
  enabled: true 시)
- `enabled: false` 토글이 모든 resource 생략

**REQs satisfied**: REQ-DEPLOY-011 (sidecar enabled toggle), REQ-
DEPLOY-012 (GPU state-driven), REQ-DEPLOY-013 (embedder startupProbe).

### 5.5 Sub-phase IMPROVE-5 — In-chart custom services (litellm,
searxng, meilisearch)

**Files**:
- `templates/litellm/deployment.yaml`
- `templates/litellm/service.yaml`
- `templates/litellm/configmap.yaml` (inline `/app/config.yaml`)
- `templates/searxng/deployment.yaml`
- `templates/searxng/service.yaml`
- `templates/searxng/configmap.yaml` (inline `/etc/searxng/settings.
  yml`)
- `templates/meilisearch/statefulset.yaml`
  (in-chart, not subchart per D3)
- `templates/meilisearch/service.yaml`
- `templates/meilisearch/pvc.yaml` (via statefulset volumeClaimTemplates)

**Verification**:
- searxng AGPL warning emitted in NOTES.txt + README
- searxng image tag matches dev-compose `searxng/searxng:
  2026.04.22-74f1ca203` + digest pin
- litellm config.yaml content rendered correctly from values
- meilisearch StatefulSet rolling update strategy verified

**REQs satisfied**: REQ-DEPLOY-015 (in-chart litellm + searxng +
meilisearch).

### 5.6 Sub-phase IMPROVE-6 — Migration + smoke-test Jobs

**Files**:
- `templates/jobs/migrate.yaml` (pre-install/pre-upgrade hook)
- `templates/jobs/smoke-test.yaml` (helm test hook)
- `scripts/build-migrate-image.sh` (NEW, local dev helper)

**Verification**:
- migration Job runs once per `helm install` / `helm upgrade`
- `helm.sh/hook-weight: "-5"` ordering verified (migration completes
  before app Deployments start)
- helm test invocation returns success on healthy cluster
- migration Job container image pulled successfully + entrypoint
  works against test PG instance

**REQs satisfied**: REQ-DEPLOY-006 (migration hook), REQ-DEPLOY-022
(helm test).

### 5.7 Sub-phase IMPROVE-7 — Multi-environment values overlays

**Files**:
- `charts/universal-search/ci/values-test.yaml` (NEW, minimal smoke)
- `charts/universal-search/ci/values-prod.yaml` (NEW, production ref)
- `charts/universal-search/ci/values-gpu.yaml` (NEW, embedder.gpu
  override)
- `charts/universal-search/ci/values-external-pg.yaml` (NEW, external
  postgres example)
- `charts/universal-search/ci/values-eso.yaml` (NEW, tier 3 secret
  example)

**Verification**:
- 각 values overlay에 대해 `helm template` 성공
- production overlay에서 HPA/PDB/NetworkPolicy 활성 + secret tier 2
- gpu overlay에서 embedder.gpu.enabled + GPU node tolerations
- eso overlay에서 ExternalSecret CRD 출현

**REQs satisfied**: REQ-DEPLOY-005 (schema validation), D8
(multi-env layering).

### 5.8 Sub-phase IMPROVE-8 — CI workflows (3 new)

**Files**:
- `.github/workflows/build-images.yml` (NEW)
- `.github/workflows/chart-ci.yml` (NEW)
- `.github/workflows/chart-release.yml` (NEW)
- `scripts/compose-chart-parity.sh` (NEW)
- `scripts/cosign-verify.sh` (NEW, end-user verify helper)
- `scripts/release-chart.sh` (NEW, local dev helper)

**Verification**:
- build-images.yml: triggered by main merge + tag; 7 image × multi-
  arch build + cosign + SBOM + SLSA all green
- chart-ci.yml: triggered by PR; helm lint + helm template +
  kubeconform 1.28..1.31 + kind smoke-test all green within hosted
  runner resource limits
- chart-release.yml: triggered by `v*.*.*` tag; chart package +
  cosign sign-blob + helm push to oci://ghcr.io
- parity script catches injected env-var drift
- cosign-verify.sh shipped as user-facing helper (DOC-001 cross-
  link)

**REQs satisfied**: REQ-DEPLOY-017 (chart OCI publish), REQ-DEPLOY-018
(image build + signing), REQ-DEPLOY-020 (chart CI), REQ-DEPLOY-024
(compose-chart parity).

### 5.9 Sub-phase IMPROVE-9 — DOC cross-link finalization

**Files** (편집만, 신규 페이지 작성은 DOC-001 owner):
- `charts/universal-search/README.md` (cross-link to `docs/content/
  {en,ko}/operators/deployment-helm.mdx`)
- `charts/universal-search/templates/NOTES.txt` (post-install message
  with docs URL + cosign verify command)
- `charts/universal-search/CHANGELOG.md` (initial 0.1.0 entry)

**Verification**:
- README + NOTES.txt manual review by manager-docs
- DOC-001 페이지 stub 존재 확인 (DOC-001 owner와 협의)
- DOC-002 페이지 stub 존재 확인 (DOC-002 owner와 협의)

**REQs satisfied**: A10 acceptance gate.

### 5.10 Sub-phase IMPROVE-10 — `.env.example` sync (BOOT-001 retroactive)

**Files**:
- `.env.example` (amend — OIDC env-var 추가; OQ3 close-out)

**Verification**:
- `.env.example` 에 OIDC_ISSUER_URL, OIDC_CLIENT_ID, OIDC_CLIENT_SECRET,
  OIDC_REDIRECT_URL, OIDC_SCOPES, JWT_SIGNING_KEY 또는 JWT_PUBLIC_KEY_
  URL, SESSION_SECRET 명세 추가
- compose-chart-parity.sh가 추가된 변수까지 verify

**REQs satisfied**: OQ3 close-out + REQ-DEPLOY-007/008 (env-var
완전성).

---

## 6. Phased schedule (priority-ordered, no time estimates)

Phase ordering reflects priority + dependency. independent sub-phases는
parallel 가능; dependent는 sequential.

### Priority High (release-blocking foundation)

- **Phase 6.1** — Sub-phase 5.1 (chart skeleton)
- **Phase 6.2** — Sub-phase 5.2 (Dockerfiles) — parallel with 6.1
- **Phase 6.3** — Sub-phase 5.10 (.env.example sync) — parallel with
  6.1, 6.2
- **Phase 6.4** — Sub-phase 5.3 (api + mcp templates) — after 6.1
- **Phase 6.5** — Sub-phase 5.6 (migration + smoke-test Jobs) —
  after 6.2 (migration image dependency)

### Priority Medium (release-blocking expansion)

- **Phase 6.6** — Sub-phase 5.4 (Python sidecar templates) — after
  6.4 (helper reuse)
- **Phase 6.7** — Sub-phase 5.5 (in-chart litellm/searxng/meilisearch)
  — after 6.4 (helper reuse)
- **Phase 6.8** — Sub-phase 5.7 (multi-env values overlays) — after
  6.4..6.7 (all template ready)

### Priority High (release-blocking gate)

- **Phase 6.9** — Sub-phase 5.8 (CI workflows) — after 6.2 (build-
  images depends on Dockerfile), 6.1..6.7 (chart-ci depends on chart
  artifact ready)
- **Phase 6.10** — Sub-phase 5.9 (DOC cross-link) — after 6.1, 6.8

### Cross-SPEC integration (post-both-SPEC ship)

- **Phase 6.11** — NFR-DEPLOY-008 cross-SPEC verification (post-SEC-
  001 ship). 본 SPEC main acceptance에 포함되지 않음 — 별도 close-out
  PR.

---

## 7. Risk register (operational + technical)

risk 항목은 spec.md §7 표 R1..R12와 1:1 매핑. plan 단계에서 risk 별
mitigation을 phase로 wiring:

| Risk ID | Phase that addresses | Mitigation summary |
|---------|---------------------|--------------------|
| R1 subchart drift | 6.1 (Chart.lock pinning) + 6.9 (CI checks subchart upgrade) | exact patch pin + quarterly audit |
| R2 secret rotation race | 6.9 (chart-ci NOTES.txt verify) + DOC-001 cross-link | rolling restart procedure + ESO tier 3 |
| R3 migration non-idempotent SQL | 6.5 (PRESERVE phase audit) | grep CREATE TABLE IF NOT EXISTS + characterization test |
| R4 Docker Hub rate-limit | 6.10 (README documentation) | imagePullSecrets guidance |
| R5 embedder amd64-only | 6.6 (nodeAffinity enforcement) + 6.10 (docs) | NFR-DEPLOY-007 acknowledgment |
| R6 SEC-001 unimplemented | 6.11 (cross-SPEC integration deferred) | chart-SEC decoupled |
| R7 kind cluster smoke resource | 6.7 (ci/values-test.yaml minimal) | minimal profile + self-hosted runner deferred |
| R8 cert-manager pre-install | 6.10 (README documented requirement) | pre-install docs + recoverable failure mode |
| R9 NetworkPolicy CNI absence | 6.10 (NOTES.txt warning) | docs warning + CI smoke-test on kind with CNI |
| R10 cosign helm tooling churn | 6.9 (build-images.yml pinning) | cosign v2.4.0 pin + fallback documented |
| R11 Bitnami chart maintainer | 6.1 (Chart.yaml dependency declaration) + 6.10 (CHANGELOG) | external mode always available + quarterly audit |
| R12 multi-arch sidecar fail | 6.9 (build-images.yml per-image matrix) | per-image arm64 disablement allowed |

---

## 8. Dependency gates (cross-SPEC)

본 SPEC implementation 시작 / 완료 시점에 확인해야 할 cross-SPEC
dependency gate.

### Pre-implementation gates

- **G1** — BOOT-001 / DEP-001 / OBS-001 implementation status verify
  (roadmap §M1 closed)
- **G2** — IDX-001/002/003/CACHE-001 implementation status verify
  (roadmap §M3 closed)
- **G3** — AUTH-001/002/003 implementation status verify (roadmap §M6)
- **G4** — IDX-004/005 implementation status verify
- **G5** — `deploy/postgres/migrations/0003_audit_events.sql` +
  `0004_audit_cost_ledger_trigger.sql` (currently untracked per
  git status) status verify — committed before chart 작업 시작

### Sibling gates

- **G6** — DOC-001 페이지 IA structure 합의 (operators/deployment-
  helm.mdx + operators/security/secrets.mdx slot reserved)
- **G7** — DOC-002 adapter env-var 표준 합의 (chart values.schema의
  adapter section 기준점)

### Post-implementation gates (close-out)

- **G8** — SEC-001 ship 후 NFR-DEPLOY-008 integration test 별도 PR
- **G9** — MCP-001 implementation 후 chart `mcp.enabled` default
  true 전환 + DOC-001 업데이트

---

## 9. Verification matrix (REQ ↔ Phase ↔ Acceptance gate)

| REQ | Phase | Acceptance gate |
|-----|-------|----------------|
| REQ-DEPLOY-001 chart structure | 6.1 | A1, A2 |
| REQ-DEPLOY-002 api Dockerfile | 6.2 | A5 |
| REQ-DEPLOY-003 mcp+migrate Dockerfile | 6.2 | A5 |
| REQ-DEPLOY-004 sub-directory | 6.1, 6.4..6.7 | A1 |
| REQ-DEPLOY-005 values.schema.json | 6.1 | A2 |
| REQ-DEPLOY-006 migration hook | 6.5 | A4 |
| REQ-DEPLOY-007 ConfigMap | 6.4, 6.6, 6.7 | A8 (parity) |
| REQ-DEPLOY-008 Secret | 6.4 | A8 |
| REQ-DEPLOY-009 per-service resources | 6.4, 6.6 | A1 |
| REQ-DEPLOY-010 NetworkPolicy | 6.4 | A4 (kind CNI) |
| REQ-DEPLOY-011 sidecar toggle | 6.6 | A2 |
| REQ-DEPLOY-012 GPU state-driven | 6.6 | A2 |
| REQ-DEPLOY-013 probes | 6.4, 6.6 | A4 |
| REQ-DEPLOY-014 subchart deps | 6.1 | A9 |
| REQ-DEPLOY-015 in-chart custom | 6.7 | A2 |
| REQ-DEPLOY-016 secret backend | 6.4 | A2 (3 modes) |
| REQ-DEPLOY-017 chart OCI publish | 6.9 | A9 |
| REQ-DEPLOY-018 image build+sign | 6.9 | A5, A6, A7 |
| REQ-DEPLOY-019 ServiceMonitor | 6.4, 6.6 | A4 |
| REQ-DEPLOY-020 chart-ci.yml | 6.9 | A3, A4 |
| REQ-DEPLOY-021 Ingress optional | 6.4 | A2 |
| REQ-DEPLOY-022 helm test | 6.5 | A4 |
| REQ-DEPLOY-023 ExternalSecret P2 | 6.4 | A2 |
| REQ-DEPLOY-024 parity script | 6.9 | A8 |

NFR matrix:

| NFR | Phase | Acceptance gate |
|-----|-------|----------------|
| NFR-DEPLOY-001 fail-fast misconfig | 6.1 | A2 (S6) |
| NFR-DEPLOY-002 reproducible install | 6.9 | A2 |
| NFR-DEPLOY-003 cold install ≤5min | 6.9 | A4 |
| NFR-DEPLOY-004 rollback support | 6.5, 6.10 | A4 (S8) |
| NFR-DEPLOY-005 subchart pinning policy | 6.1 + ongoing | A9 |
| NFR-DEPLOY-006 image pull rate-limit awareness | 6.10 | A10 |
| NFR-DEPLOY-007 multi-arch coverage | 6.2, 6.9 | A5 |
| NFR-DEPLOY-008 cross-SPEC verification | 6.11 | (deferred) |

---

## 10. Exit criteria (plan-level)

본 plan은 다음 조건 모두 만족 시 spec.md acceptance gate A1..A13 PASS
가능 — release-ready:

- [ ] §3 ANALYZE 산출물 (research.md) ↔ spec.md REQ 1:1 매핑 verify
- [ ] §4 PRESERVE CT1..CT5 모두 PASS
- [ ] §5 IMPROVE sub-phase 5.1..5.10 모두 ship + helm-unittest pass
- [ ] §6 phase 6.1..6.10 sequence 완료
- [ ] §7 risk R1..R12 각 mitigation phase 완료
- [ ] §8 dependency gate G1..G7 verified
- [ ] §9 verification matrix 24 REQ + 8 NFR ↔ acceptance gate cross-
      check 완료
- [ ] spec.md §6 acceptance gate A1..A13 PASS
- [ ] Sprint Contract (§2 본 plan) checklist 모두 satisfied
- [ ] evaluator-active 4-dimension score: Functionality ≥ 0.85,
      Operability ≥ 0.80, Craft ≥ 0.75, Consistency ≥ 0.80 (Sprint
      Contract pass conditions)
- [ ] plan-auditor sign-off (P0 release-blocker, thorough harness)

---

## 11. Annotation cycle preparation

본 plan은 evaluator-active와의 Sprint Contract 협상 (per §11 design
constitution) 시작점. 협상 시 다음 항목 우선 합의:

- chart artifact 최종 위치 (`charts/universal-search/` confirmed) +
  OCI registry path (`oci://ghcr.io/<org>/charts/universal-search`
  confirmed)
- subchart 선정 (Bitnami postgresql + Bitnami redis + qdrant official)
  vs alternative operator path (Zalando, CrunchyData) — V1은 Bitnami
  default + external mode opt-out 합의
- secret 3-tier strategy의 V1.0.0 default tier — tier 2
  (existingSecret) recommended; tier 1 (values)은 dev/CI 한정 경고
  강화; tier 3 (ESO)은 ESO pre-install 운영자 opt-in
- multi-arch policy — embedder amd64-only 명시적 acknowledgment +
  NFR-DEPLOY-007 acceptance
- compose-chart parity invariant — REQ-DEPLOY-024 강제; allowlist
  format 합의
- DOC-001 / DOC-002 cross-link integrity — 두 SPEC slot reserved
  확인
- SEC-001 integration deferral — NFR-DEPLOY-008 close-out PR로 분리

annotation cycle은 최대 6 iteration; 본 plan + research가 충분히
구체적이라면 1-2 iteration로 close 가능.

---

**Plan total: 11 sections + 5 implementation principles + Sprint
Contract + DDD ANALYZE/PRESERVE/IMPROVE phases (10 IMPROVE sub-
phases, file-by-file) + 11 ordered phases + risk register (12 risks)
+ dependency gate (9 gates) + verification matrix (24 REQ + 8 NFR).
Companion: spec.md + research.md.**
