# Universal Search — 빠른 시작

5분 안에 검색 한 번 돌리기. 자세한 내용은 [USAGE.md](USAGE.md).

## 1. 빌드

```bash
go build -o usearch ./cmd/usearch
./usearch --version
```

## 2. 의존성 스택 기동

검색은 Python 사이드카 + LLM 게이트웨이가 필요. Docker로 한 번에:

```bash
make compose-up      # 11개 서비스 (postgres, redis, qdrant, meilisearch,
                     # litellm, researcher, embedder, tokenizer-ko, searxng ...)
docker ps            # 전부 healthy 확인
```

## 3. 환경 설정

`.env.example`를 복사하고 키를 채운다. 셸에 로드:

```bash
cp .env.example .env     # LITELLM_MASTER_KEY 등 채우기
set -a; . ./.env; set +a # 현재 셸에 export
```

> 한국 뉴스 소스를 쓰려면 `USEARCH_ADP009_RSS_FEEDS`가 설정돼야 한다
> (`.env.example`에 검증된 RSS 피드 4개 기본 제공).

## 4. 검색

```bash
# 영어
./usearch query "what is retrieval augmented generation"

# 한국어 (자동 라우팅)
./usearch query "트랜스포머 아키텍처 설명"

# JSON 출력 / 소스 한정 / 타임아웃
./usearch query --format json "golang generics"
./usearch query --source arxiv,searxng "diffusion models"
./usearch query --timeout 10s "quick lookup"
```

출력: 인용([1][2]...) 포함 합성 요약은 stdout, 진행/에러는 stderr.

## 5. 소스 확인

```bash
./usearch sources list      # 사용 가능한 어댑터
./usearch sources status     # 연결 상태 + 키 보유 여부
```

## 6. 히스토리

모든 `query`는 JSONL로 자동 기록된다.

```bash
./usearch history list
./usearch history list --limit 5 --since 7d
./usearch history search "golang"
./usearch history show <id>
./usearch history clear --confirm
```

## 7. 대화형 REPL

```bash
./usearch repl
# 질의 입력, 또는 /help /sources /history /config /quit
```

## 알려진 제약

- `usearch deep` — 스텁(미구현, 출력만 찍힘).
- bluesky 어댑터 — Bluesky 공개 API가 일부 네트워크/IP를 차단(HTTP 403)할 수 있음.
- arxiv — `export.arxiv.org` 지연 시 `context deadline exceeded`. 재시도하면 해결.

## 문제 해결

| 증상 | 원인 | 해결 |
|---|---|---|
| `no adapters matched` | 라우팅 결과 비어있음 | 질의 구체화 또는 `--source` 지정 |
| `synthesis: unavailable` | LLM 키 미설정 | `.env`의 `LITELLM_MASTER_KEY` 확인 |
| koreanews `no feed URLs` | RSS 피드 env 미설정 | `USEARCH_ADP009_RSS_FEEDS` 로드 |
| 어댑터 `HTTP 403/timeout` | 업스트림 차단/지연 | 외부 문제, 다른 소스 사용 또는 재시도 |
