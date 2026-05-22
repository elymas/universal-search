# SPEC-EVAL-002 Research — Adapter reliability dashboard

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22

이 research artifact 는 SPEC-EVAL-002 의 Phase 0.5 deep-dive 다.
Prometheus rolling-window 패턴, Grafana dashboard-as-code 트레이드
오프, Alertmanager 라우팅 모델, 비교 가능한 OSS adapter health
dashboard, 12+ adapter 의 failure-mode 매트릭스, cardinality 예산
계산을 정리한다. SPEC 의 6개 pinned decision (HISTORY D1-D6) 은
모두 이 문서의 근거 위에 서 있다.

---

## 1. Failure taxonomy: 기존 `OutcomeFromError` 6-tuple 의 충분성

### 1.1 현재 enum

`pkg/types/errors.go:174` `OutcomeFromError` 는 SPEC-CORE-001 에서
canonical 로 정의된 매핑이다:

| Outcome label | 원인 |
|---------------|------|
| `success` | `err == nil` |
| `timeout` | `errors.Is(err, context.DeadlineExceeded)` |
| `rate_limited` | `CategoryRateLimited` (`ErrRateLimited`, HTTP 429, source-specific quota signals) |
| `unavailable` | `CategoryUnavailable` (`ErrSourceUnavailable`, DNS NXDOMAIN, dial timeout, HTTP 503 without retry-after) |
| `transient` | `CategoryTransient` (`ErrTransient`, HTTP 5xx, network blips, classified retryable) |
| `failure` | `CategoryPermanent`, `CategoryUnknown`, fallthrough (HTTP 4xx, parse errors, unclassified) |

이 6-tuple 은 운영자가 "성공 / 잠시 다시 시도 가능 / 장기 실패 /
quota 초과 / 인프라 다운 / 분류 불가" 의 5+1 구분을 즉시 읽을 수
있도록 설계됐다.

### 1.2 더 세분된 cuts 가 필요한가?

운영 시나리오상 다음 cuts 가 유의미하다:

- HTTP 5xx vs 4xx (transient 안에 둘 다 포함)
- TLS handshake failure (unavailable 안 에)
- DNS NXDOMAIN vs dial timeout (둘 다 unavailable)
- Parse error / schema mismatch (failure 안에)
- YouTube transcript extraction failure (failure 안에)
- OAuth token refresh failure (Reddit / GitHub) (failure 안에)
- Korean adapter HTML 구조 변경 (parse error sub-class)

이걸 **Prometheus label 로 끌어올리면** cardinality 가
12 adapter × 6 outcome × 7 failure_class = **504 시리즈**.
NFR-OBS-002 의 cardinality allowlist 정책 위반.

### 1.3 해결: slog `failure_class` attribute + Loki 드릴다운

REQ-EVAL2-005 의 결론은:

- Prometheus label 은 6-tuple 그대로 유지
- `failure_class` 는 slog record 의 attribute 로만 노출
- Grafana 패널에 Loki query link 제공: `{service="usearch"} | json |
  adapter = "<selected>" and outcome != "success"` 로 드릴다운
- Loki 가 없는 deployment 에서는 패널이 비어 보이지만 alert / 대
  시보드 핵심 기능은 무손상

