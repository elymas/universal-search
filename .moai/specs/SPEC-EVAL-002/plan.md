# SPEC-EVAL-002 Plan — phased implementation

Status: draft companion to spec.md
Author: limbowl via manager-spec
Date: 2026-05-22
Methodology: **DDD** (per `.moai/config/sections/quality.yaml`
`development_mode: tdd` default — **overridden to DDD for this
SPEC**). 근거: EVAL-002 는 기존 `internal/obs/metrics/`,
`internal/adapters/registry.go`, `internal/fanout/dispatch.go` 의
emission contract 에 instrumentation 을 더할 뿐, 새 도메인 로직
은 거의 없음. characterization tests on existing metric emission
behaviour dominate over new-logic tests. 신규 메트릭 family 3개의
registration / increment 동작은 TDD 스타일 unit test 로 검증하되,
adapter wrapper / fanout dispatch / Prometheus rule / Grafana JSON
은 모두 기존 동작을 보존하는 DDD 사이클로 진행.

Coverage target: 85% (NFR-EVAL2 호환).
Harness: **standard** (per `.moai/config/sections/harness.yaml`
auto-routing — 10 REQs (4 × P0 + 4 × P1 + 2 × P2) + 5 NFRs + 3
metric family + 1 endpoint + 5 recording rules + 4 alert rules +
1 Grafana JSON + 1 Alertmanager config + 1 runbook. 새 도메인
로직 거의 없고 declarative 산출물 비중이 큼 — standard로 충분.
Sprint Contract 권장하지만 필수 아님).

Per `.claude/rules/moai/core/agent-common-protocol.md`, time
estimates 는 금지. 단계는 priority + ordering 만 사용.

---

## 1. Implementation principle

EVAL-002 는 **declarative artifact 가 6**, **Go code 변경이 3**:

| Type | 산출물 |
|------|--------|
| Declarative | `recording-rules.yml`, `alerts.yml`, `alertmanager.yml`, `adapter-reliability.json`, 2x provisioning YAML, runbook |
| Go code | `internal/obs/metrics/fanout_partial.go` (NEW), `internal/fanout/dispatch.go` (MODIFY), `internal/adapters/registry.go` (MODIFY), `cmd/usearch-api/admin_health_adapters.go` (NEW) |

기본 원칙:

1. **Zero invention** — `OutcomeFromError` 6-tuple 은 frozen,
   기존 metric collector 들은 그대로. 새 metric 은 cardinality
   budget 안에서만 추가.
2. **Recording rule first** — Grafana 패널과 alert rule 은 모두
   사전 집계된 시리즈만 쿼리. Raw `rate()` 가 dashboard JSON 에
   나타나면 즉시 reject.
3. **Loose coupling to circuit state** — `usearch_adapter_circuit_
   state` 는 아무도 emit 하지 않는 상태로 시작; metric family 만
   등록. SPEC-CACHE-001 v2 가 등장하면 그 SPEC 에서 emit 코드를
   추가 (EVAL-002 plan 의 책임 아님).
4. **Operator-tunable defaults** — alert threshold (85%, 50%, 30%,
   10min) 은 권장값. runbook 에 tuning 가이드 포함.
5. **No external notification destination shipped** — Alertmanager
   default receiver 는 `null`. 운영자가 자기 채널 연결.
6. **Characterization tests over invention** — DDD: 기존 emission
   동작에 새 hook 을 더할 때마다 "기존 호출 경로의 behavior 가
   바뀌지 않음" 을 fanout/adapter unit test 로 보장.
7. **Sprint Contract optional** — standard harness 라 필수 아님.
   복잡도 (3 패키지 동시 수정) 가 있으므로 plan-auditor 가 권고하
   면 도입.

---

## 2. Phase ordering

Priority labels per MoAI rule (no time estimates).

### Phase 0 — Plan-auditor PASS (Priority High)

- Plan-auditor 가 spec.md + research.md + plan.md + acceptance.md
  (병행 작성) 를 검토
- MAJOR / MINOR / NIT 결과를 annotation 사이클로 처리
- Open question §8 의 (1) circuit-state emission source, (2)
  Grafana 최소 버전 두 항목은 annotation 단계에서 확정
- 상태 전이: `draft → approved`
- 구현 작업은 Phase 0 PASS 전에 시작하지 않음

### Phase 1 — Metric family registration (Priority High)

