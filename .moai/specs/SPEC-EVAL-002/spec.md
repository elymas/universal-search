---
id: SPEC-EVAL-002
version: 0.2.0
status: approved
created: 2026-05-22
updated: 2026-05-30
author: limbowl
priority: P1
issue_number: 0
title: Adapter reliability dashboard — 7-day rolling success rate per adapter with alerting
milestone: M8 — Eval + polish
owner: expert-performance
methodology: ddd
coverage_target: 85
depends_on: [SPEC-OBS-001, SPEC-FAN-001, SPEC-ADP-001, SPEC-CACHE-001, SPEC-UI-002]
blocks: [SPEC-REL-001]
related: [SPEC-EVAL-001, SPEC-EVAL-003, SPEC-AUTH-003]
---

# SPEC-EVAL-002: Adapter reliability dashboard — 7-day rolling success rate per adapter

## HISTORY

- 2026-05-30 (amendment v0.2.0, limbowl via manager-spec — resolve
  manager-strategy findings + apply user-approved scope decisions):
  plan-auditor 재감사 전 5건 수정. (A1) **Admin endpoint 중복 제거**
  — REQ-EVAL2-010 이 별도 :9090 서버에 새 `/admin/health/adapters`
  를 만들려던 것을, SPEC-UI-002 가 이미 출시한 `/api/admin/adapters`
  + `Registry.SnapshotForAdmin()` (`internal/adapters/registry.go:243`,
  LoopbackOnly middleware, `cmd/usearch-api/main.go:71`) 를 **재사용**
  하도록 재작성. `AdapterAdminView.success_count`/`fail_count`
  (`registry.go:212,215`) 는 현재 0 stub 인데 이 SPEC 이 adapter
  telemetry 로 채운다. 새 sibling `/api/admin/adapters/health` 를
  **같은 mux + LoopbackOnly** 로 추가 (별도 포트 없음). SPEC-UI-002
  를 `depends_on` 에 추가 (admin handler 소유권). (A2) **circuit_state
  dead alert/panel deferral** — V1 에 `usearch_adapter_circuit_state`
  를 emit 하는 upstream 이 없으므로 alert #4 (circuit-open) + Grafana
  panel #5 (circuit-state matrix) 를 **post-V1 로 연기**. metric family
  는 forward-compat 위해 등록 유지하되, V1 acceptance gate 에서 제거
  (영구 no-data alert/panel 이 V1 을 막지 않도록). 미래 resilience SPEC
  이 emit 하면 재활성화. (A3) **stale emit line 수정** —
  `wrappedAdapter.emit` 는 `registry.go:223` 이 아니라 `registry.go:433`.
  (A4) **prometheus evaluation_interval reconcile** — recording rule 은
  1m evaluation 을 가정하지만 `deploy/prometheus/prometheus.yml` 은
  현재 15s (line 9). T4 에서 `evaluation_interval: 1m` 로 변경하는 것을
  명시 (구현자가 놓치지 않도록 plan 에 explicit task 추가). (A5)
  **Loki log-link panel optional 명시** — REQ-EVAL2-005 의 Loki log-link
  panel 은 Loki 미배포 시 빈 패널이므로 **optional/conditional** 로
  명확히 표기, V1 gate 항목 아님. Version 0.1.0 → 0.2.0. status=draft
  유지 (재감사가 결정).

- 2026-05-22 (initial draft v0.1.0, limbowl via manager-spec):
  M8 운영 폴리시 SPEC의 첫 정식 초안. 이 SPEC은 새 로직을 만드는
  것이 아니라, M1 SPEC-OBS-001 이 이미 등록한 `usearch_adapter_calls_
  total{adapter,outcome}` Counter와 SPEC-CORE-001 의 6-tuple `Outcome
  FromError` enum (success / failure / timeout / rate_limited /
  unavailable / transient — `pkg/types/errors.go:174`) 위에 **rolling-
  window aggregation, 시각화, 알람** 레이어를 올린다. M3 까지 출시된
  9 개 SPEC-ADP-001..009 (Reddit / HN / arXiv+paper-search / GitHub /
  YouTube / Bluesky+X / SearXNG / Naver suite / KoreaNewsCrawler+
  Daum+RSS — `.moai/project/roadmap.md` §M3) 의 12+ adapter 소스가
  모두 동일한 통제면 (control plane) 으로 들어와야 한다. M9 exit
  criterion `.moai/project/roadmap.md` §5 M8 의 "adapter success rate
  dashboard live" 는 이 SPEC 하나로 충족된다.

  Pinned decisions:
  (D1) Success-rate formula: `success_rate = success / (success +
       failure + timeout + rate_limited + unavailable + transient)`
       — 6-tuple denominator는 `pkg/types/errors.go:OutcomeFromError`
       canonical mapping을 그대로 사용. `transient` 는 retryable 이지만
       호출 자체는 실패로 본다 (retry 후 재호출이 별도 outcome 으로
       기록됨). `partial_success` (fanout 레벨에서 일부 adapter 만
       실패) 는 **별도 게이지** (`usearch_fanout_partial_total`)
       로 분리 — REQ-EVAL2-007. 두 지표를 한 패널에서 합치지 않는
       이유는 partial은 fanout 의 invariant 이고 per-adapter 성공률
       과 다른 시간축에서 해석돼야 하기 때문. See research §1, §2.

  (D2) Rolling-window mechanism: **Prometheus recording rules** 사용.
       `rate(usearch_adapter_calls_total[7d])` PromQL을 24h evaluation
       window 의 `usearch:adapter_success_rate_7d` 등 5 개 recording
       rule 로 사전 집계 (`deploy/prometheus/recording-rules.yml`).
       대안인 in-process ring buffer는 (a) usearch process 재시작 시
       7일 윈도우 손실, (b) admin 포트에 새 metric family 추가로 NFR-
       OBS-002 cardinality allowlist 확장 필요, (c) Prometheus 가
       이미 retention 30일을 보장 — 세 가지 이유로 **배제**. 대시보드/
       알람은 모두 recording rule 결과를 쿼리한다. See research §3.

  (D3) Per-adapter label cardinality bound: 기존 `{adapter, outcome}`
       label pair 만 사용. **새 label 도입 없음.** 추가되는 metric
       family는 `usearch_fanout_partial_total` (label: `adapter`),
       `usearch_adapter_health_status` (gauge, label: `adapter`),
       `usearch_adapter_circuit_state` (gauge, label: `adapter, state`)
       — 12 adapter × 6 outcome = 72 series + 12 + 12 + 12×3 = 132
       시리즈로 NFR-EVAL2-001 의 ≤500 series 예산 안. `team_id`,
       `user_id`, `region` 등은 label 에 절대 넣지 않는다 (cardinality
       explosion). See research §6.

  (D4) Alert backend: **Alertmanager (Prometheus 표준 컴포넌트)** 를
       1차로 채택. `deploy/prometheus/alerts.yml` 에 4개 alert rule
       을 정의하고, `deploy/alertmanager/alertmanager.yml` 에 default
       null receiver 와 webhook receiver template 을 제공. 운영자가
       Discord webhook / OpsGenie / PagerDuty 로 라우팅하려면
       receiver 섹션만 교체. OpsGenie / Discord 를 SPEC 자체에
       하드코딩하지 않는 이유: self-hosted 사용자마다 알람 채널이
       달라서. See research §4.

  (D5) Dashboard surface: **Grafana JSON committed to repo** 으로
       단일화 (`deploy/grafana/dashboards/adapter-reliability.json`).
       Grafana provisioning convention (`deploy/grafana/provisioning/
       dashboards/`, `deploy/grafana/provisioning/datasources/`) 으로
       compose-up 시 자동 로드. SPEC-UI-001 의 Web UI 안에 별도 admin
       탭을 만들지 **않음** — admin/observability UI 는 SPEC-UI-002
       (M7 후반) 의 책임. Grafana 가 self-hosted operator 의 표준
       observability UI 이므로 이중 surface 비용을 피한다. See
       research §5.

  (D6) Failure taxonomy: `OutcomeFromError` 의 6 값을 canonical 로 유지.
       `5xx`, `parse_error`, `TLS`, `DNS`, `circuit_open` 같은 더 세분된
       cuts 는 **adapter 내부 slog 필드 `failure_class`** (per-call log
       attribute) 로 노출하고, Loki / 로그 검색으로 드릴다운한다.
       Prometheus label 로 끌어올리면 cardinality 가 12×6×7 = 504 로
       NFR-EVAL2-001 budget 을 즉시 초과. 대시보드는 6-outcome stacked
       bar로 시각화 + 로그 드릴다운 링크로 보완. `circuit_open` 은
       (D3) 의 `usearch_adapter_circuit_state{adapter,state}` 별도
       gauge로 측정 — adapter 호출 분모에 들어가지 않는 cross-cutting
       상태이기 때문. See research §1, §6, §7.

  Companion artifacts:
  - `.moai/specs/SPEC-EVAL-002/research.md` — Phase 0.5 research
    (Prometheus rolling-window patterns, Grafana-as-code, Alertmanager
    routing, OSS dashboard prior art, 12-adapter failure-mode matrix,
    cardinality budgeting math)
  - `.moai/specs/SPEC-EVAL-002/plan.md` — phased DDD implementation
    plan (instrumentation → recording rules → dashboard JSON → alert
    rules → operator runbook)

  10 EARS REQs (4 × P0 + 4 × P1 + 2 × P2) + 5 NFRs + 3 새 metric
  family + 1 health 엔드포인트 + 5 recording rule + 4 alert rule + 1
  Grafana dashboard JSON + 1 Alertmanager config + 1 operator
  runbook. Methodology: **DDD** (기존 adapter / fanout / cache 코드
  characterization → instrumentation 추가, 새 도메인 로직 없음).
  Coverage target 85%. Harness: standard. Owner: expert-performance.
  No GitHub issue (`issue_number: 0`).

