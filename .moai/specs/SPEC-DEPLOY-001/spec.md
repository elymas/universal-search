---
id: SPEC-DEPLOY-001
version: 0.1.0
status: draft
created: 2026-05-22
updated: 2026-05-22
author: limbowl
priority: P0
issue_number: 0
title: Helm chart — k8s team-scale deploy with multi-service topology, 3-tier secret strategy, golang-migrate migration job, cosign-signed multi-arch images, ServiceMonitor + OTLP observability wiring, and helm-lint + kubeconform + kind smoke-test CI gates
milestone: M9 — V1 release
owner: expert-devops
methodology: ddd
coverage_target: 85
depends_on: [SPEC-BOOT-001, SPEC-CACHE-001, SPEC-IDX-001, SPEC-IDX-002, SPEC-IDX-003, SPEC-IDX-004, SPEC-IDX-005, SPEC-OBS-001, SPEC-AUTH-001, SPEC-AUTH-002, SPEC-AUTH-003, SPEC-SEC-001]
blocks: [SPEC-REL-001]
related: [SPEC-DOC-001, SPEC-DOC-002]
---

# SPEC-DEPLOY-001: Helm chart — k8s team-scale deploy for universal-search

## HISTORY

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  M9 (V1 release)의 세 번째 SPEC이자 SPEC-REL-001 V1.0.0 태깅의
  **release-blocking dependency**. `.moai/project/roadmap.md:114`
  ("SPEC-DEPLOY-001 | Helm chart | k8s deploy for team scale |
  expert-devops")의 full EARS 확장 + §5 M9 exit criterion
  ("Helm chart deployable") 의 단독 담보 SPEC. SPEC-DEPLOY-001이
  PASS하지 못하면 SPEC-REL-001 V1.0.0 tagging이 차단되며, 외부
  운영자에게 ship되는 binary에 **supported team-scale deploy
  path가 부재**한 상태가 된다 — single-node `docker compose up`
  으로만 사용 가능한 V1은 "team scale" 약속을 어긴다.

  본 SPEC은 **신규 보안 시스템이나 신규 deploy 패러다임을 발명하지
  않는다**. `deploy/docker-compose.yml` 265 lines + GPU overlay 19
  lines가 정의한 13-service dev-compose topology를 **parameterized
  Helm v3 chart로 추출**하는 것이 본질이며, DDD methodology가 채택된
  이유다 (ANALYZE existing dev-compose surface → PRESERVE working
  dev workflow [compose-up 동일 동작 유지] → IMPROVE with chart-
  ization, multi-arch image authoring, signing, observability wiring,
  3-tier secret strategy).

  현재 코드베이스에 이미 배치되어 있어 chart가 의존하는 자산:

  - `deploy/docker-compose.yml` (265 lines): 13 service topology
    (qdrant v1.16.3, meili v1.42.1, postgres 16.13, redis 7,
    searxng dated tag, litellm v1.83.7, prometheus v2.54.1,
    researcher Python sidecar, embedder Python sidecar with GPU
    overlay, tokenizer-ko Python sidecar) — chart values.yaml의
    image tag default값 + service topology의 ground truth.
  - `deploy/postgres/migrations/0001..0007*.sql` (9 SQL files):
    SPEC-IDX-001/IDX-005/AUTH-002/AUTH-003/LLM-001/DEEP-001의
    schema migration — chart의 `pre-install`/`pre-upgrade` Job이
    golang-migrate v4.18+ + distroless container로 실행.
  - `services/{researcher,embedder,tokenizer-ko,storm,koreanews}/
    Dockerfile`: 5개 Python sidecar의 container image source —
    본 SPEC은 Dockerfile을 추가 작성하지 않고 chart values에서
    image reference만 명세화.
  - `cmd/{usearch-api,usearch-mcp}/main.go`: 2개 Go binary —
    본 SPEC scope에서 multi-stage Dockerfile (`deploy/Dockerfile.
    usearch-api`, `Dockerfile.usearch-mcp`, `Dockerfile.usearch-
    migrate`) 신규 작성. CLI `cmd/usearch/`는 release artifact로
    별도 배포 (SPEC-REL-001 scope), chart에 포함 안 됨.
  - `.env.example` (~50 vars): ConfigMap (non-secret) + Secret
    (sensitive) chart values의 ground truth + drift detection
    benchmark.
  - SPEC-OBS-001 implemented: `/metrics` Prometheus endpoint +
    `/healthz` + OTLP wiring (slog → Loki는 phase 2 reserved).
    chart는 ServiceMonitor + OTLP collector reference만 추가.
  - SPEC-AUTH-001/002/003 implemented: OIDC + Casbin RBAC + audit
    log. chart는 OIDC issuer URL + JWKS endpoint + audit log DB
    table을 env-var + migration으로 wire.
  - SPEC-CACHE-001 implemented: Redis dep + 5-phase fallback. chart
    의 `redis.enabled: true` (Bitnami subchart default) 또는
    `redis.external.*` opt-out으로 운영자 선택.
  - SPEC-IDX-001..005 implemented per roadmap M3/M6 status: Qdrant
    + Meili + PG hybrid index — chart subchart strategy 결정 §
    D3 참조.
  - SPEC-SEC-001 draft (commit 761381d): `internal/security/secrets/
    Resolver` interface 설계 예정. 본 SPEC chart는 K8s Secret
    refs까지만 책임 — runtime resolution은 SEC-001 implementation
    이 담당. 두 SPEC decoupled (chart는 SEC-001 implementation
    지연에도 ship 가능). risk 노트는 research §14.1.
  - SPEC-DOC-001 draft (commit 6b70742): `operators/deployment-helm.
    mdx` 페이지 + `operators/security/secrets.mdx` 페이지가 본
    chart의 user-facing 표면. chart NOTES.txt가 DOC-001 canonical
    URL을 reference.
  - SPEC-DOC-002 draft (commit d492f09): adapter env-var reference
    + chart `values.schema.json`의 adapter-key section과 cross-
    validation (drift detection — schema가 ground truth, DOC-002
    MDX가 사람 표면).

  본 SPEC이 신규로 도입하는 것:

  - `charts/universal-search/` chart artifact (NEW): Chart.yaml
    (apiVersion v2, kubeVersion `>=1.28-0 <1.32-0`), values.yaml
    (~300+ keys), values.schema.json (strict JSON Schema Draft-07
    + additionalProperties: false), templates/ (per-service sub-
    directory 구조 — api/, mcp/, researcher/, embedder/, tokenizer-
    ko/, storm/, koreanews/, litellm/, searxng/, jobs/), README.md,
    NOTES.txt, ci/ (values-test.yaml, values-prod.yaml).
  - 3개 신규 Dockerfile (`deploy/Dockerfile.usearch-api`,
    `deploy/Dockerfile.usearch-mcp`, `deploy/Dockerfile.usearch-
    migrate`): multi-stage build (golang:1.24.x → distroless/static-
    debian12), multi-arch (linux/amd64 + linux/arm64), non-root
    USER, HEALTHCHECK (운영자 docker-only deploy 호환).
  - `.github/workflows/build-images.yml` (NEW): main merge + tag
    push 시 7개 image (api/mcp/migrate + 5개 Python sidecar) build
    + multi-arch + cosign keyless signing + SBOM via syft + SLSA
    L2 provenance + push to ghcr.io.
  - `.github/workflows/chart-ci.yml` (NEW): chart lint (helm lint,
    chart-testing) + schema validation (helm template + values.
    schema.json) + kubeconform on k8s 1.28..1.31 + kind cluster
    smoke-test (minimal profile: api + postgres + redis만 enabled).
  - `.github/workflows/chart-release.yml` (NEW): tag 시 chart
    package + cosign sign-blob + push to `oci://ghcr.io/<org>/
    charts/`.
  - `charts/universal-search/Chart.lock` + `charts/universal-search/
    charts/` (subchart 캐시): postgresql Bitnami + redis Bitnami +
    qdrant official subchart pinned versions.

  Pinned decisions (10개 scope pillar D1..D10):

  (D1) **Chart engine choice — Helm v3 only**: Kustomize raw
       manifests + overlays는 12+ service multi-deploy에서 patch
       hell 우려로 배제. Operator-SDK (CRD + controller)는 V1
       target user (small team self-hosted)에는 과학습 부담; deferred
       to post-V1 (federated multi-tenant SaaS 시 재검토).
       Carvel kapp+ytt는 ecosystem maturity 부족. ArgoCD/Flux는
       chart의 **소비자**이지 chart 자체가 아님 — 본 chart는 OCI
       분배 + standard Helm shape 유지로 어떤 GitOps tool로도 deploy
       가능. 결정 근거: research §7 다각 분석.
       - Anti-decision: Helm + Kustomize hybrid (post-render)은
         post-V1로 deferred. 운영자가 필요 시 `helm template |
         kustomize build` chain 직접 구성 가능.

  (D2) **Topology — multi-service per-component sub-directory**:
       `templates/` 디렉토리를 service별 sub-directory로 구조화
       (api/, mcp/, researcher/, embedder/, tokenizer-ko/, storm/,
       koreanews/, litellm/, searxng/, jobs/, observability/).
       각 sub-directory에 Deployment + Service + ConfigMap (필요시)
       + Secret reference + HPA + PDB + NetworkPolicy + ServiceMonitor.
       `_helpers.tpl`은 chart root에 단일화 — common helper
       (fullname, labels, image-ref, secret-resolver). research §6.3.
       - Anti-decision: 모든 template을 flat directory에 두는 패턴은
         13+ resource 시 가독성 폭락으로 배제.

  (D3) **Dependency strategy — bundled subchart default + external
       opt-out**:
       - Postgres: Bitnami `bitnami/postgresql` v16.4.x subchart
         default. 운영자가 `postgresql.enabled: false` +
         `postgresql.external.{host,port,database,existingSecret}`
         로 외부 RDS/Cloud SQL 사용 가능.
       - Redis: Bitnami `bitnami/redis` v20.x subchart default
         (architecture: standalone). `redis.enabled: false` +
         `redis.external.*` opt-out. Sentinel/Cluster는 V1 untested.
       - Qdrant: official `qdrant/qdrant-helm-chart` subchart default.
         `qdrant.enabled: false` + `qdrant.external.{host,port}` opt.
       - Meilisearch: **in-chart custom Deployment + StatefulSet**
         (외부 chart의 pin lag 회피). Bitnami / Meili official chart
         maturity 평가 후 post-V1 re-evaluation.
       - SearXNG: in-chart custom Deployment (AGPL-3.0 — chart
         README + NOTES.txt에 license 명시). subchart 사용 안 함
         (license-conscious 운영자가 service-boundary 유지하도록).
       - LiteLLM: in-chart custom Deployment. official chart
         미존재.
       - Prometheus: NOT bundled. 운영자가 kube-prometheus-stack
         cluster-wide 설치 가정; chart는 ServiceMonitor CRD만 생성
         (`observability.serviceMonitor.enabled: true` default).
         opt-out: `observability.serviceMonitor.enabled: false` +
         별도 scrape config.
       - Subchart pinning policy: `Chart.yaml` `dependencies[].
         version`은 정확한 patch 버전 고정 (예: `16.4.5`, not
         `^16.4`). Quarterly audit + manual bump (NFR-DEPLOY-005).
       - Anti-decision: kube-prometheus-stack subchart bundling은
         (a) chart 크기 폭증, (b) 운영자 prometheus-operator 충돌
         risk로 배제.

  (D4) **Migration job ownership — golang-migrate + pre-install
       Helm hook + idempotent SQL audit**:
       - Tool: `golang-migrate/migrate v4.18.x`. Go-native, distroless
         container, atomic transaction per migration (PG), built-in
         `schema_migrations` table + `migrate.lock` (concurrent run
         차단).
       - Container: `ghcr.io/<org>/usearch-migrate:<tag>` — multi-
         stage (alpine downloader + distroless runtime), 9개 SQL
         file `/migrations/` 디렉토리에 COPY.
       - Helm hook: `pre-install`,`pre-upgrade` + `hook-weight: -5`
         + `hook-delete-policy: before-hook-creation,hook-succeeded`.
         backoffLimit: 3 후 실패 시 helm install 자체 실패.
       - Idempotency audit: PRESERVE phase에서 9개 SQL 파일이 CREATE
         TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS 사용
         하는지 grep verify. 위반 시 IMPROVE에서 fix (SQL 파일
         소유 SPEC owner와 협의).
       - Rollback: chart manifest rollback (`helm rollback`)은
         schema 자동 rollback **하지 않음**. operator runbook (DOC-
         001 cross-link)에서 forward-fix migration 권장 명시.
         `down.sql` 적용은 데이터 손실 위험 (수동 SQL review 필요).
       - Anti-decision: pressly/goose, ariga.io/atlas는 (a) Go-
         native + 9 SQL 호환성, (b) declarative vs imperative
         mismatch로 배제.

  (D5) **Secret backend — 3-tier strategy**:
       - Tier 1 (`secrets.backend: "values"`, dev/CI only): values.
         yaml에 secret 직접 작성 → chart가 K8s Secret 자동 생성.
         **production 사용 절대 금지** — NOTES.txt + README에서
         경고. git committed values.yaml에 plain secret 노출.
       - Tier 2 (`secrets.backend: "existingSecret"`, production
         small-team default 권장): 운영자가 `kubectl create secret`
         으로 사전 생성한 K8s Secret을 chart가 `secretKeyRef`로
         reference. Deployment env에 `valueFrom.secretKeyRef`.
         rotation은 운영자 책임 (manual kubectl apply + rolling
         restart).
       - Tier 3 (`secrets.backend: "externalSecrets"`, production
         enterprise): chart가 `ExternalSecret` CRD 생성. ESO
         operator가 Vault/AWS SM/GCP SM에서 fetch → K8s Secret 자동
         생성 + refresh. `refreshInterval: "1h"` default.
       - SEC-001 D5와 정렬: SEC-001 `internal/security/secrets/
         Resolver` interface는 binary runtime layer; 본 chart는
         deploy-time secret expose layer. 두 layer는 K8s Secret
         resource를 통해 통신 — decoupled. SEC-001 implementation
         지연되어도 chart는 ship 가능 (research §14.1).
       - Anti-pattern: chart values에 production secret 작성한 채
         git commit (D2 gitleaks가 enforce per SEC-001).

  (D6) **Observability wiring — ServiceMonitor + OTLP refs only**:
       - Prometheus ServiceMonitor: `monitoring.coreos.com/v1/
         ServiceMonitor` CRD 생성 (`observability.serviceMonitor.
         enabled: true` default). 운영자가 kube-prometheus-stack
         cluster-wide 설치 가정.
       - OTLP: `observability.otlp.endpoint` env-var을 ConfigMap
         으로 주입. 운영자가 cluster-internal OpenTelemetry Collector
         의 ClusterIP service 주소 (예: `otel-collector.observability.
         svc.cluster.local:4317`) 설정. chart는 collector를 ship 안 함.
       - Healthcheck → probe 매핑: research §11.3 표 적용. embedder
         model load 시간 보호 (`startupProbe.failureThreshold: 120,
         periodSeconds: 1`).
       - Grafana dashboard JSON: OUT-OF-SCOPE (별도 SPEC-EVAL-002에
         위탁).
       - Anti-decision: chart가 Prometheus + Grafana + Loki + OTel
         Collector를 모두 bundle하는 옵션은 V1에서 배제 — chart
         크기 + 운영자 환경 충돌 위험.

  (D7) **Image distribution + signing — ghcr.io + cosign keyless +
       SBOM + SLSA L2**:
       - Registry: `ghcr.io/<org>/` default. 운영자 override:
         `global.imageRegistry`.
       - Multi-arch: linux/amd64 + linux/arm64 manifest list. `docker
         buildx build --platform linux/amd64,linux/arm64`. Go binaries
         (CGO_ENABLED=0 static)는 cross-compile 가능. Python sidecar
         multi-arch는 §10.1 sidecar별 평가: researcher/tokenizer-ko/
         storm/koreanews는 multi-arch; **embedder는 amd64 only**
         (torch + CUDA 의존; NFR-DEPLOY-007 acknowledgment;
         embedder.nodeAffinity로 amd64 강제).
       - Signing: `sigstore/cosign v2.4.0+` keyless (GitHub Actions
         OIDC identity). image + chart 모두 sign. 검증 명령은
         DOC-001 `operators/security/image-verification.mdx`에 cross-
         link.
       - SBOM: `anchore/syft v1.x` SPDX format. release artifact에
         attach + ghcr.io에 OCI attached artifact로 push.
       - SLSA: Level 2 (provenance + signed releases) via `slsa-
         framework/slsa-github-generator`. Level 3 (isolated builder)
         은 GitHub Actions hosted runner 한계로 post-V1.
       - Chart artifact: `oci://ghcr.io/<org>/charts/universal-
         search:<chart-version>`. Helm 3.8+ OCI mode.
       - Anti-decision: GPG-based Helm provenance (helm package
         --sign)은 (a) cosign-based image signing과 도구 통일성,
         (b) key 관리 부담 회피로 배제.

  (D8) **Multi-environment values layering**:
       - `values.yaml` (chart default, 모든 옵션 안전한 minimal
         설정 — dev 환경에서 즉시 사용 가능).
       - `ci/values-test.yaml` (CI smoke-test minimal profile —
         api + postgres + redis만 enabled; sidecar + qdrant + meili
         + searxng 모두 disabled. hosted runner resource 한계 보호.
         research §14.9).
       - `ci/values-prod.yaml` (production reference example —
         HPA/PDB/NetworkPolicy 모두 enabled, secrets.backend:
         existingSecret, observability.serviceMonitor.enabled:
         true, ingress.enabled: true, replicas: 2+).
       - `ci/values-gpu.yaml` (embedder.gpu.enabled: true override).
       - 운영자 권장: `values.yaml` + `values-prod.yaml`을 base로
         하고 본인 환경 overlay를 별도 파일로 작성 → `helm install
         -f values.yaml -f values-prod.yaml -f my-overlay.yaml`.
       - values.schema.json strict validation (`additionalProperties:
         false`) — 오타 fail-fast. 단 forward-compat extension point
         (예: `extraEnv: []`, `podAnnotations: {}`)는 명시적 schema
         포함.
       - Anti-decision: 환경별 chart 분기 (`values-prod.tgz`)는
         chart artifact 다양화로 maintainability 폭락; 단일 chart
         + multi-values layering이 표준.

  (D9) **NetworkPolicy + PDB + HPA — production hardening defaults**:
       - NetworkPolicy: `enabled: true` default. 모든 Deployment에
         ingress + egress policy. ingress: ingress-nginx namespace
         + 동일 namespace의 다른 service만 허용. egress: postgres /
         redis / qdrant / meili / litellm / sidecar / DNS (kube-
         system kube-dns) + cluster-external OIDC issuer + LLM API
         + adapter API endpoint들.
       - PDB: `enabled: true` default for api/mcp. `minAvailable:
         1`. sidecar는 default OFF (운영자 opt-in).
       - HPA: `enabled: true` default for api/mcp. `targetCPUUtilization
         Percentage: 70`, `minReplicas: 2`, `maxReplicas: 10`.
         sidecar는 default OFF (대부분 stateful 또는 expensive cold-
         start).
       - Anti-decision: NetworkPolicy default OFF는 (a) production
         security baseline 미달, (b) operator opt-in 부담으로 배제.
         단 운영자가 NetworkPolicy controller (Calico, Cilium 등)
         미설치 환경에서 정책이 no-op이 됨을 NOTES.txt에 명시.

  (D10) **Ingress + TLS — cert-manager + ingress-nginx default off**:
       - Ingress: `usearch.api.ingress.enabled: false` default
         (운영자 명시적 opt-in). 활성화 시 cert-manager.io/cluster-
         issuer annotation default `letsencrypt-prod`; 운영자가
         self-signed 또는 internal CA 사용 시 override.
       - TLS: ingress.tls 섹션에서 cert-manager가 secret 자동 발급.
         HSTS header는 ingress-nginx annotation으로 enforce
         (`nginx.ingress.kubernetes.io/configuration-snippet`).
       - mcp: ingress NOT 노출 (cluster-internal only; HTTP mode
         사용 시 ClusterIP Service만).
       - Anti-decision: cert-manager subchart bundling은 (a) cluster-
         wide singleton 요구 충돌, (b) 운영자 사전 install 표준으로
         배제. chart README에서 cert-manager + ingress-nginx
         pre-install를 documented requirement로 명시.

  Companion artifacts:
  - `.moai/specs/SPEC-DEPLOY-001/research.md` — Phase 0.5 research
    (16 sections, ≈38 KB: dev-compose surface inventory, migration
    inventory, env-var surface map, Helm v3 pattern survey, alternative
    chart tooling, dependency strategy, secret management, multi-arch
    + signing, observability, migration job patterns, OSS Helm chart
    audit, open risks).
  - `.moai/specs/SPEC-DEPLOY-001/plan.md` — DDD phased plan (Sprint
    Contract REQUIRED per harness: thorough).

  24 EARS REQs (16 × P0 + 6 × P1 + 2 × P2) + 8 NFRs + 1 new chart
  directory (`charts/universal-search/`) + 3 new Dockerfiles + 3 new
  CI workflows + ≥ 30 new template files. Methodology: **DDD**
  (existing dev-compose surface consolidation — byte-fidelity
  preservation of dev workflow, Helm-shaped IMPROVE). Coverage
  target 85% applies to chart templates (helm-unittest) + CI scripts
  (build-images.sh + smoke-test.sh); YAML chart content는 byte-
  fidelity equivalent to dev-compose으로 검증 (compose ↔ chart parity
  smoke test per REQ-DEPLOY-024). Harness: **thorough** (P0 release-
  blocking + production-deploy domain + cross-SPEC integration —
  Sprint Contract MANDATORY per `.claude/rules/moai/design/
  constitution.md` §11). Owner: expert-devops.

---

## 1. Overview

SPEC-DEPLOY-001은 M9 (V1 release)의 세 번째 SPEC이자 SPEC-REL-001
V1.0.0 tagging의 release-blocking dependency다. 본 SPEC은 **새로운
deploy 시스템을 발명하지 않으며**, 13-service dev-compose stack과
9-file Postgres migration sequence를 (a) parameterized Helm v3 chart
로 추출, (b) 3-tier secret strategy + production hardening defaults
(NetworkPolicy/PDB/HPA) 추가, (c) multi-arch image + cosign signing +
SBOM CI pipeline 구축의 세 축으로 ship한다.

### 1.1 What ships

| Layer | Artifact | Purpose |
|-------|----------|---------|
| Chart | `charts/universal-search/Chart.yaml` (NEW, apiVersion v2) | chart 메타 + subchart deps (postgres, redis, qdrant) |
| Chart | `charts/universal-search/values.yaml` (NEW, ~300 keys) | safe minimal default (dev-ready) |
| Chart | `charts/universal-search/values.schema.json` (NEW) | JSON Schema Draft-07 + additionalProperties: false |
| Chart | `charts/universal-search/templates/_helpers.tpl` (NEW) | fullname/labels/image-ref/secret-resolver helpers |
| Chart | `charts/universal-search/templates/NOTES.txt` (NEW) | 사후-install 가이드 (port-forward, OIDC redirect URI, docs site link) |
| Chart | `charts/universal-search/templates/{api,mcp}/{deployment,service,hpa,pdb,networkpolicy,configmap,secret,servicemonitor,ingress,serviceaccount}.yaml` (NEW) | 핵심 Go binary 2개의 모든 k8s resource |
| Chart | `charts/universal-search/templates/{researcher,embedder,tokenizer-ko,storm,koreanews}/{deployment,service,configmap}.yaml` (NEW) | 5개 Python sidecar |
| Chart | `charts/universal-search/templates/embedder/pvc.yaml` (NEW) | HuggingFace model cache PVC (compose `embedder_models` volume equivalent) |
| Chart | `charts/universal-search/templates/{litellm,searxng}/{deployment,service,configmap}.yaml` (NEW) | proxy + metasearch (in-chart, AGPL warning) |
| Chart | `charts/universal-search/templates/jobs/migrate.yaml` (NEW) | pre-install/pre-upgrade Helm hook Job |
| Chart | `charts/universal-search/templates/jobs/smoke-test.yaml` (NEW) | helm test Job (`/healthz` + `/metrics` curl) |
| Chart | `charts/universal-search/ci/{values-test,values-prod,values-gpu}.yaml` (NEW) | environment overlay 예제 |
| Chart | `charts/universal-search/README.md` (NEW) | install + upgrade + uninstall + troubleshoot |
| Chart | `charts/universal-search/Chart.lock` + `charts/universal-search/charts/` (subchart 캐시) | reproducible install |
| Image | `deploy/Dockerfile.usearch-api` (NEW, multi-stage, multi-arch) | api binary container source |
| Image | `deploy/Dockerfile.usearch-mcp` (NEW) | mcp binary container source |
| Image | `deploy/Dockerfile.usearch-migrate` (NEW, distroless + golang-migrate v4.18 + 9 SQL files) | migration job container source |
| CI | `.github/workflows/build-images.yml` (NEW) | 7 image multi-arch + cosign + SBOM + SLSA on main merge / tag |
| CI | `.github/workflows/chart-ci.yml` (NEW) | helm lint + chart-testing + kubeconform 1.28..1.31 + kind smoke-test on PR |
| CI | `.github/workflows/chart-release.yml` (NEW) | chart package + cosign + OCI push on tag |
| Docs | (DOC-001 cross-link) `docs/content/{en,ko}/operators/deployment-helm.mdx` | user-facing install walkthrough |
| Docs | (DOC-001 cross-link) `docs/content/{en,ko}/operators/security/{secrets,image-verification}.mdx` | 3-tier secret guide + cosign verify procedure |

### 1.2 Motivation

V1 release (`v1.0.0` tag in SPEC-REL-001) 직전 chart 부재는 **team-
scale deploy 불가능**을 의미한다. roadmap §5 M9 exit criterion
"Helm chart deployable" 미충족 시 결과:

- **5+ user team 운영자**가 chart 없이 13-service stack을 k8s에서
  운영하려면 매 service의 Deployment + Service + ConfigMap + Secret
  + HPA + PDB + NetworkPolicy + ServiceMonitor + Ingress YAML을
  수작업 작성해야 함. 13 service × 평균 7 resource = 90+ YAML 파일.
  upgrade 시 매 파일 manually 동기화 — operational debt 폭증.
- **multi-replica deploy**가 부재 (compose의 `restart: unless-stopped`
  는 single-host single-replica만 제공). HA + rolling deploy + canary
  + horizontal scaling 모두 불가.
- **secret rotation**이 부재 (compose의 `${VAR}` interpolation은
  shell env-var; rotation 시 stack 재기동 필요). production에서
  zero-downtime rotation 부재 = security incident.
- **observability integration**이 ad-hoc (각 운영자가 prometheus-
  operator 환경에 chart resource를 어떻게 scrape할지 별도 구성).
- **image trust chain**이 부재 (cosign signature + SBOM + SLSA
  attestation 없이 운영자가 ghcr.io image를 pull하여 production에
  설치 → supply chain attack surface 노출).

본 SPEC이 **PASS**해야 하는 이유: M9 exit criterion "Helm chart
deployable" (`roadmap.md:157`) 미달성 시 SPEC-REL-001 V1.0.0 tagging
차단. SPEC-DOC-001 `operators/deployment-helm.mdx`는 본 chart를
reference (DOC-001은 본 SPEC의 user-facing 표면 — 본 SPEC이 chart
artifact를 ship하지 못하면 DOC-001 페이지가 dead-link).

### 1.3 Forward-compatibility commitments

본 SPEC은 다음 sibling/downstream SPEC과의 contract를 명시한다:

- **SPEC-REL-001 (M9 sibling)** — V1.0.0 tag + release notes. 본
  SPEC PASS는 REL-001의 "Helm chart deployable" exit gate. REL-001
  release notes가 chart OCI URL (`oci://ghcr.io/<org>/charts/universal-
  search:1.0.0`) + cosign 검증 procedure를 cite.
- **SPEC-DOC-001 (M9 sibling, drafted)** — user guide site. 본 chart
  install walk-through은 DOC-001 `operators/deployment-helm.mdx`로
  user-facing. 본 SPEC의 NOTES.txt + README는 minimal (kubectl
  port-forward + first-login guide); 깊이 있는 운영 narrative는
  DOC-001에 위탁.
- **SPEC-DOC-002 (M9 sibling, drafted)** — adapter env-var reference.
  본 chart `values.schema.json`의 adapter-key section이 ground truth;
  DOC-002 MDX는 사람용 표면. drift detection: CI에서 schema ↔ DOC-002
  cross-validate (DOC-002 책임).
- **SPEC-SEC-001 (M8 sibling, drafted)** — security hardening. 본 chart
  의 secret backend tier 3 (ESO)는 SEC-001 D5 `internal/security/
  secrets/Resolver` interface와 정렬. SEC-001 implementation 지연 시
  chart는 ship 가능 (decoupled); SEC-001 ship 후 통합 integration
  test 1회 추가 (NFR-DEPLOY-008 cross-SPEC verification).
- **SPEC-BOOT-001 (implemented)** — repo scaffold + CI + dev-compose.
  본 SPEC은 BOOT-001의 `deploy/docker-compose.yml`을 ground truth로
  사용. BOOT-001 retrospective amendment 없이 본 SPEC scope에서
  Dockerfile (`Dockerfile.usearch-api`, `Dockerfile.usearch-mcp`,
  `Dockerfile.usearch-migrate`) 신규 작성.
- **SPEC-OBS-001 (implemented)** — observability baseline. 본 chart
  ServiceMonitor + OTLP wiring은 OBS-001의 `/metrics` endpoint + OTLP
  exporter를 reference. 신규 metric 추가 없음.
- **SPEC-AUTH-001/002/003 (implemented)** — OIDC + RBAC + audit log.
  본 chart ConfigMap에 OIDC env-var 명세화 (`.env.example`이 누락한
  OIDC vars 본 chart values에서 채움); migration job이 `0003_audit_
  events.sql` + `0003_casbin_rules.up.sql` + `0005_team_id_not_null.sql`
  + `0006_user_id_column.sql` 적용.
- **SPEC-CACHE-001 (implemented)** — Redis + 5-phase fallback. 본
  chart Redis subchart wiring + `internal/access/ssrf.go` SSRF guard
  의 egress NetworkPolicy 호환 (cluster-external HTTP allowed; private
  IP blocked by binary).
- **SPEC-IDX-001..005 (implemented)** — hybrid index + multi-tenancy
  + answer reuse. 본 chart Qdrant + Meilisearch subchart wiring +
  migration job이 `0007_answer_cache.up.sql` 적용 (IDX-005).
- **SPEC-DEEP-001 / SPEC-ADP-009 (implemented per roadmap)** — STORM
  + KoreaNews. 본 chart는 두 sidecar Deployment를 `enabled: false`
  default; 운영자 opt-in.

---

## 2. EARS Requirements

EARS Pattern legend:
- Ubiquitous: "The system shall ..."
- Event-driven: "When <event>, the system shall ..."
- State-driven: "While <condition>, the system shall ..."
- Optional: "Where <feature available>, the system shall ..."
- Unwanted: "If <unwanted>, then the system shall ..."

### 2.1 P0 — Release-blocking (16 REQs)

**REQ-DEPLOY-001 [Ubiquitous, P0]** — Chart shall publish a single
Helm v3 chart artifact at `charts/universal-search/` with `Chart.yaml`
declaring `apiVersion: v2`, `type: application`, `kubeVersion:
">=1.28-0 <1.32-0"`, semver `version` independent from binary
`appVersion`, and `dependencies:` entries pinned to exact patch
versions for postgresql (Bitnami), redis (Bitnami), and qdrant
(official). [Trace: research §6.1, §8, D2, D3]

**REQ-DEPLOY-002 [Ubiquitous, P0]** — Chart shall ship multi-stage
Dockerfile for `cmd/usearch-api` (`deploy/Dockerfile.usearch-api`)
producing distroless static-debian12 final image, CGO_ENABLED=0, non-
root USER 65532 (distroless `nonroot`), EXPOSE 8080, with build stage
using `golang:1.24.x-alpine` and target tags `linux/amd64,linux/arm64`
via `docker buildx`. [Trace: research §3, §10.1, D7]

**REQ-DEPLOY-003 [Ubiquitous, P0]** — Chart shall ship multi-stage
Dockerfile for `cmd/usearch-mcp` (`deploy/Dockerfile.usearch-mcp`)
following the identical pattern as REQ-DEPLOY-002 but with EXPOSE
matching MCP HTTP transport port (default 8081, configurable via
values), and shall ship `deploy/Dockerfile.usearch-migrate` producing
a distroless image with `golang-migrate/migrate v4.18.x` binary
COPY-ed from a downloader stage plus `deploy/postgres/migrations/`
contents COPY-ed to `/migrations/`. [Trace: research §3.3, §12.2, D4]

**REQ-DEPLOY-004 [Ubiquitous, P0]** — Chart shall create per-service
sub-directory under `charts/universal-search/templates/` for at
minimum the following services: api, mcp, researcher, embedder,
tokenizer-ko, storm, koreanews, litellm, searxng, jobs, with shared
helpers exclusively in `templates/_helpers.tpl`. [Trace: research §6.3,
D2]

**REQ-DEPLOY-005 [Ubiquitous, P0]** — Chart shall declare
`values.schema.json` at chart root conforming to JSON Schema Draft-07
with `additionalProperties: false` at top-level and at every nested
object, with explicit forward-compatibility extension points
(`extraEnv`, `podAnnotations`, `podLabels`, `extraVolumes`,
`extraVolumeMounts`) included in schema as nullable arrays/objects.
Schema validation shall be triggered automatically by `helm install`
and `helm upgrade`. [Trace: research §6.2, D8]

**REQ-DEPLOY-006 [Event-driven, P0]** — When `helm install` or `helm
upgrade` is invoked on the chart, the system shall execute a Helm
hook Job annotated `helm.sh/hook: pre-install,pre-upgrade` and
`helm.sh/hook-weight: "-5"` that runs `golang-migrate/migrate -path=/
migrations -database=$DATABASE_URL up`, with `backoffLimit: 3` and
`helm.sh/hook-delete-policy: before-hook-creation,hook-succeeded`,
blocking the release until all migrations are applied. [Trace:
research §12.3, D4]

**REQ-DEPLOY-007 [Ubiquitous, P0]** — Chart shall produce a per-
service ConfigMap mapping every non-secret environment variable
documented in `.env.example` (§5 of research.md) to its
corresponding service Deployment via `envFrom.configMapRef` or
explicit `env[].valueFrom.configMapKeyRef`, with values overridable
per service in `values.yaml` and validated by
`values.schema.json`. [Trace: research §5, §6.6, D8]

**REQ-DEPLOY-008 [Ubiquitous, P0]** — Chart shall expose every secret
environment variable (MEILI_MASTER_KEY, POSTGRES_PASSWORD,
SEARXNG_SECRET, LITELLM_MASTER_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY,
OIDC_CLIENT_SECRET, JWT_SIGNING_KEY, SESSION_SECRET, plus adapter
keys documented in DOC-002) via `env[].valueFrom.secretKeyRef`, with
the underlying K8s Secret resource sourced according to the active
secret backend tier (REQ-DEPLOY-017). [Trace: research §5.1, §9, D5]

**REQ-DEPLOY-009 [Ubiquitous, P0]** — Chart shall produce, for each
of `usearch-api` and `usearch-mcp`, the following k8s resources:
Deployment (with multi-replica support, configurable via `replicas`),
Service (ClusterIP by default), ServiceAccount (default name
`{fullname}-<component>`), HorizontalPodAutoscaler (enabled by
default per D9), PodDisruptionBudget (enabled by default per D9),
NetworkPolicy (enabled by default per D9), ServiceMonitor (gated by
`observability.serviceMonitor.enabled` per D6). [Trace: research §6.3,
D2, D6, D9]

**REQ-DEPLOY-010 [State-driven, P0]** — While
`networkPolicy.enabled: true`, the chart shall emit NetworkPolicy
resources permitting (a) ingress from ingress-nginx namespace and
intra-chart services only, (b) egress to declared dependencies
(postgres, redis, qdrant, meili, litellm, sidecar services), to
`kube-system/kube-dns`, and to declared cluster-external endpoints
(OIDC issuer host, LLM provider hosts, adapter API hosts) explicitly
listed in values. [Trace: research §6.6, D9]

**REQ-DEPLOY-011 [Ubiquitous, P0]** — Chart shall produce per-Python-
sidecar resources (researcher, embedder, tokenizer-ko, storm,
koreanews) each gated by `<sidecar>.enabled: true|false` toggle in
values, producing Deployment + Service + optional ConfigMap when
enabled. Embedder shall additionally produce a PersistentVolumeClaim
mapped to `/root/.cache/huggingface` matching the dev-compose
`embedder_models` named volume. [Trace: research §1.4, §6.3]

**REQ-DEPLOY-012 [State-driven, P0]** — While
`embedder.gpu.enabled: true`, the embedder Deployment shall declare
`resources.limits."nvidia.com/gpu": 1`, `nodeSelector` derived from
values, and `tolerations` derived from values, mapping the dev-
compose `docker-compose.gpu.yml` `deploy.resources.reservations.
devices` entry. [Trace: research §1.4, §10.1]

**REQ-DEPLOY-013 [Ubiquitous, P0]** — Chart shall produce
livenessProbe, readinessProbe, and where applicable startupProbe
for every Deployment, with parameters derived from the corresponding
dev-compose healthcheck entries via the mapping table in research
§11.3. Embedder shall use `startupProbe` with sufficient
`failureThreshold` to accommodate model load (≥120 seconds total).
[Trace: research §11.3]

**REQ-DEPLOY-014 [Ubiquitous, P0]** — Chart shall declare in
`Chart.yaml` `dependencies:` entries for `postgresql` (Bitnami,
default-enabled, default-pinned), `redis` (Bitnami, default-enabled,
default-pinned), `qdrant` (official chart, default-enabled, default-
pinned), each with `condition: <name>.enabled` to permit opt-out via
`<name>.enabled: false` plus `<name>.external.{host,port,...}` fields
for external service references. [Trace: research §8, D3]

**REQ-DEPLOY-015 [Ubiquitous, P0]** — Chart shall ship in-chart
custom Deployment + Service + ConfigMap + StatefulSet (as
appropriate) for meilisearch, litellm, and searxng with images
pinned to the exact tags used in `deploy/docker-compose.yml` as of
chart version 0.1.0, with NOTES.txt and README explicitly disclosing
that searxng is licensed AGPL-3.0 and consumed as service-boundary.
[Trace: research §1.2, §8.5, D3]

**REQ-DEPLOY-016 [Ubiquitous, P0]** — Chart shall implement secret
backend abstraction via `secrets.backend: "values" | "existingSecret"
| "externalSecrets"` switch in values, with chart templates
producing the appropriate underlying resource (K8s Secret authored
from values, K8s Secret referenced by name + key map, or
ExternalSecret CRD with `secretStoreRef` + `remoteKeys` map),
mutually exclusive per release. [Trace: research §9, D5]

### 2.2 P1 — Production-readiness + signing (6 REQs)

**REQ-DEPLOY-017 [Ubiquitous, P1]** — Chart shall publish OCI
artifact to `oci://ghcr.io/<org>/charts/universal-search:<chart-
version>` on every git tag matching `v*.*.*`, with
`.github/workflows/chart-release.yml` invoking `helm package` +
`cosign sign-blob` (keyless, GitHub Actions OIDC identity) + `helm
push`. [Trace: research §10.2, §10.4, D7]

**REQ-DEPLOY-018 [Ubiquitous, P1]** — Chart shall ship
`.github/workflows/build-images.yml` building all 7 container images
(usearch-api, usearch-mcp, usearch-migrate, researcher, embedder,
tokenizer-ko, storm, koreanews — embedder amd64-only per NFR-DEPLOY-
007) for `linux/amd64,linux/arm64` via `docker buildx`, signing
each with cosign keyless, generating SBOM via `anchore/syft v1.x`
in SPDX format, and attaching SLSA Level 2 provenance via
`slsa-framework/slsa-github-generator`. [Trace: research §10, D7]

**REQ-DEPLOY-019 [State-driven, P1]** — While
`observability.serviceMonitor.enabled: true` (default true), the
chart shall emit `monitoring.coreos.com/v1/ServiceMonitor` CRDs for
api, mcp, and each enabled Python sidecar, with `interval: 30s`
default and `scrapeTimeout: 10s` default, both overridable per
service. [Trace: research §11.1, D6]

**REQ-DEPLOY-020 [Ubiquitous, P1]** — Chart shall ship
`.github/workflows/chart-ci.yml` running on every pull request,
executing in sequence: `helm lint` (must PASS with zero error,
warnings permitted), `helm template` against `values.yaml` and
`ci/values-prod.yaml` (must succeed), `kubeconform` against k8s API
versions 1.28, 1.29, 1.30, 1.31 (must PASS with zero error), and
`helm install` smoke-test in a kind cluster using `ci/values-test.
yaml` profile (api + postgres + redis only) followed by `helm test`
invocation. [Trace: research §14.9, D8]

**REQ-DEPLOY-021 [Optional, P1]** — Where `ingress.enabled: true`,
the chart shall emit `networking.k8s.io/v1/Ingress` resources with
`ingressClassName: nginx` default and `cert-manager.io/cluster-
issuer: letsencrypt-prod` annotation default, both overridable, and
shall configure HSTS via `nginx.ingress.kubernetes.io/configuration-
snippet` to inject `Strict-Transport-Security: max-age=31536000;
includeSubDomains`. [Trace: research §6.6, D10]

**REQ-DEPLOY-022 [Ubiquitous, P1]** — Chart shall ship `helm test`
hook Job at `templates/jobs/smoke-test.yaml` annotated
`helm.sh/hook: test` executing `curl -fsS http://<api-service>:
<port>/healthz` and `curl -fsS http://<api-service>:<port>/metrics`
with success status required; the Job shall be discoverable via
`helm test universal-search` and shall PASS on a default `helm install`
invocation against a healthy cluster. [Trace: research §6.5]

### 2.3 P2 — Forward-compatibility + nice-to-have (2 REQs)

**REQ-DEPLOY-023 [Optional, P2]** — Where `secrets.backend:
"externalSecrets"`, the chart shall emit
`external-secrets.io/v1beta1/ExternalSecret` resources with
`refreshInterval: 1h` default and `secretStoreRef` + `target.name`
+ `data[].remoteRef` populated from values, and shall NOT require
External Secrets Operator (ESO) to be pre-installed (chart shall
detect CRD absence and surface a clear error in NOTES.txt rather
than fail mid-install). [Trace: research §9.3, D5]

**REQ-DEPLOY-024 [Ubiquitous, P2]** — Chart shall ship a compose ↔
chart parity smoke-test script at `scripts/compose-chart-parity.sh`
invoked in `chart-ci.yml`, comparing the set of environment
variables surfaced by `deploy/docker-compose.yml` and `.env.example`
against the union of values declared in `charts/universal-search/
values.yaml` and `values.schema.json`, with the build failing if
either side adds a variable not present in the other (modulo a
documented allowlist for genuinely chart-specific or compose-
specific knobs). [Trace: research §5, §14.4]

---

## 3. Non-Functional Requirements

**NFR-DEPLOY-001 [Fail-fast misconfig]** — `helm install` against
invalid values shall fail before any k8s resource is created, with
schema validation surfacing the specific path of the invalid value
(e.g. `usearch.api.replicas: must be integer >= 1`). No partial-
install state.

**NFR-DEPLOY-002 [Reproducible install]** — Identical chart version
+ identical values shall produce identical k8s resource manifests
across machines and time (modulo random Secret seed values which
shall be deterministically derived from a stable `helm.sh/release-
name` annotation only when `secrets.backend: "values"` is active).
Verified via CI step `helm template | sha256sum` reproducibility
check.

**NFR-DEPLOY-003 [Cold install time]** — On a 3-node k8s 1.30
cluster with cached images, `helm install universal-search` against
`ci/values-prod.yaml` shall reach all Deployments `Ready` within 5
minutes wall-clock time (excludes initial image pull which is
network-bound). Migration Job shall complete within 60 seconds for
the 9-file migration sequence on an empty database.

**NFR-DEPLOY-004 [Rollback support]** — `helm rollback <release>
<revision>` shall successfully reverse chart manifest changes
without manual intervention. Database schema rollback is OUT-OF-
SCOPE and explicitly documented as a manual operator procedure with
forward-fix migration as the recommended approach.

**NFR-DEPLOY-005 [Subchart pinning policy]** — Every entry in
`Chart.yaml` `dependencies:` shall pin an exact patch version (no
`~` or `^` ranges). Subchart version bumps shall undergo quarterly
audit with documented rationale recorded in `charts/universal-
search/CHANGELOG.md`.

**NFR-DEPLOY-006 [Image pull rate-limit awareness]** — Chart README
shall document Docker Hub anonymous-pull rate limit (100 pulls /
6h) impact when consuming Bitnami subchart images and shall provide
operator guidance for `global.imagePullSecrets` configuration and
internal registry mirror options.

**NFR-DEPLOY-007 [Multi-arch coverage]** — Container images shall
be published for `linux/amd64` and `linux/arm64` for every artifact
**except** `usearch-embedder` which shall be `linux/amd64` only due
to PyTorch + CUDA runtime constraints, with this exception
explicitly documented in chart README and a `nodeAffinity`
preventing the embedder Deployment from being scheduled onto
`arm64` nodes when the GPU feature is disabled.

**NFR-DEPLOY-008 [Cross-SPEC integration verification]** — Following
both SEC-001 implementation (`internal/security/secrets/Resolver`
interface ship) and DEPLOY-001 implementation, a documented
integration test shall verify that chart-deployed binaries
successfully resolve every secret declared in
`values.schema.json` via the SEC-001 Resolver, with the test
explicitly listed in DOC-001 `operators/security/secrets.mdx` as a
post-install verification step.

---

## 4. Scope Boundary

### 4.1 In Scope

- Helm v3 chart at `charts/universal-search/` (Chart.yaml, values.
  yaml, values.schema.json, templates/, ci/, README.md, CHANGELOG.md,
  NOTES.txt, Chart.lock)
- 3 new Dockerfile for usearch-api / usearch-mcp / usearch-migrate
  (deploy/Dockerfile.usearch-*)
- 3 new CI workflows (build-images.yml, chart-ci.yml, chart-release.
  yml)
- Subchart wiring for postgresql / redis / qdrant (Bitnami + official)
- In-chart custom Deployment for meilisearch / litellm / searxng
- Migration Job (golang-migrate + 9 SQL files) as pre-install Helm hook
- 3-tier secret strategy (values / existingSecret / externalSecrets)
- NetworkPolicy + PDB + HPA + ServiceMonitor defaults
- Multi-arch image (amd64+arm64; embedder amd64-only)
- cosign keyless signing (image + chart) + SBOM (syft SPDX) + SLSA L2
- kind cluster smoke-test CI gate
- Compose ↔ chart parity verification

### 4.2 Exclusions (What NOT to Build) [HARD]

본 SPEC scope **밖**의 항목 — 명시적으로 다른 SPEC 또는 post-V1로
deferred:

- **Terraform / OpenTofu / Pulumi IaC for cloud infrastructure
  (VPC, EKS/GKE/AKS cluster provisioning, RDS, ElastiCache)** —
  운영자 환경에 cluster + 기본 services가 이미 존재한다고 가정. cluster
  provisioning을 chart에 포함하면 multi-cloud × multi-IaC matrix가
  폭증.
- **Multi-tenant SaaS deployment (one chart instance serving multiple
  isolated tenants)** — V1 target은 single-team self-hosted. multi-
  tenancy는 SPEC-IDX-004의 application-layer로 처리; chart-layer는
  single-tenant.
- **Autoscaling tuning beyond baseline HPA** (예: KEDA event-driven
  scaling, VPA vertical autoscaler, custom metric scaling) — baseline
  CPU-based HPA만 V1 ship. 운영자가 KEDA/VPA 별도 install + 본인
  metric으로 reconfigure 가능.
- **ArgoCD ApplicationSet / Flux HelmRelease GitOps integration** —
  본 chart는 OCI 분배 + standard Helm shape 유지로 어떤 GitOps tool
  로도 deploy 가능. 단 GitOps-specific manifest (ApplicationSet YAML,
  Flux HelmRelease CRD)는 chart에 ship 안 함. V1 docs에 example만
  (post-V1에 별도 SPEC 가능).
- **Grafana dashboard JSON** — SPEC-EVAL-002 (adapter reliability
  dashboard) scope. 본 chart는 ServiceMonitor + metric endpoint
  exposure까지만.
- **Loki / Tempo / OpenTelemetry Collector deployment** — 운영자
  cluster-wide install 가정. chart는 endpoint reference만 (env-var).
- **cert-manager / ingress-nginx / external-secrets-operator
  subchart bundling** — cluster-wide singleton 요구 → 운영자 pre-
  install. chart README에 documented requirement.
- **Database schema down-migration as automatic rollback** — NFR-
  DEPLOY-004. forward-fix migration이 권장 경로; down은 수동.
- **macOS Docker Desktop deploy automation** — single-host dev는
  `make compose-up`이 담당 (BOOT-001 scope). chart는 multi-node k8s
  cluster 대상.
- **iOS/Android device deploy** — out of project scope (roadmap §6
  post-V1 backlog).
- **Federated multi-cluster deploy** — out of project scope.

### 4.3 Deferred to Post-V1

- Operator-SDK custom controller (federated multi-tenant)
- Helm + Kustomize hybrid (post-render hook) examples
- SLSA Level 3 (isolated builder) — hosted runner 한계
- Sentinel/Cluster Redis architecture support
- Meilisearch official subchart adoption
- KEDA event-driven autoscaling
- VPA vertical autoscaling

---

## 5. Test Scenarios (Given-When-Then)

본 섹션은 evaluator-active와의 Sprint Contract 협상 시 test scenarios
의 ground truth. 12 시나리오 (S1..S12).

### S1 — `helm lint` PASS

**Given** chart at `charts/universal-search/` with valid Chart.yaml +
values.yaml + templates/.
**When** `helm lint charts/universal-search` is executed.
**Then** exit code 0, no `[ERROR]` lines (warnings permitted).
[REQ-DEPLOY-001, REQ-DEPLOY-020]

### S2 — `helm template` against ci/values-prod.yaml

**Given** chart + `ci/values-prod.yaml`.
**When** `helm template universal-search charts/universal-search -f
charts/universal-search/ci/values-prod.yaml` is executed.
**Then** stdout contains valid YAML, no `<unset>` placeholders, no
`<error>` markers, all `imagePullSecrets` resolved, all
`secretKeyRef.name` references resolvable.
[REQ-DEPLOY-005, REQ-DEPLOY-008, REQ-DEPLOY-016]

### S3 — kubeconform against k8s 1.28..1.31

**Given** rendered manifests from S2.
**When** `kubeconform -kubernetes-version 1.28.0` (and 1.29, 1.30,
1.31) is invoked per manifest.
**Then** all manifests PASS schema validation against each k8s
version, exit code 0.
[REQ-DEPLOY-020]

### S4 — kind cluster smoke install

**Given** fresh kind cluster (k8s 1.30) + chart + `ci/values-test.
yaml` profile.
**When** `helm install usearch charts/universal-search -f
charts/universal-search/ci/values-test.yaml --wait --timeout 5m` is
executed.
**Then** `helm install` exits 0, all Deployments declared in profile
reach Ready state, migration Job completes successfully.
[REQ-DEPLOY-006, REQ-DEPLOY-020, NFR-DEPLOY-003]

### S5 — `helm test` smoke

**Given** successful install from S4.
**When** `helm test usearch` is executed.
**Then** smoke-test Pod completes successfully (`/healthz` 200 +
`/metrics` 200), exit code 0.
[REQ-DEPLOY-022]

### S6 — Schema validation rejects invalid values

**Given** invalid values file (e.g. `usearch.api.replicas: "two"`
[string instead of integer]).
**When** `helm install` is invoked.
**Then** install fails BEFORE any k8s resource is created, error
message identifies the specific schema path.
[REQ-DEPLOY-005, NFR-DEPLOY-001]

### S7 — Migration Job idempotency

**Given** chart installed + migration Job completed.
**When** `helm upgrade` is invoked with the same chart version
(no schema changes).
**Then** migration Job re-runs but completes successfully (idempotent
`golang-migrate up` on already-applied schema), no schema modification.
[REQ-DEPLOY-006, research §12.5]

### S8 — Helm rollback (chart manifest only)

**Given** chart installed at revision 1, upgraded to revision 2
(values change only, no migration).
**When** `helm rollback usearch 1` is executed.
**Then** revision 1 manifests are re-applied, Pods restart with
revision 1 config, migration Job runs again (idempotent).
[NFR-DEPLOY-004]

### S9 — Secret backend tier switch (values → existingSecret)

**Given** chart installed with `secrets.backend: "values"` (dev).
**When** values file is changed to `secrets.backend:
"existingSecret"` with pre-existing K8s Secret + `helm upgrade`.
**Then** chart removes self-managed Secret, Deployments restart with
secretKeyRef pointing to existing Secret, application functionality
preserved.
[REQ-DEPLOY-016]

### S10 — NetworkPolicy enforcement

**Given** chart installed with `networkPolicy.enabled: true` on a
cluster with Calico/Cilium CNI.
**When** a test Pod outside the chart namespace attempts to connect
to `usearch-api:8080`.
**Then** connection is denied by NetworkPolicy.
[REQ-DEPLOY-010]

### S11 — Multi-arch image manifest

**Given** image built via `.github/workflows/build-images.yml` for
tag `v1.0.0`.
**When** `docker buildx imagetools inspect ghcr.io/<org>/usearch-
api:1.0.0` is executed.
**Then** manifest list contains `linux/amd64` + `linux/arm64`
entries (embedder excluded — only amd64).
[REQ-DEPLOY-018, NFR-DEPLOY-007]

### S12 — Cosign signature verification

**Given** image at `ghcr.io/<org>/usearch-api:1.0.0`.
**When** `cosign verify ghcr.io/<org>/usearch-api:1.0.0 --certificate-
identity-regexp 'https://github.com/<org>/universal-search/'
--certificate-oidc-issuer 'https://token.actions.githubusercontent.
com'` is executed.
**Then** signature is verified, exit code 0.
[REQ-DEPLOY-018, D7]

---

## 6. Acceptance Gates

본 SPEC은 다음 acceptance gate 모두 PASS 시 release-ready:

| Gate | Verification | Threshold |
|------|--------------|-----------|
| **A1** Chart structure complete | `find charts/universal-search/templates -name '*.yaml' \| wc -l` | ≥ 30 template files |
| **A2** Schema validation strict | `helm lint --strict charts/universal-search` | exit 0, zero `[ERROR]` |
| **A3** Multi-k8s-version compat | `kubeconform -kubernetes-version <v>` for v ∈ {1.28..1.31} | all PASS |
| **A4** kind smoke install | `.github/workflows/chart-ci.yml` end-to-end | helm install + helm test PASS |
| **A5** Multi-arch image manifest | `docker buildx imagetools inspect` per image | amd64+arm64 (embedder amd64-only) |
| **A6** Cosign signature valid | `cosign verify` per image + chart | all PASS |
| **A7** SBOM attached | `cosign download attestation --predicate-type=spdx.dev/Document` | SPDX present for each image |
| **A8** Compose-chart parity | `scripts/compose-chart-parity.sh` | zero unexplained delta |
| **A9** OCI chart publish | `helm pull oci://ghcr.io/<org>/charts/universal-search` | chart downloadable |
| **A10** README + NOTES quality | manual review by manager-docs | DOC-001 cross-link integrity |
| **A11** TRUST 5 — Tested | helm-unittest + characterization tests | ≥ 85% coverage on scripts + helpers |
| **A12** TRUST 5 — Secured | gitleaks + Trivy + cosign | zero finding |
| **A13** TRUST 5 — Trackable | conventional commits + SPEC reference | every PR cites SPEC-DEPLOY-001 |

---

## 7. Risks + Mitigations

| ID | Risk | Likelihood | Impact | Mitigation |
|----|------|-----------|--------|-----------|
| R1 | Subchart version drift (Bitnami postgresql breaking change) | Medium | High | NFR-DEPLOY-005 quarterly audit + Chart.lock pinning. integration test catches breakage early. |
| R2 | Secret rotation race during rolling deploy (tier 2 existingSecret) | Medium | High | NOTES.txt documents rolling restart procedure. ESO tier 3 (post-V1 default-recommended) handles automatically. |
| R3 | Migration Job non-idempotent SQL | Low | High | PRESERVE phase grep audit (CREATE TABLE IF NOT EXISTS). characterization test re-applies migration on existing schema. |
| R4 | Docker Hub anonymous pull rate-limit on Bitnami subchart images | Medium | Medium | NFR-DEPLOY-006 documents `global.imagePullSecrets`. internal registry mirror guidance in DOC-001. |
| R5 | embedder amd64-only blocks arm64 cluster deploy | Low | Medium | NFR-DEPLOY-007 acknowledgment + nodeAffinity enforcement. arm64 운영자 docs guide (embedder external mode). |
| R6 | SEC-001 internal/security/secrets unimplemented blocks SEC-DEPLOY integration | High | Medium | chart-SEC decoupled via K8s Secret resource boundary (research §9.4). NFR-DEPLOY-008 cross-SPEC verification post-both-ship. |
| R7 | kind cluster smoke-test exceeds hosted runner resource limits | Medium | Medium | `ci/values-test.yaml` minimal profile (api + postgres + redis only). full-stack integration test on self-hosted runner (post-V1). |
| R8 | cert-manager / ingress-nginx pre-install assumption breaks user onboarding | Medium | Low | NOTES.txt + README explicit pre-install instructions. failure mode is `Ingress` resource created but TLS cert not issued — recoverable. |
| R9 | NetworkPolicy CNI absence silently no-ops | Medium | Medium | NOTES.txt warns. CI smoke-test runs on kind with CNI (verifies enforce). |
| R10 | OCI chart signing tooling (cosign helm support) version churn | Low | Low | cosign v2.4.0+ pin. helm-cosign integration tested in CI. |
| R11 | Bitnami chart maintainer policy change (recall like 2023) | Low | High | document fallback to operator (Zalando, CrunchyData) in CHANGELOG. external mode (`postgresql.enabled: false`) always available. |
| R12 | Multi-arch build fails for one Python sidecar (e.g. mecab-ko on arm64) | Medium | Medium | build-images.yml per-image matrix; individual sidecar arm64 disablement allowed via per-sidecar nodeAffinity. acknowledged in NFR-DEPLOY-007. |

---

## 8. Open Questions (for plan-auditor / annotation cycle)

본 SPEC draft가 implementation 전 해소해야 할 open question:

- **OQ1** — `storm` + `koreanews` Python sidecar의 dev-compose 통합
  상태가 명확하지 않음. roadmap M5 / M3 status update가 "implemented"
  로 표기되어 있으나 `deploy/docker-compose.yml`에 두 service 미존재.
  chart에서 `enabled: false` default로 정의했으나, V1.0.0 ship 시점
  운영자가 활성화하면 working state여야 함. **Verification**: run
  phase ANALYZE에서 두 service의 `/health` endpoint 응답 확인 +
  Dockerfile build 성공 확인.

- **OQ2** — `cmd/usearch-mcp` HTTP transport 모드의 port 표준. SPEC-
  MCP-001 draft 상태이므로 본 SPEC은 임의로 8081 default 가정. MCP-
  001 implementation 시 port 변경 시 chart values.schema.json + DOC-
  002 cross-validation 필요. **Mitigation**: values.yaml에 `mcp.port`
  변수로 추출 + schema에서 valid range 명시 (1024..65535).

- **OQ3** — `.env.example`이 OIDC env-var (`OIDC_ISSUER_URL`,
  `OIDC_CLIENT_ID` 등)를 누락하고 있음 (research §5.10). 본 SPEC에서
  chart values + ConfigMap으로 명세하면 자연스레 채워지지만, dev
  workflow (compose-up)에서도 OIDC 사용 가능하도록 `.env.example`
  sync 필요. **Mitigation**: 본 SPEC IMPROVE phase에서 `.env.example`
  update를 BOOT-001 retrospective amendment 없이 chart 작업의 일부로
  포함 (single PR이 두 파일 모두 수정).

- **OQ4** — Helm chart OCI signing 의 helm-cosign integration tooling
  matureness 평가. cosign v2.4.0의 helm OCI signature 검증은 third-
  party action에 의존 (예: `sigstore/cosign-installer` + manual
  `cosign sign-blob`). 공식 helm 명령에 내장될 timeline 미정.
  **Mitigation**: 본 SPEC은 cosign sign-blob fallback path를 chart-
  release.yml에서 명시 + DOC-001 운영자 검증 절차에 manual
  cosign 명령 documented.

- **OQ5** — embedder PVC storage class default. dev에서는 named volume
  (`embedder_models`) → docker driver storage. k8s에서는 PVC →
  StorageClass. chart values `global.storageClass: ""` (cluster
  default 신뢰)가 운영자에게 직관적이지만 production에서 cluster-
  default가 ephemeral storage인 경우 model load loop 발생. **Mitigation**:
  NOTES.txt + DOC-001 `operators/deployment/storage.mdx`에서 SSD-backed
  durable StorageClass 권장 명시. CI smoke-test은 kind의 default
  StorageClass (rancher.io/local-path) 신뢰.

- **OQ6** — `values.schema.json`의 adapter-key section vs DOC-002 MDX
  ground truth 결정. 본 SPEC은 schema를 ground truth로 선언 (DOC-002
  drift detection을 schema와의 cross-validation으로 정의). 단 DOC-002
  draft 상태이므로 adapter env-var 완전 목록이 미고정. **Mitigation**:
  본 SPEC schema는 `adapters.<name>.apiKey.existingSecret` extension
  point만 정의 (구체적 env-var 이름은 ConfigMap/Secret에서 명세);
  DOC-002 ship 후 cross-validation script (REQ-DEPLOY-024)에서 schema
  ↔ DOC-002 적합성 검증.

- **OQ7** — Bitnami subchart의 license 정책 변경 monitoring 책임자.
  2023 Bitnami License Update 이후 long-term maintainability 우려.
  **Mitigation**: NFR-DEPLOY-005 quarterly audit에 Bitnami license
  status 점검 포함. 변경 시 본 SPEC retrospective amendment로
  Zalando postgres-operator 또는 CrunchyData PGO 마이그레이션 plan
  반영.

---

**SPEC-DEPLOY-001 draft v0.1.0 — total: 24 EARS REQs (16 P0 + 6 P1 +
2 P2) + 8 NFRs + 12 test scenarios + 13 acceptance gates + 12 risks
+ 7 open questions. Companion: research.md (16 sections), plan.md
(DDD phased plan with Sprint Contract). Methodology: DDD (consolidation
of existing dev-compose surface + Helm-shaped IMPROVE). Coverage
target: 85%. Harness: thorough (P0 release-blocking + production
deploy + cross-SPEC integration). Owner: expert-devops.**