Goal: 3개 새 metric family 가 `Registry` 에 등록되고 cardinality
test 가 green.

DDD ANALYZE:
1. `internal/obs/metrics/metrics.go:117` `NewRegistry` 의 기존
   collector 등록 패턴 (registerLLM, registerRouter, ...) 을 읽
   고 동일한 helper 패턴 채택
2. `Registry.labelNames` (line 265-289) 의 현재 allowlist 파악;
   `state` 하나만 추가해야 함을 확인
3. `metrics_test.go` 의 cardinality test 가 어떤 형식을 기대하
   는지 확인 (REQ-OBS-NFR-002 enforcement 지점)

DDD PRESERVE:
4. 기존 collector 등록 동작이 변하지 않음을 보장하는
   characterization test 작성 (`TestExistingCollectorsUnchanged`)
   — `NewRegistry()` 반환 객체의 모든 기존 필드가 non-nil 임을
   확인

DDD IMPROVE:
5. 신규 파일 `internal/obs/metrics/fanout_partial.go` 작성:
   - `registerFanoutPartial(pr *prometheus.Registry) *fanoutPartialCollectors`
   - 3개 metric family 등록 (`usearch_fanout_partial_total`,
     `usearch_adapter_health_status`, `usearch_adapter_circuit_state`)
   - Pre-initialise 호출로 metric family 가 첫 scrape 부터 노출되
     도록 (SPEC-OBS-001 패턴)
6. `metrics.go:227` `NewRegistry` 의 return struct 에 3개 새
   필드 추가:
   - `FanoutPartial *prometheus.CounterVec`
   - `AdapterHealthStatus *prometheus.GaugeVec`
   - `AdapterCircuitState *prometheus.GaugeVec`
7. `Registry.labelNames` slice 마지막에 `"state",` 추가
8. `internal/obs/metrics/fanout_partial_test.go` 작성:
   - `TestFanoutPartialMetricFamiliesRegister` (Phase 1 핵심)
   - `TestCardinalityBudget12AdaptersUnder200Series`
   - `TestStateEnumBoundedThreeValues`
9. `metrics_test.go` 의 cardinality allowlist test 업데이트

Exit criterion:
- 새 metric family 3개가 `/metrics` 응답에 노출됨
- 기존 collector 동작 무변경
- cardinality test green
- `go test ./internal/obs/... -race` PASS

### Phase 2 — Fanout partial-result instrumentation (Priority High)

Goal: `usearch_fanout_partial_total{adapter}` 가 `fanout.Dispatch`
종료 시점에 정확히 한 번씩 증가.

DDD ANALYZE:
1. `internal/fanout/dispatch.go` 읽고 `eg.Wait()` 종료 후 result
   assembly 가 일어나는 지점 식별
2. `Result.AdapterErrors` 가 채워지는 lifecycle (worker → error
   slice → AdapterErrors map) 확인
3. fanout 패키지의 의존성 그래프에서 obs.Metrics 가 어디까지 흘
   러와 있는지 확인 (이미 fanout/observability.go 가 있음)

DDD PRESERVE:
4. 기존 `dispatch_test.go` 의 모든 시나리오가 새 instrumentation
   추가 후에도 동일한 결과를 내는지 확인하는 characterization
   sweep
5. `bench_test.go` 의 baseline p99 측정 → Phase 종료 후 비교

DDD IMPROVE:
6. `dispatch.go` 의 result assembly 구간에 hook 추가:
   ```go
   for adapterName := range result.AdapterErrors {
       if obs != nil && obs.Metrics != nil &&
          obs.Metrics.FanoutPartial != nil {
           obs.Metrics.FanoutPartial.WithLabelValues(adapterName).Inc()
       }
   }
   ```
   - `@MX:NOTE: [AUTO] SPEC-EVAL-002 REQ-EVAL2-004 fanout partial
     counter emission` 주석
7. 신규 테스트 `TestFanoutPartialCounterEmission` (3-adapter 시나
   리오 1 fail, §5.2)
8. `bench_test.go` 재실행 → < 1% p99 regression 확인 (NFR-EVAL2-005)

Exit criterion:
- `fanout_partial_total` 가 partial-result 시나리오에서 정확히 1
  씩 증가
- 기존 fanout 동작 무변경 (51 기존 테스트 PASS)
- bench p99 regression < 1%

### Phase 3 — Adapter failure_class slog attribute (Priority Medium)