---

## 1. Overview

SPEC-EVAL-002 는 Universal Search 운영자가 12+ adapter 의 건강
상태를 **단일 대시보드** 와 **실시간 알람** 으로 보장한다. 핵심
KPI 는 *7일 rolling success rate per adapter* 다. 운영자는 이
SPEC 의 산출물만으로 다음 질문에 답할 수 있어야 한다:

1. 지난 7일 동안 각 adapter 의 성공률은? (target ≥ 95% per adapter)
2. 어떤 adapter 가 가장 자주 실패하고 있나? (Top-N failures)
3. 실패 원인 분포는? (timeout vs rate_limited vs unavailable vs
   parse error)
4. 새 adapter 가 배포된 후 회귀가 발생했나? (baseline 대비 7d drop)
5. (post-V1, deferred) 어떤 adapter 가 현재 circuit-open 상태인가?
   (CACHE-001 fallback cascade 통계와 연계) — V1 에서는 circuit-state
   를 emit 하는 upstream 이 없어 deferred. metric family 만 등록.

### 1.1 What ships

- **`deploy/prometheus/recording-rules.yml`** — 5 recording rules로
  `usearch:adapter_success_rate_{1h,24h,7d}`, `usearch:adapter_
  failure_rate_by_outcome_24h`, `usearch:adapter_fanout_partial_
  ratio_24h` 사전 계산
- **`deploy/prometheus/alerts.yml`** — **V1: 3 alert rule** (7d
  success rate < 85%, 1h success rate < 50% (acute degradation),
  partial-result ratio > 30% sustained 15min). **(deferred, post-V1)
  4번째 alert circuit-open > 10min** 은 emit upstream 부재로 연기 —
  v0.2.0 amendment A2 참조.
- **`deploy/alertmanager/alertmanager.yml`** — 기본 라우팅 + 운영자
  지정 webhook receiver 템플릿
- **`deploy/grafana/dashboards/adapter-reliability.json`** — **V1: 4
  패널** (success-rate by adapter (24h heatmap), success-rate 7d
  trendline, failure-cause breakdown (stacked bar), fanout partial
  ratio gauge). **(deferred, post-V1) 5번째 panel circuit-state
  matrix** 은 연기 (A2). **(optional) Loki log-link panel** 은 Loki
  datasource 가 있을 때만 활성 — 미배포 시 빈 패널, V1 gate 아님 (A5).
- **`deploy/grafana/provisioning/`** — datasource + dashboard
  auto-load 설정
- **`internal/obs/metrics/fanout_partial.go`** (신규) —
  `usearch_fanout_partial_total`, `usearch_adapter_health_status`,
  `usearch_adapter_circuit_state` 세 metric family 등록 (circuit_state
  는 forward-compat 위해 등록만, V1 emit 없음) + 기존
  `Registry.labelNames` 확장
- **`internal/fanout/dispatch.go`** (수정) — partial-result 발생
  시 새 counter 증가 (fanout 종료 후 1회만)
- **`internal/api/admin/handler_adapters.go`** (수정, SPEC-UI-002
  소유) — 기존 `AdapterAdminView.success_count`/`fail_count` stub
  필드를 adapter telemetry 로 채우고, 같은 admin mux 에 sibling
  `/api/admin/adapters/health` (LoopbackOnly) 추가. **별도 :9090
  서버 신설 없음** — v0.2.0 amendment A1 참조.
- **`docs/operations/adapter-reliability-runbook.md`** — 알람 발생
  시 운영자 대응 절차

### 1.2 Motivation

M3 시점에 12+ adapter 가 모두 prod 트래픽을 받기 시작하면 운영자
는 다음과 같은 운영 부담에 직면한다:

