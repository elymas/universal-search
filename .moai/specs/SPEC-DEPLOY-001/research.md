# SPEC-DEPLOY-001 Research — Helm chart for k8s team-scale deploy

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: DDD (ANALYZE existing dev-compose surface → PRESERVE
working dev workflow → IMPROVE with parameterized Helm chart layered on
top of binary + sidecar artifacts already built by SPEC-BOOT-001 /
SPEC-IDX-002 / SPEC-IDX-003 / SPEC-DEEP-001 / SPEC-SYN-001 / SPEC-ADP-009)

본 research artifact는 SPEC-DEPLOY-001 EARS draft를 작성하기 전 수행한
Phase 0.5 investigation 결과다. 14개 섹션 (§1 dev-compose surface
inventory, §2 Postgres migration inventory, §3 Go binary inventory,
§4 Python sidecar inventory, §5 env-var surface map, §6 Helm v3
pattern survey, §7 alternative chart tooling, §8 dependency strategy
comparison, §9 secret management options, §10 multi-arch + signing,
§11 observability integration, §12 migration job patterns, §13 comparable
OSS Helm charts audit, §14 open risks + unimplemented dependencies)으로
구성된다.

---

## §1 — Existing deploy surface inventory (dev-compose ground truth)

The canonical dev-compose stack lives at `deploy/docker-compose.yml`
(265 lines, version 3.9) with the GPU overlay at
`deploy/docker-compose.gpu.yml` (19 lines). This stack is what
`make compose-up` brings up and is the **only** working deploy path as
of 2026-05-22. It defines 12 services across 4 categories:

### §1.1 Stateful infrastructure (4 services, named volumes)

| Service | Image (pinned) | Volume | Ports | Healthcheck |
|---------|---------------|--------|-------|-------------|
| qdrant | `qdrant/qdrant:v1.16.3` | `qdrant_data` | 6333 HTTP / 6334 gRPC | `wget /readyz` |
| meilisearch | `getmeili/meilisearch:v1.42.1` | `meili_data` | 7700 | `wget /health` |
| postgres | `postgres:16.13-alpine3.23` | `pg_data` | 5432 | `pg_isready` |
| redis | `redis:7-alpine` | `redis_data` | 6379 | `redis-cli ping` |

이 4개 서비스는 **state를 보유**한다. Helm chart에서는 (a) chart 내장
subchart로 ship할지, (b) prereq로 operator/external service에 위임할지
의사결정 필요 (§8 참조). 본 SPEC의 D3 결정 사항.

### §1.2 Stateless infrastructure dependencies (2 services)

| Service | Image (pinned) | Notes |
|---------|---------------|-------|
| searxng | `searxng/searxng:2026.04.22-74f1ca203` | AGPL-3.0; service-boundary consumed (SPEC-DEP-001 license compliance) — pinned to dated tag + digest `sha256:37c616a774b90fb5df9239eb143f1b11866ddf7b830cd1ebcca6ba11b38cc2bf` |
| litellm | `ghcr.io/berriai/litellm:v1.83.7-stable.patch.1` | LLM proxy gateway; depends on postgres + redis |

SearXNG는 AGPL — Helm chart 내에서도 service boundary 유지. license
compliance은 SPEC-DEP-001 결정 사항 그대로 유지하며, chart subchart로
번들하지 않고 prereq로 separately install하는 옵션 + chart 내장 옵션
둘 다 제공 (single binary distribution을 deploy하는 self-hosted user의
편의 vs license-conscious enterprise의 separation 둘 다 충족).

### §1.3 Project-built Go runtime (1 service — to be defined in chart)

dev-compose에는 **Go runtime이 명시적으로 없다**. dev workflow는 host에서
`go run ./cmd/usearch-api` 또는 `make run-api`로 실행되며, container화는
chart 작업의 일부다. cmd/ 디렉토리 inventory:

| Binary | Source | LOC | Role | Container? |
|--------|--------|-----|------|-----------|
| `cmd/usearch/` | main.go (147 lines) + 13 supporting files | ~700 | CLI (SPEC-CLI-001 / CLI-002) | NO (CLI is end-user binary, distributed via release artifact — Helm chart does NOT package this) |
| `cmd/usearch-api/` | main.go (53 lines) + `handlers/` | ~250 | Admin server (`/metrics`, `/healthz`, `/deep`, `/synthesis` per `handlers/{deep.go, synthesis.go}`) | YES — primary Deployment in chart |
| `cmd/usearch-mcp/` | main.go (40 lines) | 40 | MCP server (SPEC-MCP-001 — drafted, not yet implemented at M9 draft time) | YES — secondary Deployment in chart (optional toggle via values) |

본 SPEC은 `cmd/usearch-api/` + `cmd/usearch-mcp/`를 chart의 primary
Deployment 두 개로 ship한다. CLI binary는 별도 release artifact (SPEC-
REL-001 scope)로 user host에 직접 배포; chart 안에 들어가지 않음.
chart는 또한 `usearch-api` Deployment의 OCI image build 절차를 정의해야
하지만 (현재 `deploy/`에 `Dockerfile.usearch-api`가 없음) Dockerfile
자체는 SPEC-BOOT-001 / 본 SPEC 두 군데 중 어느 곳에서 ship할지 결정 필요
(본 SPEC research §6.3 참조).

### §1.4 Python sidecars (5 services with own Dockerfile)

`services/` 디렉토리에 5개 Python FastAPI sidecar가 있고 각각 own
Dockerfile을 보유:

| Sidecar | Dockerfile | Port | SPEC origin | Role | Container exists |
|---------|-----------|------|------------|------|------------------|
| `services/researcher/` | Dockerfile | 8081 | SPEC-SYN-001, DEEP-002 | gpt-researcher wrap + multi-agent verifier | YES |
| `services/embedder/` | Dockerfile | 8082 | SPEC-IDX-002 | BGE-M3 inference; GPU overlay 지원 | YES |
| `services/tokenizer-ko/` | Dockerfile | 8083 | SPEC-IDX-003 | mecab-ko morphological analyzer | YES |
| `services/storm/` | Dockerfile | (TBD) | SPEC-DEEP-001 | STORM long-form report generation | YES (drafted) |
| `services/koreanews/` | Dockerfile | (TBD) | SPEC-ADP-009 | KoreaNewsCrawler wrap | YES (drafted) |

dev-compose에는 `researcher`, `embedder`, `tokenizer-ko`만 wire되어
있다 (lines 165-264). `storm` + `koreanews`는 Dockerfile만 존재하고
compose에는 미통합 — SPEC-DEEP-001 / ADP-009의 implementation status
확인 필요 (둘 다 implemented per roadmap §M5 / §M3 갱신 메모).

본 SPEC chart는 5개 sidecar 모두를 optional Deployment로 모델링 (per-
sidecar `enabled: true/false` toggle in values.yaml). embedder는 GPU
overlay와 동등한 k8s node affinity + `nvidia.com/gpu` resource request
sub-section을 제공해야 함 (dev-compose의 `deploy.resources.reservations.
devices` ↔ Helm `tolerations` + `nodeSelector` + resource request 매핑).

### §1.5 Observability service (1 service)

| Service | Image | Volume | Ports |
|---------|-------|--------|-------|
| prometheus | `prom/prometheus:v2.54.1` | `prometheus_data` | 9091:9090 |

chart에서는 prometheus를 in-chart로 ship하지 **않는다**. 결정 근거:
운영자 k8s 환경에는 이미 prometheus-operator + kube-prometheus-stack이
배포되어 있는 경우가 대부분; chart는 `ServiceMonitor` CRD만 생성하여
prometheus-operator가 scrape하게 함 (§11 observability integration 결정
참조). 단 chart values에 `prometheus.bundled: false` (default) /
`true` (self-contained option, kube-prometheus-stack subchart import)
flag를 두어 운영자 선택 가능하게 함.

