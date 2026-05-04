# Changelog

All notable changes to this project are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **SPEC-CLI-001** — `usearch query` subcommand v0 (M2)
  - `cmd/usearch/integration_test.go` (NFR-CLI-001) — build-tag `integration` end-to-end test; spins up httptest stubs for Reddit, HN, and the researcher sidecar; builds the binary fresh; runs `usearch query --no-llm --format json "hello world"` against the stubs; asserts exit ∈ {0, 3} and JSON stdout with `schema_version`/`query`/`adapters`/`summary`/`citations`/`stats` keys; verified PASS with exit=0, summary="Hello world is a classic programming greeting [1] [2].", 2 citations, 685ms wall-clock
  - `cmd/usearch/query.go`: `buildProductionRegistry()` honors `REDDIT_BASE_URL` and `HN_BASE_URL` env-var overrides for stub-driven integration testing; `Execute()` orchestrator (flag parsing, Intent Router, adapter fanout, synthesis, output formatting); `runFanout()` concurrent adapter fanout via `golang.org/x/sync/errgroup`; `parseQueryFlags()` supporting `--source`, `--format`, `--timeout`, `--no-obs`; `intersectSources()` for source filtering; `determineExitCode()` for exit code policy; functional options `withRegistry()` + `withSynth()` for test injection
  - `cmd/usearch/exitcode.go`: exit code constants `ExitSuccess=0`, `ExitUserError=1`, `ExitSystemError=2`, `ExitPartial=3`; `classifyError()` helper
  - `cmd/usearch/progress.go`: `progressEmitter` interface with `humanProgress` (text mode) and `jsonProgress` (no-op) implementations
  - `cmd/usearch/output_text.go`: `formatText()` — summary + numbered citations block; degraded mode renders raw doc snippets
  - `cmd/usearch/output_json.go`: `formatJSON()` — schema version "1" JSON envelope with `query`, `category`, `lang`, `adapters`, `summary`, `citations`, `stats` fields
  - `cmd/usearch/query_response.go`: internal pipeline types `queryResponse`, `queryCitation`, `queryStats`
  - `cmd/usearch/main.go`: updated `dispatch()` subcommand router; `--help`/`-h`/`help` aliases; `usageText()` help string; `runQueryWithObs()` production path with obs.Init + optional LLM init
  - Degraded mode (REQ-CLI-009): `nopSynthClient` returns `errSynthUnavailable`; output falls back to numbered raw docs; exit code 3
  - OTel span emission on Execute() entry/exit with `cli.exit_code` attribute; ULID request ID via `internal/obs/reqid`
  - 35 unit tests (goleak `TestMain`, REQ-CLI-001..011 + NFR-CLI-001..004); 80.1% coverage
  - MX tags: `@MX:ANCHOR` on `Execute()`, `@MX:WARN` on `runFanout()`, `@MX:NOTE` on `progressEmitter` interface

- **SPEC-SYN-001** — Basic synthesis v0 (M2)
  - `services/researcher/` Python FastAPI sidecar: `app.py` (lifespan + POST /synthesize + GET /health), `models.py` (Pydantic v2 NormalizedDocPayload / SynthesizeRequest / Citation / SynthesizeResponse with ConfigDict extra=forbid), `synthesis.py` (citation-assembly + marker validation), `gateway.py` (OpenAI SDK wired to LiteLLM), `obs.py` (JSON stdlib logger + Timer context manager), `__main__.py` (uvicorn entrypoint)
  - `internal/synthesis/` Go HTTP client: `types.go` (Request/Result/Citation + ErrInvalidRequest/ErrSidecarUnreachable/ErrTimeout), `config.go` (RESEARCHER_BASE_URL + RESEARCHER_REQUEST_TIMEOUT_SECONDS env binder), `client.go` (context timeout + exponential backoff retry, 2 retries, 500ms/1500ms ±10% jitter)
  - `internal/obs/metrics/synthesis.go` — new metric family: `usearch_synthesis_calls_total{outcome}`, `usearch_synthesis_latency_seconds{outcome}`, `usearch_synthesis_cost_usd_total`
  - Degraded mode: when LiteLLM is unreachable, sidecar returns 200 with `degraded=true` + bullet-list of doc titles/URLs within 2s
  - REQ-SYN-001..007 + NFR-SYN-001..004 fully implemented; 33 Python tests + 11 Go tests; >80% coverage on both sides
  - MX tags: `@MX:ANCHOR` on `synthesize()` (Python), `Synthesize()` (Go), `Config` (Go); `@MX:WARN` on `complete()` gateway call, `withRetry()` loop