- Naver / X (rate-limit-prone): API quota 초과로 인한 짧은-기간
  rate_limited 폭주
- Reddit / GitHub (OAuth-gated): 토큰 만료 / 권한 변경으로 인한
  permanent 실패
- YouTube (transcript 추출): yt-dlp 가 깨지면 timeout 폭주
- SearXNG bridge: upstream search engine 차단 시 unavailable
- KoreaNewsCrawler: HTML 구조 변경 시 parse error (failure 로 분류)

각 adapter 가 독립적으로 깨질 수 있고, fanout (SPEC-FAN-001) 의
partial-result 정책 덕분에 전체 query 응답은 멀쩡해 보이지만 한두
adapter 는 며칠째 0% 성공률일 수 있다. 운영자에게 이것이 **보이지
않으면** Universal Search 는 사용자 신뢰를 잃는다 (citation 답변에
실제로는 한국어 소스가 빠져있는데도 그 사실이 노출되지 않음).

SPEC-EVAL-002 가 없으면 M9 V1 release 의 exit criterion 인
"adapter success rate dashboard live" 를 충족할 방법이 없다.

### 1.3 Forward-compatibility commitments

- **SPEC-OBS-001**: 기존 `usearch_adapter_calls_total{adapter,
  outcome}` Counter와 `usearch_adapter_call_duration_seconds
  {adapter}` Histogram을 **그대로 사용** — 재정의/이름 변경 없음
- **SPEC-CORE-001**: `pkg/types/errors.go:OutcomeFromError` 의
  6-tuple 매핑을 canonical 로 사용. 새 outcome 값을 만들지 않음
- **SPEC-FAN-001**: `Result.AdapterErrors` map 의 존재가 partial-
  result 감지 기준. 새 metric은 dispatch 종료 후 1회 emit
- **SPEC-CACHE-001**: `usearch_access_phase_attempts_total
  {phase,outcome}` 와 별개로 운영 — phase는 cascade 내부 단계이고,
  EVAL-002 는 adapter 호출 단위 성공률만 본다. 두 metric 의
  drilldown 링크는 Grafana dashboard 안에서 제공
- **SPEC-AUTH-003 (related)**: audit log 가 query 단위 reconstruct를
  지원할 때, dashboard 에 audit-log 드릴다운 링크를 추가 (V1.1)
- **SPEC-EVAL-001 (related)**: faithfulness benchmark CI gate와는
  독립. EVAL-002 는 production runtime metric, EVAL-001 은 offline
  test golden set
- **SPEC-EVAL-003 (related)**: 한국어 adapter 의 성공률을 별도로
  필터링할 수 있도록 Grafana variable 로 adapter set 분류 제공

### 1.4 Pinned architectural decisions

HISTORY 의 6개 D1-D6 결정이 §2 의 요구사항을 구속한다. 재논의
하지 않음.

---

## 2. EARS Requirements

### 2.1 Rolling-Window Aggregation Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL2-001** | Ubiquitous | `deploy/prometheus/recording-rules.yml` SHALL define exactly 5 recording rules with names `usearch:adapter_success_rate_1h`, `usearch:adapter_success_rate_24h`, `usearch:adapter_success_rate_7d`, `usearch:adapter_failure_rate_by_outcome_24h`, `usearch:adapter_fanout_partial_ratio_24h`. Each rule SHALL use PromQL `rate()` or `sum_over_time()` on `usearch_adapter_calls_total` partitioned by `adapter` (and `outcome` where applicable per HISTORY D1). The evaluation interval SHALL be 1 minute. The success-rate denominator SHALL include all six `outcome` enum values from `pkg/types/errors.go:OutcomeFromError` (success / failure / timeout / rate_limited / unavailable / transient). | P0 | `promtool check rules deploy/prometheus/recording-rules.yml` exits 0. Unit test loads each rule against a fixed `usearch_adapter_calls_total` fixture series and asserts numerator/denominator computation matches hand-calculated values within 0.001. |
| **REQ-EVAL2-002** | Event-Driven | WHEN Prometheus evaluates the recording rules at the 1-minute tick, the per-adapter `usearch:adapter_success_rate_7d` SHALL be queryable for each adapter listed in `Registry.List()` (at minimum the 12 SPEC-ADP-001..009 adapters) and return a value in `[0.0, 1.0]`. The 7-day window SHALL roll forward with Prometheus retention; values older than the configured retention (default 30 days per NFR-EVAL2-004) SHALL drop off cleanly. | P0 | Integration test seeds Prometheus with 8 days of synthetic adapter call data; queries `usearch:adapter_success_rate_7d{adapter="reddit"}` after the 7-day mark and asserts the value excludes day-0 data. |

### 2.2 Per-Adapter Metric Emission Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL2-003** | Ubiquitous | `internal/obs/metrics` SHALL register three new metric families: (a) `usearch_fanout_partial_total` (CounterVec, label `adapter` — incremented once per fanout dispatch when that adapter contributed an error to `Result.AdapterErrors`), (b) `usearch_adapter_health_status` (GaugeVec, label `adapter` — 1.0 healthy, 0.5 degraded, 0.0 unhealthy, updated by recording rule companion job or admin endpoint), (c) `usearch_adapter_circuit_state` (GaugeVec, labels `adapter` and `state` ∈ `{closed, open, half_open}` — updated when SPEC-CACHE-001 cascade reports a circuit transition). The `Registry.labelNames` allowlist SHALL be extended with no new label names beyond `state` (existing `adapter`, `outcome`, `reason` reused — REQ-EVAL2-003 (c) adds `state` only). | P0 | Test asserts the three metric families register without panic; cardinality test asserts total series ≤ 12 × 6 + 12 + 12 + 12 × 3 = 132 series with 12 adapters (calls_total 72 + partial 12 + health 12 + circuit 36). |
| **REQ-EVAL2-004** | Event-Driven | WHEN `fanout.Dispatch` returns a `Result` whose `AdapterErrors` map is non-empty, the `internal/fanout` package SHALL increment `usearch_fanout_partial_total{adapter=<name>}` exactly once per adapter that appeared in `AdapterErrors`. The increment SHALL happen after `eg.Wait()` returns and BEFORE the result is returned to the caller, so partial-result accounting matches the actual observed per-call outcome. | P0 | Unit test invokes `fanout.Dispatch` with 3 mock adapters where 1 fails; asserts `usearch_fanout_partial_total{adapter="failing"}` increases by exactly 1 and the other two adapters' counters are unchanged. |
| **REQ-EVAL2-005** | Ubiquitous | Adapter call failure-cause cuts finer than the 6 `OutcomeFromError` enum values (e.g., HTTP 5xx vs HTTP 4xx, TLS handshake failure, DNS NXDOMAIN, parse error, transcript extraction failure) SHALL be exposed as a `failure_class` attribute on the slog record emitted by `wrappedAdapter.emit` (per `internal/adapters/registry.go:433`). The attribute SHALL NOT be promoted to a Prometheus label (per HISTORY D6 cardinality budget). **WHERE a Loki datasource is configured**, the Grafana dashboard MAY include an **optional** Loki/log-link panel that filters `{service="usearch"} \| json \| adapter = <selected> and outcome != "success"` for failure-cause drilldown; this panel is **conditional and NOT a V1 acceptance gate item** — when Loki is absent the panel renders empty and the 4 core panels are unaffected (amendment A5). | P1 | Test asserts `wrappedAdapter.emit` writes the `failure_class` attribute when error matches taxonomy (5xx / 4xx / dns / tls / parse / transcript / unknown). The optional Loki panel, IF present, has the documented query — its absence does NOT fail the gate. |