이 패턴은 Prometheus 공식 문서의 "high-cardinality labels" 경고
와 일치한다 (https://prometheus.io/docs/practices/naming/#labels).

---

## 2. Partial-result vs per-adapter 성공률 의 의미 분리

### 2.1 fanout 의 partial-result invariant

SPEC-FAN-001 `Result.AdapterErrors` 는 `map[string]error` 타입으
로, fanout dispatch 가 종료될 때 어떤 adapter 가 에러를 반환했는지
기록한다. fanout 정책상 단 1개 adapter 라도 성공하면 query 자체는
성공으로 사용자에게 반환된다.

따라서 두 개의 다른 지표가 필요하다:

| 지표 | 측정 단위 | 의미 |
|------|---------|------|
| `usearch_adapter_calls_total{adapter,outcome}` | 호출 1개 | 해당 adapter 의 단일 호출이 성공/실패 |
| `usearch_fanout_partial_total{adapter}` | dispatch 1개 | fanout dispatch 안에서 이 adapter 가 결과를 기여하지 못함 |

전자는 adapter 자체의 건강성, 후자는 사용자가 받는 응답의 완전성
을 측정한다.

### 2.2 두 지표를 합치지 않는 이유

운영자가 "Naver adapter 성공률 70%" 와 "Naver 가 빠진 partial
응답 비율 30%" 를 따로 봐야 한다. 합쳐서 단일 지표로 노출하면
"성공률이 95% 인데 사용자는 매번 결과의 일부를 못 본다" 같은
나쁜 상황이 숨겨진다. SPEC-EVAL-002 는 두 패널을 별도로 둔다.

### 2.3 partial 의 의미를 명확히

`usearch_fanout_partial_total{adapter="reddit"}` 는 "Reddit 이
fanout 의 일원이었으나 결과를 기여하지 못한 dispatch 의 개수" 다.
*Reddit 이 호출되지 않은* dispatch (예: intent router 가 Reddit
을 라우팅 대상에서 제외) 는 분모에도 분자에도 들어가지 않는다.

---

## 3. Prometheus rolling-window 패턴

### 3.1 PromQL 옵션

7일 rolling success rate 를 계산하는 3가지 패턴:

#### Option A — `rate()` over 7d range

```promql
rate(usearch_adapter_calls_total{outcome="success"}[7d])
/
rate(usearch_adapter_calls_total[7d])
```

장점: 가장 직관적, Prometheus 공식 권장.
단점: 매 쿼리마다 7일치 raw counter sample 을 스캔 — Grafana
패널 렌더가 1-3초씩 걸린다. 12 adapter × 5 패널 = 60 시리즈 동시
연산.

#### Option B — `increase()` over 7d

```promql
increase(usearch_adapter_calls_total{outcome="success"}[7d])
/
increase(usearch_adapter_calls_total[7d])
```

장점: counter 리셋 자동 보정.
단점: `rate()` 와 본질적으로 동일한 비용, semantically `rate()` 가
더 정확.

#### Option C — Recording rule pre-aggregation (선택)

```yaml
# deploy/prometheus/recording-rules.yml
groups:
  - name: usearch_adapter_reliability
    interval: 1m
    rules:
      - record: usearch:adapter_success_rate_7d
        expr: |
          sum by (adapter) (rate(usearch_adapter_calls_total{outcome="success"}[7d]))
          /
          sum by (adapter) (rate(usearch_adapter_calls_total[7d]))
```

장점: Prometheus 가 1분마다 한 번 사전 계산, Grafana 쿼리는
이미 집계된 시리즈만 읽음 → 패널 렌더 < 200ms.
단점: 추가 시리즈 저장 (12 adapter × 1 시리즈 = 12 시리즈, 무시
가능).

**채택: Option C** (HISTORY D2). NFR-EVAL2-002 의 < 2초 렌더
타겟을 충족하는 유일한 방법.

### 3.2 In-process ring buffer 가 배제된 이유

대안으로 usearch process 내부에 직접 7일 ring buffer 를 유지하고
새 metric family 로 노출하는 패턴을 고려했다. 배제 이유:

1. **Process restart 시 데이터 손실** — usearch container 재배포
   마다 7일 데이터 0 초기화 → 운영자의 회귀 분석 불가
2. **메모리 사용량** — 12 adapter × 6 outcome × 1초 해상도 ×
   7일 = 약 7MB 만 추가지만, Prometheus 가 이미 같은 저장소를
   훨씬 효율적으로 갖고 있음
3. **NFR-OBS-002 label allowlist 확장 필요** — 새 metric family
   가 라벨 카디널리티 budget 을 소모
4. **Pull vs push 일관성** — Prometheus 의 scrape pull model 과
   in-process aggregation 이 의미적으로 충돌 (timestamp drift 문
   제)

### 3.3 Recording rule 5개 (REQ-EVAL2-001)

```yaml
groups:
  - name: usearch_adapter_reliability
    interval: 1m
    rules:
      - record: usearch:adapter_success_rate_1h
        expr: |
          sum by (adapter) (rate(usearch_adapter_calls_total{outcome="success"}[1h]))
          /
          sum by (adapter) (rate(usearch_adapter_calls_total[1h]))

      - record: usearch:adapter_success_rate_24h
        expr: |
          sum by (adapter) (rate(usearch_adapter_calls_total{outcome="success"}[24h]))
          /
          sum by (adapter) (rate(usearch_adapter_calls_total[24h]))

      - record: usearch:adapter_success_rate_7d
        expr: |
          sum by (adapter) (rate(usearch_adapter_calls_total{outcome="success"}[7d]))
          /
          sum by (adapter) (rate(usearch_adapter_calls_total[7d]))

      - record: usearch:adapter_failure_rate_by_outcome_24h
        expr: |
          sum by (adapter, outcome) (rate(usearch_adapter_calls_total{outcome!="success"}[24h]))

      - record: usearch:adapter_fanout_partial_ratio_24h
        expr: |
          sum by (adapter) (rate(usearch_fanout_partial_total[24h]))
          /
          sum by (adapter) (rate(usearch_adapter_calls_total[24h]))
```

각 rule 은 1분마다 평가, 결과는 Prometheus TSDB 에 새 시리즈로
저장. Grafana 패널은 raw counter 가 아니라 이 사전 집계 시리즈를
쿼리.

---

## 4. Alertmanager 라우팅 모델

### 4.1 왜 Alertmanager 인가

Prometheus 2.x 는 alert rule evaluation 만 한다 — 알림 전송, 그
룹핑, silencing, 라우팅은 별도 Alertmanager 컴포넌트의 책임이다.
Alternative 는 Grafana Unified Alerting (Grafana 9+) 인데:

| 비교 | Alertmanager | Grafana Unified Alerting |
|------|--------------|--------------------------|
| 의존성 | 별도 프로세스 (Go binary) | Grafana 안에 내장 |
| 표현력 | PromQL + route tree | PromQL + 추가 UI 편집 |
| 라우팅 | route tree (label match) | folder + label match |
| Silencing | `amtool silence` CLI / UI | Grafana UI |
| Multi-tenant | inhibition rules | tenancy via folders |
| 운영자 친숙도 | 표준 Prometheus stack | Grafana 전용 |

SPEC 은 Alertmanager 를 택했다 (HISTORY D4):

1. self-hosted operator 대부분 이미 Prometheus + Alertmanager 표
   준 stack 운영 중
2. `promtool` / `amtool` 가 CI gate 로 검증 가능 (Grafana alert
   는 Grafana API 호출 필요)
3. alert rule 파일이 YAML 평문 → git diff 친화적
4. Alertmanager `null` receiver 로 silent default 가능 (Grafana
   UA 는 default routing 비활성화가 명시적)

### 4.2 라우팅 트리 설계

```yaml
# deploy/alertmanager/alertmanager.yml (skeleton)
route:
  receiver: 'null'
  group_by: ['alertname', 'adapter']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  routes:
    - match:
        severity: critical
      # receiver: 'pagerduty-critical'  # uncomment + configure
      receiver: 'null'
    - match:
        severity: warning
      # receiver: 'webhook-warnings'  # uncomment + configure
      receiver: 'null'

receivers:
  - name: 'null'

  # - name: 'webhook-warnings'
  #   webhook_configs:
  #     - url: 'https://discord.com/api/webhooks/.../usearch-warnings'
  #       send_resolved: true

  # - name: 'pagerduty-critical'
  #   pagerduty_configs:
  #     - routing_key: 'YOUR_PAGERDUTY_ROUTING_KEY'

  # - name: 'opsgenie-team'
  #   opsgenie_configs:
  #     - api_key: 'YOUR_OPSGENIE_API_KEY'
  #       teams: 'usearch-oncall'

inhibit_rules:
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'adapter']
```

운영자가 알람 채널을 골라서 receiver block 만 uncomment / 편집.
SPEC 은 채널을 강제하지 않는다.

### 4.3 4개 alert rule (REQ-EVAL2-008)

```yaml
# deploy/prometheus/alerts.yml (skeleton)
groups:
  - name: usearch_adapter_reliability
    rules:
      - alert: AdapterSuccessRate7dLow
        expr: usearch:adapter_success_rate_7d < 0.85
        for: 30m
        labels:
          severity: warning
          spec: SPEC-EVAL-002
        annotations:
          summary: "Adapter {{ $labels.adapter }} 7-day success rate {{ $value | humanizePercentage }} below 85%"
          runbook_url: "https://github.com/.../docs/operations/adapter-reliability-runbook.md#adaptersuccessrate7dlow"

      - alert: AdapterSuccessRate1hCritical
        expr: usearch:adapter_success_rate_1h < 0.50
        for: 5m
        labels:
          severity: critical
          spec: SPEC-EVAL-002
        annotations:
          summary: "Adapter {{ $labels.adapter }} 1-hour success rate {{ $value | humanizePercentage }} acutely degraded"
          runbook_url: "https://github.com/.../docs/operations/adapter-reliability-runbook.md#adaptersuccessrate1hcritical"

      - alert: FanoutPartialRatioHigh
        expr: usearch:adapter_fanout_partial_ratio_24h > 0.30
        for: 15m
        labels:
          severity: warning
          spec: SPEC-EVAL-002
        annotations:
          summary: "Adapter {{ $labels.adapter }} missing from {{ $value | humanizePercentage }} of fanout dispatches over 24h"
          runbook_url: "https://github.com/.../docs/operations/adapter-reliability-runbook.md#fanoutpartialratiohigh"

      - alert: AdapterCircuitOpen
        expr: usearch_adapter_circuit_state{state="open"} == 1
        for: 10m
        labels:
          severity: critical
          spec: SPEC-EVAL-002
        annotations:
          summary: "Adapter {{ $labels.adapter }} circuit has been open for over 10 minutes"
          runbook_url: "https://github.com/.../docs/operations/adapter-reliability-runbook.md#adaptercircuitopen"
```

---

## 5. Grafana dashboard-as-code 트레이드오프

### 5.1 옵션 비교

| 옵션 | 장점 | 단점 |
|------|------|------|
| Raw JSON in repo | 단순, 표준, Grafana UI 에서 export 가능 | 변경 추적 노이즈 큼, YAML 보다 verbose |
| Grafonnet / Grafana Tanka (jsonnet) | DRY, 재사용 가능한 helper | jsonnet 학습 곡선, 추가 빌드 단계 |
| Terraform Grafana provider | IaC 통합 | Terraform state 관리 부담 |
| Grafana provisioning + 기존 dashboard JSON | 표준, compose 자연스러움 | git diff 가 크다 |

**채택: Raw JSON + Provisioning** (HISTORY D5).

이유:
1. 첫 번째 dashboard 하나만 V1 에 출시 → DRY 의 이득 작다
2. `deploy/grafana/provisioning/` 패턴은 Grafana 공식 권장
3. 운영자가 Grafana UI 에서 패널을 직접 편집 후 JSON export →
   git commit 의 흐름이 단순
4. jsonnet 빌드 단계 추가 시 CI 복잡도 증가

### 5.2 5개 패널 설계 (REQ-EVAL2-006)

| 패널 # | 타입 | 데이터 source | 시각화 |
|--------|------|--------------|--------|
| 1 | Heatmap | `usearch:adapter_success_rate_24h` per adapter | x: adapter, y: 시간 (1h bucket), color: 성공률 |
| 2 | Time-series | `usearch:adapter_success_rate_7d` | 12개 라인 (adapter 별), y: 0-100%, threshold line at 85% |
| 3 | Stacked bar | `usearch:adapter_failure_rate_by_outcome_24h` by outcome | x: 시간, stack: outcome (5 colors), y: failures/sec |
| 4 | Time-series | `usearch:adapter_fanout_partial_ratio_24h` | 12개 라인, threshold line at 30% |
| 5 | State timeline | `usearch_adapter_circuit_state` | x: 시간, y: adapter, color: state {closed=green, half_open=yellow, open=red} |

상단에 Grafana template variable `adapter`:
```
label_values(usearch_adapter_calls_total, adapter)
```
운영자가 특정 adapter (예: `naver`) 만 보거나 다중 선택 가능.

### 5.3 Provisioning 파일

```yaml
# deploy/grafana/provisioning/datasources/prometheus.yaml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
```

```yaml
# deploy/grafana/provisioning/dashboards/adapter-reliability.yaml
apiVersion: 1
providers:
  - name: 'usearch-dashboards'
    folder: 'Universal Search'
    type: file
    options:
      path: /etc/grafana/provisioning/dashboards/files
```

docker-compose volume:
```yaml
grafana:
  image: grafana/grafana:11.3.0
  volumes:
    - ./deploy/grafana/provisioning:/etc/grafana/provisioning
    - ./deploy/grafana/dashboards:/etc/grafana/provisioning/dashboards/files
```

---

## 6. Cardinality budgeting

### 6.1 기존 부담

`internal/obs/metrics/metrics.go:265` 의 labelNames 는 SPEC-OBS-001
이후 다음과 같이 누적되어 있다:

```
method, route, status_class, adapter_class, adapter, outcome,
version, commit, go_version, provider, model, mode, store, op,
shard, agent, result, reason, trigger, reason_class
```

전체 시리즈 카운트는 운영 상태에서 약 500-1500 수준 (정확한 측정
은 `count({__name__=~"usearch_.*"})` Prometheus 쿼리 결과).

### 6.2 EVAL-002 추가분

| 새 metric family | label set | 12 adapter 기준 시리즈 |
|------------------|-----------|----------------------|
| `usearch_fanout_partial_total` | `{adapter}` | 12 |
| `usearch_adapter_health_status` | `{adapter}` | 12 |
| `usearch_adapter_circuit_state` | `{adapter, state}` | 12 × 3 = 36 |

총 추가 = 60 시리즈. 새 label NAME 은 `state` 하나만 (값 enum
`{closed, open, half_open}` 3개로 bounded).

### 6.3 시리즈 카운트 검증

`internal/obs/metrics/metrics_test.go` 의 NFR-OBS-002 cardinality
test 를 다음과 같이 확장:

```go
// 기존 allowlist 에 "state" 만 추가
expected := append(baseLabels, "state")
got := registry.AllLabelNames()
if !reflect.DeepEqual(got, expected) {
    t.Errorf("labelNames drifted: got %v, want %v", got, expected)
}
```

12 adapter × 6 outcome (기존) = 72 + 60 (신규) = 132 시리즈. NFR-
EVAL2-001 의 ≤500 budget 안.

향후 38 adapter 까지 확장해도 38 × 6 + 38 × (1+1+3) = 228 + 190
= 418 시리즈 — 여전히 budget 안.

### 6.4 절대 추가하지 않을 label

- `team_id` / `user_id` / `tenant_id`: cardinality explosion
- `query_hash` / `query_text`: PII risk + cardinality explosion
- `region` / `datacenter`: V1 single-tenant self-hosted이므로 무
  의미
- `request_id`: per-call unique, 무한 cardinality
- `failure_class`: 7+ 값 × 12 adapter × 6 outcome = 504+ 시리즈

전부 slog / Loki 로 빠지거나 admin endpoint JSON 응답에서만 노출.

---

## 7. 12+ adapter failure-mode 매트릭스

`.moai/project/roadmap.md` §M3 의 9개 SPEC-ADP 가 실제로 통합하는
12+ 어댑터별 예상 failure mode:

| Adapter | SPEC | 주요 failure mode | 매핑 outcome |
|---------|------|-------------------|--------------|
| Reddit | ADP-001 | OAuth token refresh 실패, rate-limit, 500 errors | failure / rate_limited / transient |
| Hacker News | ADP-002 | Algolia API 다운 (드뭄), 5xx | unavailable / transient |
| arXiv | ADP-003 | XML schema 변경, 검색 latency | failure / timeout |
| paper-search | ADP-003 | MCP wrapper 프로세스 다운 | unavailable |
| GitHub | ADP-004 | 토큰 만료, 5xx, secondary rate limit | failure / rate_limited |
| YouTube | ADP-005 | yt-dlp HTTP parse 깨짐, transcript timeout, IP block | failure / timeout / rate_limited |
| Bluesky | ADP-006 | AT Protocol 인증 실패, 5xx | failure / transient |
| X (Twitter) | ADP-006 | ScrapeCreators quota 소진, scraper detection | rate_limited / failure |
| SearXNG | ADP-007 | upstream search engine 차단 → SearXNG 500, network | unavailable / transient |
| Naver web/news/blog/shopping | ADP-008 | API key quota 초과, MCP wrapper crash | rate_limited / unavailable |
| Naver DataLab | ADP-008 | 일별 quota 매우 낮음 (1000회) | rate_limited |
| KoreaNewsCrawler | ADP-009 | HTML 구조 변경 (parse error), 한국 IP block | failure (parse) / unavailable |
| Daum | ADP-009 | 사이트 구조 변경 | failure (parse) |
| Korean RSS | ADP-009 | 사용자 지정 RSS 다운 | unavailable |

= **12개 어댑터 base + 5개 Naver sub-API + 2-N 개 사용자 정의 RSS**.
`adapter` label cardinality 는 동적이지만 12-20 범위 안에서 안정.

### 7.1 운영 관점 우선순위

- **High-noise**: Naver suite (rate_limit 자주), X (scraper detect)
- **High-criticality**: SearXNG (general web fallback 없으면 적
  음), GitHub (코드 검색 핵심)
- **Silent-failure-prone**: KoreaNewsCrawler (parse error 시 결과
  0 개), YouTube (transcript 만 깨질 때 metadata 는 정상)

Dashboard 와 runbook 은 이 우선순위를 반영해서 한국어/소셜
adapter 의 임계값을 더 보수적으로 둘 수 있다 (operator-tunable).

---

## 8. OSS adapter health dashboard 사례

### 8.1 SearXNG instance monitoring

SearXNG 공식 admin dashboard
(https://docs.searxng.org/admin/instance.html):

- per-engine response time histogram
- per-engine error rate (last 24h)
- per-engine result count distribution
- "engine state" indicator (healthy / unstable / failing)

EVAL-002 는 이 4개 핵심 패널을 동등하게 제공한다 (success rate,
duration, fanout partial ratio, circuit state).

### 8.2 Crawlee statistics

Crawlee
(https://crawlee.dev/api/core/class/Statistics) 의 통계 모델:

- requestsFinished, requestsFailed, requestsRetries
- requestAvgDuration, requestAvgFinishedDuration
- crawlerRuntimeMillis, errorCounts (per error class)

크게 다르지 않다. Crawlee 의 `errorCounts` 가 EVAL-002 의 slog
`failure_class` 와 매핑.

### 8.3 Tabby / RAG agent monitoring

Tabby (LLM 코드 어시스턴트) 는 model latency + cache hit rate
중심으로 dashboard 가짐. Search-specific 한 partial-result invariant
는 부재. EVAL-002 만의 unique 한 측면이 fanout partial ratio.

### 8.4 결론

OSS 선례는 일관되게 `(rate, error_rate, latency)` 3축을 강조한다.
EVAL-002 는 여기에 `fanout partial ratio` (fanout-specific) +
`circuit state` (future) 를 추가한 **5축** 으로 차별화. 모든 축
의 데이터 source 는 기존 metric, 새로운 instrumentation 은 partial
counter 하나만 추가.

---

## 9. Health endpoint 설계 (REQ-EVAL2-010)

### 9.1 왜 별도 endpoint 가 필요한가

Prometheus `/metrics` 는 raw counter 만 노출 → readiness probe /
운영자 ad-hoc check 용으로 적합하지 않다. `/admin/health/adapters`
는 의도된 derivation:

- k8s readiness/liveness probe 가 직접 사용
- 운영자 CLI 가 `curl localhost:9090/admin/health/adapters | jq`
  로 한 눈에 상태 확인
- Grafana 가 없는 환경 (단순 배포) 에서도 핵심 가시성 제공

### 9.2 JSON 스키마

```json
{
  "timestamp": "2026-05-22T03:14:15Z",
  "overall_status": "degraded",
  "adapters": [
    {
      "name": "reddit",
      "status": "healthy",
      "success_rate_24h": 0.98,
      "success_rate_7d": 0.97,
      "last_call_at": "2026-05-22T03:13:42Z",
      "circuit_state": "closed"
    },
    {
      "name": "naver_news",
      "status": "degraded",
      "success_rate_24h": 0.89,
      "success_rate_7d": 0.92,
      "last_call_at": "2026-05-22T03:14:10Z",
      "circuit_state": "closed"
    },
    {
      "name": "youtube",
      "status": "unhealthy",
      "success_rate_24h": 0.42,
      "success_rate_7d": 0.71,
      "last_call_at": "2026-05-22T03:12:55Z",
      "circuit_state": "half_open"
    }
  ]
}
```

HTTP status code 매핑:
- 모든 adapter healthy → 200
- 하나라도 degraded, unhealthy 없음 → 200 (degraded 는 비차단)
- 하나라도 unhealthy → 503 (k8s readiness 가 트래픽 끊을 수 있게)

### 9.3 데이터 source

핵심 결정: endpoint 는 Prometheus 를 쿼리하지 않는다 (순환 의존
회피 + cold start 문제). 대신 usearch process 내부의 in-process
counter snapshot 을 읽음:

- `usearch_adapter_calls_total{adapter, outcome}` 의 현재 값을
  prometheus.Collector.Collect API 로 직접 가져옴
- 1h / 24h 윈도우는 process restart 이후 데이터만 — 신뢰 가능
- 7d 는 process lifetime 보다 작으면 비신뢰 → field 에 `null`
  반환

운영자는 7d 정확도가 필요하면 Grafana 대시보드를 본다 (Prometheus
가 retention 보장). Endpoint 는 즉시성을 위한 보조.

---

## 10. 열린 질문 (Open questions, deferred decisions)

SPEC §8 의 _TBD_ 와 정확히 같은 목록. 여기서는 추가 컨텍스트:

1. **Circuit-state emission source**: SPEC-CACHE-001 cascade 가
   현재 phase-level outcomes 만 추적. Adapter-level circuit-state
   transition 을 emit 하는 모듈이 V1 에 없음. _TBD_ — V1 은 metric
   family 만 등록, gauge 는 항상 0. 향후 SPEC-RESIL-001 (가칭)
   이나 CACHE-001 v2 가 채움.

2. **Grafana 최소 버전**: 11.x 가 LTS 이지만 일부 운영자는 10.x
   이하 환경. Dashboard JSON 의 backward compat 결정 _TBD_ — 첫
   파일럿 deployment 의 Grafana 버전 확인 후 minimum 결정.

3. **Alertmanager 표준화 vs 옵션화**: SPEC 은 Alertmanager 를 표
   준으로 채택했지만, 일부 운영자는 Grafana UA 만 사용 (Alertmanager
   프로세스 추가 부담). Grafana UA 컨버터 가이드를 runbook 에 추
   가할지 _TBD_.

4. **`failure_class` taxonomy 완전성**: 7개 클래스 (5xx / 4xx /
   dns / tls / parse / transcript / unknown) 가 출발점. 실제 운
   영에서 새 클래스 발견 시 (e.g., OAuth refresh failure) 어떻게
   추가할지 — open-set 으로 두고 runbook 업데이트로 흡수.

5. **Dashboard demo 호스팅**: V1 self-hosted-only 정책상 공개
   staging Grafana URL 은 없음. 단, 운영자 onboarding 을 위해
   screenshot 을 runbook 에 포함. _Resolution_: NO 호스팅, YES
   screenshot.

6. **Prometheus 30일 retention storage 비용**: 12 adapter × 6
   outcome × 1 series × 30d × ~31M obs ≈ TSDB 압축 후 50-100MB.
   대규모 deployment 의 경우 다를 수 있어 runbook 에서 sizing
   가이드 제공.

7. **Multi-Prometheus federation** for multi-cluster: V1 out-of-
   scope. Single Prometheus 가정.

8. **Planned maintenance 시 alert silencing 워크플로**: Alertmanager
   `amtool silence add` CLI 사용 가이드를 runbook 에 포함. 자동
   silencing wiring 은 V1 out-of-scope.

---

## 11. Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Recording rule expression bug 가 잘못된 success rate 계산 | High | REQ-EVAL2-001 acceptance 가 hand-calculated fixture 와 비교; promtool unit test gate |
| Alert fatigue (warning 알람이 너무 자주 발생) | Medium | 30분 sustained `for:` window, 7d 윈도우 (단기 변동 흡수); 운영자가 threshold tunable |
| Acute 알람 (1h) 의 false positive (트래픽 적은 adapter 에서 한두 호출 실패가 50% 미만으로 떨어짐) | Medium | `for: 5m` 적용 + 알람 annotation 에 raw counter 값 포함; runbook 에 low-traffic adapter 명시 |
| Grafana 패널 렌더 < 2초 미충족 | Medium | Recording rule 사전 집계로 해소; raw `rate()` 쿼리 사용 금지 (REQ-EVAL2-006 명시) |
| `usearch_adapter_circuit_state` 데이터 부재로 panel "no data" 표시 | Low | V1 은 의도된 상태 — gauge 가 항상 0 이면 alert 도 안 발사, panel 도 빈 줄 표시 (오류 아님) |
| Prometheus 30d retention 디스크 부족 | Low | runbook 의 sizing 가이드; 실제 cardinality 측정 후 운영자가 retention 조정 |
| Alertmanager 미설치 환경에서 alert rule 적재가 무의미 | Low | compose 가 Alertmanager 를 default 로 띄움; standalone Prometheus only deployment 는 알림 비활성화 — `prometheus.yml` 에서 `alerting:` 블록 제거하면 됨 |
| `failure_class` slog attribute 가 Loki 없는 환경에서 활용 불가 | Low | dashboard 의 Loki 패널이 "no datasource" 표시; 핵심 5 패널은 무손상 |
| 새 adapter 등록 시 dashboard 자동 반영 안 됨 | Low | Grafana template variable `label_values(...)` 가 자동 발견 — 새 adapter 호출이 한 번이라도 발생하면 dropdown 에 등장 |
| 카디널리티 검증 테스트가 새 metric family 누락 | Medium | NFR-EVAL2-001 의 labelNames test 가 CI gate; 누락 시 build 실패 |

---

## 12. References

External (verified):

- Prometheus recording rules best practices:
  https://prometheus.io/docs/practices/rules/
- Prometheus alerting overview:
  https://prometheus.io/docs/alerting/latest/overview/
- Prometheus PromQL functions (rate, increase, sum_over_time):
  https://prometheus.io/docs/prometheus/latest/querying/functions/
- Prometheus label naming guide:
  https://prometheus.io/docs/practices/naming/#labels
- Grafana provisioning (datasources, dashboards):
  https://grafana.com/docs/grafana/latest/administration/provisioning/
- Grafana dashboard JSON model:
  https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/view-dashboard-json-model/
- Grafana template variables:
  https://grafana.com/docs/grafana/latest/dashboards/variables/
- Alertmanager configuration:
  https://prometheus.io/docs/alerting/latest/configuration/
- `promtool` reference:
  https://prometheus.io/docs/prometheus/latest/command-line/promtool/
- `amtool` reference:
  https://github.com/prometheus/alertmanager/blob/main/cmd/amtool/README.md
- prometheus/client_golang docs:
  https://pkg.go.dev/github.com/prometheus/client_golang/prometheus
- SearXNG instance monitoring:
  https://docs.searxng.org/admin/instance.html
- Crawlee Statistics class:
  https://crawlee.dev/api/core/class/Statistics

Internal (project files):

- `.moai/project/roadmap.md` — M8 SPEC-EVAL-002 row + M8 exit
  criterion
- `.moai/project/product.md` — V1 self-hosted positioning
- `.moai/specs/SPEC-OBS-001/spec.md` — adapter metric collectors,
  NFR-OBS-002 cardinality discipline
- `.moai/specs/SPEC-FAN-001/spec.md` — `Result.AdapterErrors`
  partial-result invariant
- `.moai/specs/SPEC-CACHE-001/spec.md` — phase-level observability
  prior art
- `pkg/types/errors.go:OutcomeFromError` — 6-tuple canonical mapping
- `internal/obs/metrics/metrics.go:265` — `Registry.labelNames`
  allowlist
- `internal/adapters/registry.go:223` — `wrappedAdapter.emit` slog
  emission point (extend with failure_class)
- `internal/fanout/dispatch.go` — partial-result counter increment
  hook
- `internal/access/observability.go` — phase metric emission pattern
  (cross-link reference, not modified)
- `deploy/prometheus/prometheus.yml` — extend with `rule_files`
- `deploy/docker-compose.yml` — extend with grafana + alertmanager
  services

---

*End of SPEC-EVAL-002 research v0.1.0 (draft).*