- **SPEC-ADP-002** — Hacker News reference adapter (M2)
  - `internal/adapters/hn/` package implementing `types.Adapter` against the Algolia HN Search public API (`https://hn.algolia.com/api/v1/search`)
  - `Search()`: query + tags=story, hitsPerPage clamped [1,100], integer-cursor pagination, numeric filters (`since` → `created_at_i>=`, `min_points` → `points>=`), SSRF-guard redirect allowlist (`{hn.algolia.com, news.ycombinator.com}`, max 3 hops), 5 MB body cap
  - `Capabilities()`: SourceID=hackernews, SupportsSince=true, RateLimitPerMin=60, DefaultMaxResults=25, RequiresAuth=false
  - `Healthcheck()`: TCP dial to `hn.algolia.com:443`
  - `normalizeScore()`: `clamp(0.5 + 0.5*tanh(points/100), 0, 1)` Tanh formula
  - `stripHTML()`: conservative stdlib-only HTML tag stripper with entity decoding
  - `parseRetryAfter()`: RFC 7231 §7.1.3, integer-seconds first then HTTP-date, 5s default, 60s cap
  - Two-branch URL construction: external `url` for link posts, `news.ycombinator.com/item?id=<objectID>` permalink for self-posts
  - Defensive `_tags` filter: client-side guard against Algolia API drift
  - 7 golden-file fixtures under `testdata/`, 95.1% statement coverage, 441 allocs/op per NFR-ADP2-001
  - MX tags: `@MX:ANCHOR` on `Search` + `parseHits`, `@MX:WARN` on `doRequest`, `@MX:NOTE` on constants and `categorizeStatus`

- **SPEC-BOOT-001** — M1 Foundation repo scaffold and CI bootstrap
  - Go module `github.com/elymas/universal-search` with `cmd/usearch` CLI (prints semver via `--version`), `internal/` domain stubs, `pkg/` public interfaces
  - Python `uv` workspace with three services (`researcher`, `storm`, `embedder`), each with `pyproject.toml`, `Dockerfile`, test skeleton
  - Next.js 16 web scaffold under `web/` with Tailwind, shadcn/ui config, ESLint + Prettier
  - `deploy/docker-compose.yml` with six pinned services (Qdrant v1.16.3, Meilisearch v1.42.1, PostgreSQL 16.13-alpine3.23, SearXNG, LiteLLM v1.83.7-stable.patch.1, Redis 7-alpine), all healthchecked, `${VAR}` env interpolation, named volumes
  - GitHub Actions CI matrix (`go.yml`, `python.yml`, `web.yml`, `compose-check.yml`, `pre-commit.yml`) on Node 22 LTS with all actions pinned
  - `.pre-commit-config.yaml` (gofmt, goimports, ruff, prettier, eslint, trailing-whitespace, end-of-file-fixer, hadolint, shellcheck, yamllint)
  - `Makefile` (dev, test, lint, build, clean, compose-up/down, fmt, tidy, install-py), `.editorconfig`, `LICENSE` (Apache-2.0), `NOTICE`, `README.md`

### Changed

