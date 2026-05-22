# SPEC-UI-002 — Compact Reference

> Auto-generated compact view of `./spec.md` + `./acceptance.md`. Use this as
> the load-cheap entry point during `/moai run`. Source of truth is `spec.md`.
> Version 0.1.1 (post-audit-1 revision).

## Identity

- **ID**: SPEC-UI-002
- **Version**: 0.1.1
- **Title**: Admin UI v1 — adapter status, API key view+toggle, query audit viewer
- **Milestone**: M7 — Surfaces
- **Owner**: expert-frontend
- **Methodology**: tdd | Coverage target: 85
- **Depends on**: SPEC-UI-001, SPEC-ADP-001..009
- **Blocks**: (none)

## One-Sentence Purpose

운영자가 `/admin` 한 페이지에서 (1) 9개 어댑터 상태, (2) 어떤 어댑터에
어떤 시크릿 소스가 set/unset인지 + enable/disable 토글, (3) 최근 쿼리
audit 로그를 **localhost 전용**으로 확인할 수 있게 한다.

## EARS Modules — Compact Index

| 모듈 | REQ ID | Pattern | 한 줄 요약 | Priority |
|------|--------|---------|------------|----------|
| adapter-status | REQ-AS-001 | Ubiquitous | 9개 어댑터 한 줄씩 메타 표시 | P0 |
| adapter-status | REQ-AS-002 | Event-Driven | Re-sync 버튼 → POST resync | P0 |
| adapter-status | REQ-AS-003 | Unwanted | 메타 부분 누락 시 `—` 표시, 렌더링 비차단 | P0 |
| api-key-view | REQ-AK-001 | Ubiquitous | 시크릿 소스 식별자 + set/unset indicator | P0 |
| api-key-view | REQ-AK-002 | Event-Driven | enable/disable 토글 → POST toggle | P0 |
| api-key-view | REQ-AK-003 | Ubiquitous-negative | 시크릿 편집 UI/엔드포인트 부재 (HARD) | P0 |
| audit-viewer | REQ-AV-001 | Ubiquitous | 쿼리 로그 페이지네이션 테이블 | P0 |
| audit-viewer | REQ-AV-002 | State-Driven | "Errors only" 필터 ON 동안 에러 행만 | P0 |
| audit-viewer | REQ-AV-003 | Optional | cursor 지원 시 cursor token으로 페이지 이동 | P1 |
| audit-viewer | REQ-AV-004 | Ubiquitous | limit/offset baseline 페이지네이션 항상 지원 | P0 |
| localhost-guard | REQ-LH-001 | Unwanted | non-loopback IP → 403, **RemoteAddr-only, IP-claim 헤더 무시** | P0 |
| localhost-guard | REQ-LH-002 | Ubiquitous | admin route group은 127.0.0.1 bind 기본 | P0 |
| localhost-guard | REQ-LH-003 | Event-Driven | 403/네트워크 실패 시 안내 화면 fallback | P1 |
| navigation-integration | REQ-NV-001 | Ubiquitous | sidebar NAV_ITEMS 3개 → /admin 추가 후 4개 [DELTA] | P0 |
| navigation-integration | REQ-NV-002 | State-Driven | pathname `/admin`일 때 Admin active 스타일 | P0 |

## REQ-LH-001 SECURITY HARDENING (압축 인용)

loopback 판정은 transport 계층 `RemoteAddr`만 신뢰한다. admin 미들웨어는
**`X-Forwarded-For`, `X-Real-IP`, `Forwarded` (RFC 7239), 그 외 클라이언트
설정 가능한 IP-claim 헤더를 결정에 사용해서는 안 된다(MUST NOT)**. 이
헤더들은 외부에서 위조 가능하며 신뢰할 경우 loopback gate 전체가 무력화된다.
0.0.0.0 bind + 리버스 프록시 조합으로 admin endpoint가 노출되더라도
RemoteAddr 외 어떤 입력도 신뢰하지 않으므로 외부 IP는 항상 403으로 차단된다.

## Acceptance Scenarios — Compact Index