Goal: `wrappedAdapter.emit` 가 `failure_class` slog attribute 추가.

DDD ANALYZE:
1. `internal/adapters/registry.go:223` `emit` 함수 읽고 현재
   attribute 목록 (adapter, outcome, elapsed_seconds, result_count,
   error) 확인
2. error → failure_class 변환 규칙 설계:
   - HTTP `*SourceError` 의 `HTTPStatus` 가 500-599 → `"5xx"`
   - HTTP 400-499 → `"4xx"`
   - `net.DNSError` → `"dns"`
   - `tls.RecordHeaderError`, `tls.AlertError`, x509 errors → `"tls"`
   - json/xml unmarshal errors → `"parse"`
   - yt-dlp transcript-specific errors → `"transcript"` (adapter-
     specific; ADP-005 도움 필요)
   - 그 외 → `"unknown"`

DDD PRESERVE:
3. 기존 `registry_test.go` 의 모든 attribute assertion 유지

DDD IMPROVE:
4. `registry.go` 에 `classifyFailure(err error) string` 헬퍼 추가:
   - `errors.As(err, &*types.SourceError{})` 로 HTTPStatus 검사
   - `errors.As` 로 net.DNSError / tls 에러 패밀리 검사
   - JSON/XML 에러는 strings 검사 (간단한 휴리스틱)
   - 매칭 안 되면 `"unknown"`
5. `emit` 의 attrs slice 에 추가:
   ```go
   if err != nil {
       attrs = append(attrs,
           slog.String("error", err.Error()),
           slog.String("failure_class", classifyFailure(err)),
       )
   }
   ```
6. 신규 테스트 `TestFailureClassClassification` (table-driven):
   - 5xx, 4xx, dns, tls, parse, transcript, unknown 각각
7. `TestEmitIncludesFailureClassAttribute` (§5.3)

Exit criterion:
- 실패 호출의 slog record 에 `failure_class` attribute 존재
- 7개 클래스 모두 표 기반 테스트 PASS
- 기존 adapter 동작 무변경

### Phase 4 — Recording rules + Prometheus 통합 (Priority High)

Goal: 5개 recording rule 이 promtool 검증 통과 + Prometheus 가
정상 평가.

순수 declarative 작업 (DDD 사이클 minimal).

Tasks:
1. `deploy/prometheus/recording-rules.yml` 작성:
   - `usearch:adapter_success_rate_1h`
   - `usearch:adapter_success_rate_24h`
   - `usearch:adapter_success_rate_7d`
   - `usearch:adapter_failure_rate_by_outcome_24h`
   - `usearch:adapter_fanout_partial_ratio_24h`
   - 각 PromQL 표현은 research §3.3 그대로
2. `deploy/prometheus/prometheus.yml` 수정:
   - 최상단에 `rule_files: ['recording-rules.yml', 'alerts.yml']`
   - `alerting:` 블록 추가 (Alertmanager target)
3. CI gate `.github/workflows/promtool-validate.yml` 신규:
   - `promtool check rules deploy/prometheus/recording-rules.yml`
   - `promtool check rules deploy/prometheus/alerts.yml` (Phase 5
     산출물 사전 placeholder 라도 둠)
4. Recording rule unit test (Go 또는 promtool 의 `--test` 모드):
   - `deploy/prometheus/recording-rules-test.yml` 작성
   - synthetic series fixture → 기대값 비교
   - REQ-EVAL2-001 acceptance 충족
5. Prometheus container 띄워서 integration smoke test:
   - synthetic 8일치 데이터 seed
   - 1 분 후 `curl localhost:9090/api/v1/query?query=usearch:adapter_success_rate_7d`
   - 응답 검증

Exit criterion:
- `promtool check rules` exits 0
- `promtool test rules` (unit 모드) PASS
- Prometheus container 가 rule evaluation 시작 → 8일 fixture 로
  REQ-EVAL2-002 검증 PASS

### Phase 5 — Alert rules + Alertmanager 통합 (Priority High)

Goal: 4개 alert rule 정의 + Alertmanager config + amtool 검증.

Tasks:
1. `deploy/prometheus/alerts.yml` 작성 (research §4.3 의 4 rule):
   - `AdapterSuccessRate7dLow`
   - `AdapterSuccessRate1hCritical`
   - `FanoutPartialRatioHigh`
   - `AdapterCircuitOpen`
