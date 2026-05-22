# SPEC-UI-002 — Implementation Plan

> Companion to `./spec.md`. Decomposes SPEC-UI-002 (Admin UI v1) into ordered
> task units suitable for `/moai run` (TDD methodology, brownfield project).
> **No time estimates** — use priority labels and ordering only.

## 1. Approach Overview

SPEC-UI-002는 **브라운필드 SPEC**입니다. 새로운 모듈 추가는 있지만 모두
기존 자산 위에 [DELTA]로 얹습니다. 접근 순서는 다음 4단계 — 백엔드부터,
프론트는 마지막입니다(프론트가 의존하는 응답 스키마가 백엔드 일단 정착해야
실용적):

1. **Phase A — Backend foundation (P0)**
   loopback 미들웨어 + admin route group 결합 + `/api/admin/adapters` GET
2. **Phase B — Backend actions (P0)**
   resync, toggle, audit queries 핸들러
3. **Phase C — Frontend integration (P0)**
   `/admin` 라우트, 컴포넌트, sidebar [DELTA]
4. **Phase D — Hardening (P0/P1)**
   loopback gate UI, 에러 fallback, a11y/접근성 검증

각 Phase 내부는 TDD(RED → GREEN → REFACTOR) 사이클로 진행합니다.

## 2. Phase A — Backend Foundation (Priority: High)

**목표**: admin 엔드포인트 자체가 외부에서 접근 불가능함을 보장한 뒤,
어댑터 메타를 안전하게 노출합니다.

### Task A1 — Loopback middleware

- 위치: `internal/api/admin/middleware_loopback.go` (새 파일)
- 책임: `r.RemoteAddr` 또는 `X-Forwarded-For` 미설정(직접 연결) 기준으로
  127.0.0.1 / ::1 만 통과시키는 `http.Handler` 래퍼
- 만족 EARS: REQ-LH-001
- 테스트(RED 먼저):
  - loopback IPv4(127.0.0.1) → next 호출
  - loopback IPv6(::1) → next 호출
  - 외부 IPv4(예: 192.168.1.42) → 403, body 정보 노출 없음
  - 외부 IPv6 → 403
  - `X-Forwarded-For` 헤더로 우회 시도 → 403 (RemoteAddr만 신뢰)

### Task A2 — Admin route group 등록

- 위치: `cmd/usearch-api/main.go` 또는 `internal/api/handlers/` ([DELTA])
- 책임: `/api/admin/*` 서브트리에 A1 미들웨어를 mount, 기존 사용자 API와
  격리
- 만족 EARS: REQ-LH-002
- 테스트: 통합 테스트에서 외부 NIC IP로 호출 시 403 검증, 일반 검색 API
  (`/api/search/*`)는 영향 없음을 확인

### Task A3 — Adapter registry read-only view

- 위치: `internal/adapters/registry.go` ([DELTA]) — read-only helper 추가
  (예: `SnapshotForAdmin() []AdapterAdminView`)
- 책임: 9개 어댑터의 id, status, last sync, success/fail counter, last
  error tail, 시크릿 소스 이름(env var key 또는 store path), 키 존재
  여부(`set` | `unset`) 반환. **시크릿 값 자체는 반환하지 않음**.
- 만족 EARS: REQ-AS-001, REQ-AK-001, REQ-AS-003 (부분 실패 시 빈 값)
- 테스트: 각 어댑터 mock 등록 후 9 entry 반환, 시크릿 값이 응답
  payload 어디에도 포함되지 않음을 assertion

### Task A4 — `GET /api/admin/adapters` 핸들러

- 위치: `internal/api/admin/handler_adapters.go` (새 파일)
- 책임: A3의 snapshot을 JSON 응답으로 직렬화
- 만족 EARS: REQ-AS-001, REQ-AK-001
- 테스트(end-to-end): A1+A2+A3+A4 체이닝, 응답 JSON 구조 스냅샷, 시크릿
  미노출 회귀 테스트

## 3. Phase B — Backend Actions (Priority: High)

### Task B1 — `POST /api/admin/adapters/{id}/resync`

- 위치: `internal/api/admin/handler_adapters.go`
- 책임: 어댑터 ID 검증 → 어댑터 manager에 resync 요청 전달 → 결과 반환
- 만족 EARS: REQ-AS-002
- 테스트: 존재하지 않는 ID → 404, 정상 ID → 200 + 결과 페이로드

