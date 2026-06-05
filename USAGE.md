# Universal Search 활용 가이드

하이브리드 AI 검색 엔진 — Go 오케스트레이션 평면(`usearch` CLI / `usearch-api` / `usearch-mcp`) + Python 사이드카(researcher, embedder, tokenizer-ko) + Next.js 웹 UI.

> 이 문서는 실제 소스 코드(`cmd/usearch/*.go`, `Makefile`, `deploy/docker-compose.yml`, `.env.example`, `web/package.json`)를 기준으로 검증해 작성했습니다. 코드와 기존 docs 사이트 내용이 다를 경우 **코드가 정답**입니다. 일부 서브커맨드는 아직 스텁(미구현) 상태이며, 본문에 명시했습니다.

## 목차

1. [개요](#1-개요)
2. [사전 요구사항](#2-사전-요구사항)
3. [설치](#3-설치)
4. [의존성 스택 기동](#4-의존성-스택-기동)
5. [환경 설정 (.env)](#5-환경-설정-env)
6. [CLI 설정 관리 (usearch config)](#6-cli-설정-관리-usearch-config)
7. [CLI 활용](#7-cli-활용)
8. [API 서버 (usearch-api)](#8-api-서버-usearch-api)
9. [MCP 서버 (usearch-mcp)](#9-mcp-서버-usearch-mcp)
10. [웹 UI](#10-웹-ui)
11. [개발 워크플로](#11-개발-워크플로)
12. [문제 해결](#12-문제-해결)

---

## 1. 개요

Universal Search는 하나의 질의를 의도 분류 → 다중 어댑터 팬아웃 → LLM 합성(synthesis) 파이프라인으로 처리하는 검색 엔진입니다. 세 개의 평면으로 구성됩니다.

- **Go 평면**: `usearch`(CLI), `usearch-api`(REST/SSE 서버), `usearch-mcp`(MCP 서버)
- **Python 사이드카**: `researcher`(합성/딥리서치), `embedder`(임베딩), `tokenizer-ko`(한국어 토크나이저)
- **웹 UI**: Next.js 16 App Router (`web/`)

검색 어댑터(소스): `reddit`, `hn`(Hacker News), `arxiv`, `github`(토큰 필요), `youtube`, `searxng`, `bluesky`, `naver`(키 필요), `koreanews`.

---

## 2. 사전 요구사항

| 도구 | 최소 버전 | 용도 |
| ------- | --------------- | -------------------------------------------- |
| Docker | 24+ | `make compose-up` 의존성 스택 기동 |
| Go | 1.25+ | `make build`, `make test` |
| Python | 3.11+ | Python 사이드카 |
| Node.js | 22+ | 웹 프론트엔드 |
| make | 임의 | 빌드 표준 도구 |
| uv | 0.4+ | Python 패키지 매니저 (`pip install uv`) |
| pnpm | 9+ | Node 패키지 매니저 (`npm install -g pnpm`) |

---

## 3. 설치

### 3.1 릴리스 바이너리 (v1.0.0+)

macOS / Linux용 사전 빌드 바이너리:

```bash
# Linux amd64
curl -L https://github.com/elymas/universal-search/releases/download/v1.0.0/usearch_1.0.0_linux_amd64.tar.gz \
  | tar xz -C /usr/local/bin/

# macOS amd64
curl -L https://github.com/elymas/universal-search/releases/download/v1.0.0/usearch_1.0.0_darwin_amd64.tar.gz \
  | tar xz -C /usr/local/bin/

# 설치 확인
usearch --version   # 출력: usearch v1.0.0
```

arm64 등 다른 아키텍처는 [releases](https://github.com/elymas/universal-search/releases) 참고.

### 3.2 소스 빌드 (개발)

```bash
# 1. 저장소 클론
git clone <repo-url>
cd universal-search

# 2. 환경 템플릿 복사
cp .env.example .env
# .env 편집 — MEILI_MASTER_KEY, POSTGRES_PASSWORD 등 채우기

# 3. 의존성 스택 기동
make compose-up

# 4. usearch 바이너리 빌드
make build           # 결과물: cmd/usearch/usearch

# 5. 확인
./cmd/usearch/usearch --version
# 출력: usearch v<semver> (예: usearch v0.1.0-dev)
```

버전 출력 형식은 항상 `usearch v<semver>` 입니다 (`cmd/usearch/root.go`의 `SetVersionTemplate`).

---

## 4. 의존성 스택 기동

`deploy/docker-compose.yml`이 정의하는 실제 서비스는 다음과 같습니다(총 12개). 포트는 `.env`로 재정의 가능하며, 표의 값은 기본값입니다.

| 서비스 | 이미지 | 기본 포트 | 역할 |
| --- | --- | --- | --- |
| qdrant | qdrant/qdrant:v1.16.3 | 6333(HTTP) / 6334(gRPC) | 벡터 DB |
| meilisearch | getmeili/meilisearch:v1.42.1 | 7700 | 키워드 검색 |
| postgres | postgres:16.13-alpine3.23 | 5432 | 관계형 저장소 |
| redis | redis:7-alpine | 6379 | 캐시 |
| searxng | searxng/searxng | 8080 | 메타 검색 |
| litellm | ghcr.io/berriai/litellm | 4000 | LLM 게이트웨이 |
| researcher | (로컬 빌드) | 8081 | 합성 / 딥리서치 사이드카 |
| embedder | (로컬 빌드) | 8082 | 임베딩 사이드카 |
| tokenizer-ko | (로컬 빌드) | 8083 | 한국어 토크나이저 |
| prometheus | prom/prometheus:v2.54.1 | 9091 → 9090 | 메트릭 수집 |
| grafana | grafana/grafana:11.3.0 | 127.0.0.1:3000 | 대시보드 |
| alertmanager | prom/alertmanager:v0.27.0 | 127.0.0.1:9093 | 알림 |

```bash
make compose-up     # 전체 서비스 기동 (--wait 로 헬스체크 대기)
make compose-logs   # 로그 실시간 추적
make compose-down   # 중지 및 컨테이너 제거
```

> GPU 임베딩이 필요하면 `deploy/docker-compose.gpu.yml`을 추가로 사용합니다.

---

## 5. 환경 설정 (.env)

`cp .env.example .env` 후 값을 채웁니다. `.env.example`이 정의하는 주요 변수 그룹:

| 그룹 | 대표 변수 | 비고 |
| --- | --- | --- |
| Meilisearch | `MEILI_MASTER_KEY`, `MEILI_ENV` | 마스터 키 필수 |
| Postgres | `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `DATABASE_URL` | |
| Redis | `REDIS_PORT`, `REDIS_URL` | |
| Qdrant | `QDRANT_HTTP_PORT`, `QDRANT_GRPC_PORT` | |
| SearXNG | `SEARXNG_BASE_URL`, `SEARXNG_SECRET` | |
| LLM (LiteLLM) | `LITELLM_MASTER_KEY`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `LITELLM_BUDGET_USD` | LLM 합성에 필요 |
| 관측성(obs) | `LOG_LEVEL`, `OTLP_ENDPOINT`, `OTLP_SAMPLE_RATIO`, `LOKI_ENDPOINT`, `USEARCH_ADMIN_PORT` | |
| researcher | `RESEARCHER_BASE_URL`, `RESEARCHER_REQUEST_TIMEOUT_SECONDS`, `RESEARCHER_MODEL_DEFAULT` | |
| embedder | `EMBEDDER_BASE_URL`, `EMBEDDER_MODEL_NAME`, `EMBEDDER_DEVICE`, `EMBEDDER_USE_FP16` | |
| tokenizer-ko | `TOKENIZER_KO_BASE_URL`, `TOKENIZER_KO_TIMEOUT_MS` | |
| 딥 에이전트 | `DEEP_AGENT_RESEARCHER_MODEL`, `DEEP_AGENT_WRITER_MODEL`, `DEEP_TREE_ENABLED` 등 | 딥리서치 파이프라인 모델 설정 |

`usearch` CLI가 직접 읽는 런타임 환경변수:

- `LITELLM_MASTER_KEY` — 설정 시(그리고 `--no-llm` 미사용 시) LLM 합성 클라이언트 초기화
- `LOG_LEVEL`, `OTLP_ENDPOINT` — 관측성
- `USEARCH_ADMIN_PORT` — admin 서버 바인드 포트(`127.0.0.1:<port>`)
- `GITHUB_TOKEN` — 설정 시에만 GitHub 어댑터 등록
- `NAVER_CLIENT_ID`, `NAVER_CLIENT_SECRET` — Naver 어댑터(없으면 조용히 비활성)
- `USEARCH_SEARXNG_URL` — SearXNG 인스턴스 URL (기본 `http://searxng:8080`)
- `*_BASE_URL`(REDDIT/HN/ARXIV/GITHUB/YOUTUBE/BLUESKY) — 어댑터 엔드포인트 재지정(주로 테스트용)

---

## 6. CLI 설정 관리 (usearch config)

`usearch`는 XDG 규약 TOML 파일을 설정으로 사용합니다. 우선순위(높은 순):

1. 커맨드라인 플래그
2. 환경변수 (`USEARCH_*` 접두사)
3. 설정 파일 (`~/.config/usearch/config.toml`)
4. 내장 기본값

```bash
usearch config path                                   # 해석된 설정 파일 경로 출력
usearch config show                                   # 병합된 최종 설정을 TOML로 출력
usearch config init                                   # 기본 설정 파일 생성 (--force 로 덮어쓰기)
usearch config get server.endpoint                    # 단일 키 값 조회 (형식: section.name)
usearch config set server.endpoint http://localhost:9090   # 키 값 설정
```

> 보안: `usearch config set auth.token ...`은 거부됩니다. 토큰은 별도 자격증명 파일(`<config_home>/usearch/credentials`)로 관리합니다.

---

## 7. CLI 활용

### 7.1 query — 검색 + 합성

질의를 의도 분류 → 어댑터 팬아웃 → LLM 합성으로 처리합니다. 결과는 stdout, 진행/에러는 stderr로 분리됩니다.

```bash
usearch query "latest AI research"
usearch query --format json --source reddit "golang generics"
usearch query --source reddit,hn --timeout 10s "quick lookup"
```

플래그:

| 플래그 | 기본값 | 설명 |
| --- | --- | --- |
| `--source` | (빈 값 = 활성 어댑터 전체) | 쉼표 구분 어댑터 이름 |
| `--format` | `text` | `text` / `json` / `markdown` |
| `--timeout` | `30s` | 전체 파이프라인 데드라인 (최대 `5m`) |
| `--no-llm` | `false` | LLM 클라이언트 초기화 생략 |
| `--no-obs` | `false` | 관측성 초기화 비활성(테스트용) |

종료 코드:

| 코드 | 의미 |
| --- | --- |
| 0 | 성공 (문서 1개 이상 + 비어있지 않은 요약) |
| 1 | 사용자 입력 오류 (빈 프롬프트, 잘못된 format, 알 수 없는 어댑터 등) |
| 2 | 시스템 오류 (타임아웃, 전체 어댑터 실패, 알 수 없는 서브커맨드) |
| 3 | 부분 성공 (합성 실패 또는 일부 어댑터 오류, 단 문서 1개 이상 확보) |

### 7.2 deep — 딥리서치 파이프라인 (스텁)

다단계 파이프라인(Researcher → Reviewer → Writer → Verifier)을 호출합니다.

```bash
usearch deep "comprehensive analysis of AI safety"
usearch deep --budget 0.50 "latest quantum computing breakthroughs"
```

- `--budget` (기본 `1.0`): USD 비용 한도.
- **현재 상태**: 파이프라인 미연결 스텁. 진행 단계를 출력한 뒤 "not yet wired (requires LLM client)" 메시지와 함께 종료 코드 2를 반환합니다. 실제 실행은 LLM 클라이언트 연동 이후 활성화됩니다.

### 7.3 repl — 대화형 모드

인자 없이 실행하면 stdin이 TTY일 때 REPL로 진입합니다. `--repl`로 강제 진입도 가능합니다.

```bash
usearch              # TTY면 REPL 진입
usearch --repl       # 강제 REPL
usearch repl         # 서브커맨드로도 진입 (별칭: interactive, i)
```

슬래시 명령: `/help`, `/quit`(`/q`,`/exit`), `/sources`, `/history`, `/config`. 각 질의는 자동으로 히스토리에 저장됩니다.

> **현재 상태**: REPL 안에서의 파이프라인 실행은 아직 인라인 연결되지 않았습니다("pipeline execution not yet wired in REPL"). 슬래시 명령과 히스토리 저장은 동작합니다.

### 7.4 history — 질의 히스토리

히스토리는 기본적으로 JSONL 파일에 저장됩니다(프롬프트, 요약, 사용 어댑터, 지연시간, 비용 포함).

```bash
usearch history list                  # 최근 항목 (기본 20개)
usearch history list --limit 5
usearch history list --since 7d       # 기간 필터 (d/h/m 단위)
usearch history show <id>             # 상세 (--format json 지원)
usearch history search "golang"       # 프롬프트 부분 문자열 검색
usearch history clear                 # 전체 삭제 (대화형 확인)
usearch history clear --confirm       # 비대화형 삭제 (--confirm 필수)
usearch history clear --since 30d --confirm   # 30일보다 오래된 항목만 삭제
```

> 비대화형 환경(파이프/CI)에서 `clear`는 `--confirm` 없이는 거부됩니다(종료 코드 1).

### 7.5 sources — 소스 관리

```bash
usearch sources list           # 사용 가능한 어댑터 목록 (이름/카테고리/설명)
usearch sources status         # 소스 헬스 상태
usearch sources show reddit    # 특정 소스 상세
```

`sources list`가 노출하는 어댑터: `arxiv`, `github`, `hn`, `koreanews`, `naver`, `reddit`, `searxng`, `youtube`.

> **현재 상태**: `sources status`는 헬스 체크 미구현으로 모든 소스를 `unknown`으로 표시합니다. 또한 `sources list`의 목록은 코드 내 정적 목록이라, query 파이프라인이 실제 등록하는 `bluesky` 어댑터는 여기에 표시되지 않습니다.

### 7.6 login — 인증 (스켈레톤)

```bash
usearch login status     # 인증 상태 (현재 항상 "Not authenticated.")
usearch login logout      # 자격증명 정리
```

> **현재 상태**: 실제 OIDC 인증은 향후 릴리스(SPEC-AUTH-001) 예정. 현재는 스켈레톤입니다.

---

## 8. API 서버 (usearch-api)

REST/SSE API 서버 진입점은 `cmd/usearch-api`입니다.

- 환경변수: `LOG_LEVEL`, `OTLP_ENDPOINT`, `USEARCH_ADMIN_PORT`.
- 라우트(등록됨): `POST /query/stream`(SSE 스트리밍 합성), `/api/admin/adapters`, `/api/admin/adapters/health`, `POST /api/admin/adapters/{id}/resync`, `POST /api/admin/adapters/{id}/toggle`, `/api/admin/audit/queries`. admin 라우트는 모두 loopback 전용 미들웨어로 보호됩니다.

> **현재 상태**: 메인 진입점은 라우트를 mux에 등록하지만 아직 `ListenAndServe`를 호출하지 않는 스텁입니다("usearch-api: not implemented (see SPEC-IR-001)"). 즉 현재는 기동 즉시 종료합니다. 완전한 HTTP 서버는 SPEC-IR-001에서 활성화됩니다. 컨테이너 빌드는 `deploy/Dockerfile.usearch-api` 참고.

---

## 9. MCP 서버 (usearch-mcp)

MCP(Model Context Protocol) 서버로, `search`, `deep_research`, `list_sources`, `get_citation` 도구를 노출합니다.

```bash
usearch-mcp --version              # 버전 출력
usearch-mcp                        # stdio 트랜스포트(기본)로 기동
usearch-mcp --transport http       # Streamable HTTP 트랜스포트
```

- 트랜스포트는 `--transport`(`stdio`/`http`) 또는 환경변수 `MCP_TRANSPORT`로 지정. 플래그가 환경변수를 덮어씁니다.
- 기타 환경변수: `LOG_LEVEL`, `OTLP_ENDPOINT`, `USEARCH_ADMIN_PORT`.

Claude 등 MCP 클라이언트에 stdio로 등록하는 예시:

```json
{
  "mcpServers": {
    "usearch": {
      "command": "usearch-mcp",
      "args": ["--transport", "stdio"]
    }
  }
}
```

> **참고**: 현재 MCP 서버의 어댑터 레지스트리는 비어있는 상태로 초기화됩니다(`adapters.NewRegistry(nil)`). 실제 검색 결과를 얻으려면 레지스트리 연동이 필요합니다. 서버 기동·트랜스포트·도구 노출 자체는 동작합니다.

---

## 10. 웹 UI

`web/`는 Next.js 16 (App Router) + React 19 기반입니다. `web/package.json`의 실제 스크립트:

```bash
pnpm --dir web dev          # 개발 서버 (기본 http://localhost:3000)
pnpm --dir web build        # 프로덕션 빌드
pnpm --dir web start        # 프로덕션 서버 (build 이후)
pnpm --dir web lint         # eslint
pnpm --dir web typecheck    # tsc --noEmit
pnpm --dir web test         # vitest run
pnpm --dir web format       # prettier
```

> Grafana 컨테이너도 기본 3000 포트를 쓰므로(`127.0.0.1:3000`), `make compose-up` 상태에서 `pnpm --dir web dev`를 같은 포트로 띄우면 충돌합니다. `next dev -p 3001`처럼 포트를 바꾸거나 Grafana 포트를 조정하세요.

---

## 11. 개발 워크플로

`Makefile` 타깃:

```bash
make help          # 사용 가능한 타깃 목록
make build         # cmd/usearch/usearch 빌드
make test          # 전체 테스트 (Go + Python + Node)
make test-go       # go test ./... -race -cover
make test-py       # 각 사이드카(researcher/storm/embedder) pytest
make test-node     # pnpm --dir web typecheck
make lint          # go vet + golangci-lint + ruff + eslint + hadolint
make fmt           # gofmt + ruff format + prettier
make tidy          # go mod tidy
make install-py    # uv sync (Python 워크스페이스)
make clean         # 빌드 산출물 제거
make dev           # compose 스택 기동 + 준비 완료 메시지
```

> `make test-node`는 현재 타입체크만 수행합니다. 다만 `web/package.json`에는 `vitest` 기반 `test` 스크립트가 추가되어 있어 `pnpm --dir web test`로 별도 실행할 수 있습니다.

디렉터리 구조:

```
cmd/usearch       → Go CLI 바이너리 (query/deep/repl/config/login/history/sources)
cmd/usearch-api   → REST/SSE API 서버 (스텁)
cmd/usearch-mcp   → MCP 서버
internal/         → 도메인 패키지 (adapters, router, fanout, synthesis, obs, usearch/config, usearch/history ...)
pkg/              → 공개 Go API (types 등)
services/         → Python 사이드카 (researcher, storm, embedder)
web/              → Next.js 16 프론트엔드
deploy/           → docker-compose 개발 스택 + Dockerfile
docs/             → Nextra 문서 사이트 (ko/en)
```

---

## 12. 문제 해결

| 증상 | 원인 | 해결 |
| --- | --- | --- |
| `make compose-up`이 헬스체크에서 멈춤 | 특정 서비스 비정상 | `make compose-logs`로 로그 확인, `.env` 필수 값 점검 |
| query에 LLM 요약이 비고 `[synthesis: unavailable]` 출력 | `LITELLM_MASTER_KEY` 미설정 또는 researcher 미기동 | `.env`에 키 설정 + `litellm`/`researcher` 컨테이너 확인 |
| `usearch query` 종료 코드 3 | 일부 어댑터 오류 또는 합성 실패(문서는 일부 확보) | stderr의 어댑터 오류 메시지 확인 |
| GitHub 결과 없음 | `GITHUB_TOKEN` 미설정 시 GitHub 어댑터 미등록 | `.env`에 `GITHUB_TOKEN` 추가 |
| Naver 어댑터 동작 안 함 | `NAVER_CLIENT_ID`/`NAVER_CLIENT_SECRET` 미설정 | 키 설정 후 재기동 |
| 웹 dev 서버 포트 충돌 | Grafana가 3000 사용 중 | `next dev -p 3001` 또는 Grafana 포트 변경 |
| `usearch deep`가 결과 없이 종료 | deep 파이프라인 미연결 스텁 | LLM 연동 릴리스 대기 |
| 모듈 오류 | go.mod 불일치 | `make tidy && go mod verify` |

상세 문서는 `docs/` Nextra 사이트(한/영) 및 [공식 문서](https://elymas.github.io/universal-search/)를 참고하세요.