2. `deploy/alertmanager/alertmanager.yml` 작성 (research §4.2 의
   skeleton):
   - null receiver default
   - 3 commented receiver examples (webhook, PagerDuty, OpsGenie)
   - inhibit rules (critical → warning suppression)
3. CI gate 확장:
   - `amtool check-config deploy/alertmanager/alertmanager.yml`
   - alert rule unit test (`promtool test rules`)
4. Alert firing integration test (compose 환경):
   - synthetic time series seed
   - 4 시나리오 (§5.5, §5.6, §5.7, §5.8) 각각 alert 가 inactive
     → pending → firing 으로 전이하는지 확인

Exit criterion:
- `promtool check rules deploy/prometheus/alerts.yml` exits 0
- `amtool check-config` exits 0
- 4 alert 시나리오 통합 테스트 모두 PASS

### Phase 6 — Grafana dashboard + provisioning (Priority Medium)

Goal: 5 패널 dashboard JSON 가 Grafana 11.x 에 import 되고 모든
패널이 데이터 렌더.

Tasks:
1. `deploy/grafana/dashboards/adapter-reliability.json` 작성:
   - 5 패널 (research §5.2 매트릭스)
   - 모든 패널이 recording rule 시리즈만 쿼리 (raw rate() 금지)
   - Grafana template variable `adapter` 선언
   - Loki datasource 가 있을 경우 활성화되는 log-link panel (#6,
     선택적)
2. `deploy/grafana/provisioning/datasources/prometheus.yaml` 작성
3. `deploy/grafana/provisioning/dashboards/adapter-reliability.yaml`
   작성
4. `deploy/docker-compose.yml` 수정:
   - `grafana` 서비스 추가 (grafana/grafana:11.3.0)
   - volume mount: provisioning + dashboards
   - port: 3000 노출 (localhost only)
5. JSON 스키마 검증:
   - Grafana CLI: `grafana-cli admin lint dashboards/...` 또는
   - 수동: jsonschema 라이브러리로 dashboard JSON 검증
6. Visual smoke test (integration):
   - `docker compose up grafana prometheus` 한 후
   - `curl localhost:3000/api/dashboards/uid/adapter-reliability` →
     HTTP 200
   - synthetic data 로 panel rendering 확인 (스크린샷 캡처해서
     `docs/operations/screenshots/` 에 보관)

Exit criterion:
- Dashboard JSON 가 grafana-cli lint 통과
- compose up 후 30초 내 dashboard 접근 가능 (NFR-EVAL2-007)
- 5 패널 모두 fixture 데이터로 < 2초 렌더 (NFR-EVAL2-002)

### Phase 7 — Health endpoint (Priority Medium)

Goal: `/admin/health/adapters` 가 NFR-EVAL2-010 스키마로 응답.

DDD ANALYZE:
1. `cmd/usearch-api/` (또는 admin server 가 어느 cmd 에 있는지
   확인) 의 admin port handler 등록 패턴 파악
2. obs.Metrics.AdapterCalls collector 에서 현재 상태 snapshot
   읽는 방법 (`prometheus.Collector.Collect(ch)` 채널)

DDD PRESERVE:
3. 기존 admin endpoint (`/metrics`) 가 영향받지 않음을 확인

DDD IMPROVE:
4. `cmd/usearch-api/admin_health_adapters.go` 작성:
   - HTTP handler `GET /admin/health/adapters`
   - in-process counter snapshot 읽기 (research §9.3)
   - 7d 정확도 한계 명시: process lifetime < 7d 이면 field `null`
   - status 분류: healthy / degraded / unhealthy
   - JSON marshal + HTTP status code 매핑 (503 if any unhealthy)
5. `cmd/usearch-api/admin_health_adapters_test.go`:
   - 시나리오: all healthy → 200, mixed → 200 (degraded 만), one
     unhealthy → 503
   - JSON 스키마 검증

Exit criterion:
- endpoint 가 모든 분기에서 정확한 status code + JSON 반환
- 7d 정확도 한계 문서화 완료

### Phase 8 — Operator runbook (Priority Medium)

Goal: 알람 발생 시 운영자가 따라할 수 있는 절차서 완성.

