---
id: SPEC-UI-002
version: 1.0.0
status: implemented
created: 2026-05-23
updated: 2026-05-26
author: limbowl
priority: P1
issue_number: 0
title: Admin UI v1 — adapter status, API key view+toggle, query audit viewer
milestone: M7 — Surfaces
owner: expert-frontend
methodology: tdd
coverage_target: 85
depends_on:
  - SPEC-UI-001
  - SPEC-ADP-001
  - SPEC-ADP-002
  - SPEC-ADP-003
  - SPEC-ADP-004
  - SPEC-ADP-005
  - SPEC-ADP-006
  - SPEC-ADP-007
  - SPEC-ADP-008
  - SPEC-ADP-009
blocks: []
---

# SPEC-UI-002: Admin UI v1 — Adapter 상태, API 키 view+toggle, 쿼리 Audit 뷰어

## HISTORY

- 2026-05-23 (post-audit-1 revision v0.1.1, limbowl via manager-spec):
  plan-auditor iteration 1 (`.moai/reports/plan-audit/SPEC-UI-002-review-1.md`)
  결과 반영. 주요 변경:
  - **D1 (major)** REQ-NV-001 본문의 "4개 기존 항목" 표기를 "3개 기존 항목
    (Search/History/Sources) → /admin 추가 후 4개"로 정정하여 acceptance.md
    NV-5.1 (정확히 4개 표시, 마지막이 Admin)과 일치시킴.
  - **D7 (critical security)** REQ-LH-001 본문에 loopback 판정은 transport
    레이어 `RemoteAddr`만 신뢰하고 `X-Forwarded-For`, `X-Real-IP`, `Forwarded`
    헤더는 **결정에 사용해서는 안 된다**는 normative 제약을 명시적으로 추가.
    spec-compact.md에도 같은 hardening 문구 반영.
  - **D6 (major)** acceptance.md에 알 수 없는 어댑터 ID에 대한 404 시나리오
    2개 추가(AS-1.4 resync, AK-2.4 toggle) 및 5xx 업스트림 실패 → 인라인
    에러 행 시나리오 추가(AS-1.5).
  - **D4/D5/D8–D13 (minor)** EARS 라벨링 정정(REQ-AK-003 Ubiquitous-negative
    재라벨), implicit subject에 "the system shall" 명시 추가, REQ-AV-003
    split → REQ-AV-003 + REQ-AV-004, weasel phrase 정리("정상 응답한다" →
    "응답 코드 200 + 9 entry JSON 페이로드", "비활성 상태이거나 노출되지
    않는다" → 단일 선택, NFR-PERF-001 baseline 구체화).
  - 회피한 권고: D2(`labels` 필드 추가), D3(`created_at` rename)은 SPEC-UI-001
    부터 사용 중인 프로젝트 컨벤션(`created`/`updated`, no `labels`)에 맞춰
    유지. plan-auditor 권고와 어긋나지만 일관성이 우선.

- 2026-05-23 (initial draft v0.1, limbowl via manager-spec):
  SPEC-UI-001(M7 Web UI v1)의 위에 얹는 **운영자 전용** 관리 화면 첫 번째
  버전입니다. 별도 admin 앱이 아니라 같은 Next.js 16 App Router 앱 안의
  `/admin` 라우트로 들어가며, **localhost(127.0.0.1) 전용**으로 노출합니다.
  인증 토큰은 도입하지 않습니다(M7 범위 외, SPEC-AUTH-001/002/003에서 별도
  취급). 대신 백엔드 `/api/admin/*` 엔드포인트가 **루프백 IP가 아닌 요청은
  거부**하는 미들웨어로 강제됩니다.

  V1 범위는 사용자 락(lock)에 따라 세 가지로 한정합니다:

  1. **Adapter 상태 패널** — SPEC-ADP-001..009로 등록된 어댑터들의 인증·연결
     상태, 마지막 동기화 시각, 성공/실패 카운트, 최근 에러 로그 tail,
     수동 재동기화(re-sync) 트리거. 한 줄당 하나의 어댑터를 보여주며 시간당
     쿼리 수·레이턴시 차트 같은 상세 메트릭은 V1 범위 밖입니다.
  2. **API Key view + toggle** — 어떤 어댑터가 어느 시크릿 소스(env var
     이름 또는 secret store path)를 사용하는지 보여주고, 키 존재 여부를
     마스킹된 indicator(set/unset)로 표시합니다. **편집/CRUD는 없음.**
     어댑터별 enable/disable 토글만 허용합니다. 값 변경은 UI 밖(env, secret
     store)에서 이뤄져야 합니다.
  3. **Audit 뷰어** — 쿼리 로그 중심입니다. 페이지네이션된 최근 N개의
     쿼리, 응답 레이턴시, 토큰 사용량, 검색된 소스(citation source) 목록,
     쿼리 시점의 설정 스냅샷, 에러 필터를 보여줍니다. 통합(unified) Audit
     은 아닙니다 — admin 액션 로그, 인증 이벤트는 다루지 않습니다.

  기존 자산을 최대한 재사용합니다:

  - 좌측 네비게이션은 SPEC-UI-001이 만든 `web/src/components/sidebar-nav.tsx`
    의 `NAV_ITEMS` 배열에 `/admin` 항목 1개를 추가합니다([DELTA]).
  - 어댑터 한 줄 행/리스트 패턴은 SPEC-UI-001이 만든
    `web/src/app/sources/page.tsx`를 참조 구현으로 삼습니다(상태 뱃지·메타
    표시·접근성 패턴).
  - SSE/스트리밍은 V1에서 사용하지 않습니다 — Admin은 polling/REST 기반
    (선택적 polling은 EVOL 단계, V1 기본은 수동 새로고침)입니다. SPEC-UI-001
    의 `web/src/lib/sse-client.ts`는 재사용 대상 아닙니다.
  - 백엔드는 기존 `cmd/usearch-api/` 진입점과 `internal/api/handlers/`,
    `internal/audit/` 패키지에 [DELTA]로 admin 핸들러를 추가합니다 — 별도
    프로세스/포트로 분리하지 않습니다. 같은 프로세스 내에서 `/api/admin/*`
    라우트 그룹에 **loopback-only middleware**를 체이닝합니다.

  5개 EARS 모듈(`adapter-status`, `api-key-view`, `audit-viewer`,
  `localhost-guard`, `navigation-integration`)로 구성됩니다. 4개의 NFR
  (보안=loopback 강제, 성능=admin 페이지 응답 1초 이내, 접근성=WCAG 2.1 AA
  기본 준수, 호환성=Next.js 16 App Router/React 19) 포함. SPEC-UI-001과
  동일 milestone(M7)이며 SPEC-UI-001 완료 후 진행됩니다.

  GitHub issue tracking 없음(`issue_number: 0`). plan-auditor 검토와
  annotation cycle 후 `/moai run` 진입 가능.

---

## 1. Purpose

`.moai/project/roadmap.md` M7 Surfaces 마일스톤은 두 개의 surface를 정의합니다.
SPEC-UI-001이 **사용자(검색자)** 화면을 만들었다면, SPEC-UI-002는
**운영자(operator)** 화면을 만듭니다. 운영자가 알아야 하는 것은:

> "9개 어댑터 중 지금 무엇이 깨졌는가? 어떤 API 키가 빠져 있는가? 마지막에
> 들어온 쿼리는 무엇이고 왜 느렸는가?"

이 세 가지 질문에 1초 안에 답하는 단일 페이지가 V1의 정의입니다.

## 2. Scope

### 2.1 In Scope (V1)

- `/admin` 단일 라우트, 같은 Next.js 앱 안의 segment
- Adapter status 단일 패널 (9개 어댑터 한 줄씩, 단일 페이지)
- API Key view + adapter enable/disable toggle (값 편집 없음)
- Query audit viewer (페이지네이션 + 에러 필터)
- Localhost-only access control (백엔드 미들웨어 + 프론트 가드)
- 좌측 네비게이션 `/admin` 항목 1개 추가

### 2.2 Out of Scope — Exclusions (HARD)

다음은 V1에서 **명시적으로 만들지 않습니다**. PR 리뷰 시 이 목록을 reject
근거로 사용합니다:

- **API key editing / CRUD** — UI에서 시크릿 값을 입력·수정·삭제하는 어떤
  필드도 없습니다. 값 변경은 env/secret store 외부 워크플로우에서만 이뤄집니다.
- **Multi-user authentication** — 로그인, 세션, 역할 기반 접근제어(RBAC),
  JWT, OAuth 모두 V1 범위 밖. SPEC-AUTH-001/002/003 진행 시 합류.
- **Separate admin app** — `admin/` 같은 별도 Next.js 앱이나 별도 포트는
  생성하지 않습니다. 같은 프로세스, 같은 라우트 트리.
- **Hourly metrics charts** — 시간당 쿼리 수/p95 레이턴시/처리량 그래프,
  Grafana/Prometheus 임베드 등 시각화 차트.
- **Unified audit log** — admin 액션(누가 enable/disable 했는지), 인증
  이벤트, 시크릿 회전 로그 등 쿼리 외 이벤트를 한 곳에 모은 통합 감사 로그.
  V1 audit 뷰어는 **쿼리 로그만** 보여줍니다.
- **Adapter detail page** — 어댑터당 상세 페이지(`/admin/adapters/:id`)는
  V1에서 만들지 않음. 단일 페이지에서 한 줄로 끝.
- **Real-time push / WebSocket / SSE** — admin 화면 갱신은 수동 새로고침
  기반. polling은 V1.1 후보.

---

## 3. Requirements (EARS)

5개 모듈로 구성되며 각 모듈은 1–3개의 EARS 요구사항을 가집니다. 우선순위는
P0(필수, V1 출시 조건) / P1(권장)로 나눕니다.

### 3.1 Module: `adapter-status` (P0)

**REQ-AS-001 (Ubiquitous, P0)** — `/admin` 페이지의 Adapter Status 섹션은
SPEC-ADP-001부터 SPEC-ADP-009까지 등록된 모든 어댑터를 **단일 리스트로 한
줄씩** 렌더링해야 한다. 각 줄에는 다음 정보가 포함된다:

1. 어댑터 식별자(예: `drive`, `notion`, `slack`, `confluence`, `gmail`,
   `calendar`, `web`, `local`, `arxiv` — `internal/adapters/registry.go` 기준)
2. 인증/연결 상태(`connected` | `auth_required` | `disabled` | `error`)
3. 마지막 동기화 ISO 8601 시각 (없으면 `—`)
4. 성공/실패 카운트(누적, 정수)
5. 최근 에러 메시지 1줄 tail (없으면 `—`)
6. 수동 re-sync 트리거 버튼

**REQ-AS-002 (Event-Driven, P0)** — WHEN 사용자가 어댑터 행의 "Re-sync"
버튼을 클릭하면, the system shall `POST /api/admin/adapters/{id}/resync`를
호출하고, 응답이 도달할 때까지 해당 행을 disabled 상태로 표시한 뒤 성공/실패
결과를 같은 줄의 상태 영역에 반영해야 한다.

**REQ-AS-003 (Unwanted, P0)** — IF 어댑터 레지스트리에서 특정 어댑터의 메타
데이터(예: 마지막 동기화 시각)를 가져올 수 없다면, the system shall **빈
값 또는 `—`로 표시하되 페이지 렌더링은 절대 차단하지 않아야** 한다(부분
실패 격리, SPEC-UI-001의 results-panel 패턴과 일치).

### 3.2 Module: `api-key-view` (P0)

**REQ-AK-001 (Ubiquitous, P0)** — the system shall 각 어댑터 행에 **그 어댑터가
사용하는 시크릿 소스의 식별자**(env var 이름 또는 secret store path)와
**존재 여부 indicator** (`set` | `unset`)를 표시해야 한다. 실제 시크릿 값
(토큰, 키 원본)은 절대 응답 페이로드에 포함되지 않으며, 클라이언트로도
전달되지 않는다.

**REQ-AK-002 (Event-Driven, P0)** — WHEN 사용자가 어댑터의 enable/disable
토글을 조작하면, the system shall `POST /api/admin/adapters/{id}/toggle`을
호출해 어댑터 활성 상태를 변경하고, 변경 결과(`enabled` | `disabled`)를 같은
행에 반영해야 한다.

**REQ-AK-003 (Ubiquitous-negative, P0)** — the system shall NOT 어떤 형태로든
시크릿 편집 UI(입력창, 저장 버튼, 삭제 버튼)를 렌더링한다. 또한 백엔드
admin API는 시크릿 값을 **읽거나 쓰는 엔드포인트를 노출하지 않는다**(POST/
PUT/PATCH 경로에 `*/secret*`, `*/key*` 패턴 부재).

### 3.3 Module: `audit-viewer` (P0)

**REQ-AV-001 (Ubiquitous, P0)** — `/admin` 페이지의 Audit Viewer 섹션은
`GET /api/admin/audit/queries`를 호출해 최근 쿼리 로그를 페이지네이션된
테이블로 표시해야 한다. 각 행은:

1. 쿼리 ID (또는 timestamp 기반 식별자)
2. 쿼리 시각 (ISO 8601)
3. 응답 레이턴시 (ms)
4. 토큰 사용량 (prompt + completion 합계 또는 분리 표기)
5. 검색된 소스(citation source) 개수 또는 어댑터 키 리스트
6. 쿼리 시점의 설정 스냅샷에 대한 요약 또는 expand 토글
7. 에러 여부 indicator

**REQ-AV-002 (State-Driven, P0)** — WHILE 사용자가 "Errors only" 필터를
ON 상태로 두는 동안, the system shall 응답에 에러 indicator가 있는 행만
표시해야 한다.

**REQ-AV-003 (Optional, P1)** — WHERE 백엔드가 cursor 기반 페이지네이션을
지원하는 경우, the system shall 다음/이전 페이지 이동을 cursor token으로
처리해야 한다.

**REQ-AV-004 (Ubiquitous, P0)** — the system shall `limit`/`offset` 기반
페이지네이션을 baseline 동작으로 지원해야 한다(cursor 지원 여부와 무관하게
항상 동작하는 fallback 경로).

### 3.4 Module: `localhost-guard` (P0)

**REQ-LH-001 (Unwanted, P0)** — IF `/api/admin/*` 엔드포인트로 들어온 요청
의 source IP가 **127.0.0.1 또는 ::1(loopback)이 아니면**, the system shall
요청을 인증/처리 없이 `403 Forbidden`으로 즉시 거부해야 한다. 응답 본문에
시스템 정보(버전, hostname, stack trace, 내부 경로)는 포함하지 않는다.

**[SECURITY HARDENING — Normative]** loopback 판정은 **반드시 transport
계층의 RemoteAddr**(`net/http`의 `*http.Request.RemoteAddr`로 노출되는,
실제 TCP 커넥션의 peer 주소)에서만 도출되어야 한다. admin 미들웨어는 다음
HTTP 헤더를 **loopback 결정에 사용해서는 안 된다(MUST NOT trust)**:

- `X-Forwarded-For`
- `X-Real-IP`
- `Forwarded` (RFC 7239)
- 그 외 클라이언트가 임의로 설정 가능한 모든 IP-claim 헤더

이 헤더들은 외부에서 위조 가능하므로(`X-Forwarded-For: 127.0.0.1` 헤더만
붙여 외부에서 접근하는 우회 공격이 표준화된 OWASP 패턴), 신뢰할 경우
loopback gate 전체가 무력화된다. admin route group은 리버스 프록시 뒤에
배치되어서는 안 되며(REQ-LH-002의 127.0.0.1 bind 기본값과 함께 두 겹의
방어선), 만약 운영자가 이 제약을 어겨 0.0.0.0 bind와 리버스 프록시 조합으로
admin endpoint를 노출하더라도 미들웨어는 RemoteAddr 외에는 어떤 입력도
신뢰하지 않으므로 외부 IP는 항상 403으로 차단된다.

**REQ-LH-002 (Ubiquitous, P0)** — `cmd/usearch-api`의 admin route group은
**기본값으로 `127.0.0.1`에 bind**해야 한다. 외부 인터페이스(`0.0.0.0`,
공인 IP)에 admin 포트를 노출하지 않아야 하며, 일반 검색 API와 같은
프로세스/포트를 공유하는 경우에도 라우트 단위 loopback 미들웨어가 항상
선행되어야 한다.

**REQ-LH-003 (Event-Driven, P1)** — WHEN `/admin` 페이지가 클라이언트에서
로드되고 백엔드 admin endpoint 호출이 403/네트워크 에러로 실패하면, the
system shall "Admin UI is only accessible from localhost. Open this page on
the machine running usearch-api." 안내 화면을 렌더링해야 한다(에러를 그대로
노출하지 않음).