### 2.3 Dashboard Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL2-006** | Ubiquitous | The `deploy/grafana/dashboards/adapter-reliability.json` SHALL be a Grafana 11.x-compatible dashboard JSON containing the **V1: 4 core panels**: (1) per-adapter 24h success-rate heatmap, (2) per-adapter 7d success-rate trendline, (3) failure-cause stacked bar by outcome, (4) fanout partial-result ratio time-series. **Panel (5) circuit-state matrix is DEFERRED to post-V1 (amendment A2)** — no upstream emits `usearch_adapter_circuit_state` in V1, so a permanently-no-data panel is excluded from the V1 gate; it MAY be added (rendering "no data") when a future resilience SPEC emits the gauge. An **optional** Loki log-link panel (REQ-EVAL2-005, A5) is conditional on a Loki datasource. Each core panel SHALL reference the recording rules from REQ-EVAL2-001 (not raw `rate()` queries) to keep render time within NFR-EVAL2-002. The dashboard SHALL declare a Grafana template variable `adapter` populated from `label_values(usearch_adapter_calls_total, adapter)` so operators can filter views per adapter. | P0 | `jsonnet-lint` or equivalent passes; dashboard imports cleanly into a Grafana 11.x instance via `grafana-cli` import; visual smoke test exports a screenshot showing the **4 core panels** with non-empty data when fed the recording-rule fixture. (Deferred circuit panel + optional Loki panel are NOT gate items.) |
| **REQ-EVAL2-007** | Optional | WHERE the operator deploys via `deploy/docker-compose.yml`, the dashboard SHALL be auto-provisioned via `deploy/grafana/provisioning/dashboards/adapter-reliability.yaml` and the Prometheus datasource SHALL be auto-configured via `deploy/grafana/provisioning/datasources/prometheus.yaml`. Running `docker compose up` SHALL produce a working dashboard URL within 30 seconds with no manual import step. | P1 | Integration test runs `docker compose up grafana prometheus`, polls `http://localhost:3000/api/dashboards/uid/adapter-reliability` and asserts HTTP 200 within 30 seconds. |

### 2.4 Alerting Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL2-008** | Event-Driven | WHEN `usearch:adapter_success_rate_7d{adapter=<name>}` drops below 0.85 for a sustained 30 minutes, OR `usearch:adapter_success_rate_1h{adapter=<name>}` drops below 0.50 for 5 minutes (acute degradation), OR `usearch:adapter_fanout_partial_ratio_24h` exceeds 0.30 for 15 minutes, Alertmanager SHALL fire a corresponding alert. **These are the V1: 3 alert rules.** A **4th circuit-open alert** (`usearch_adapter_circuit_state{state="open"} == 1` for 10 minutes) is **DEFERRED to post-V1 (amendment A2)** — no upstream emits the gauge in V1, so the rule would never fire and is excluded from the V1 gate; it is re-enabled when a future resilience SPEC emits circuit transitions. The alerts SHALL be defined in `deploy/prometheus/alerts.yml` with labels `severity=warning|critical`, `spec=SPEC-EVAL-002`, and `adapter=<name>` (where applicable). Alert annotations SHALL include a `runbook_url` linking to `docs/operations/adapter-reliability-runbook.md#<alert-name>`. | P0 | `promtool check rules deploy/prometheus/alerts.yml` exits 0. Unit test for each of the **3 V1 alerts**: inject synthetic time series satisfying the alert condition, assert the alert transitions to `pending` then `firing` per the documented `for:` duration. (Deferred circuit alert is NOT a V1 gate item.) |
| **REQ-EVAL2-009** | Ubiquitous | `deploy/alertmanager/alertmanager.yml` SHALL define a default route with a `null` receiver (silent by default for operators not yet wired to a notification channel) and SHALL include commented-out example receiver blocks for: (a) generic webhook (e.g., Discord, Slack), (b) OpsGenie, (c) PagerDuty. The config SHALL pass `amtool check-config`. Operators wire their channel by uncommenting and editing the receiver block; the SPEC does NOT ship a default external notification destination (privacy-first + multi-tenant deploy reality). | P1 | `amtool check-config deploy/alertmanager/alertmanager.yml` exits 0. Inspection test asserts all three commented receiver examples are present in the YAML. |

### 2.5 Operational Endpoint Module

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| **REQ-EVAL2-010** | Event-Driven | The adapter health surface SHALL **reuse the existing SPEC-UI-002 admin handler** rather than introduce a new server or port. Two parts: **(a) populate stub counts** — the existing `AdapterAdminView.success_count` / `fail_count` fields (`internal/adapters/registry.go:212,215`, currently 0 stubs) SHALL be populated from per-adapter call telemetry, and a derived `success_rate` field SHALL be added to `AdapterAdminView`, so `GET /api/admin/adapters` (already served via `cmd/usearch-api/main.go:71` with `LoopbackOnly` middleware) reflects real call statistics. **(b) health sibling** — WHEN an operator (or external readiness probe) issues `GET /api/admin/adapters/health` on the **same admin mux** (LoopbackOnly, no new port), the server SHALL respond with HTTP 200 (or 503 if any adapter is `unhealthy` per the threshold rules) and a JSON body containing an `adapters` array. Each element SHALL include `name`, `status` ∈ `{healthy, degraded, unhealthy}`, `success_rate_24h` (float `[0.0, 1.0]`), `success_rate_7d` (float), `last_call_at` (ISO-8601). The `circuit_state` field is **deferred (post-V1, amendment A2)** — it MAY be present but always reports `closed` until an upstream emits circuit transitions. Status SHALL be derived from the same 7d threshold used by REQ-EVAL2-008 alert (≥ 0.95 healthy, 0.85–0.95 degraded, < 0.85 unhealthy). The endpoint SHALL be loopback-only (existing SPEC-UI-002 `LoopbackOnly` middleware) and SHALL NOT be exposed on the public API listener. **The separate `:9090` admin server proposed in v0.1.0 is removed.** Handler ownership: SPEC-UI-002 owns `internal/api/admin/handler_adapters.go`; this SPEC extends it. | P2 | Integration test seeds metrics fixture, issues `GET /api/admin/adapters/health`, asserts response schema and status-code mapping; asserts `GET /api/admin/adapters` now returns non-zero `success_count`/`fail_count`/`success_rate` for adapters with telemetry; asserts the endpoint is rejected from a non-loopback `RemoteAddr` (existing LoopbackOnly test pattern). |