Tasks:
1. `docs/operations/adapter-reliability-runbook.md` 작성:
   - 각 4 alert 별 섹션 (annotation 의 runbook_url anchor 와 매칭)
   - 각 섹션 구조: 무엇이 보이나 → 즉시 확인 / 단기 완화 / 장기
     대응
   - Common scenarios:
     - Naver rate-limit 폭주 → API key rotation + Alertmanager
       silence
     - GitHub OAuth 만료 → token 갱신 절차
     - YouTube transcript 깨짐 → yt-dlp 버전 확인
     - SearXNG upstream block → 다른 search engine 활성화
     - KoreaNewsCrawler parse error → 라이브러리 업데이트
   - Threshold tuning 가이드 (트래픽 적은 adapter 의 false
     positive 줄이는 방법)
   - Storage sizing 가이드 (Prometheus 30d retention 예측치)
   - `amtool silence add` 명령어 예제
   - 대시보드 스크린샷 (Phase 6 캡처) 포함
2. parent README 에서 runbook 으로 링크
3. SPEC-DOC-001 (M9) 작업과 협조 — runbook 이 사용자 문서로 합
   쳐질 수도 있음

Exit criterion:
- 4 alert 모두 runbook 섹션 존재
- 스크린샷 1개 이상 포함
- threshold tuning 가이드 포함

### Phase 9 — End-to-end integration validation (Priority Medium)

Goal: 전체 파이프라인 (instrumentation → recording rule → alert
→ dashboard) 이 실제 환경에서 동작.

Tasks:
1. `docker compose up` 으로 전체 스택 (usearch-api + Prometheus +
   Grafana + Alertmanager) 띄움
2. Synthetic load generator:
   - 1 adapter (e.g., noop) 가 의도적으로 50% 실패 반환하도록 설정
   - 30분 이상 트래픽 흘림
3. 검증 순서:
   - `usearch_adapter_calls_total{adapter="failing", outcome="failure"}`
     가 `/metrics` 에 노출
   - `usearch:adapter_success_rate_7d{adapter="failing"}` 가 < 0.85
   - Alertmanager 에 `AdapterSuccessRate7dLow{adapter="failing"}` 가
     `firing` 상태로 등장
   - Grafana dashboard 의 패널 #2 가 해당 adapter 의 라인을 빨간색
     으로 표시 (threshold line 아래)
   - `/admin/health/adapters` 가 해당 adapter `status: "unhealthy"`,
     overall_status `"degraded"`, HTTP 503 반환
4. 결과 캡처 → runbook 의 "정상 동작 예시" 섹션에 추가

Exit criterion:
- 4 단계 검증 모두 PASS
- 스크린샷 / curl 출력 capture 완료

### Phase 10 — Sync phase (Priority Low)

Goal: 문서 + PR.

Tasks:
1. `manager-docs` 업데이트:
   - parent `README.md`: "Observability" 섹션에 Grafana dashboard
     스크린샷 + runbook 링크 추가
   - SPEC-DOC-001 (M9) 가 publish 되면 user docs 사이트에서도 cross-link
2. `CHANGELOG.md` 에 SPEC-EVAL-002 entry (M8 섹션)
3. `.moai/project/roadmap.md` §M8 SPEC-EVAL-002 row 상태를
   `implemented` 로 업데이트
4. `manager-git` PR 생성:
   - title: `feat(eval): implement SPEC-EVAL-002 — adapter
     reliability dashboard with 7-day rolling success rate (M8)`
   - body: spec.md / research.md / acceptance.md 링크, acceptance
     scenario 매트릭스, screenshot 임베드
5. 상태 전이: `approved → implemented` (merge 후)

---

## 3. File-level task inventory

Phase별 file write 일관성을 위해 파일 단위 매트릭스:

### `internal/obs/metrics/`

| File | Phase | Action |
|------|-------|--------|
| `fanout_partial.go` | 1 | NEW — 3 metric family register helper |
| `fanout_partial_test.go` | 1 | NEW — registration + cardinality test |
| `metrics.go` | 1 | MODIFY — Registry struct, NewRegistry, labelNames |
| `metrics_test.go` | 1 | MODIFY — cardinality allowlist extension |

### `internal/fanout/`

| File | Phase | Action |
|------|-------|--------|
| `dispatch.go` | 2 | MODIFY — partial counter emission post `eg.Wait()` |
| `dispatch_test.go` | 2 | MODIFY — add `TestFanoutPartialCounterEmission` |
| `bench_test.go` | 2 | (read-only) — used to verify NFR-EVAL2-005 |

### `internal/adapters/`

| File | Phase | Action |
|------|-------|--------|
| `registry.go` | 3 | MODIFY — `classifyFailure` helper + emit attr |
| `registry_test.go` | 3 | MODIFY — `TestFailureClassClassification` table-driven |