### Task B2 — `POST /api/admin/adapters/{id}/toggle`

- 위치: `internal/api/admin/handler_adapters.go`
- 책임: 어댑터 enable/disable 상태 전환, 결과 반환
- 만족 EARS: REQ-AK-002
- 테스트: enable→disable→enable 라운드트립, 알 수 없는 ID 거부

### Task B3 — `GET /api/admin/audit/queries`

- 위치: `internal/api/admin/handler_audit.go` (새 파일) +
  `internal/audit/` 조회 helper ([DELTA])
- 책임: 쿼리 로그를 페이지네이션해 반환. `limit`, `offset` 또는 `cursor`,
  `errors_only=true` 필터 지원. 응답: id, ts, latency_ms, tokens, sources,
  config_snapshot_ref, error.
- 만족 EARS: REQ-AV-001, REQ-AV-002, REQ-AV-003, REQ-AV-004
- 테스트: 빈 로그 → 빈 배열 + 안정적 페이지네이션 토큰, errors_only 필터
  로 에러 행만 반환, 알 수 없는 cursor → 400

### Task B4 — Admin API 회귀 테스트

- 위치: `internal/api/admin/...` integration test
- 책임: A+B 전체에 대해 "non-loopback origin 모든 엔드포인트 거부" 회귀
  테스트 한 묶음
- 만족 EARS: REQ-LH-001 (강제 회귀)

## 4. Phase C — Frontend Integration (Priority: High)

### Task C1 — `/admin` 라우트 셸 + sidebar [DELTA]

- 위치:
  - `web/src/app/admin/page.tsx` (새 파일)
  - `web/src/app/admin/layout.tsx` (필요 시 생성, layout 재사용 가능하면 생략)
  - `web/src/components/sidebar-nav.tsx` ([DELTA])
- 책임: NAV_ITEMS에 `/admin` 항목 1개 추가, `/admin`이 sidebar에서 active로
  표시
- 만족 EARS: REQ-NV-001, REQ-NV-002
- 테스트(Vitest + RTL): sidebar에 4개 항목 렌더링, pathname `/admin`일 때
  Admin 항목에 `aria-current="page"` 적용 확인

### Task C2 — API helpers

- 위치: `web/src/lib/api.ts` ([DELTA])
- 책임: `fetchAdminAdapters()`, `fetchAdminAudit(params)`, `toggleAdapter
  (id, enabled)`, `resyncAdapter(id)` 타입 안전 함수 + Zod 스키마
- 만족 EARS: 모든 admin 모듈의 데이터 fetching 경로
- 테스트: 모의 응답에 대해 schema parse 성공/실패 케이스

### Task C3 — Adapter status panel + API key view

- 위치:
  - `web/src/app/admin/_components/adapter-status-panel.tsx` (Client)
  - `web/src/app/admin/_components/api-key-row.tsx` (Client, panel 내부 행)
- 책임: 한 줄 = 한 어댑터. 상태 뱃지, 마지막 동기화, 성공/실패 카운트,
  최근 에러 tail, 시크릿 소스 이름 + set/unset indicator, enable/disable
  토글, Re-sync 버튼
- 참조 구현: `web/src/app/sources/page.tsx` (리스트/상태 표시 패턴)
- 만족 EARS: REQ-AS-001, REQ-AS-002, REQ-AS-003, REQ-AK-001, REQ-AK-002,
  REQ-AK-003
- 테스트:
  - 9개 어댑터 mock 응답 → 9 행 렌더링, 부분 데이터 누락 시 `—` 표시
  - 토글 클릭 → `toggleAdapter` 호출 → 행 상태 갱신
  - Re-sync 클릭 → 행 disabled → 응답 후 enabled 복귀
  - 시크릿 원본 값이 DOM 어디에도 등장하지 않음 (보안 회귀)

### Task C4 — Audit viewer