---

## 3. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| **NFR-EVAL2-001** | Metric cardinality cap | The total Prometheus series count across the three new metric families from REQ-EVAL2-003 SHALL NOT exceed 500 series under the production label set (12 adapters × {6 outcome × adapter_calls_total existing} + 12 partial + 12 health + 36 circuit_state = 132 series headroom for 38 adapters). The `Registry.labelNames` allowlist test in `internal/obs/metrics/metrics_test.go` SHALL be extended to include the new `state` enum value and SHALL fail the build if any unexpected label appears. No new label NAMES are added beyond `state`; `adapter`, `outcome`, `reason` are reused from the existing allowlist. |
| **NFR-EVAL2-002** | Dashboard render latency | The Grafana dashboard SHALL render all 4 core panels in < 2 seconds against a Prometheus instance with 30 days of retention and 12 adapters × ~1 req/sec sustained traffic (~31M observations total). This is achieved by reading from the pre-aggregated recording rules (REQ-EVAL2-001), not raw counter rates. Measured via Grafana built-in panel render time displayed at dashboard load. |
| **NFR-EVAL2-003** | Alert delivery latency | The end-to-end alert latency from `usearch:adapter_success_rate_7d` crossing threshold to Alertmanager `firing` state SHALL be < 60 seconds: 1 min recording-rule evaluation interval + 30 sec alert `for:` minimum + Alertmanager group_wait (default 30 sec) — total upper bound 120 sec, target 60 sec under default config. Acute (1h) alert latency target 5 min from threshold cross to receiver fan-out. **The recording rules and alert `for:` durations in this SPEC assume `evaluation_interval: 1m`. The current `deploy/prometheus/prometheus.yml` sets `evaluation_interval: 15s` (line 9); T4 (Phase 4) MUST change it to `1m` so the rule evaluation cadence matches the documented latency math. The 15s `scrape_interval` is unchanged (scrape and evaluation are independent).** |
| **NFR-EVAL2-004** | Metric retention | Prometheus retention SHALL be configured to ≥ 30 days in `deploy/prometheus/prometheus.yml` (`--storage.tsdb.retention.time=30d` flag). This guarantees the 7-day rolling window has full history even after a Prometheus restart and supports week-over-week regression analysis (baseline derivable from `usearch:adapter_success_rate_7d offset 7d`). |
| **NFR-EVAL2-005** | Backward compatibility — zero impact on adapter hot path | The new instrumentation (REQ-EVAL2-004 fanout partial counter, REQ-EVAL2-003 health/circuit gauges) SHALL add < 1% p99 latency overhead to `fanout.Dispatch` measured on the existing `internal/fanout/bench_test.go`. Counter `.Inc()` and gauge `.Set()` calls are O(1) lock-free in `client_golang`; the bound is enforced by re-running the bench in CI after this SPEC ships. |

---

## 4. Exclusions (What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has
a known destination, rationale, or follow-up; this list prevents
scope creep into EVAL-002.

- **Citation faithfulness scoring** (golden-set evaluation, DeepEval
  CI gate at ≥ 0.85). → Owned by **SPEC-EVAL-001**. EVAL-002 is
  production runtime metric; EVAL-001 is offline test gate. The two
  do not share infrastructure.

- **Korean-locale benchmark** (50-query Korean-first eval, manual
  scoring protocol). → Owned by **SPEC-EVAL-003**. The Korean
  adapter set (Naver / Daum / KoreaNewsCrawler / Korean RSS) is
  visible in the EVAL-002 dashboard via the Grafana adapter template
  variable, but no Korean-specific benchmark metric is emitted here.

- **Security hardening / dependency audit / SSRF mitigation** (per
  M8 SPEC-SEC-001 row). → Owned by **SPEC-SEC-001**. EVAL-002
  emits no security telemetry beyond the existing `reason` label
  reused for non-security alerts.

- **Per-team / per-user success-rate breakdown.** → Adding `team_id`
  or `user_id` labels to `usearch_adapter_calls_total` would expand
  cardinality by O(N_teams × N_adapters × N_outcomes) — easily
  thousands of series and a violation of NFR-EVAL2-001. Per-team
  visibility belongs to **SPEC-AUTH-003** audit-log replay, not to
  Prometheus metrics.

- **Real-time anomaly detection / ML-based regression detection.** →
  V1 uses fixed thresholds. Anomaly-detection (Prometheus
  `predict_linear`, Grafana ML, or external services) is post-V1
  scope. The 7d offset baseline (NFR-EVAL2-004) is the V1
  regression-detection mechanism.

- **New Web UI admin / observability dashboard inside the Universal
  Search Web UI.** → Owned by **SPEC-UI-002** (Admin UI). Operators
  use Grafana as the canonical observability surface; building a
  second admin UI doubles maintenance cost. SPEC-UI-002 will
  cross-link to the Grafana dashboard, not reimplement panels. **NOTE
  (amendment A1):** EVAL-002 does NOT build a new admin server/port —
  it extends SPEC-UI-002's existing `/api/admin/adapters` handler
  (`internal/api/admin/handler_adapters.go`, LoopbackOnly) by filling
  the already-present `success_count`/`fail_count` stub fields and
  adding a sibling `/api/admin/adapters/health` route on the same mux.
  This is a telemetry fill-in of an existing surface, not a new UI.

- **Alert routing to specific external services** (Slack channel,
  Discord webhook, PagerDuty integration key). → Operators configure
  Alertmanager receiver per their environment. SPEC ships only the
  null default + commented examples (REQ-EVAL2-009).

- **SLO definition / burn-rate alerts / error budget tracking.** →
  Adding SLO framework (Sloth, Pyrra, OpenSLO) is post-V1. EVAL-002
  ships threshold-based alerts (operationally simpler, no SLO
  infrastructure required).

- **Long-term metric storage** (Thanos, Mimir, Cortex). → Prometheus
  single-instance with 30-day retention is V1 sufficient. Long-term
  storage for multi-month trend analysis is operator infrastructure
  choice, not a usearch SPEC concern.