### `cmd/usearch-api/`

| File | Phase | Action |
|------|-------|--------|
| `admin_health_adapters.go` | 7 | NEW — HTTP handler |
| `admin_health_adapters_test.go` | 7 | NEW — schema + status code tests |
| (existing admin server file) | 7 | MODIFY — register the new route |

### `deploy/prometheus/`

| File | Phase | Action |
|------|-------|--------|
| `recording-rules.yml` | 4 | NEW — 5 recording rules |
| `recording-rules-test.yml` | 4 | NEW — `promtool test rules` fixture |
| `alerts.yml` | 5 | NEW — 4 alert rules |
| `alerts-test.yml` | 5 | NEW — `promtool test rules` fixture |
| `prometheus.yml` | 4 | MODIFY — rule_files + alerting section |

### `deploy/alertmanager/`

| File | Phase | Action |
|------|-------|--------|
| `alertmanager.yml` | 5 | NEW — null default + commented receivers |

### `deploy/grafana/`

| File | Phase | Action |
|------|-------|--------|
| `dashboards/adapter-reliability.json` | 6 | NEW — 5-panel dashboard |
| `provisioning/datasources/prometheus.yaml` | 6 | NEW |
| `provisioning/dashboards/adapter-reliability.yaml` | 6 | NEW |

### `deploy/`

| File | Phase | Action |
|------|-------|--------|
| `docker-compose.yml` | 6 | MODIFY — grafana + alertmanager services |

### `docs/operations/`

| File | Phase | Action |
|------|-------|--------|
| `adapter-reliability-runbook.md` | 8 | NEW — 4 alert 섹션 + tuning + sizing |
| `screenshots/*.png` | 8/9 | NEW — dashboard / Alertmanager 캡처 |

### `.github/workflows/`

| File | Phase | Action |
|------|-------|--------|
| `promtool-validate.yml` | 4/5 | NEW — promtool + amtool CI gate |

### `.moai/project/` + `CHANGELOG.md`

| File | Phase | Action |
|------|-------|--------|
| `.moai/project/roadmap.md` | 10 | MODIFY — M8 status update |
| `CHANGELOG.md` | 10 | MODIFY — M8 entry |

총 신규 파일 약 14개, 수정 파일 약 8개.

---

## 4. MX tag plan

### 신규 추가 예상

| File | Tag | Reason |
|------|-----|--------|
| `internal/obs/metrics/fanout_partial.go::registerFanoutPartial` | `@MX:ANCHOR` | fan_in ≥ 3 (NewRegistry, tests, possibly admin endpoint) |
| `internal/fanout/dispatch.go` (partial counter increment 지점) | `@MX:NOTE` | SPEC-EVAL-002 REQ-EVAL2-004 emission point |
| `internal/adapters/registry.go::classifyFailure` | `@MX:NOTE` | failure_class taxonomy mapping; open-set 유의 |
| `cmd/usearch-api/admin_health_adapters.go::handler` | `@MX:ANCHOR` | public HTTP API boundary |

### 기존 수정 예상

| File | Tag | Reason |
|------|-----|--------|
| `internal/obs/metrics/metrics.go::Registry` (line ~28) | `@MX:ANCHOR` (existing) | extend `@MX:REASON` to mention "SPEC-EVAL-002 partial/health/circuit gauges" |
| `internal/obs/metrics/metrics.go::NewRegistry` | `@MX:NOTE` (new) | new metric families documented |

### 제거 예상

없음. (EVAL-002 는 코드 삭제 없음, instrumentation 추가만.)

---

## 5. Risk-driven sequencing notes

Research §11 의 risk 와 mitigation phase:

| Risk | Mitigation Phase |
|------|------------------|
| Recording rule 표현식 버그 | Phase 4 (`promtool test rules` fixture) |
| Alert fatigue | Phase 5 (`for: 30m` 적용) + Phase 8 (runbook tuning) |
| Acute 알람 false positive | Phase 5 (`for: 5m`) + Phase 8 (low-traffic adapter 명시) |
| Grafana 패널 렌더 < 2초 미충족 | Phase 6 (recording rule만 쿼리, raw rate 금지 — peer review gate) |
| `usearch_adapter_circuit_state` "no data" | Phase 1 (의도된 동작 명시) + Phase 8 (runbook 안내) |
| Prometheus 30d retention 디스크 부족 | Phase 8 (sizing 가이드) |
| Alertmanager 미설치 환경 | Phase 5 (`alerting:` 섹션 optional 명시) |
| `failure_class` slog Loki 부재 환경 | Phase 6 (Loki panel optional, 핵심 5 panel 무손상) |
| 새 adapter dashboard 미반영 | Phase 6 (template variable `label_values()` 자동 발견) |
| Cardinality test 가 새 family 누락 | Phase 1 (cardinality test 명시적 업데이트) |