- **SPEC-BOOT-001** — toolchain and scaffold polish (post-implementation sync)
  - Go toolchain version aligned with reality: SPEC + tech.md + README updated from Go 1.23 to Go 1.25 (matches `go.mod` since bootstrap; CI workflows already pin via `go-version-file: go.mod`)
  - CI workflow filenames renamed to match SPEC §6.3: `go-ci.yml` → `go.yml`, `python-ci.yml` → `python.yml`, `web-ci.yml` → `web.yml`
  - `.pre-commit-config.yaml` adds local `eslint` hook delegating to `pnpm -C web exec eslint` (REQ-BOOT-008)
  - `web/src/components/` and `web/src/lib/` placeholders added under existing src layout (REQ-BOOT-003)
  - SPEC-BOOT-001 frontmatter status flipped `approved` → `implemented` with `implemented_at: 2026-04-28`
- **SPEC-DEP-001** — Dependency pinning policy and audit CI
  - `docs/dependencies.md` manifest with Go pinning policy, future-dependencies placeholder table (chi → SPEC-IR-001, client_golang → SPEC-OBS-001, asynq → SPEC-LLM-001, pgx → SPEC-DB-001, qdrant/go-client → SPEC-VECTOR-001), compose service table, license allowlist
  - `.github/workflows/deps-audit.yml` running `govulncheck`, `pip-audit` (per-service matrix), `pnpm audit`, `hadolint`, license scan with allowlist enforcement, and SearXNG digest regression check on every PR and weekly cron
  - `.github/workflows/pre-commit-autoupdate.yml` weekly cron (Monday 06:00 UTC) opening automated PR
  - `renovate.json` with `prConcurrentLimit: 5`, minor/patch grouping, `.moai/**` ignored, docker digest updates disabled (manual SPEC-gated)
  - `scripts/gen-deps-manifest.sh` idempotent manifest generator
  - `scripts/check-license-allowlist.sh` enforcing MIT / Apache-2.0 / BSD-\* / ISC / PostgreSQL / MPL-2.0 with SearXNG AGPL service-boundary exception, supporting `$LICENSE_DIR` override for tests
  - `tests/spec_dep_001_test.go` — 11 TDD acceptance tests covering REQ-DEP-001..007
- **SPEC-OBS-001** — M1 observability baseline (slog + Prometheus + OTel + request-ID)
  - `internal/obs/` — central `Obs` bundle (Logger, Metrics, Tracer) with idempotent `Init()` lifecycle and graceful shutdown
  - `internal/obs/log/` — slog JSON handler with level from env; structured key=value logs
  - `internal/obs/metrics/` — Prometheus registry with `usearch_adapter_calls_total{adapter,outcome}` counter, `usearch_adapter_call_duration_seconds{adapter}` histogram, HTTP request metrics, LLM cost/latency families; static cardinality allowlist enforced by `TestNoUnboundedLabels` (NFR-OBS-002); admin HTTP server on `:6090` (configurable)
  - `internal/obs/trace/` — OpenTelemetry TracerProvider with gRPC OTLP exporter, configurable endpoint, 10% default sample ratio, no-op fallback when endpoint unset
  - `internal/obs/reqid/` — ULID-based request-ID generation, X-Request-ID HTTP middleware (ingress) and Transport wrapper (egress), context propagation
  - 18 @MX tags across 5 source files; coverage: obs 86.5% / log 89.6% / metrics 89.7% / reqid 95.2% / trace 90.5%
  - Merged in PR #3 (commit 0234b71)