### §1.6 Network topology + restart policy

dev-compose의 4개 cross-cutting setting:
- `networks: app: driver: bridge` — single bridge network. k8s에서는
  default Service-Service communication + NetworkPolicy로 대체.
- `restart: unless-stopped` — k8s에서는 Deployment의 default
  `RestartPolicy: Always` 매핑.
- Healthcheck `test:`, `interval:`, `timeout:`, `retries:`, `start_period:`
  — k8s에서는 `livenessProbe` / `readinessProbe` / `startupProbe`로 매핑.
  매핑 표는 §11.3 참조.
- `depends_on: condition: service_healthy` — k8s에서는 init-container
  + `wait-for-it` 패턴 또는 readiness gate로 대체.

---

## §2 — Postgres migrations inventory

`deploy/postgres/migrations/` 디렉토리에 9개 SQL 파일:

| Filename | Type | Origin SPEC | Purpose |
|----------|------|------------|---------|
| `0001_create_docs.sql` | DDL | SPEC-IDX-001 | base doc + index metadata tables |
| `0002_cost_ledger.sql` | DDL | SPEC-LLM-001 | per-request cost tracking |
| `0002_deep_runs.up.sql` | DDL | SPEC-DEEP-001 | /deep run state |
| `0002_deep_runs.down.sql` | DDL (down) | SPEC-DEEP-001 | rollback |
| `0003_audit_events.sql` | DDL | SPEC-AUTH-003 | audit event table (newly added, in git status untracked) |
| `0003_casbin_rules.up.sql` | DDL | SPEC-AUTH-002 | Casbin policy rules table |
| `0004_audit_cost_ledger_trigger.sql` | DDL/trigger | SPEC-AUTH-003 + LLM-001 | join audit events to cost ledger (newly added) |
| `0005_team_id_not_null.sql` | DDL/alter | SPEC-AUTH-002 | team_id NOT NULL constraint |
| `0006_user_id_column.sql` | DDL/alter | SPEC-AUTH-001 | user_id column for audit |
| `0007_answer_cache.up.sql` | DDL | SPEC-IDX-005 | team-shared answer reuse cache |

**Numbering collision** 주의: `0002_*` + `0003_*`가 각각 두 개 존재
(cost_ledger ↔ deep_runs, audit_events ↔ casbin_rules). 현재 dev workflow
는 두 파일을 모두 실행 — order-independent 가정 (실제로는 모두 CREATE
TABLE IF NOT EXISTS / ALTER TABLE 이므로 OK). Helm migration job은
모든 파일을 알파벳 순으로 실행하므로 collision은 무해; 단 plan-auditor
sign-off 시 numbering convention rebase 권유 (post-V1 cleanup).

migration runner 도구 후보:
- **`golang-migrate/migrate` v4.18.x** — Go-native, official Postgres
  driver, supports `up.sql` / `down.sql` convention, K8s Job-friendly
  (single-shot CLI). 본 SPEC 채택 (§12.2 참조).
- pressly/goose — Go-native, Go-embedded migrations 지원 (post-V1).
- Atlas (ariga.io/atlas) — declarative schema; 본 프로젝트 imperative
  migration 패턴과 mismatch.

migration job ownership 결정 (§12.4 참조): chart의 `pre-install` +
`pre-upgrade` Helm hook으로 Job 실행; `helm.sh/hook-weight` 로 ordering
guarantee.

---

## §3 — Go binary inventory + container artifact requirements

§1.3의 3개 binary 중 chart container화 대상은 `usearch-api` +
`usearch-mcp`. CLI는 release artifact로 별도 배포.

### §3.1 cmd/usearch-api binary