| Scenario | EARS | 핵심 검증 |
|----------|------|-----------|
| AS-1.1 | REQ-AS-001 | HTTP 200 + 9 entry JSON, 시크릿 원본 미노출 |
| AS-1.2 | REQ-AS-002 | Re-sync 클릭 → POST, 행만 disabled, 다른 행 무영향 |
| AS-1.3 | REQ-AS-003 | last_sync 누락 → `—`, 페이지 정상 |
| AS-1.4 | REQ-AS-002 | 알 수 없는 ID resync → HTTP 404 + 구조화 에러 body |
| AS-1.5 | REQ-AS-002 | 업스트림 5xx → HTTP 502, UI 인라인 에러 행, 다른 행 무영향 |
| AK-2.1 | REQ-AK-001 | set/unset 두 어댑터 indicator + 원본 값 없음 |
| AK-2.2 | REQ-AK-002 | enable→disable→enable 라운드트립 + aria-pressed |
| AK-2.3 | REQ-AK-003 | secret-like input 0개, 시크릿 쓰기 엔드포인트 부재 |
| AK-2.4 | REQ-AK-002 | 알 수 없는 ID toggle → HTTP 404 + 구조화 에러 body |
| AV-3.1 | REQ-AV-001 | 100건 중 50건 표시 + Next로 페이지 이동 |
| AV-3.2 | REQ-AV-002 | Errors only ON → 5개 에러 행만 |
| AV-3.3 | REQ-AV-001 | 0건일 때 empty state, 페이지네이션 버튼 disabled |
| AV-3.4 | REQ-AV-003/004 | cursor 비지원 → limit/offset, 지원 → cursor token |
| LH-4.1 | REQ-LH-001 | 외부 NIC IP → 403, 정보 누설 없음 |
| LH-4.2 | REQ-LH-001/002 | 127.0.0.1, ::1 통과 |
| LH-4.3 | REQ-LH-001 | X-Forwarded-For / X-Real-IP / Forwarded 위조 모두 403 |
| LH-4.4 | REQ-LH-003 | 403/네트워크 실패 → 안내 화면, raw 미노출 |
| NV-5.1 | REQ-NV-001 | 기존 3개 (Search/History/Sources) + Admin → 총 4개, 기존 순서 보존 |
| NV-5.2 | REQ-NV-002 | `/admin` 시 active 스타일 + aria-current=page |
| NV-5.3 | REQ-NV-001 | mobile hamburger 열고 클릭 → 정상 라우팅 |

## Reference Implementations (참조 대상, 수정 대상 아님)

- `web/src/components/sidebar-nav.tsx` — NAV_ITEMS 패턴(현재 3개), active
  스타일, mobile overlay 규칙 ([DELTA] 대상, /admin 추가 후 4개)
- `web/src/app/sources/page.tsx` — 리스트/상태 행 렌더링 참조 구현
- `internal/audit/store.go`, `internal/audit/types.go` — 쿼리 로그 원본
- `internal/adapters/registry.go` — 어댑터 메타 단일 진입점 ([DELTA] 대상)
- `cmd/usearch-api/main.go` + `internal/api/handlers/` — admin route group
  결합 지점 ([DELTA] 대상)

## API Surface (Admin only, loopback only)

- `GET  /api/admin/adapters`
- `POST /api/admin/adapters/{id}/resync` — 200 / 404 (unknown ID) / 502 (upstream failure)
- `POST /api/admin/adapters/{id}/toggle` — 200 / 404 (unknown ID)
- `GET  /api/admin/audit/queries` — limit/offset (baseline) 또는 cursor (선택)

모든 endpoint는 REQ-LH-001 SECURITY HARDENING의 RemoteAddr-only 룰이
선행된다. IP-claim 헤더는 미들웨어 레벨에서 무시된다.

## Exclusions (HARD) — 7 items

본 SPEC에서 **만들지 않는 것**. PR에 등장하면 reject:

1. UI에서 시크릿 값 입력/편집/삭제 컴포넌트
2. 로그인/세션/JWT/OAuth/RBAC
3. `web/admin/` 별도 앱 또는 별도 포트
4. 시간당 메트릭 차트 / Grafana / Prometheus 임베드
5. 통합 audit (admin 액션, 인증 이벤트)
6. `/admin/adapters/[id]` 어댑터 상세 페이지
7. WebSocket/SSE 기반 admin 실시간 push

## DoD (HARD)

- 모든 P0 EARS(15개) 자동화 테스트 통과 — REQ-AV-004 포함
- Negative path: AS-1.4, AS-1.5, AK-2.4 CI 통과
- 백엔드 admin / 프론트 admin coverage ≥ 85%
- Scenario LH-4.1, LH-4.3, AK-2.3 회귀 테스트 CI 포함
- 7 Exclusion 항목 PR diff 미포함 확인
- TRUST 5 통과