- **SPEC-LLM-001** — M1 LiteLLM proxy integration (provider routing + cost tracking + circuit breaker)
  - `deploy/litellm/` — LiteLLM proxy v1.83.7 docker-compose service with model aliases (claude-opus-4-5, claude-sonnet-4-6, gpt-4o, ollama), per-key budgets via `LITELLM_BUDGET_USD`, Postgres + Redis backing, `/health` endpoint
  - `internal/llm/client.go` — openai-go v0.x client with Bearer auth, observability emission (counter + histogram + span + slog per call), cost-header extraction, streaming support
  - `internal/llm/router.go` — provider priority router with sync.RWMutex, per-provider fallthrough on transient errors, capacity-aware routing
  - `internal/llm/retry.go` — typed-error classification (transient/permanent/timeout), exponential backoff with jitter
  - `internal/llm/cost.go` — Anthropic prompt-cache hit detection, cumulative cost tracking, per-call cost histogram emission
  - `internal/llm/stream.go` — SSE streaming with backpressure handling and circuit-breaker integration
  - `internal/llm/config/` — koanf-layered config (TOML + env + flag), validation, hot-reload guard
  - 18 @MX tags across 7 source files; coverage: llm 89.9% / config 94.7%
  - Merged in PR #4 (commit 5005eb0); depends on SPEC-OBS-001 (Obs bundle DI)
- **SPEC-CORE-001** — Adapter contract foundation (NormalizedDoc + Registry)
  - `pkg/types/` — public SDK boundary with stdlib-only imports: NormalizedDoc (15 fields, Validate, CanonicalHash), Adapter interface (Name/Search/Healthcheck/Capabilities), Query, Capabilities, four-sentinel error taxonomy (`ErrTransient`, `ErrPermanent`, `ErrRateLimited`, `ErrSourceUnavailable`), `*ValidationError` and `*SourceError` typed errors, `CategorizeError` and `OutcomeFromError` helpers
  - `internal/adapters/registry.go` — concurrency-safe Registry with sync.RWMutex; `Register(Adapter, RegisterOptions)` rejects duplicates with typed `*RegistryError` wrapping `ErrDuplicateAdapter`; `Get(name)` returns wrappedAdapter that emits exactly one Prometheus counter increment + one histogram observation + one OTel span + one slog record per Search call (reuses existing AdapterCalls / AdapterCallDuration from SPEC-OBS-001 — zero new metric families)
  - `internal/adapters/noop/` — 46-LoC reference adapter as compile-time interface check and stable test fixture for FAN-001/IR-001
  - Race-clean Registry concurrency tests; benchmark gates: `Validate` 2 ns/op (target 1 µs), `CanonicalHash` 182 ns/op (target 5 µs)
  - Unblocks SPEC-IR-001, SPEC-ADP-001..009, SPEC-FAN-001, SPEC-IDX-001 (12 downstream SPECs)
- **SPEC-IR-001** — M2 Intent Router v0 (rule-based + LLM fallback, library-only)
  - `internal/router/` — pure library Router classifying free-text Query into RoutingDecision{Category, Confidence, AdapterSet, Lang, Source, Metadata}
  - Six categories: web, social, academic, korean, mixed, unknown (Unknown is recoverable via web ∪ social ensemble fallback)
  - Pre-flight validation → deterministic confidence-scoring formula (spec.md §2.3) over four signals (hangul_ratio, particle_density, kwd_density_C, has_english_token) → six per-category aggregators with fixed tie-break order (academic > korean > social > mixed > web > unknown)
  - LLM escalation when confidence < τ_high=0.85 via `internal/llm.Client.Classify` (Haiku 4.5 default per `provider.go:34-38`); 2-second deadline; circuit-breaker-aware degraded fallback
  - Korean detection via Hangul Unicode regex (4 blocks: U+AC00–D7A3, U+1100–11FF, U+3130–318F, U+A960–A97F); thresholds 0.10/0.30 with LLM in the ambiguous band; 11 Korean particle function-words for additional signal
  - AdapterSet selection: Category-eligible DocTypes ∩ Lang-compatible adapters (Capabilities-driven); web fallback when intersection empty
  - `internal/obs/metrics/router.go` — two new Prometheus families (`usearch_router_classifications_total{adapter,outcome}`, `usearch_router_classification_duration_seconds{adapter}`); cardinality allowlist UNCHANGED
  - 67 tests + 2 benchmarks; coverage 90.6% router / 90.8% metrics; race-clean (TestClassifyConcurrent: 50 goroutines × 20 calls); BenchmarkRulesScore 2.5 µs/op (~400× under NFR-IR-001 1 ms p50 target)
  - 12 @MX tags applied; independent plan-auditor review-1 FAIL → review-2 PASS (4 non-blocking findings)
  - Unblocks SPEC-FAN-001, SPEC-CLI-001, SPEC-SYN-001, SPEC-ADP-001, SPEC-ADP-002
