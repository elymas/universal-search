# SPEC-UI-002 — Acceptance Criteria

> Given / When / Then 시나리오. 모듈당 **2개 이상**, 보안·경계·에러 케이스
> 포함. EARS 요구사항 ID와 1:1 매핑됩니다. `/moai run`의 TDD 사이클에서 각
> 시나리오는 최소 한 개 이상의 자동화 테스트로 검증됩니다.

---

## Module 1: `adapter-status`

### Scenario AS-1.1 — 정상 표시 (REQ-AS-001)

- **Given** `internal/adapters/registry.go`에 SPEC-ADP-001..009의 9개
  어댑터가 등록되어 있고, 각 어댑터에 `status`, `last_sync`, `success_count`,
  `fail_count`, `last_error` 메타가 존재한다
- **And** 사용자가 localhost에서 `/admin` 페이지를 연다
- **When** 페이지가 로드되고 `GET /api/admin/adapters`가 **HTTP 200** 응답을
  반환하며 응답 body가 정확히 9개의 entry로 구성된 JSON 배열을 포함한다
- **Then** Adapter Status 섹션에 정확히 9개의 행이 렌더링된다
- **And** 각 행은 (id, status badge, last_sync ISO 8601, success/fail count,
  last_error tail, key indicator, toggle, Re-sync 버튼)을 포함한다
- **And** 시크릿 원본 값은 DOM 어디에도 나타나지 않는다 (자동화 assertion:
  `screen.queryByText(/^(sk-|ghp_|xoxb-)/)` 결과가 null)

### Scenario AS-1.2 — Re-sync 트리거 (REQ-AS-002)

- **Given** Adapter Status 패널이 표시되어 있고, `drive` 어댑터 행에 Re-sync
  버튼이 보인다
- **When** 사용자가 `drive` 행의 Re-sync 버튼을 클릭한다
- **Then** `POST /api/admin/adapters/drive/resync`가 호출된다
- **And** 응답이 도착할 때까지 해당 행의 toggle/Re-sync 컨트롤이 disabled
  된다
- **And** 성공 응답 시 마지막 동기화 시각·success_count가 업데이트된다
- **And** 실패 응답 시 last_error tail이 갱신되되 다른 8개 행은 영향을
  받지 않는다

### Scenario AS-1.3 — 메타 부분 누락 (REQ-AS-003) [edge]

- **Given** `notion` 어댑터의 `last_sync` 필드가 누락(`null`)된 상태로
  `GET /api/admin/adapters`가 응답한다
- **When** 페이지가 렌더링된다
- **Then** `notion` 행의 last_sync 컬럼은 `—`로 표시된다
- **And** 페이지 전체 렌더링은 차단되지 않으며 나머지 8 행은 정상 표시된다

### Scenario AS-1.4 — Re-sync on unknown adapter ID (REQ-AS-002) [negative, P0]

- **Given** `internal/adapters/registry.go`에 `ghost`라는 ID의 어댑터가
  등록되어 있지 않다
- **And** Admin UI가 localhost에서 정상 로드되어 있다
- **When** 클라이언트가 `POST /api/admin/adapters/ghost/resync`를 호출한다
- **Then** 응답은 **HTTP 404** 이다
- **And** 응답 body는 구조화된 에러 페이로드 형태이다: `{"error":
  "adapter_not_found", "adapter_id": "ghost"}` 또는 동등한 스키마. stack
  trace, hostname, 내부 경로는 포함하지 않는다
- **And** 다른 9개 등록된 어댑터의 상태/카운터는 어떤 변경도 받지 않는다
  (응답 후 `GET /api/admin/adapters` 결과가 호출 전과 동일)

### Scenario AS-1.5 — Re-sync upstream failure (REQ-AS-002) [error path, P0]

- **Given** `drive` 어댑터는 정상 등록되어 있으나 어댑터 manager의 resync
  호출이 업스트림 에러로 실패하도록 mocking 되어 있다 (예: Google Drive
  API 5xx 응답을 시뮬레이션)
- **When** 클라이언트가 `POST /api/admin/adapters/drive/resync`를 호출한다
- **Then** 응답은 **HTTP 502 Bad Gateway** (또는 `503 Service Unavailable`)
  이며 body는 `{"error": "upstream_adapter_error", "adapter_id": "drive",
  "detail": "<sanitized message>"}` 형태이다 (raw upstream stack trace는
  포함하지 않음)