- 위치: `web/src/app/admin/_components/audit-viewer.tsx` (Client)
- 책임: 페이지네이션 테이블, "Errors only" 필터 토글, config_snapshot expand
- 만족 EARS: REQ-AV-001, REQ-AV-002, REQ-AV-003, REQ-AV-004
- 테스트:
  - 빈 응답 → empty state 메시지 ("No queries yet")
  - errors_only ON → mock에서 에러 행만 노출
  - cursor 페이지네이션 fallback (cursor 미지원 응답일 때 limit/offset
    네비게이션 동작)

## 5. Phase D — Hardening (Priority: High/Medium)

### Task D1 — Localhost gate UI fallback

- 위치: `web/src/app/admin/_components/localhost-gate.tsx` (Client)
- 책임: 초기 fetch가 403/네트워크 에러로 실패 시 "Admin UI is only
  accessible from localhost..." 안내 화면을 전체 admin 페이지 대체로
  렌더링. raw error stack을 노출하지 않음.
- 만족 EARS: REQ-LH-003 (P1)
- 테스트: fetch mock이 403 응답할 때 안내 화면 렌더링, raw error 비노출

### Task D2 — A11y/접근성 검증

- Sidebar `aria-current`, 토글 버튼 `aria-pressed`, 테이블 `<th scope>`
- axe-core 또는 RTL `toHaveAccessibleName` 어서션으로 회귀 방지
- 만족 NFR-A11Y-001

### Task D3 — 보안 회귀 시나리오

- Backend: B4 + 외부 IP 통합 시뮬레이션 케이스 추가
- Frontend: API key row가 시크릿 값을 절대 렌더링하지 않음을 회귀 테스트
- 만족 REQ-LH-001, REQ-AK-001, REQ-AK-003

## 6. Risks & Mitigations

| 리스크 | 영향 | 완화 |
|--------|------|------|
| `X-Forwarded-For` 우회 | loopback 가드 무력화 | RemoteAddr만 신뢰, FWD-FOR는 admin scope에서 무시 |
| 어댑터 레지스트리에 시크릿 원본이 in-memory로 들고 있음 | 응답에 실수로 포함될 위험 | `SnapshotForAdmin()`이 시크릿 필드를 절대 포함하지 않는 별도 struct로 매핑, 타입 시스템으로 강제 |
| audit 로그 양이 큼 → 페이지네이션 누락 시 응답 지연 | NFR-PERF-001 위반 | 기본 limit 50, 서버사이드 페이지네이션 enforced |
| `/admin` 라우트가 sidebar 위치 차지 → 다른 라우트와 충돌 | 사용자 검색 UX 손상 | NAV_ITEMS 끝에 추가, active 스타일 SPEC-UI-001과 동일 |
| `127.0.0.1` bind가 macOS/Linux/Docker 환경별 차이 | localhost 접근 막힘 | `cmd/usearch-api` bind 설정을 명시적 환경변수로 노출하되, 외부 IP는 명시적 opt-in 시에만 |

## 7. Test Strategy

- **Backend (Go)**:
  - Unit tests per handler + middleware (table-driven)
  - Integration test: 전체 admin route group + loopback 미들웨어 결합
  - 회귀 테스트: 시크릿 누출, 외부 IP 거부
  - 목표 coverage: 85% (프로젝트 기본값, methodology=tdd)
- **Frontend (TypeScript)**:
  - Vitest + React Testing Library (컴포넌트 단위)
  - Mocked fetch responses, Zod schema 검증
  - 접근성 어서션 (aria-current, aria-pressed)

## 8. Deliverables

PR 단위 제안:

1. **PR 1 (Phase A+B 백엔드)**: loopback 미들웨어 + 4 admin 엔드포인트 +
   회귀 테스트
2. **PR 2 (Phase C 프론트)**: `/admin` 라우트 + 3 컴포넌트 + sidebar
   [DELTA] + api helpers
3. **PR 3 (Phase D 하드닝)**: localhost gate UI + a11y + 보안 회귀 추가

세 개의 PR로 분할하면 SPEC-UI-001 위에 점진적으로 합류 가능합니다. 단일
PR로 묶으면 리뷰 부담이 큰 변경 폭이라 분할을 권장합니다.

## 9. Out of Plan

`./spec.md` §2.2 / §9 의 7개 Exclusion 항목은 본 plan에 **태스크가 존재하지
않습니다**. 추가 요구가 들어오면 별도 SPEC(SPEC-UI-003 등)으로 분리합니다.