- **SPEC-ADP-001** — M2 Reddit reference adapter (public `.json`, NSFW filter, Retry-After + redirect allowlist + circuit-breaker)
  - `internal/adapters/reddit/` — first real adapter consuming the SPEC-CORE-001 contract end-to-end (12 source files: `reddit.go`, `search.go`, `client.go`, `parse.go`, `score.go`, `errors.go`, plus tests + bench)
  - `parseListing` — JSON Listing → `[]NormalizedDoc` with `kind=="t3"` filter, pagination cursor on the last doc's `Metadata["next_cursor"]`, NSFW guard
  - `categorizeStatus` — HTTP 429 / 4xx / 5xx / network → `*types.SourceError` Category mapping (RateLimited / Permanent / Unavailable / Unknown)
  - `parseRetryAfter` — RFC 7231 §7.1.3 (integer-seconds tried first, then HTTP-date), 60s cap, 5s default when missing or malformed
  - `redirectAllowlist` — 4-host SSRF guard (`www.reddit.com`, `old.reddit.com`, `new.reddit.com`, `reddit.com`); maximum 3 redirect hops per REQ-ADP-010; cross-domain rejection mapped to `CategoryPermanent` via `isCrossDomainRedirectErr`
  - `normalizeScore` — `clamp(0.5 + 0.5*tanh(score/100), 0, 1)`; semantic center at score=0 (0.5), inflection at score=100 (~0.88)
  - 55 tests + 1 benchmark, 92.4% coverage, race-clean (`TestSearchConcurrentSafe`: 50 goroutines × 1 stub server)
  - Performance: parse p50 = 0.115 ms (NFR-ADP-001 ≤ 5 ms PASS); allocs/op = 460 (NFR-ADP-001 revised target ≤ 500 PASS, raised from ≤ 250 in run-phase iteration 3 after empirical baseline showed `pkg/types.NormalizedDoc.Metadata = map[string]any` forces a structural floor of ~17 allocs/doc)
  - Goroutine-leak guard via `goleak.VerifyTestMain` in `bench_test.go` (NFR-ADP-003)
  - 9 @MX tags (2 ANCHOR + 1 WARN + 6 NOTE) — see `.moai/reports/mx-validation/SPEC-ADP-001-validation.md`
  - Public no-auth `https://www.reddit.com/search.json` endpoint (D1 user-locked decision); OAuth (`oauth.reddit.com`) deferred to a future ADP-001a SPEC if measured value warrants
  - Adapter is stateless: no internal retry, no per-instance state mutation, no circuit; the fanout layer (SPEC-FAN-001, M3) owns retry orchestration — division-of-labor matches SPEC-CORE-001 §6.3
  - Implemented in commit 41372d4 (TDD); refactored in e3d1f7d (parseListing slice pre-size, allocs/op 465 → 460)
  - Unblocks SPEC-ADP-002, SPEC-FAN-001, SPEC-CLI-001, SPEC-SYN-001

### Changed

- **SearXNG image** pinned from `searxng/searxng:latest` to `searxng/searxng:2026.04.22-74f1ca203` (digest `sha256:37c616a774b90fb5df9239eb143f1b11866ddf7b830cd1ebcca6ba11b38cc2bf`, captured 2026-04-24 via Docker Hub API) per REQ-DEP-005
- **NOTICE** updated to point at `docs/dependencies.md` as the authoritative manifest

[Unreleased]: https://github.com/elymas/universal-search/commits/main