`cmd/usearch-api/main.go` 53 lines. handlers/ 디렉토리에 `deep.go`,
`synthesis.go` + 관련 테스트. http server listen에 `USEARCH_ADMIN_PORT`
사용 (.env.example line 60: "usearch admin server port for /metrics and
/healthz (localhost-only)"). 본 chart는 다음 사항을 반영해야 함:
- **localhost-only 제거**: dev에서는 localhost-only (`USEARCH_ADMIN_PORT`
  blank 가능); production에서는 ClusterIP Service + NetworkPolicy로 다른
  in-cluster service만 접근 가능. (REQ-DEPLOY-010 / D9 NetworkPolicy 참조).
- **graceful shutdown**: SIGTERM 처리는 main.go 확인 필요 (run phase에서
  manager-ddd가 검증). 없으면 PRESERVE 단계에서 characterization test +
  IMPROVE에서 fix.
- **config 표면**: 모든 환경변수가 viper / pflag 또는 직접 `os.Getenv`로
  주입됨. chart는 ConfigMap (non-secret) + Secret (sensitive) 두 채널로
  주입 (§9 secret strategy).

### §3.2 cmd/usearch-mcp binary

`cmd/usearch-mcp/main.go` 40 lines (stub draft per SPEC-MCP-001 draft
status). MCP는 stdio transport이지만 SPEC-MCP-001은 HTTP transport도
선택 가능 — chart는 HTTP-mode일 때만 Deployment + Service 생성, stdio-
only일 때는 chart에서 제외 (운영자가 별도 sidecar로 attach).

### §3.3 Dockerfile ownership question

현재 `deploy/`에 Go binary용 Dockerfile이 **없음**. chart 작업은 다음
중 하나를 선택:
- (a) `deploy/Dockerfile.usearch-api` + `deploy/Dockerfile.usearch-mcp`
  를 본 SPEC scope으로 신설. multi-stage build (golang:1.24.x → distroless/
  static-debian12), multi-arch (linux/amd64 + linux/arm64) via `docker
  buildx`. CI workflow `.github/workflows/build-images.yml` (new) 에서
  매 release + main merge 시 ghcr.io에 push.
- (b) Dockerfile은 SPEC-BOOT-001 scope으로 위탁 (BOOT-001 retrospective
  amendment). 본 SPEC은 chart values에서 image reference만 정의.

**결정**: (a) 채택. 근거: BOOT-001은 이미 implemented + closed; 본
SPEC이 release-blocking M9 SPEC이므로 Dockerfile authoring을 본 SPEC
scope에 포함해 ownership을 단일화. REQ-DEPLOY-002 + REQ-DEPLOY-003에서
Dockerfile 명세화.

---

## §4 — Python sidecar Dockerfile inventory

5개 sidecar 각각의 Dockerfile은 이미 작성됨 (per §1.4). 본 SPEC은
**Dockerfile을 새로 작성하지 않는다** — IMPROVE 단계에서 각 Dockerfile이
다음 chart 호환 조건을 만족하는지 audit만 수행:

| 조건 | 검증 방법 | 위반 시 조치 |
|------|----------|------------|
| Non-root USER 지정 | `grep -E '^USER ' services/*/Dockerfile` | sidecar 소관 SPEC에 amendment 권유 (out-of-scope for DEPLOY-001) |
| `EXPOSE <port>` 명시 | grep EXPOSE | 동상 |
| HEALTHCHECK 또는 `/health` HTTP endpoint | dev-compose에서 사용 중인 healthcheck 그대로 livenessProbe로 매핑 | 매핑 불가 sidecar는 chart에서 livenessProbe만 (readinessProbe는 healthcheck-가능 sidecar 한정) |
| Multi-arch capability | `docker buildx ls` → linux/amd64,linux/arm64 | embedder는 ML model + torch — arm64에서 정상 동작 안 할 가능성. embedder만 amd64 한정으로 표시 (NFR-DEPLOY-007 acknowledgment) |

embedder GPU 옵션은 §1.4에서 언급한 대로 dev-compose의 `nvidia` device
reservation을 chart values의 `embedder.gpu.enabled: true` toggle로
매핑. GPU node 부재 시 `enabled: false` (CPU fallback per `EMBEDDER_DEVICE:
cpu`).

---

## §5 — Env-var surface map (`.env.example` audit)

`.env.example`는 deploy/docker-compose.yml의 모든 `${VAR}` reference를
포함한다 (compose-build invariant per docs/dependencies.md). 총 50+
변수를 카테고리별로 정리:

### §5.1 Stateful infra credentials (5 vars — SECRET)

```
MEILI_MASTER_KEY              # Meilisearch master key
POSTGRES_USER                  # PG username
POSTGRES_PASSWORD              # PG password
POSTGRES_DB                    # PG database name (not strictly secret but couples to DSN)
SEARXNG_SECRET                 # SearXNG signing secret
```

chart에서는 K8s Secret으로 주입. `helm install` 시 `--set
existingSecret=usearch-secrets` 또는 ESO ExternalSecret resource로
주입 가능.

### §5.2 Stateful infra ports (5 vars — non-secret)

```
QDRANT_HTTP_PORT (6333)
QDRANT_GRPC_PORT (6334)
MEILI_PORT (7700)
POSTGRES_PORT (5432)
REDIS_PORT (6379)
SEARXNG_PORT (8080)
```

chart에서는 ConfigMap 또는 values.yaml 기본값. 운영자 변경 가능.

### §5.3 LLM proxy + provider keys (5 vars — SECRET)

```
LITELLM_MASTER_KEY             # LiteLLM proxy master key (SECRET)
LITELLM_BUDGET_USD (0.50)      # per-request budget cap (non-secret)
DATABASE_URL                    # Postgres DSN for LiteLLM (DERIVED)
REDIS_URL                       # Redis DSN for LiteLLM (DERIVED)
OPENAI_API_KEY                  # provider key (SECRET, may be empty)
ANTHROPIC_API_KEY               # provider key (SECRET, may be empty)
OLLAMA_BASE_URL                 # local Ollama URL (non-secret)
```

DATABASE_URL + REDIS_URL은 chart helper template (`_helpers.tpl`)에서
postgres / redis subchart의 service name + secret-referenced password로
조립.

### §5.4 Observability (6 vars — non-secret)

```
LOG_LEVEL (info)
USEARCH_ADMIN_PORT (9090)
OTLP_ENDPOINT                   # OTLP gRPC endpoint
OTLP_SAMPLE_RATIO (0.1)
LOKI_ENDPOINT                   # Phase 2 reserved
PROMETHEUS_PORT (9091)
PROMETHEUS_SCRAPE_PORT (9090)
```

chart는 OTLP_ENDPOINT default를 cluster-internal OTLP collector
service (e.g. `otel-collector.observability.svc.cluster.local:4317`)로
주입 가능; LOKI_ENDPOINT는 V1 unused.

### §5.5 Researcher sidecar (4 vars — non-secret)

```
RESEARCHER_BASE_URL
RESEARCHER_PORT (8081)
RESEARCHER_MODEL_DEFAULT (claude-haiku-4-5)
RESEARCHER_TIMEOUT_SECONDS (8)
```

chart는 RESEARCHER_BASE_URL을 sidecar Service ClusterIP에 자동 wire.

### §5.6 Embedder sidecar (7 vars — non-secret)

```
EMBEDDER_BASE_URL
EMBEDDER_REQUEST_TIMEOUT_SECONDS (15)
EMBEDDER_PORT (8082)
EMBEDDER_MODEL_NAME (BAAI/bge-m3)
EMBEDDER_DEVICE (cpu)            # cpu | cuda
EMBEDDER_USE_FP16 (false)
EMBEDDER_CACHE_MAX_ENTRIES (1024)
EMBEDDER_LOG_LEVEL (INFO)
```

EMBEDDER_DEVICE = `cuda`는 chart values `embedder.gpu.enabled: true` ↔
nodeSelector + resource limit.

### §5.7 Tokenizer-ko sidecar (5 vars — non-secret)

```
TOKENIZER_KO_BASE_URL
TOKENIZER_KO_TIMEOUT_MS (500)
TOKENIZER_KO_MAX_RETRIES (2)
TOKENIZER_KO_PORT (8083)
TOKENIZER_KO_LOG_LEVEL (INFO)
```

### §5.8 Deep agent + tree (12+ vars — non-secret)

```
DEEP_AGENT_RESEARCHER_MODEL
DEEP_AGENT_REVIEWER_MODEL
DEEP_AGENT_WRITER_MODEL
DEEP_AGENT_VERIFIER_MODEL
DEEP_AGENT_MAX_RETRIES (2)
DEEP_AGENT_WRITER_RETRY_DELAY_MS (500)
DEEP_AGENT_VERIFIER_TIMEOUT_MS (30000)
DEEP_AGENT_FAITHFULNESS_URL
DEEP_TREE_ENABLED (false)
DEEP_TREE_DEFAULT_BREADTH (4)
DEEP_TREE_DEFAULT_DEPTH (3)
DEEP_TREE_TOKEN_BUDGET (60000)
DEEP_TREE_ROOT_TOKEN_ESTIMATE (5000)
DEEP_TREE_NODE_TIMEOUT_MS (30000)
DEEP_TREE_DECOMPOSE_MODEL
DEEP_TREE_PERSISTENCE_DIR (.moai/runs)
```

chart는 DEEP_TREE_PERSISTENCE_DIR을 PersistentVolumeClaim mount path
(예: `/var/lib/usearch/runs`)로 override (production에서 ephemeral
directory 부적절).

### §5.9 Future env-vars (forward-compatibility)

SPEC-DOC-002 (adapter reference) draft에서 추가될 예정인 adapter API
key vars:
- `REDDIT_CLIENT_ID`, `REDDIT_CLIENT_SECRET` (SPEC-ADP-001)
- `GITHUB_TOKEN` (SPEC-ADP-004)
- `YOUTUBE_API_KEY` (SPEC-ADP-005)
- `NAVER_CLIENT_ID`, `NAVER_CLIENT_SECRET` (SPEC-ADP-008)
- `BLUESKY_APP_PASSWORD` (SPEC-ADP-006)
- 기타

본 chart는 모든 adapter key를 K8s Secret의 optional key로 모델링; chart
values `adapters.<name>.apiKey.existingSecret`로 운영자가 외부 Secret
참조 가능. ADP-001..009가 implemented되어 있으므로 (per roadmap)
chart는 모든 adapter env-var를 V1에서 완전 표현해야 함. DOC-002의
adapter-env-var-reference는 본 chart의 values.schema.json에 의해 enforce
(drift detection — values.schema.json이 ground truth, DOC-002 MDX가 사
람용 표면).

### §5.10 OIDC + auth (NEW — currently not in .env.example, SPEC-AUTH-001 implemented)

`internal/auth/config.go` 분석 시 추가 env-var 필요:
- `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`,
  `OIDC_REDIRECT_URL`, `OIDC_SCOPES`
- `JWT_SIGNING_KEY` (HS256) 또는 `JWT_PUBLIC_KEY_URL` (RS256/JWKS)
- `SESSION_SECRET` (cookie HMAC)

`.env.example`이 OIDC 변수를 누락하고 있음 — AUTH-001 implementation
이후 `.env.example` sync가 빠진 것으로 추정. 본 SPEC scope에서 chart
ConfigMap + Secret으로 명세화하면 자연스레 채워짐 (DOC-002 adapter-env-
ref 작업에서 cross-validation).

---

## §6 — Helm v3 chart pattern survey

본 SPEC은 **Helm v3 chart only**를 채택 (D1 결정 참조). Helm v3.16+의
다음 capabilities를 활용:

### §6.1 Chart.yaml apiVersion v2 (Helm 3 native)

```yaml
apiVersion: v2
name: universal-search
description: Universal search engine — k8s deploy for team scale
type: application
version: 0.1.0          # chart version (semver, separate from binary)
appVersion: "0.1.0"     # binary version (V1 release == "1.0.0")
kubeVersion: ">=1.28-0 <1.32-0"
home: https://github.com/<org>/universal-search
sources:
  - https://github.com/<org>/universal-search
maintainers:
  - name: <maintainer>
    email: <email>
icon: https://<docs-site>/logo.svg     # SPEC-DOC-001 docs site
annotations:
  artifacthub.io/license: <project-license>
  artifacthub.io/changes: |
    - Initial chart release
```

`appVersion` ↔ chart binary version pinning policy (D7 결정 참조):
chart `appVersion`은 chart에 reference된 image tag와 동일 (예: 둘 다
"1.0.0"). chart `version`은 chart artifact 자체의 semver — chart-only
변경 (예: values.schema.json 보강)은 patch bump, binary 변경 동반 시
minor 이상.

### §6.2 values.yaml schema (values.schema.json)

Helm 3은 `values.schema.json` (JSON Schema Draft-07) 지원. 본 SPEC은
schema validation을 **mandatory**로 설정:
- chart install 시 `helm install` 명령이 values를 schema 검증 — invalid
  config는 install 전 reject (NFR-DEPLOY-001 fail-fast).
- `helm lint` CI gate에서 schema 자체의 well-formedness 검증.
- IDE auto-complete (예: VSCode YAML extension) — 운영자 UX 향상.

schema는 §6.6 values.yaml 구조 참조. additionalProperties: false strict
mode (오타 방지). 단 known forward-compat extension point (예:
`extraEnv: []`)는 명시적으로 schema에 포함.

### §6.3 templates/ directory (per-service Deployment + Service + ...)

본 chart는 multi-service이므로 `templates/` 디렉토리를 service별
sub-directory로 구조화:
```
templates/
  _helpers.tpl                 # 공통 helper templates (name, labels, image refs)
  NOTES.txt                    # helm install 완료 시 출력 — kubectl port-forward 명령, OIDC redirect URI 가이드
  api/
    deployment.yaml            # usearch-api Deployment
    service.yaml               # ClusterIP
    serviceaccount.yaml
    hpa.yaml                   # HorizontalPodAutoscaler
    pdb.yaml                   # PodDisruptionBudget
    networkpolicy.yaml
    servicemonitor.yaml        # Prometheus ServiceMonitor
    ingress.yaml               # cert-manager + ingress-nginx
    configmap.yaml             # non-secret config
    secret.yaml                # SECRET — populated by helm with sensitive values OR existingSecret reference
  mcp/
    deployment.yaml
    service.yaml
    # mcp는 ingress 없음 (cluster-internal)
  researcher/
    deployment.yaml
    service.yaml
    pdb.yaml
  embedder/
    deployment.yaml
    service.yaml
    pvc.yaml                   # HuggingFace model cache PVC (`embedder_models` volume equivalent)
  tokenizer-ko/
    deployment.yaml
    service.yaml
  storm/
    deployment.yaml
    service.yaml
  koreanews/
    deployment.yaml            # DaemonSet 또는 CronJob 가능성 (crawler 특성) — SPEC-ADP-009 design 확인 필요
    service.yaml
  jobs/
    migrate.yaml               # pre-install/pre-upgrade Job (golang-migrate + 9 SQL files)
    smoke-test.yaml            # post-install Helm Test hook
  litellm/
    deployment.yaml            # bundled by default; toggle via `litellm.bundled: true|false`
    service.yaml
    configmap.yaml             # /app/config.yaml content (LiteLLM provider routing table)
  searxng/
    deployment.yaml            # AGPL — chart docs explicitly call out license; subchart NOT used (service-boundary preserved)
    service.yaml
    configmap.yaml             # settings.yml content
```

### §6.4 Subchart dependencies (Chart.yaml `dependencies:`)

```yaml
dependencies:
  - name: postgresql
    version: "16.4.x"                       # Bitnami chart version
    repository: oci://registry-1.docker.io/bitnamicharts
    condition: postgresql.enabled
    alias: postgres
  - name: redis
    version: "20.x.x"
    repository: oci://registry-1.docker.io/bitnamicharts
    condition: redis.enabled
  - name: qdrant
    version: "1.x.x"                        # official qdrant chart (qdrant/qdrant-helm-chart)
    repository: https://qdrant.github.io/qdrant-helm
    condition: qdrant.enabled
  - name: meilisearch
    version: "1.x.x"                        # meilisearch/meilisearch-kubernetes chart
    repository: https://meilisearch.github.io/meilisearch-kubernetes
    condition: meilisearch.enabled
  # kube-prometheus-stack은 optional subchart NOT bundled by default
  # (운영자가 cluster-wide로 설치된 prometheus-operator를 사용한다고 가정).
  # external-secrets-operator도 동상 (optional, NOT bundled).
```

각 subchart `condition`은 values에서 `<name>.enabled`로 토글. 운영자가
external Postgres operator (예: zalando, CrunchyData) 사용 시 `postgres.
enabled: false` + `postgres.external.host`로 외부 호스트 주입.

### §6.5 Helm hooks (Job ordering)

`pre-install`, `pre-upgrade`, `post-install`, `post-upgrade`,
`pre-delete`, `test` hooks 활용:

- **`pre-install` + `pre-upgrade`** (`hook-weight: -5`) — migration Job
  (golang-migrate). 모든 migration이 완료될 때까지 install 차단.
- **`post-install` + `post-upgrade`** — none (자동 reconciliation 신뢰).
- **`test`** (`helm.sh/hook: test`) — smoke test Job: usearch-api
  `/healthz` + `/metrics` curl 검증; embedder `/health` 검증.
- **`pre-delete`** — none (k8s GC 자동).

hook resources에 `helm.sh/hook-delete-policy: hook-succeeded` 적용
(성공 Job 자동 정리).

### §6.6 values.yaml 표면 (top-level keys, schema sketch)

```yaml
global:
  imageRegistry: ghcr.io/<org>
  imagePullSecrets: []
  storageClass: ""              # 빈 문자열 = cluster default

image:
  pullPolicy: IfNotPresent
  tag: "1.0.0"                  # chart appVersion과 동일

usearch:
  api:
    enabled: true
    replicas: 2
    image:
      repository: ghcr.io/<org>/usearch-api
    resources:
      requests: { cpu: 100m, memory: 256Mi }
      limits:   { cpu: 1, memory: 1Gi }
    hpa:
      enabled: true
      minReplicas: 2
      maxReplicas: 10
      targetCPUUtilizationPercentage: 70
    pdb:
      enabled: true
      minAvailable: 1
    service:
      type: ClusterIP
      port: 8080
    ingress:
      enabled: false            # default disabled
      className: nginx
      annotations:
        cert-manager.io/cluster-issuer: letsencrypt-prod
      hosts:
        - host: usearch.example.com
          paths: [{path: /, pathType: Prefix}]
      tls:
        - secretName: usearch-tls
          hosts: [usearch.example.com]
    config:
      logLevel: info
      otlpEndpoint: ""           # cluster-internal OTLP collector URL
      otlpSampleRatio: "0.1"
    auth:
      oidc:
        issuerUrl: ""
        clientIdSecretRef: { name: "", key: "" }
        clientSecretSecretRef: { name: "", key: "" }
        scopes: "openid profile email"
    networkPolicy:
      enabled: true
      ingressFrom:
        - namespaceSelector: {matchLabels: {app: ingress-nginx}}
      egressTo:
        - to: postgres
        - to: redis
        - to: qdrant
        # ...
  mcp:
    enabled: false              # default off; HTTP mode is opt-in
    # ...

researcher:
  enabled: true
  # ...
embedder:
  enabled: true
  gpu:
    enabled: false
    nodeSelector: { "nvidia.com/gpu.product": "" }
    tolerations: []
  # ...
tokenizer-ko:
  enabled: true
storm:
  enabled: false                # SPEC-DEEP-001 stub at draft time
koreanews:
  enabled: false                # SPEC-ADP-009 stub

litellm:
  enabled: true
  bundled: true                 # false → external LiteLLM service
  config: {}                    # entire config.yaml content (inline)

searxng:
  enabled: true
  bundled: true                 # AGPL warning surfaced in NOTES.txt

postgres:
  enabled: true                 # default bundled Bitnami subchart
  external:
    host: ""
    port: 5432
    existingSecret: ""
  auth:
    database: usearch
    username: usearch
    existingSecret: ""
  primary:
    persistence:
      enabled: true
      size: 50Gi

redis:
  enabled: true
  external: {host: "", port: 6379, existingSecret: ""}
  architecture: standalone
  master:
    persistence:
      enabled: true
      size: 10Gi

qdrant:
  enabled: true
  external: {host: "", port: 6333}
  persistence: {enabled: true, size: 50Gi}

meilisearch:
  enabled: true
  external: {host: "", port: 7700, existingSecret: ""}
  persistence: {enabled: true, size: 20Gi}

migrations:
  enabled: true
  image:
    repository: ghcr.io/<org>/usearch-migrate
    tag: "1.0.0"

secrets:
  # 3-tier secret strategy (D5 결정 참조):
  # - "values": chart values에 secret 직접 작성 → K8s Secret 자동 생성 (dev/staging)
  # - "existingSecret": 운영자가 사전 생성한 K8s Secret을 reference (production small-team)
  # - "externalSecrets": ExternalSecretsOperator의 ExternalSecret CRD 생성 (production enterprise)
  backend: "existingSecret"     # 권장 default
  values:                       # backend=values 일 때만 사용
    meiliMasterKey: ""
    postgresPassword: ""
    # ...
  existingSecret:
    name: "usearch-secrets"
    keys:
      meiliMasterKey: "MEILI_MASTER_KEY"
      postgresPassword: "POSTGRES_PASSWORD"
      # ...
  externalSecrets:
    enabled: false
    refreshInterval: "1h"
    secretStoreRef: { name: "vault-backend", kind: "ClusterSecretStore" }
    remoteKeys: {meiliMasterKey: "secret/data/usearch/meili"}

observability:
  serviceMonitor:
    enabled: true                # prometheus-operator 존재 가정
    interval: "30s"
    scrapeTimeout: "10s"
  otlp:
    enabled: false               # OTLP_ENDPOINT 활성 시 true
    endpoint: ""
```

schema validation: 모든 항목 type + enum + pattern 검증; secrets.backend
는 enum ["values", "existingSecret", "externalSecrets"].

---

## §7 — Alternative chart tooling considered

본 SPEC은 Helm v3을 채택했지만 대안을 명시적으로 평가한다 (D1 rationale
근거 자료).

### §7.1 Kustomize (raw manifests + overlays)

- 장점: 추가 도구 없음 (kubectl 내장); 단순; declarative.
- 단점: parameterization 빈약 (patches 누적 시 가독성 폭락); values
  schema validation 없음; helm ecosystem의 OCI 분배 + 서명 인프라 부재.
- **Rejected**: 12+ service multi-deploy에 patch hell 우려. values.yaml의
  flat config + helper template이 훨씬 가독.

### §7.2 Helm + Kustomize hybrid (post-render hook)

- 장점: helm의 templating + kustomize의 final patching.
- 단점: 두 도구 학습 부담 + debugging 복잡 (post-render는 chart-author
  invisible).
- **Rejected for V1**: post-V1로 deferred (advanced 운영자가 필요 시
  본인 환경에서 `helm template | kustomize build` chain 직접 구성 가능).

### §7.3 Operator-SDK (Custom Resource + Controller)

- 장점: complex state machine (예: 무중단 schema migration coordination)
  표현력 우수; CR로 declarative API.
- 단점: V1 대상 user (small team self-hosted)에는 operator 학습 부담
  과다; operator 자체의 lifecycle 관리 부담; 우리 use case는 Deployment +
  Job + Service 정도로 충분 (stateful 부분은 Bitnami subchart에 위탁).
- **Rejected for V1**: deferred to post-V1 (Federated multi-tenant SaaS
  운영 시 재검토).

### §7.4 Carvel kapp + ytt

- 장점: schema validation 강력 (ytt overlay); kapp의 transactional
  apply.
- 단점: Helm ecosystem 대비 사용자 base + tooling 빈약; OCI 분배 ecosystem
  Helm 대비 약함; 운영자 익숙도 낮음.
- **Rejected**.

### §7.5 ArgoCD ApplicationSet / Flux HelmRelease

- 본 SPEC scope 밖. GitOps는 chart의 **소비자**이지 chart 자체가 아님.
  본 chart는 ArgoCD / Flux 어느 GitOps 도구로도 deploy 가능하도록 OCI
  분배 + standard Helm CRD shape 유지. GitOps integration 가이드는 V1
  docs에 단순 example로만 (post-V1에 별도 SPEC 가능).

### §7.6 Conclusion

Helm v3 native chart가 (a) 운영자 학습 부담 최소화, (b) ecosystem
tooling (helm-docs, chart-testing, kubeval, kubeconform, helm-secrets,
helm-diff, etc.) 활용, (c) OCI 분배 + cosign 서명 무료 통합, (d)
prometheus-operator / ESO / cert-manager 등 cluster-wide operator와
clean하게 통합하는 점에서 최적.

---

## §8 — Dependency strategy comparison (4 stateful services)

D3 결정 — Postgres / Redis / Qdrant / Meilisearch 각각에 대한 chart-
inclusion 전략.

### §8.1 Postgres

| 전략 | 장점 | 단점 | 채택 |
|------|------|------|------|
| Bitnami `bitnami/postgresql` subchart | 성숙; PVC + Secret + Service 표준; well-maintained; OCI 분배 | Bitnami 라이선스 변경 이력 (2023) 모니터링 필요; production HA 부재 (replica는 별도 chart) | YES (default) |
| Zalando postgres-operator | production HA + backup + failover; operator-managed | operator pre-install 필요; learning curve | NO (V1); post-V1 옵션 |
| CrunchyData PGO | 동상 | 동상 + 라이선스 (PGO Apache 2.0 OK) | NO (V1) |
| External (운영자 제공) | 가장 유연; production 환경에서는 외부 RDS/Cloud SQL이 일반적 | chart는 DSN reference만; 운영자가 별도 provision | YES (alternative — `postgres.enabled: false` + `postgres.external.*`) |

본 SPEC: **default bundled Bitnami subchart** + **opt-out to external**.
production-HA-critical 운영자는 operator 별도 install 후 external 모드
사용.

### §8.2 Redis

dev-compose는 `redis:7-alpine` standalone. 본 SPEC chart:
- default: Bitnami `bitnami/redis` subchart, architecture: standalone.
- production: 운영자가 Redis Cluster 또는 ElastiCache 사용 가능
  (`redis.enabled: false` + `redis.external.*`).

Sentinel architecture는 V1에서 untested; production HA가 필요한 운영자는
external mode 권장.

### §8.3 Qdrant

dev-compose: `qdrant/qdrant:v1.16.3` single-node. 본 SPEC chart:
- official `qdrant/qdrant-helm-chart` (https://qdrant.github.io/qdrant-
  helm) 사용. 활성 maintained; cluster mode 지원 (V1에서는 single-node).
- alternative: `qdrant.enabled: false` + external Qdrant Cloud 등.

### §8.4 Meilisearch

dev-compose: `getmeili/meilisearch:v1.42.1`. 본 SPEC chart:
- official `meilisearch/meilisearch-kubernetes` chart 사용 OR
  custom in-chart Deployment (Meilisearch는 single-node 운영이 일반적).
- V1 결정: **custom in-chart Deployment + StatefulSet** (외부 chart의
  pin lag 회피). subchart도입 시 maintenance overhead 검토 후 post-V1
  결정.

### §8.5 LiteLLM + SearXNG

§1.2 참조. 둘 다 in-chart custom Deployment + 운영자 toggle
(`enabled: true|false`). subchart 없음 (각 프로젝트의 official Helm
chart maturity 낮음 — V1에서는 fork 부담 회피).

### §8.6 Risk: subchart version drift

bundled subchart (postgres, redis, qdrant)는 upstream chart 버전이
변경되면 본 chart도 sync 필요. 본 SPEC NFR-DEPLOY-005 (sub-chart pinning
policy): chart.yaml `dependencies[].version`은 정확한 patch 버전 고정
(예: `16.4.5`, not `^16.4.0`). 분기별 audit + manual bump.

---

## §9 — Secret management options (D5 결정 자료)

3-tier strategy (SEC-001 D5와 정렬). 각 tier의 chart-side 구현:

### §9.1 Tier 1 — values-injected secrets (dev/CI 한정)

`values.yaml`에 직접 secret 값 작성 → chart가 K8s Secret resource를
자동 생성:
```yaml
secrets:
  backend: "values"
  values:
    meiliMasterKey: "dev_key_change_me"
    postgresPassword: "dev_pw"
```

- **장점**: 1-command install (dev demo).
- **단점**: production 사용 절대 금지 — git committed values.yaml에 plain
  secret 노출. NOTES.txt + chart README에 명시적 경고.
- chart-side: `templates/api/secret.yaml`에서 `if .Values.secrets.backend
  == "values"` 블록.

### §9.2 Tier 2 — existingSecret reference (production small-team)

운영자가 `kubectl create secret generic usearch-secrets --from-literal=
MEILI_MASTER_KEY=...`로 사전 생성:
```yaml
secrets:
  backend: "existingSecret"
  existingSecret:
    name: "usearch-secrets"
    keys:
      meiliMasterKey: "MEILI_MASTER_KEY"
      postgresPassword: "POSTGRES_PASSWORD"
```

- **장점**: secret을 git에서 분리; 표준 K8s 패턴.
- **단점**: rotation은 운영자가 별도 수행 (kubectl apply); rolling deploy
  와 race condition 가능 (§14 risk 참조).
- chart-side: Deployment env에 `valueFrom.secretKeyRef`로 reference.

### §9.3 Tier 3 — External Secrets Operator (production enterprise)

ESO가 Vault / AWS Secrets Manager / GCP Secret Manager 등에서 fetch:
```yaml
secrets:
  backend: "externalSecrets"
  externalSecrets:
    enabled: true
    refreshInterval: "1h"
    secretStoreRef: {name: "vault-backend", kind: "ClusterSecretStore"}
    remoteKeys:
      meiliMasterKey: "secret/data/usearch/prod#meili_master_key"
      postgresPassword: "secret/data/usearch/prod#postgres_password"
```

- **장점**: rotation 자동화; audit trail; least-privilege backend access.
- **단점**: ESO operator pre-install 필요 (chart가 require하지 않고
  detect-only — ESO CRD 없으면 chart install 실패).
- chart-side: `templates/api/externalsecret.yaml` (only when backend=
  externalSecrets) 생성; ESO가 ExternalSecret을 reconcile하여 K8s Secret
  자동 생성.

### §9.4 SEC-001 internal/security/secrets/ runtime abstraction과의 통합

SEC-001 D5 (현재 draft)에서 `internal/security/secrets/Resolver` interface
정의 — env / K8s / Vault backend 추상화. 본 chart는 runtime-side
secret resolution을 binary에 위탁; chart는 secret을 어떤 backend에서
오든 **K8s Secret resource까지만** 책임지고, 그 Secret을 Deployment env
또는 file mount로 노출. binary 안에서 SEC-001 Resolver가 그 표면을
읽어 unified API로 코드에 제공.

따라서 본 chart는 SEC-001 Resolver의 unimplemented 상태 (currently
draft)와 **decoupled** — chart는 env-var 주입까지만 책임지므로 SEC-001
implementation이 지연되어도 chart는 ship 가능. 단 docs (SPEC-DOC-001
`operators/security/secrets.mdx`)에서 SEC-001 unimplemented 상태를
명시하고, V1.0.0 시점 권장 backend는 "existingSecret" tier 2.

---

## §10 — Multi-arch + signing (D7 결정 자료)

### §10.1 Multi-arch image build

`docker buildx build --platform linux/amd64,linux/arm64 ...`로 manifest
list 생성. 본 SPEC scope:
- **Go binaries** (usearch-api, usearch-mcp, usearch-migrate):
  multi-arch 지원 (linux/amd64 + linux/arm64). CGO_ENABLED=0 static
  build로 cross-compile 부담 최소.
- **Python sidecars**: § 4 표 참조. researcher / tokenizer-ko /
  storm / koreanews는 multi-arch 가능; embedder는 torch + CUDA 제약으로
  **amd64 only**. NFR-DEPLOY-007에서 acknowledge.

### §10.2 Image registry

`ghcr.io/<org>/`를 default registry로 사용. 운영자가 internal registry로
재 push 시 chart values `global.imageRegistry`로 override.

Image 명명 규약:
- `ghcr.io/<org>/usearch-api:<tag>`
- `ghcr.io/<org>/usearch-mcp:<tag>`
- `ghcr.io/<org>/usearch-migrate:<tag>` (golang-migrate + 9 SQL files)
- `ghcr.io/<org>/usearch-researcher:<tag>` (Python sidecar)
- `ghcr.io/<org>/usearch-embedder:<tag>`
- ...
- `ghcr.io/<org>/charts/universal-search:<chart-version>` (chart OCI
  artifact)

### §10.3 Image signing — cosign

`sigstore/cosign v2.4.0+` keyless signing (GitHub Actions OIDC identity).
검증:
```bash
cosign verify ghcr.io/<org>/usearch-api:1.0.0 \
  --certificate-identity-regexp 'https://github.com/<org>/universal-search/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com'
```

검증 절차는 DOC-001 `operators/security/image-verification.mdx`에
documented (cross-link).

### §10.4 Chart signing — Helm provenance

Helm 3은 `helm package --sign` + `helm verify`를 지원 (GPG 기반). 단
GitHub Actions의 GPG key 관리 부담 + 운영자 검증 부담 (gpg key
distribution) 때문에 본 SPEC은:
- **cosign-based chart signing** (cosign sign-blob on chart .tgz)
  채택 — image signing과 도구 통일.
- artifact 분배: `ghcr.io/<org>/charts/universal-search` OCI registry.
  Helm 3.8+의 `helm install --insecure-skip-tls-verify=false` OCI mode.

### §10.5 SBOM + SLSA provenance

- **SBOM**: `anchore/syft v1.x.x`로 Go binary + container image SBOM
  생성 (SPDX format). release artifact에 attach.
- **SLSA**: SEC-001 D8와 align — SLSA Level 2 (provenance + signed
  releases). `slsa-framework/slsa-github-generator` action 활용.

---

## §11 — Observability integration patterns

### §11.1 Prometheus ServiceMonitor

usearch-api / mcp / sidecar 모두 `/metrics` endpoint 노출 (SPEC-OBS-001
이미 구현). chart는 `monitoring.coreos.com/v1/ServiceMonitor` CRD
생성:
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: {{ include "usearch.fullname" . }}-api
  labels:
    {{- include "usearch.labels" . | nindent 4 }}
spec:
  selector: {matchLabels: {app.kubernetes.io/component: api}}
  endpoints:
    - port: metrics
      interval: 30s
      scrapeTimeout: 10s
      path: /metrics
```

운영자 사전조건: prometheus-operator (kube-prometheus-stack) cluster-wide
설치. 운영자가 prometheus-operator를 사용하지 않으면 `observability.
serviceMonitor.enabled: false` + 본인 환경에 맞는 scrape config 추가
(예: prometheus.yml `scrape_configs:` static_configs).

### §11.2 OTLP collector wiring

OTLP_ENDPOINT 환경변수를 ConfigMap으로 주입; 운영자가 cluster-internal
OTLP collector (예: `opentelemetry-operator`로 deploy된 OpenTelemetry
Collector) 의 ClusterIP service 주소를 chart values `observability.
otlp.endpoint`에 설정. chart는 collector를 ship하지 않음 (별도 ecosystem).

### §11.3 Healthcheck → liveness/readiness probe mapping

dev-compose의 healthcheck를 k8s probe로 매핑하는 표:

| dev-compose | k8s probe |
|-------------|-----------|
| `test: ["CMD-SHELL", "wget -qO- http://localhost:8080/health"]` | `httpGet: {path: /health, port: 8080}` |
| `interval: 10s` | `periodSeconds: 10` |
| `timeout: 5s` | `timeoutSeconds: 5` |
| `retries: 5` | `failureThreshold: 5` |
| `start_period: 30s` | `startupProbe: {failureThreshold: 30, periodSeconds: 1}` (또는 readinessProbe initialDelaySeconds) |

embedder는 `start_period: 120s` (model load) — `startupProbe.
failureThreshold: 120, periodSeconds: 1` 또는 더 conservative.

### §11.4 Grafana dashboard JSON

본 SPEC scope **out**: dashboard JSON shipping은 별도 SPEC-EVAL-002
(adapter reliability dashboard)에 위탁. 본 SPEC은 ServiceMonitor + metric
endpoint exposure까지만 책임.

---

## §12 — Migration job patterns (D4 결정 자료)

### §12.1 Tool — golang-migrate/migrate v4.18.x

`golang-migrate/migrate` 채택 근거:
- Go-native (CGO_ENABLED=0 static binary 빌드 가능 → distroless container)
- 9개 SQL 파일 (§2 표) 이미 호환되는 naming convention 사용 (`NNNN_name.
  sql`, optional `.up.sql` / `.down.sql`).
- atomic transaction per migration (PG)
- `migrate.lock` 메커니즘 (concurrent run 방지) — multi-replica chart
  install 시 첫 replica만 migration 수행 (다른 replica는 skip).

### §12.2 Migration container image

`ghcr.io/<org>/usearch-migrate:<tag>` — multi-stage Dockerfile:
```dockerfile
# Stage 1: download golang-migrate binary
FROM alpine:3.20 AS downloader
ARG MIGRATE_VERSION=v4.18.1
RUN apk add --no-cache curl tar && \
  curl -L https://github.com/golang-migrate/migrate/releases/download/${MIGRATE_VERSION}/migrate.linux-amd64.tar.gz | tar xz

# Stage 2: distroless runtime + SQL files
FROM gcr.io/distroless/static-debian12
COPY --from=downloader /migrate /usr/local/bin/migrate
COPY deploy/postgres/migrations/ /migrations/
ENTRYPOINT ["/usr/local/bin/migrate", "-path=/migrations", "-database"]
```

(실제 Dockerfile은 IMPROVE phase에서 작성; 위는 sketch.)

### §12.3 Helm hook ordering

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ include "usearch.fullname" . }}-migrate
  annotations:
    "helm.sh/hook": pre-install,pre-upgrade
    "helm.sh/hook-weight": "-5"
    "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
spec:
  template:
    spec:
      restartPolicy: OnFailure
      containers:
        - name: migrate
          image: {{ .Values.migrations.image.repository }}:{{ .Values.migrations.image.tag }}
          args:
            - "{{ .Values.migrations.databaseUrl }}"
            - "up"
          env:
            - name: PGPASSWORD
              valueFrom: {secretKeyRef: {name: ..., key: ...}}
  backoffLimit: 3
```

`hook-weight: -5`로 모든 다른 resource보다 먼저 실행. `delete-policy:
before-hook-creation,hook-succeeded`로 (a) 이전 Job 잔재 제거, (b) 성공
시 자동 cleanup.

### §12.4 Failure handling

migration Job 실패 시 helm install/upgrade 자체 실패. backoffLimit=3 후
실패하면 운영자가 Job log 확인 → 수동 디버깅 → `helm rollback` 가능 (단
schema migration의 down 적용은 데이터 손실 위험; D4 결정에서 production
운영자에게는 manual SQL review 권장).

### §12.5 Idempotency

`golang-migrate`는 `schema_migrations` 테이블로 적용 이력 추적. 동일
migration 재실행은 idempotent. 단 0007_answer_cache.up.sql 이 CREATE
TABLE without IF NOT EXISTS인 경우 두 번째 실행 실패 — 모든 SQL 파일이
IF NOT EXISTS / IF EXISTS 사용하는지 audit 필요 (PRESERVE 단계 task).

### §12.6 Rollback strategy

Helm rollback (`helm rollback usearch <revision>`)은 manifest를 되돌리지만
**schema는 자동 되돌리지 않음**. operator runbook (DOC-001 `operators/
deployment/rollback.mdx` cross-link)에서 명시:
- chart manifest rollback: `helm rollback`
- DB schema rollback: 수동 `migrate down <N>` (down.sql 존재 시) 또는
  점진적 forward-fix migration (권장)

---

## §13 — Comparable OSS Helm chart audit

본 SPEC chart 작성 시 reference한 production OSS Helm chart 5개:

### §13.1 Grafana Labs Loki Helm chart

- `grafana/loki` chart (현재 v6.x). Multi-component (read/write/backend)
  Deployment + ServiceMonitor + Ingress 표준 패턴 — 본 chart의 service별
  sub-directory 구조 (§6.3) 영감.
- 본받을 점: `_helpers.tpl`의 `loki.fullname` / `loki.labels` /
  `loki.image` helper 패턴. 본 chart `_helpers.tpl`에 유사 helper 적용.

### §13.2 Bitnami PostgreSQL Helm chart

- subchart로 사용 (§8.1). values.schema.json strict pattern 영감.

### §13.3 Vector Helm chart (timberio/vector)

- Single-binary Go-like project의 chart 표준 — 본 chart usearch-api
  Deployment 작성 시 reference.
- 본받을 점: NetworkPolicy egress 표현 (vector → loki/elasticsearch
  egress pattern).

### §13.4 ingress-nginx Helm chart

- `kubernetes/ingress-nginx`. annotations + ingress class 표준화 ref.
  본 chart `ingress.yaml` 작성 시 cert-manager 통합 패턴 ref.

### §13.5 jetstack cert-manager

- chart 자체보다 chart consumer 입장에서 cert-manager.io/cluster-issuer
  annotation 표준. 본 chart는 cert-manager pre-install 가정 (NOT a
  dependency; documented requirement only).

### §13.6 OpenObserve Helm chart

- multi-component observability stack의 chart 패턴. multi-tier secrets
  표면 (`existingSecret` vs ESO vs values) ref.

---

## §14 — Open risks + unimplemented dependencies

본 SPEC이 의존하지만 현재 unimplemented한 SPEC + chart 구현 위험.

### §14.1 SEC-001 internal/security/secrets/ unimplemented

SPEC-SEC-001 D5에서 `internal/security/secrets/Resolver` interface 정의
예정이지만 현재 **draft** 상태 (SEC-001 commit 761381d). 본 chart는
secret을 K8s Secret resource로 expose하면 binary가 어떻게 읽든 무관 —
**decoupled**. 단 docs (DOC-001) 작성 시 SEC-001 Resolver의 현재
unimplemented 상태를 명시하고, V1.0.0 ship 시점에 SEC-001이 implemented
이거나 chart의 K8s Secret refs가 binary code의 직접 env-var read와
정렬되는지 verification 필요.

**Mitigation**: 본 SPEC run phase에서 SEC-001 implementation status를
재확인. 두 SPEC 모두 implemented 후 통합 integration test 1회 추가
(NFR-DEPLOY-008 cross-SPEC verification).

### §14.2 SEC-001 ops/security/runbook.md unimplemented

SEC-001 §1.1 What ships에 `ops/security/runbook.md` 명시. 본 chart의
NOTES.txt + DOC-001 cross-link에서 reference 예정. 현재 미존재.

**Mitigation**: chart NOTES.txt + README에서 docs site URL만 reference;
DOC-001 작업이 SEC-001 산출물 migration을 책임지므로 본 SPEC scope
밖.

### §14.3 SPEC-DEEP-001 (STORM) / ADP-009 (KoreaNews) 통합 상태

services/storm/ + services/koreanews/ Dockerfile은 존재하나 dev-compose
미통합 (§1.4). chart는 두 sidecar를 `enabled: false` default로 선언;
운영자가 implementation status 확인 후 활성화. roadmap §M3 ("All 12+
adapters pass contract tests") 기준 ADP-009는 implemented; DEEP-001
도 roadmap §M5 implemented per roadmap M5 status update.

**Verification**: run phase ANALYZE 단계에서 두 sidecar의 runtime
status (binary launch + `/health` 응답) 확인.

### §14.4 Subchart version drift

§8.6 참조. NFR-DEPLOY-005 quarterly audit 정책. dev-compose가 pinned
patch version (qdrant v1.16.3, meili v1.42.1, redis 7, postgres
16.13)을 사용하므로 chart subchart도 동일 patch range로 pin (예:
postgresql Bitnami chart의 postgresql.image.tag를 `16.13`으로 pin).

### §14.5 Migration job idempotency + rolling deploy race

§12.5 / §12.6 risk. 추가 risk: HPA scale-up + 동시 migration Job
실행은 `migrate.lock` 메커니즘으로 차단되지만, 동일 chart의 두 다른
release (예: blue-green deploy) 동시 install 시 lock 경합 가능.

**Mitigation**: chart README에서 "single release per namespace" 권장.
운영자가 blue-green deploy 시 두 release 간 migration이 동일 schema에
적용된다는 invariant를 가정 (forward-only migration 정책).

### §14.6 Image pull rate limit (Docker Hub anonymous 100/6h)

Bitnami subchart는 docker.io/bitnami/ image를 사용. anonymous pull은
rate limit 영향. 운영자가 Docker Hub authenticated pull credential
또는 internal registry mirror 사용 권장.

**Mitigation**: chart values `global.imagePullSecrets` 노출; NOTES.txt
+ DOC-001에서 권장 사항 명시. Bitnami OCI registry (registry-1.docker.
io OCI mode)는 rate limit 동일.

### §14.7 OpenSearch / OpenSearch operator?

본 프로젝트는 **OpenSearch를 사용하지 않는다** (initial prompt 추측 정정
— 실제 search backend는 Qdrant + Meilisearch + SearXNG). 본 SPEC은
OpenSearch 관련 결정을 포함하지 않음.

### §14.8 ARM64 multi-arch 미완성 사항

embedder amd64 only (§10.1). chart values에서 embedder의 nodeAffinity로
amd64 node 강제 가능. arm64-only k8s cluster (예: Apple Silicon
Kubernetes IN Docker / Graviton AWS) 운영자는 embedder를 `enabled:
false` 후 external embedder 사용해야 함. DOC-001에서 명시.

### §14.9 chart-testing kind cluster 가용성

CI에서 `helm install` smoke-test를 위한 kind cluster setup 필요. GitHub
Actions hosted runner에서 kind 가능 (helm/chart-testing-action,
helm/kind-action 활용). 단 embedder + Python sidecar 5개 + Postgres +
Redis + Qdrant + Meili 동시 실행은 hosted runner 4-core/14GB-RAM
한계 우려. **Mitigation**: smoke-test profile은 minimal (api +
postgres + redis만 enabled, sidecar + qdrant + meili는 disabled) —
chart `values-test.yaml` 별도 작성.

---

## §15 — Decision summary (전체 D-pillar 결정 요약)

| D | 결정 | 근거 |
|---|------|------|
| D1 | Helm v3 chart (Kustomize/Operator/Carvel 배제) | §7 다각 분석; 운영자 학습 부담 + ecosystem maturity |
| D2 | multi-service topology, service별 `templates/<name>/` 디렉토리 | §6.3; 13+ service multi-deploy 가독성 |
| D3 | Postgres/Redis는 Bitnami subchart default + external option; Qdrant는 official chart; Meili는 in-chart custom | §8; subchart 유지 부담 vs upstream maturity |
| D4 | Migration: golang-migrate + pre-install Helm hook + idempotent SQL audit | §12; Go-native tool + atomic transaction |
| D5 | Secret 3-tier: values (dev) / existingSecret (production small) / ESO (production enterprise) | §9; SEC-001 D5 정렬 |
| D6 | Prometheus ServiceMonitor + 운영자 cluster-wide prometheus-operator 가정; OTLP collector도 외부 가정 | §11; 운영자 환경 표준화 |
| D7 | OCI registry: ghcr.io; cosign keyless signing (chart + image); SBOM via syft; SLSA L2 | §10; SEC-001 D8 정렬 |
| D8 | values-dev/staging/prod.yaml 다단 layering; values.schema.json strict validation; additionalProperties: false | §6.2; misconfig fail-fast |
| D9 | NetworkPolicy default-on; PDB default-on; HPA default-on for api/mcp; default OFF for sidecar | §6.6; production hardening |
| D10 | Ingress: cert-manager + ingress-nginx default; cluster-issuer letsencrypt-prod default off; user opt-in | §6.6; security defaults |

---

## §16 — Verification plan summary (research → spec → plan trace)

본 research가 spec.md EARS REQ 작성에 직접 input으로 들어가는 trace:
- §1 deploy surface → REQ-DEPLOY-001..005 (chart structure REQs)
- §2 migrations → REQ-DEPLOY-006 (migration job REQ) + NFR-DEPLOY-004
  (rollback policy)
- §3 binary → REQ-DEPLOY-002, REQ-DEPLOY-003 (Dockerfile authoring)
- §5 env-var → REQ-DEPLOY-007 (ConfigMap structure) + REQ-DEPLOY-008
  (Secret structure)
- §6 Helm pattern → REQ-DEPLOY-009 (Chart.yaml + values.schema.json) +
  REQ-DEPLOY-010..015 (per-service template requirements)
- §8 subchart → REQ-DEPLOY-016 (dependencies declaration)
- §9 secrets → REQ-DEPLOY-017, REQ-DEPLOY-018 (3-tier secret backend)
- §10 multi-arch + signing → REQ-DEPLOY-019..021 (image + chart signing
  REQ)
- §11 observability → REQ-DEPLOY-022, REQ-DEPLOY-023 (ServiceMonitor +
  OTLP)
- §12 migration job → REQ-DEPLOY-006 + REQ-DEPLOY-024
- §13 OSS audit → reference only (no REQ — design guidance)
- §14 risk → spec.md §7 Risks; §14.1 → NFR-DEPLOY-008 (cross-SPEC
  verification gate)

---

**Research artifact total: 16 sections, ≈ 38 KB. Companion: spec.md +
plan.md.** EARS REQ 작성 + plan 작성 시 본 research를 ground truth로
사용.