- **And** UI는 해당 어댑터 행의 인라인 영역에 에러 메시지("Re-sync failed:
  upstream adapter error")를 표시한다
- **And** 페이지 전체는 크래시되지 않으며 다른 8개 어댑터 행은 영향을
  받지 않는다 (Scenario AS-1.3과 동일한 부분 실패 격리 원칙)

---

## Module 2: `api-key-view`

### Scenario AK-2.1 — 키 indicator 표시 (REQ-AK-001)

- **Given** `slack` 어댑터의 시크릿 소스가 환경변수 `SLACK_BOT_TOKEN`이고
  해당 환경변수가 set 상태이다
- **And** `gmail` 어댑터의 시크릿 소스는 `secret://gmail/oauth`이고 unset
  상태이다
- **When** `/admin` 페이지가 로드된다
- **Then** `slack` 행에 소스 식별자 `SLACK_BOT_TOKEN`와 indicator `set`이
  표시된다
- **And** `gmail` 행에 소스 식별자 `secret://gmail/oauth`와 indicator
  `unset`이 표시된다
- **And** 두 행 어디에도 시크릿 원본 값(토큰 문자열, OAuth client secret)이
  포함되지 않는다

### Scenario AK-2.2 — 어댑터 enable/disable 토글 (REQ-AK-002)

- **Given** `confluence` 어댑터가 현재 `enabled` 상태이다
- **When** 사용자가 `confluence` 행의 토글을 클릭한다
- **Then** `POST /api/admin/adapters/confluence/toggle`이 호출된다
- **And** 응답 후 행 상태가 `disabled`로 업데이트되고 토글의 `aria-pressed`
  속성이 반전된다
- **When** 다시 클릭한다
- **Then** 같은 엔드포인트가 호출되고 상태가 `enabled`로 복귀한다

### Scenario AK-2.3 — 시크릿 편집 UI 부재 검증 (REQ-AK-003) [security]

- **Given** `/admin` 페이지의 DOM
- **When** 자동화 테스트가 어댑터 행 영역에서 `<input type="password">`,
  `<input type="text">` 중 name이 secret-like(`token`, `key`, `secret`,
  `password`)인 요소를 검색한다
- **Then** 검색 결과는 0개이다
- **And** 백엔드 `/api/admin/*` 엔드포인트 목록에는 시크릿 값을 쓰는
  메서드(POST/PUT/PATCH `*/secret*`, `*/key*`)가 존재하지 않는다

### Scenario AK-2.4 — Toggle on unknown adapter ID (REQ-AK-002) [negative, P0]

- **Given** `internal/adapters/registry.go`에 `ghost`라는 ID의 어댑터가
  등록되어 있지 않다
- **And** Admin UI가 localhost에서 정상 로드되어 있다
- **When** 클라이언트가 `POST /api/admin/adapters/ghost/toggle`를 호출한다
- **Then** 응답은 **HTTP 404** 이다
- **And** 응답 body는 구조화된 에러 페이로드 형태이다: `{"error":
  "adapter_not_found", "adapter_id": "ghost"}` 또는 동등한 스키마. stack
  trace, hostname, 내부 경로는 포함하지 않는다
- **And** 다른 9개 등록된 어댑터의 enable/disable 상태는 어떤 변경도 받지
  않는다 (호출 전후 `GET /api/admin/adapters` 결과 동일)

---

## Module 3: `audit-viewer`

### Scenario AV-3.1 — 쿼리 audit 페이지네이션 (REQ-AV-001)

- **Given** `internal/audit/store.go`에 100개의 쿼리 로그가 존재한다
- **When** 사용자가 `/admin` 페이지의 Audit Viewer 섹션을 본다
- **Then** `GET /api/admin/audit/queries?limit=50&offset=0`이 호출된다
- **And** 테이블에 50개의 행이 표시되며 각 행은 (id, ts, latency_ms,
  tokens, sources, config snapshot ref, error indicator)를 포함한다
- **When** 사용자가 "Next" 페이지 컨트롤을 클릭한다
- **Then** offset이 50으로 증가한 요청이 발행되고 나머지 50개가 표시된다

### Scenario AV-3.2 — "Errors only" 필터 (REQ-AV-002)

- **Given** 최근 50개 쿼리 중 5개가 에러 상태이다
- **And** 사용자가 Audit Viewer를 보고 있다
- **When** 사용자가 "Errors only" 토글을 ON으로 전환한다
- **Then** `GET /api/admin/audit/queries?errors_only=true&...`가 호출된다
- **And** 테이블에 5개의 에러 행만 표시된다
- **When** 토글을 OFF로 전환한다
- **Then** 원래 50개 행이 복귀된다

### Scenario AV-3.3 — 빈 audit log (REQ-AV-001) [edge]

- **Given** `internal/audit/store.go`에 쿼리 로그가 0건이다
- **When** Audit Viewer가 로드된다
- **Then** "No queries yet" empty state 메시지가 표시된다
- **And** 페이지네이션 컨트롤("Next", "Previous" 버튼)이 DOM에 존재하되
  `disabled` 속성이 적용된 비활성 상태로 렌더링된다 (자동화 assertion:
  `button.getAttribute("disabled") !== null` AND `button.getAttribute(
  "aria-disabled") === "true"`)
- **And** 페이지 전체는 에러 없이 렌더링된다

### Scenario AV-3.4 — Cursor / offset 분기 (REQ-AV-003 P1 / REQ-AV-004 P0)

- **Given** 백엔드가 cursor 기반 페이지네이션을 **지원하지 않는** 환경이다
  (응답 페이로드에 `next_cursor`/`prev_cursor` 필드 부재)
- **When** Audit Viewer가 다음 페이지로 이동하려 한다
- **Then** 클라이언트는 `limit`/`offset` 쿼리 파라미터를 사용해 요청한다
  (REQ-AV-004 baseline)
- **And** 응답이 도착하면 다음 페이지 행이 정상 표시된다
- **Given** 별도의 케이스에서, 백엔드가 cursor를 **지원하는** 환경이다
  (응답에 `next_cursor` 필드 존재)
- **When** Audit Viewer가 다음 페이지로 이동한다
- **Then** 클라이언트는 `cursor=<token>` 쿼리 파라미터를 사용해 요청한다
  (REQ-AV-003)

---

## Module 4: `localhost-guard`

### Scenario LH-4.1 — 외부 IP 거부 (REQ-LH-001) [security, P0]

- **Given** `usearch-api`가 동작 중이고 admin route group에 loopback
  미들웨어가 체인되어 있다
- **When** 외부 NIC IP(예: `192.168.1.42`)에서 `GET /api/admin/adapters`로
  요청이 들어온다
- **Then** 응답은 `403 Forbidden`이다
- **And** 응답 본문에 버전, hostname, stack trace, 내부 경로가 포함되지
  않는다
- **And** 같은 미들웨어가 `POST /api/admin/adapters/{id}/toggle`,
  `POST /api/admin/adapters/{id}/resync`, `GET /api/admin/audit/queries`
  모두에서 동일하게 적용된다

### Scenario LH-4.2 — Loopback 통과 (REQ-LH-001, REQ-LH-002)

- **Given** `usearch-api`가 `127.0.0.1`에 bind되어 있다
- **When** 같은 호스트의 `127.0.0.1` 또는 `::1`에서 `/api/admin/*`로
  요청이 들어온다
- **Then** 미들웨어를 통과해 핸들러가 호출되고 정상 응답이 반환된다

### Scenario LH-4.3 — IP-claim 헤더 위조 차단 (REQ-LH-001) [security, P0]

- **Given** `usearch-api` admin 미들웨어가 동작 중이며 REQ-LH-001 SECURITY
  HARDENING 절(RemoteAddr만 신뢰)이 구현되어 있다
- **When** 외부 NIC IP(192.168.1.42)에서 다음 헤더 조합으로 위조 요청을
  보낸다:
  - `X-Forwarded-For: 127.0.0.1`
  - `X-Real-IP: ::1`
  - `Forwarded: for=127.0.0.1`
- **Then** 응답은 여전히 **HTTP 403 Forbidden** 이다
- **And** 위 세 케이스(헤더 한 개만, 헤더 세 개 모두 조합, RFC 7239
  `Forwarded` 변형) 모두에서 동일하게 403이 반환된다
- **And** 미들웨어 로직은 어떤 IP-claim 헤더도 읽지 않는다 (코드 리뷰에서
  `r.Header.Get("X-Forwarded-For")` 등의 호출 부재 확인)

### Scenario LH-4.4 — 비-loopback 환경에서 UI fallback (REQ-LH-003) [P1]

- **Given** 사용자가 `/admin` 페이지를 비-loopback 환경(예: 리버스 프록시
  뒤)에서 열어 백엔드 admin 호출이 403으로 반환된다
- **When** 페이지가 로드된다
- **Then** "Admin UI is only accessible from localhost. Open this page on
  the machine running usearch-api." 안내 화면이 페이지 전체로 표시된다
- **And** raw 403 에러나 stack trace는 노출되지 않는다

---

## Module 5: `navigation-integration`

### Scenario NV-5.1 — Sidebar `/admin` 항목 추가 (REQ-NV-001)

- **Given** SPEC-UI-001이 sidebar에 (Search, History, Sources) 3개 항목을
  제공한다
- **When** SPEC-UI-002 [DELTA]가 적용된 후 사용자가 어떤 페이지든 연다
- **Then** sidebar에는 정확히 4개 항목이 표시되며 마지막이 Admin 항목이다
- **And** 기존 3개 항목의 순서, 아이콘, 접근성 속성은 변경되지 않는다

### Scenario NV-5.2 — `/admin` active 상태 (REQ-NV-002)

- **Given** sidebar가 4개 항목을 표시한다
- **When** 사용자가 `/admin` 경로로 이동해 pathname이 `/admin`이 된다
- **Then** Admin 항목에 `bg-accent text-accent-foreground` 스타일이 적용된다
- **And** Admin 항목의 `aria-current="page"` 속성이 설정된다
- **And** 다른 3개 항목에는 active 스타일이 적용되지 않는다

### Scenario NV-5.3 — Sidebar 모바일 토글과의 호환 (REQ-NV-001) [edge]

- **Given** 모바일 폭에서 sidebar가 hamburger 메뉴로 닫혀 있다
- **When** 사용자가 hamburger를 열고 Admin 항목을 클릭한다
- **Then** `/admin`으로 라우팅된다
- **And** SPEC-UI-001의 `setMobileOpen(false)` 동작이 그대로 적용되어
  overlay가 닫힌다

---

## Cross-Cutting: Definition of Done (HARD)

다음 항목이 **모두 충족**되어야 SPEC-UI-002 V1이 완료된 것으로 간주합니다:

- [ ] 5개 모듈의 모든 P0 EARS 요구사항(13개, REQ-AV-004 포함)에 대응하는
      자동화 테스트 통과
- [ ] P1 요구사항(REQ-AV-003, REQ-LH-003)도 자동화 테스트 통과 또는
      explicit fallback path 검증
- [ ] Negative path 시나리오(AS-1.4 resync 404, AS-1.5 resync 5xx, AK-2.4
      toggle 404) 모두 CI에서 통과
- [ ] 백엔드 admin 패키지 coverage ≥ 85% (methodology=tdd, project default)
- [ ] 프론트 `web/src/app/admin/**` 컴포넌트 coverage ≥ 85%
- [ ] Scenario LH-4.1, LH-4.3, AK-2.3 (보안 핵심) 회귀 테스트가 CI에 포함
- [ ] `spec.md §2.2 / §9` 의 7개 Exclusion 항목이 어떤 파일에도 등장하지
      않음을 PR 리뷰에서 확인
- [ ] TRUST 5 quality gate 통과 (Tested / Readable / Unified / Secured /
      Trackable)
- [ ] 접근성 어서션(aria-current, aria-pressed, th scope) 회귀 테스트 포함

## Out-of-Scope Negative Acceptance (HARD)

다음이 PR diff에 등장하면 SPEC scope drift로 reject:

- 시크릿 입력 필드(`<input type="password">` 등 secret-like name)
- `next-auth`, `passport`, JWT 관련 라이브러리 추가
- `web/admin/` 또는 별도 admin Next.js 앱 디렉터리
- 시계열 차트 라이브러리(recharts, chart.js, d3 등) admin 화면에 도입
- 통합 audit 테이블 schema에 admin 액션/인증 이벤트 컬럼 추가
- `/admin/adapters/[id]/page.tsx` 같은 어댑터 상세 라우트
- WebSocket/SSE 기반 admin push 코드