특히 위험도 높은 두 항목:

- **Recording rule 정확성** (Phase 4 핵심): hand-calculated fixture
  와 비교하는 unit test 없이 머지하지 않음
- **NFR-EVAL2-005 bench non-regression** (Phase 2 핵심): 측정 전후
  값을 CHANGELOG 와 plan-auditor 응답에 명시

---

## 6. Sprint Contract recommendation

`.moai/config/sections/harness.yaml` 의 standard 레벨에서는 Sprint
Contract 가 optional. EVAL-002 는 다음 이유로 **권장**:

1. 3개 패키지 (`internal/obs/metrics`, `internal/fanout`,
   `internal/adapters`) 를 동시 수정 — 변경 단위 결합도 명시 필요
2. Declarative artifact (Grafana / Prometheus / Alertmanager) 와
   Go code 가 1:1 매칭되어야 alert / dashboard / metric 사이 정합
   성 보장
3. Acceptance scenario 12개 중 multi-component 4개 (§5.4, §5.5,
   §5.10, §5.11) 는 contract 가 없으면 evaluator 가 무엇을 채점할
   지 모호

권장 Sprint Contract 항목:

- Phase 1 끝: 3개 새 metric family 가 `/metrics` 출력에 존재
- Phase 2 끝: `usearch_fanout_partial_total` 가 fanout 시나리오에
  서 increase
- Phase 4 끝: `promtool check rules` exits 0 + recording-rules-test
  PASS
- Phase 5 끝: 4 alert 가 synthetic data 로 firing 상태 도달
- Phase 6 끝: dashboard JSON import + 5 panel 렌더 < 2초
- Phase 9 끝: end-to-end synthetic load 로 알람 → dashboard →
  health endpoint 전체 흐름 검증

---

## 7. Open factoring decisions deferred to run phase

다음 항목은 plan 시점이 아닌 run 시점에 결정:

1. **`classifyFailure` 의 휴리스틱 강도**: TLS / DNS 검사를 얼마
   나 깊이 할 것인지 (간단한 errors.As vs 완전한 type assertion
   tree). Run 시점에 실제 adapter 가 어떤 에러 타입을 던지는지
   보고 결정.

2. **Health endpoint 의 in-process counter 읽기 방식**: Prometheus
   `Collector.Collect(ch)` 채널 패턴 vs 별도 `sync/atomic` 카운터.
   첫 구현은 Collector.Collect 로 시작, 성능 이슈 발견 시 atomic
   으로 전환.

3. **Grafana template variable 동작 디테일**: multi-select 가능,
   `All` 옵션 포함 등 UX 디테일은 Phase 6 implementation 시 panel
   별 최적화.

4. **Compose 에서 Alertmanager 를 default 로 띄울지**: 일부 운영
   자는 알람 없이 사용. compose profile 로 optional 분리할지
   (`docker compose --profile alerts up`) Phase 5/6 에서 결정.

5. **30d retention 의 정확한 sizing 측정**: synthetic load 로 실
   측 후 runbook 의 sizing 가이드 작성 (Phase 8/9).

6. **runbook 스크린샷 자동 생성 vs 수동 캡처**: 자동화는 부담 큼,
   수동 1회 캡처 후 dashboard 변경 시 업데이트 정책 — Phase 8 에
   서 결정.

7. **Loki integration 적극성**: optional log-link panel 을 dashboard
   에 포함할지, 별도 dashboard 로 분리할지 — Phase 6 에서 평가.

8. **AlertmanagerWebhook receiver 의 default 활성화 여부**: docs
   가이드 수준에서 충분한지, default minimal config 가 필요한지 —
   Phase 5 에서 운영자 피드백 받아 결정.

이 항목들은 SPEC contract 를 바꾸지 않으며, 모두 mechanical
implementation choices.

---

*End of SPEC-EVAL-002 plan v0.1.0 (draft).*