- **Promoting `failure_class` to a Prometheus label** for per-class
  rate alerting. → Cardinality 12 × 6 × ~7 = 504 series exceeds
  NFR-EVAL2-001 immediately. `failure_class` lives in slog/Loki for
  drilldown (REQ-EVAL2-005). If a specific failure class needs an
  alert (e.g., "all adapters DNS-failing simultaneously"), it gets a
  bespoke alert rule reading log-derived metrics from Loki via
  `loki_distinct_streams` — out of scope V1.

- **Adapter-level retry policy / circuit breaker implementation.** →
  EVAL-002 *would consume* a hypothetical `usearch_adapter_circuit_
  state` signal from SPEC-CACHE-001 cascade (or a future SPEC-RESIL-
  001) but does NOT implement the circuit breaker itself. **Per
  amendment A2, because no upstream emits this gauge in V1, the
  circuit-open alert (REQ-EVAL2-008 4th rule) and the circuit-state
  matrix dashboard panel (REQ-EVAL2-006 panel #5) are DEFERRED to
  post-V1 and removed from the V1 acceptance gate.** The metric family
  `usearch_adapter_circuit_state` is still registered (REQ-EVAL2-003c)
  for forward compatibility; it stays at default `closed` and the
  deferred alert/panel are re-enabled when a future SPEC emits real
  transitions.

- **Synthesis / LLM cost dashboard.** → Owned by future SPEC; LLM
  cost metrics already exist (SPEC-LLM-001 `usearch_llm_cost_total`)
  but their dashboard is post-V1.

- **Multi-tenant dashboard isolation** (per-team Grafana folders /
  RBAC). → Single global dashboard for V1 self-hosted operator.
  Per-team isolation arrives with SPEC-IDX-004 / SPEC-AUTH-002 in
  Grafana org/folder permissions — outside EVAL-002 scope.

- **GitHub Issue tracking on this SPEC** (`issue_number: 0`). → Per
  project pattern for M8 operational SPECs.

- **Modifications to `pkg/types/errors.go:OutcomeFromError`** to add
  new outcome values. → Canonical taxonomy is **frozen** per HISTORY
  D1 + D6; SPEC-CORE-001 owns the enum. New cuts go to
  `failure_class` slog attribute, not the Prometheus outcome label.

- **Dashboard for Python sidecar services** (services/researcher,
  services/storm, services/embedder). → Each Python service emits
  its own metrics per SPEC-OBS-001 §2.2. EVAL-002 covers only Go
  adapter call surface.

---

## 5. Acceptance Criteria

Per-REQ acceptance summaries are documented inline in §2. Detailed
Given-When-Then scenarios live in `.moai/specs/SPEC-EVAL-002/
acceptance.md` (authored alongside plan-auditor cycle). Scenario
index:

| Scenario | Description | Coverage |
|----------|-------------|----------|
| §5.1 | Recording-rule correctness: seed `usearch_adapter_calls_total` with deterministic per-adapter outcome counts; query the 5 recording rules after 1 evaluation tick; compare to hand-calculated success-rate values. | REQ-EVAL2-001, REQ-EVAL2-002 |
| §5.2 | Partial-result emission: invoke `fanout.Dispatch` with 5 adapters where 2 fail; assert `usearch_fanout_partial_total{adapter}` increments exactly once per failed adapter; assert success-rate accounting unaffected for the 3 passing adapters. | REQ-EVAL2-003, REQ-EVAL2-004 |
| §5.3 | Failure-class slog attribute: invoke an adapter that returns a TLS handshake error; assert the emitted slog record contains `failure_class="tls"` while `outcome="failure"`. | REQ-EVAL2-005 |
| §5.4 | Dashboard auto-provisioning: `docker compose up grafana prometheus`; assert dashboard `adapter-reliability` is queryable via Grafana API within 30 seconds; assert all 4 core panels return non-empty data when fed seeded fixtures. | REQ-EVAL2-006, REQ-EVAL2-007 |
| §5.5 | 7d alert firing: seed Prometheus with synthetic time series where one adapter's `usearch:adapter_success_rate_7d` < 0.85 sustained 30 min; assert the alert transitions `inactive → pending → firing` and Alertmanager receives the alert with correct labels. | REQ-EVAL2-008, REQ-EVAL2-009 |
| §5.6 | Acute 1h alert firing: simulate sudden adapter outage (success_rate_1h < 0.50 for 5 min); assert acute alert fires within 5 min of threshold cross. | REQ-EVAL2-008 |
| §5.7 | Partial-result ratio alert: simulate fanout where >30% of dispatches have at least one failing adapter sustained 15 min; assert ratio alert fires. | REQ-EVAL2-008 |
| §5.8 | **(DEFERRED post-V1, A2)** Circuit-state alert: emit `usearch_adapter_circuit_state{state="open"}=1` for one adapter sustained 10 min; assert alert fires. NOT a V1 gate item — re-enabled when an upstream emits circuit state. | REQ-EVAL2-008 (deferred) |
| §5.9 | Health endpoint (reuses SPEC-UI-002 admin mux): with mixed-status fixture, GET /api/admin/adapters/health (LoopbackOnly); assert response JSON schema correctness and status mapping (healthy / degraded / unhealthy); assert HTTP 503 when any adapter is unhealthy. Also assert GET /api/admin/adapters returns populated `success_count`/`fail_count`/`success_rate`. | REQ-EVAL2-010 |
| §5.10 | Cardinality budget: register all three new metric families with 12 adapters × all label permutations; assert total series ≤ 132 (per NFR-EVAL2-001 math); assert `Registry.labelNames` test passes with only `state` added. | NFR-EVAL2-001, REQ-EVAL2-003 |
| §5.11 | Bench non-regression: run `internal/fanout/bench_test.go` before and after instrumentation; assert p99 latency delta < 1% (NFR-EVAL2-005). | NFR-EVAL2-005 |
| §5.12 | 30-day retention: configure Prometheus with 30d retention flag; verify the 7d recording rule has continuous data after a Prometheus container restart (no data loss). | NFR-EVAL2-004 |

---

## 6. Dependencies & Blocks

### 6.1 Upstream SPEC dependencies (depends_on)

- **SPEC-OBS-001 (implemented)** — provides `usearch_adapter_calls_
  total` and `usearch_adapter_call_duration_seconds` collectors via
  `internal/obs/metrics/metrics.go`. EVAL-002 reads these via PromQL
  recording rules and extends the registry with three new families.
  Without OBS-001 there is no canonical metric to aggregate.

- **SPEC-FAN-001 (implemented)** — produces `Result.AdapterErrors`
  map at end of `fanout.Dispatch`. EVAL-002 instruments that map's
  contents as the `usearch_fanout_partial_total` source. Without
  FAN-001 there is no partial-result invariant to count.

- **SPEC-ADP-001..009 (implemented or in-progress)** — the 12+
  adapter sources whose names populate the `adapter` label
  cardinality. EVAL-002 assumes Registry.List() returns these
  adapters. New adapters added after EVAL-002 auto-flow into the
  dashboard via the Grafana template variable (no SPEC update
  required).

- **SPEC-CACHE-001 (implemented)** — produces `usearch_access_phase_
  attempts_total{phase,outcome}` for the 5-phase access fallback.
  EVAL-002 reads this metric ONLY for dashboard cross-links (no
  recording-rule dependency); the `usearch_adapter_circuit_state`
  gauge is independent and is empty in V1 because no upstream package
  emits circuit transitions yet — hence the circuit alert + panel are
  deferred (amendment A2).

- **SPEC-UI-002 (implemented)** — owns the admin route group
  `internal/api/admin/handler_adapters.go` (mounted at `cmd/usearch-
  api/main.go:71` behind `LoopbackOnly` middleware) and the
  `AdapterAdminView` struct with its `success_count`/`fail_count`
  stub fields (`internal/adapters/registry.go:212,215`). EVAL-002
  REQ-EVAL2-010 extends this handler (fills the stubs, adds the
  `/api/admin/adapters/health` sibling) rather than building a new
  admin server. Without SPEC-UI-002 there is no admin handler or
  LoopbackOnly middleware to reuse — this is why SPEC-UI-002 is now
  a hard `depends_on` (added in v0.2.0).

### 6.2 Related but soft (related)

- **SPEC-EVAL-001 (M8, not yet drafted)** — citation faithfulness
  CI gate. Independent of EVAL-002; the two share no metric. EVAL-001
  emits CI test metrics, EVAL-002 emits runtime production metrics.

- **SPEC-EVAL-003 (M8, not yet drafted)** — Korean benchmark.
  Independent; EVAL-002 dashboard provides Grafana template variable
  to filter Korean adapters but does not implement the benchmark.

- **SPEC-AUTH-003 (M6, draft)** — audit log. Forward dep: when
  AUTH-003 ships query-replay capability, EVAL-002 dashboard adds a
  drilldown link from "high-failure adapter" panel to audit-log
  reconstruction. V1 EVAL-002 has no AUTH-003 hard dep.

### 6.3 Downstream blocked SPECs (blocks)

- **SPEC-REL-001 (M9)** — V1 release tag. M9 exit criterion includes
  "adapter success rate dashboard live"; EVAL-002 ships exactly that.
  REL-001 cannot tag V1.0.0 without EVAL-002 dashboards + alerts
  rendered against a live Prometheus.

### 6.4 External dependencies (run-phase pins)

- Prometheus ≥ 2.45 (recording rules + `promtool check rules` schema
  stable since 2.45)
- Grafana ≥ 11.0 (dashboard JSON model v40 compatibility)
- Alertmanager ≥ 0.27 (`amtool check-config` stable)
- `promtool` and `amtool` available in CI container for rule
  validation gates
- Existing `prometheus/client_golang` v1.x already pinned by
  SPEC-DEP-001; no new Go dependency
- Loki / Promtail are OPTIONAL — REQ-EVAL2-005 dashboard log-link
  panel renders empty if Loki datasource is not configured

---

## 7. Files to Create / Modify

### 7.1 Created

| Marker | Path | Purpose |
|--------|------|---------|
| [NEW] | `deploy/prometheus/recording-rules.yml` | 5 recording rules per REQ-EVAL2-001 |
| [NEW] | `deploy/prometheus/alerts.yml` | 4 alert rules per REQ-EVAL2-008 |
| [NEW] | `deploy/alertmanager/alertmanager.yml` | Default config + commented receivers per REQ-EVAL2-009 |
| [NEW] | `deploy/grafana/dashboards/adapter-reliability.json` | 5-panel dashboard per REQ-EVAL2-006 |
| [NEW] | `deploy/grafana/provisioning/dashboards/adapter-reliability.yaml` | Dashboard auto-provisioning per REQ-EVAL2-007 |
| [NEW] | `deploy/grafana/provisioning/datasources/prometheus.yaml` | Prometheus datasource auto-config per REQ-EVAL2-007 |
| [NEW] | `internal/obs/metrics/fanout_partial.go` | New CounterVec + GaugeVec registration per REQ-EVAL2-003 |
| [NEW] | `internal/obs/metrics/fanout_partial_test.go` | Cardinality + emission tests per NFR-EVAL2-001 |
| [NEW] | `internal/api/admin/handler_adapters_health.go` | `/api/admin/adapters/health` sibling handler on the existing admin mux per REQ-EVAL2-010 (SPEC-UI-002 package; no new server/port) |
| [NEW] | `internal/api/admin/handler_adapters_health_test.go` | Health endpoint + LoopbackOnly integration test |
| [NEW] | `docs/operations/adapter-reliability-runbook.md` | Operator runbook linked from alert annotations per REQ-EVAL2-008 |
| [NEW] | `.github/workflows/promtool-validate.yml` | CI gate running `promtool check rules` + `amtool check-config` |

### 7.2 Modified

| Path | Change |
|------|--------|
| `internal/obs/metrics/metrics.go` | Extend `Registry.labelNames` with `state` enum value (line ~265-289); register the three new families in `NewRegistry` |
| `internal/obs/metrics/metrics_test.go` | Update cardinality allowlist test to include new label values |
| `internal/fanout/dispatch.go` | After `eg.Wait()`, iterate `Result.AdapterErrors` and increment `usearch_fanout_partial_total{adapter}` once per failing adapter (REQ-EVAL2-004) |
| `internal/fanout/dispatch_test.go` | Add partial-result emission test (per §5.2 scenario) |
| `internal/adapters/registry.go` | (a) In `wrappedAdapter.emit` (line ~433, was wrongly cited as :223 in v0.1.0), add `failure_class` slog attribute derived from error type (REQ-EVAL2-005). (b) Populate `AdapterAdminView.success_count`/`fail_count` (lines 212/215, currently 0 stubs) and add a derived `success_rate` field + populate it in `SnapshotForAdmin` (line ~243) from per-adapter telemetry (REQ-EVAL2-010a). NOTE: `AdapterAdminView` is SPEC-UI-002-owned — coordinate the struct change with that SPEC. |
| `internal/adapters/registry_test.go` | Add failure-class attribute assertion (§5.3) + populated-count assertion for SnapshotForAdmin |
| `internal/api/admin/handler_adapters.go` | (SPEC-UI-002 package) Reflect populated counts in `GET /api/admin/adapters` response (REQ-EVAL2-010a). Register the new `/api/admin/adapters/health` sibling in `cmd/usearch-api/main.go:registerAdminRoutes` behind `LoopbackOnly` |
| `deploy/prometheus/prometheus.yml` | Add `rule_files: [recording-rules.yml, alerts.yml]` section; add Alertmanager target; **change `evaluation_interval: 15s` → `1m` (line 9) per NFR-EVAL2-003 / amendment A4** (scrape_interval stays 15s) |
| `deploy/docker-compose.yml` | Add `grafana` and `alertmanager` service definitions; mount provisioning + dashboards as volumes |
| `.moai/project/roadmap.md` | (sync phase only) Update M8 row status when SPEC ships |

### 7.3 Existing — Unchanged

- `pkg/types/errors.go` — frozen per §4 exclusion; canonical 6-tuple stays.
- All `internal/adapters/<source>/` packages — read-only consumers; emit unchanged metrics.
- `services/*` Python sidecars — out of EVAL-002 scope.
- `cmd/usearch` CLI — no admin endpoint changes.

---

## 8. Open Questions

The SPEC's `_TBD_` markers and the research artifact's §10 are the
canonical list. Restated here for plan-auditor convenience:

1. **Circuit-state emission source**: `usearch_adapter_circuit_state`
   is consumed by REQ-EVAL2-003 (c) but no current package emits it
   (SPEC-CACHE-001 cascade tracks per-phase outcomes, not adapter-
   level circuit state). _Resolution (v0.2.0 amendment A2)_: the
   metric family is registered for forward-compat, BUT the circuit-
   open alert (REQ-EVAL2-008 4th rule) and the circuit-state matrix
   panel (REQ-EVAL2-006 panel #5) are **DEFERRED to post-V1 and
   removed from the V1 acceptance gate** so a permanently-no-data
   alert/panel does not block V1. A future SPEC-RESIL-001 (or
   CACHE-001 v2) wires up actual transitions and re-enables them.
   Does NOT block EVAL-002 plan-auditor.

2. **Grafana version**: 11.x is current LTS as of 2026-05. If a
   downstream operator runs Grafana 10.x, dashboard JSON may need a
   compat shim. _TBD_: confirm minimum Grafana version with first
   operator deployment; bump CI gate to test against the chosen
   minimum.

3. **Alertmanager standalone vs Prometheus inline alerting**:
   Prometheus 2.x supports inline alerting (no separate Alertmanager
   process), but lacks routing/grouping. SPEC fixes target as
   standalone Alertmanager per HISTORY D4; revisit if operators
   prefer no extra process.

4. **`failure_class` taxonomy completeness**: REQ-EVAL2-005 documents
   7 classes (5xx / 4xx / dns / tls / parse / transcript / unknown).
   Real-world adapter failures may surface additional classes
   (websocket / OAuth-refresh / quota-headers). _Resolution_:
   taxonomy is open-set in slog attribute (no enum enforcement);
   runbook documents the canonical 7 and runbook updates absorb
   new classes.

5. **Dashboard hosting on staging**: should the SPEC ship a public
   staging Grafana URL for marketing? _Resolution_: NO — self-hosted
   only per `.moai/project/product.md` §4; dashboard JSON is the
   shipped artifact, not a hosted instance. README screenshot in
   docs/operations covers the demo need.

6. **Prometheus 30-day retention storage cost**: 12 adapters × 6
   outcomes × 1 series × ~31M observations over 30 days ≈ 50-100 MB
   on disk per Prometheus's TSDB compression. _TBD_: document
   storage sizing in runbook; operators with larger fleets configure
   own retention.

7. **Multi-Prometheus federation** for multi-cluster usearch
   deployments: out of V1 scope; single-instance Prometheus
   assumption. Federation guidance is a post-V1 ops doc.

8. **Alert silencing during planned adapter maintenance** (e.g.,
   Naver API key rotation): Alertmanager supports silences via
   `amtool silence add`. Runbook documents the operator workflow;
   no automated silence wiring in V1.

These items do NOT block plan-auditor PASS; they are tagged as
known unresolved scope edges with rationale.

---

## 9. References

External (cited in research.md §12):

- Prometheus recording rules best practices: https://prometheus.io/docs/practices/rules/
- Prometheus alerting overview: https://prometheus.io/docs/alerting/latest/overview/
- Prometheus PromQL functions: https://prometheus.io/docs/prometheus/latest/querying/functions/
- Grafana provisioning: https://grafana.com/docs/grafana/latest/administration/provisioning/
- Grafana dashboard JSON model: https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/view-dashboard-json-model/
- Alertmanager configuration: https://prometheus.io/docs/alerting/latest/configuration/
- `promtool` reference: https://prometheus.io/docs/prometheus/latest/command-line/promtool/
- `amtool` reference: https://github.com/prometheus/alertmanager/blob/main/cmd/amtool/README.md
- SearXNG instance monitoring (prior art): https://docs.searxng.org/admin/instance.html
- Crawlee monitoring (prior art): https://crawlee.dev/api/core/class/Statistics

Internal (project files):

- `.moai/project/roadmap.md` §M8 SPEC-EVAL-002 row + §5 M8 exit
  criterion "adapter success rate dashboard live"
- `.moai/project/product.md` — V1 self-hosted positioning; M9 release
  criteria
- `.moai/specs/SPEC-OBS-001/spec.md` — REQ-OBS-003 / REQ-OBS-004 /
  NFR-OBS-002 (label allowlist discipline)
- `.moai/specs/SPEC-CORE-001/` — `pkg/types/errors.go:OutcomeFromError`
  6-tuple canonical taxonomy
- `.moai/specs/SPEC-FAN-001/spec.md` — `Result.AdapterErrors` semantic
- `.moai/specs/SPEC-CACHE-001/spec.md` — 5-phase access fallback
  observability (cross-link target in dashboard)
- `internal/obs/metrics/metrics.go` — `Registry.labelNames` allowlist
  (line ~265-289); extend with `state` only
- `internal/adapters/registry.go:433` — `wrappedAdapter.emit`
  extension point for REQ-EVAL2-005 `failure_class` slog attribute
  (corrected from :223 in v0.2.0 amendment A3)
- `internal/adapters/registry.go:200,243` — `AdapterAdminView`
  (SPEC-UI-002) with `success_count`/`fail_count` stubs +
  `SnapshotForAdmin`; REQ-EVAL2-010a fill target
- `internal/api/admin/handler_adapters.go` + `cmd/usearch-api/main.go:71`
  — existing `/api/admin/adapters` handler + LoopbackOnly mount that
  REQ-EVAL2-010 reuses (no new :9090 server)
- `deploy/prometheus/prometheus.yml:9` — `evaluation_interval: 15s`,
  must become `1m` per amendment A4 / NFR-EVAL2-003
- `internal/fanout/dispatch.go` — partial-result accounting hook for
  REQ-EVAL2-004
- `internal/access/observability.go` — SPEC-CACHE-001 prior art for
  phase-level metric emission patterns
- `deploy/prometheus/prometheus.yml` — existing scrape config that
  EVAL-002 extends with `rule_files`

---

*End of SPEC-EVAL-002 v0.2.0 (draft).*