### 3.5 Module: `navigation-integration` (P0)

**REQ-NV-001 (Ubiquitous, P0)** — the system shall `web/src/components/
sidebar-nav.tsx`의 `NAV_ITEMS` 배열에 `{ href: "/admin", label: "Admin",
icon: <적절한 lucide 아이콘> }` 한 항목을 [DELTA]로 추가해야 한다.
SPEC-UI-001이 만든 **3개 기존 항목 (Search/History/Sources)** → `/admin`
추가 후 **총 4개**가 되며, 기존 3개 항목의 순서·아이콘·접근성 규칙
(`aria-current`, mobile overlay close)은 변경하지 않는다.

**REQ-NV-002 (State-Driven, P0)** — WHILE `pathname === "/admin"`인
동안, the system shall sidebar의 Admin 항목을 active 스타일(`bg-accent
text-accent-foreground`)로 표시해야 한다(SPEC-UI-001의 active 스타일 규칙
재사용).

---

## 4. Non-Functional Requirements

- **NFR-SEC-001 (loopback enforcement)** — REQ-LH-001/002 만족. 통합
  테스트에서 외부 NIC IP로 호출 시 403 응답을 검증한다.
- **NFR-PERF-001 (admin page load)** — `/admin` 첫 paint 후 1초(1000ms) 이내에
  Adapter Status 9줄 + Audit 첫 페이지(50건)가 표시되어야 한다. baseline:
  loopback fetch(127.0.0.1), fixture 9개 어댑터 + 50건 audit row, M1
  MacBook Air 또는 동등 사양(Apple Silicon, 16GB RAM), Chrome 최신 LTS,
  로컬 빌드 production 모드(`next build && next start`).
- **NFR-A11Y-001 (WCAG 2.1 AA baseline)** — Sidebar `aria-current`, 토글
  버튼 `aria-pressed`, 테이블 `<th scope>` 등 SPEC-UI-001 baseline을
  계승한다.
- **NFR-COMPAT-001** — Next.js 16 App Router, React 19, TypeScript 5+ 엄격
  모드, Tailwind 3.4, shadcn/ui slate default(SPEC-UI-001 환경 그대로).

---

## 5. Architecture Sketch (Reference, not implementation)

### 5.1 Frontend (`web/`)

새 파일:

- `web/src/app/admin/page.tsx` — `/admin` 진입점 (Server Component)
- `web/src/app/admin/_components/adapter-status-panel.tsx` (Client)
- `web/src/app/admin/_components/api-key-row.tsx` (Client)
- `web/src/app/admin/_components/audit-viewer.tsx` (Client)
- `web/src/app/admin/_components/localhost-gate.tsx` (Client) — REQ-LH-003

[DELTA] 수정:

- `web/src/components/sidebar-nav.tsx` — `NAV_ITEMS` 배열에 `/admin` 추가
- `web/src/lib/api.ts` — admin 엔드포인트 helper 추가 (`fetchAdminAdapters`,
  `fetchAdminAudit`, `toggleAdapter`, `resyncAdapter`)

참조 패턴:

- `web/src/app/sources/page.tsx` — 리스트/상태 표시 레이아웃 참조 구현

### 5.2 Backend (`cmd/usearch-api`, `internal/api`)

새 파일 (제안 위치):

- `internal/api/admin/middleware_loopback.go` — REQ-LH-001 미들웨어
- `internal/api/admin/handler_adapters.go` — REQ-AS-001/002, REQ-AK-001/002
- `internal/api/admin/handler_audit.go` — REQ-AV-001/002/003

[DELTA] 수정:

- `cmd/usearch-api/main.go` 또는 `internal/api/handlers/` — admin route
  group 등록, loopback 미들웨어 체인 결합
- `internal/adapters/registry.go` — admin 표시용 메타 노출 API 추가 (read-only
  view 함수, 시크릿 값 미포함)
- `internal/audit/` — 쿼리 로그 페이지네이션 조회 helper (기존 `store.go`
  확장 가능)

### 5.3 API Surface (Admin only, loopback only)

| 메서드 | 경로 | 책임 | EARS |
|--------|------|------|------|
| GET | `/api/admin/adapters` | 어댑터 메타 + 상태 + 키 indicator 리스트 | REQ-AS-001, REQ-AK-001 |
| POST | `/api/admin/adapters/{id}/resync` | 수동 re-sync 트리거 | REQ-AS-002 |
| POST | `/api/admin/adapters/{id}/toggle` | enable/disable 토글 | REQ-AK-002 |
| GET | `/api/admin/audit/queries` | 쿼리 audit 페이지네이션 조회 | REQ-AV-001/002/003/004 |

모든 응답은 시크릿 원본 값을 포함하지 않습니다(REQ-AK-003).

---

## 6. Dependencies & Sequencing

- **Strictly after**: SPEC-UI-001 — 사이드바, layout, API 헬퍼, Tailwind
  설정, shadcn/ui slate 베이스가 먼저 존재해야 합니다.
- **Reads from**: SPEC-ADP-001..009 (어댑터 레지스트리, 키 소스, 상태 메타)
- **Reads from**: SPEC-OBS-001 또는 `internal/audit/` (쿼리 로그 원본)
- **Does not block**: SPEC-AUTH-001/002/003 (인증 도입 시 V2에서 admin 인증
  계층 합류)

## 7. Acceptance Criteria Reference

상세 Given/When/Then 시나리오는 `./acceptance.md` 참조 (모듈당 ≥2 시나리오,
경계/에러 케이스 포함).

## 8. Implementation Plan Reference

태스크 분해·우선순위는 `./plan.md` 참조.

## 9. Exclusions (What NOT to Build) — HARD

위 §2.2 Out of Scope 섹션의 7개 항목을 본 SPEC의 **금지 항목**으로 명시합니다.
PR이 다음 중 하나라도 포함하면 SPEC scope drift로 reject 사유가 됩니다:

1. UI에서 시크릿 값을 입력·편집·삭제하는 컴포넌트
2. 로그인/세션/JWT/OAuth/RBAC 코드 또는 라이브러리 추가
3. `web/admin/`처럼 별도 Next.js 앱 또는 별도 admin 포트 분리
4. 시간당 메트릭 차트, Grafana/Prometheus 임베드, 시계열 시각화 라이브러리 추가
5. admin 액션 로그·인증 이벤트를 포함한 통합 audit 테이블 구조
6. `/admin/adapters/:id` 같은 어댑터 상세 페이지 라우트
7. WebSocket/SSE 기반 실시간 admin 푸시
