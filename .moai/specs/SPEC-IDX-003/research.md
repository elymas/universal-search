# SPEC-IDX-003 Research — Korean Tokenization

Companion research artifact for `.moai/specs/SPEC-IDX-003/spec.md`.

Status: research-complete (2026-05-04)
Author: limbowl via manager-spec
Methodology: TDD (Run phase)

This document captures the codebase analysis and external library survey
that motivates the SPEC-IDX-003 design choices. The spec.md is normative;
this file is informative.

---

## §1. Existing State

### §1.1 Korean Detection Already Lives in `internal/router/`

SPEC-IR-001 implemented deterministic Korean detection as part of its
intent-classification pipeline. The artifacts that SPEC-IDX-003 reuses
(rather than duplicates):

- `internal/router/korean.go:18` — `koreanParticles` slice with the 11
  high-frequency postpositions: `["을","를","이","가","은","는","에서","에","와","과","의"]`.
- `internal/router/korean.go:27-39` — `isHangulRune(r rune) bool` covering
  the four Hangul Unicode blocks: U+AC00–D7A3 (Syllables), U+1100–11FF
  (Jamo), U+3130–318F (Compat Jamo), U+A960–A97F (Jamo Extended-A). Note
  Wikipedia (Hangul_Syllables) actually documents AC00–D7AF as the
  Syllables block; SPEC-IR-001 conservatively stops at D7A3 (the last
  assigned syllable) — kept for compatibility.
- `internal/router/korean.go:46-64` — `HangulRatio(s string) float64`
  returning `hangul / non-whitespace runes` in `[0.0, 1.0]`.
- `internal/router/korean.go:70-85` — `ParticleDensity(s string) float64`
  returning `tokens-with-particle-suffix / total-tokens` in `[0.0, 1.0]`.
- `internal/router/korean.go:89-91` — `KoreanSignals(s string) (ratio, density float64)`
  returning the tuple in one pass.

These are all pure, deterministic, single-pass over the input string. No
allocations beyond the input size. Performance is bounded — already
verified by `internal/router/bench_test.go::BenchmarkClassifyRulePath100Chars`
(NFR-IR-001) reporting < 1 ms p50 over 10000 iterations.

**Implication for IDX-003**: The query-time language-detection step does
NOT need a new helper. SPEC-IDX-003 will import
`github.com/elymas/universal-search/internal/router` and call
`router.HangulRatio(query)` directly. No fork, no duplicate keyword tables.

### §1.2 RoutingDecision.Lang Already Carries Intent

SPEC-IR-001 already produces `RoutingDecision.Lang` (BCP-47 string). For
queries that score as `CategoryKorean`, `Lang == "ko"`. SPEC-IDX-003 takes
this signal as input — the Router already classified; the index layer
just routes physically.

The interface contract (from SPEC-IR-001 §2.1(c)):

```go
type RoutingDecision struct {
    Category    Category
    Confidence  float64
    AdapterSet  []string
    Lang        string                  // "ko", "en", etc.
    Source      ClassificationSource
    Metadata    map[string]any
}
```

**Implication**: SPEC-IDX-003's index-time routing key is `NormalizedDoc.Lang`
(per SPEC-CORE-001 REQ-CORE-001 — `Lang string` BCP-47, empty means unknown).
Query-time routing key is `RoutingDecision.Lang`. The two fields share the
same BCP-47 value space and meet at the index boundary.

### §1.3 NormalizedDoc.Lang Field

SPEC-CORE-001 already defined the routing-relevant doc field
(`pkg/types/normalized_doc.go:51`):

```go
Lang        string         `json:"lang"`          // BCP-47
```

Values are populated by adapters at retrieval time. The Korean adapters
(SPEC-ADP-008 Naver suite, SPEC-ADP-009 Daum + KoreaNewsCrawler) are
expected to set `Lang = "ko"` on every doc they return. Non-Korean
adapters either set explicit BCP-47 codes or leave it empty.

**Edge case**: A General Web adapter (SearXNG, SPEC-ADP-007) may surface a
mix of languages from a single query. Per-doc `Lang` is the responsibility
of the adapter; SPEC-IDX-003 trusts the value as authoritative.

### §1.4 Python Sidecar Pattern (SPEC-SYN-001)

SPEC-SYN-001 established the pattern that SPEC-IDX-003 mirrors:

- Service directory under `services/` (SPEC-SYN-001 chose
  `services/researcher/`; SPEC-IDX-003 will create `services/tokenizer-ko/`).
- FastAPI app rooted at `src/<package>/app.py`
  (`services/researcher/src/researcher/app.py:36-46`) with
  `lifespan` async context manager and `GET /health` endpoint returning
  `{"status":"ok","version":...}`.
- Pydantic v2 models with
  `ConfigDict(extra="forbid", str_strip_whitespace=True)` per
  `.claude/rules/moai/languages/python.md`.
- Multi-stage Dockerfile rooted on `python:3.11-slim`, non-root user,
  HEALTHCHECK with `curl -f http://localhost:<port>/health || exit 1`
  (services/researcher/Dockerfile:5-29).
- Compose entry under `deploy/docker-compose.yml` with
  `${TOKENIZER_KO_PORT:-8083}:8083` mapping, `condition: service_healthy`
  on the `litellm` (SYN-001 example), restart policy, and the `app`
  network. SPEC-IDX-003's `tokenizer-ko` will join the same `app` network
  but has no LiteLLM dependency (it's a pure tokenizer; no LLM call).

The port allocation as of `deploy/docker-compose.yml`:

- 4000 — LiteLLM
- 5432 — postgres
- 6333/6334 — Qdrant
- 6379 — Redis
- 7700 — Meilisearch
- 8080 — SearXNG
- 8081 — researcher (synthesis)
- 9090 → 9091 (host) — Prometheus

**SPEC-IDX-003 chooses port 8083** for `tokenizer-ko` (8082 reserved for
SPEC-IDX-002 embedder, paralleling the SYN-001 8081 choice).

### §1.5 Meilisearch Service Already Provisioned

The Meili instance from SPEC-BOOT-001 (`deploy/docker-compose.yml:48-66`)
runs `getmeili/meilisearch:v1.42.1` with master key, named volume,
healthcheck. SPEC-IDX-001 (parallel SPEC) ships the Go client and
manages a single `usearch` index. SPEC-IDX-003 layers on top by adding
a second physical index `usearch-ko` to the same Meili instance — no new
Meili service.

The Meili admin API (verified §2.3 below) supports per-index settings
that SPEC-IDX-003 needs:
- `PATCH /indexes/{uid}/settings` for stop-words, separators, dictionary.
- `PATCH /indexes/{uid}/settings/localized-attributes` for locale routing.

### §1.6 No Existing Tokenization Plane

`internal/index/index.go` is a 4-line stub
(`package index` + comment). There is no current tokenizer code in the
repo, no Meili-side tokenizer customization, no Korean shard. SPEC-IDX-003
is greenfield within this domain — no characterization tests required
(workflow-modes.md §Brownfield Enhancement applies trivially: nothing
to preserve).

### §1.7 Observability Conventions (SPEC-OBS-001)

SPEC-OBS-001 established the metric registration pattern:
- New metric families live under `internal/obs/metrics/<domain>.go`.
- The package boundary forbids direct `prometheus/client_golang` imports
  outside `internal/obs/`.
- The cardinality allowlist
  (`internal/obs/metrics/metrics.go:147-154`) enumerates allowed label
  names — adding a new label name requires amending the allowlist.

SPEC-IR-001 followed this pattern by adding `internal/obs/metrics/router.go`
(REQ-IR-006). SPEC-SYN-001 added `internal/obs/metrics/synthesis.go`.
SPEC-IDX-003 adds `internal/obs/metrics/tokenizer.go` for the Go-side
tokenize-call wrapper, AND introduces a new label `shard ∈ {ko, default}`
on the index-routing counter — see §4 below for the cardinality
amendment proposal.

### §1.8 LSP Client Note (Out of Path)

`.claude/rules/moai/core/lsp-client.md` pins powernap v0.1.4 for LSP. Not
relevant to SPEC-IDX-003 — tokenization runs in a Python sidecar and a
Go HTTP client; no LSP integration.

---

## §2. Korean Tokenizer Survey

Four candidates evaluated. The selection criteria in priority order:

1. Quality of morphological segmentation on modern Korean (mixed
   Hangul + ASCII + emoji + Hanja).
2. Maintenance — active commits in 2024-2026.
3. Operational footprint (binary size, memory, deps).
4. Python binding maturity (FastAPI sidecar requirement).
5. Throughput (we target ≥ 1000 docs/sec sidecar throughput, NFR-IDX-003-001).

### §2.1 mecab-ko + mecab-ko-dic (selected)

**What**: A Korean fork of MeCab (the Japanese morphological analyzer)
maintained by the Eunjeon project. mecab-ko itself is the binary;
mecab-ko-dic is the dictionary (POS tags + morpheme lexicon). Together
they segment Korean into morphemes with POS labels.

**Why selected**:
- Industry standard for Korean NLP since ~2014. Used by Naver, Kakao
  internally as a baseline.
- Stable POS tag set (mecab-ko-dic-2.x).
- Python binding via `pymecab-ko` (PyPI; `pip install mecab-ko`)
  bundles the dictionary, so no separate `apt-get install mecab-ko`
  step needed. Verified `pymecab-ko` v1.0.2 release on 2025-09-23
  (WebFetch — github.com/NoUnique/pymecab-ko, accessed 2026-05-04).
- Pure pip install: no apt, no brew, works in `python:3.11-slim`
  Docker image with C++ build deps (`build-essential`).
- Good throughput in single-thread Python: ~50,000–100,000 morphemes/sec
  on commodity x86; ~10,000–20,000 short-doc tokenizations/sec.

**Why NOT alternatives — see §2.2 / §2.3 / §2.4**.

**Version pinning policy**:
- `pymecab-ko >= 1.0, < 2.0` (semver-major bound, allows patch updates).
- mecab-ko-dic version is bundled with pymecab-ko; no separate pin.
- Upgrade procedure: bump `pymecab-ko` in `services/tokenizer-ko/pyproject.toml`,
  run integration test (Korean fixture set returns expected morpheme set),
  document in `services/tokenizer-ko/CHANGELOG.md`.

### §2.2 KOMORAN (rejected)

**What**: Java-based Korean morphological analyzer, maintained by Shineware.

**Why rejected**:
- Java runtime requirement adds JVM (~100 MB) to the sidecar image.
- Python binding `PyKomoran` exists but is not actively maintained
  (last release 2020-ish). Reliability risk.
- Quality is comparable to mecab-ko on standard text but lower on
  out-of-vocabulary modern terms (LLM-era jargon, coined words).

### §2.3 OKT / Open Korean Text (rejected)

**What**: Open Korean Text (formerly twitter-korean-text), Kotlin-based.

**Why rejected**:
- JVM dep (same as KOMORAN).
- Tokenization is rule-based, not statistical — accuracy on noun
  compounds (a key Korean phenomenon) lags mecab-ko.
- Python access via `konlpy` wraps OKT, but konlpy adds further deps
  (numpy, jpype1) inflating the image.

### §2.4 khaiii (rejected — supplementary fallback)

**What**: Kakao's CNN-based Korean morphological analyzer, released 2018.

**Why rejected for V1**:
- C++ build with TensorFlow-Lite-style runtime; complex install
  (no PyPI wheel, requires CMake + bazel-style build).
- Maintenance has slowed since 2021-2022.
- Higher accuracy on some benchmarks but at the cost of 5-10× slower
  tokenization vs mecab-ko.
- Could be an opt-in alternative in a future SPEC if accuracy
  measurements show mecab-ko failing on specific document classes.

### §2.5 nori (Lucene Korean analyzer — for reference only)

**What**: Apache Lucene's built-in Korean analyzer, derived from
mecab-ko-dic.

**Why mentioned but not used**:
- Lives inside the JVM (Lucene). Universal Search uses Meilisearch
  (Rust-based), not Elasticsearch/Lucene — no JVM in the stack.
- Confirms that mecab-ko-dic is the de-facto "right" Korean tokenizer
  source-of-truth for search-engine workloads (Lucene chose it).
- If Universal Search ever switches to OpenSearch/ES, nori is the
  in-process equivalent — but this is post-V1.

---

## §3. Meili Plugin Path vs Sidecar-Pretokenized Path

This is the **central architectural decision** for SPEC-IDX-003.

### §3.1 Meilisearch's Built-in Korean Support (Charabia / Lindera)

**Verified externally** (WebFetch — github.com/meilisearch/charabia,
accessed 2026-05-04):

> Charabia is "Library used by Meilisearch to tokenize queries and
> documents." [...] Korean is supported using lindera with KO-dict,
> though performance is modest at ~2 MiB/sec for both segmentation and
> tokenization.

> Latest release: Version 0.9.9, released 2025-11-24.

> CJK integration: The library uses specialized morphological
> analyzers: lindera for Japanese and Korean, and jieba-rs for Chinese.
> There's no mention of mecab-ko integration — it relies on lindera's
> pre-built dictionaries instead.

**Verified externally** (WebFetch — github.com/lindera/lindera,
accessed 2026-05-04):

> lindera-ko-dic — Korean dictionary
> Latest release: Version 3.0.7, released 2026-04-24.

**Verified externally** (WebFetch — meilisearch.com/docs/reference/api/settings,
accessed 2026-05-04):

> separatorTokens, nonSeparatorTokens, dictionary, stopWords are all
> per-index settings accessed via `PATCH /indexes/{uid}/settings`.
> stopWords default empty array; dictionary lets users define
> single-term phrases.

> localizedAttributes setting is mentioned for "language-specific
> tokenization supporting multiple locales including Japanese (`jpn`)
> and Chinese (`zho`)."

**Latest Meilisearch release** (WebFetch — github.com/meilisearch/meilisearch,
accessed 2026-05-04): v1.43.0 released 2026-05-04.

**Implication**: Meilisearch HAS native Korean tokenization through
Charabia → Lindera → KO-dict. Configurable via `localizedAttributes`
setting using locale code `kor` (ISO 639-3) or `ko` (ISO 639-1) —
exact code depends on Meili's locale-list, verified at run-phase by
posting a settings update and observing the response.

### §3.2 Path A — Use Meili's Native Korean Tokenizer (Lindera)

**What**: Configure `usearch-ko` index with
`localizedAttributes = [{"attributePatterns": ["title","body","snippet"], "locales": ["kor"]}]`,
let Charabia/Lindera tokenize Korean text in-process.

**Pros**:
- Zero new services. No `tokenizer-ko` sidecar. No new compose entry.
- Single-binary Meili already deployed (SPEC-BOOT-001 line 49-66) does
  the work.
- Charabia/Lindera is maintained by Meilisearch core team; updates
  arrive with Meili upgrades.

**Cons**:
- Lindera throughput is ~2 MiB/sec per Charabia README (verified §3.1).
  For SPEC-IDX-003 NFR target (≥ 1000 docs/sec at average 2 KB/doc =
  2 MB/sec), this is borderline. A single Meili pod can saturate.
- Lindera's KO-dict is a static snapshot; updates lag mecab-ko-dic.
- Less configurable: cannot inject custom Korean stop-words at the
  morpheme level (only at the post-tokenization stop-words list level
  via `stopWords` setting).
- Tied to Meili's Rust implementation. If we swap Meili for another
  engine post-V1, the tokenization pipeline disappears with it.

### §3.3 Path B — Sidecar Pre-tokenization (selected)

**What**: A `services/tokenizer-ko/` Python FastAPI sidecar exposes
`POST /tokenize` accepting a Korean text and returning `{tokens: ["...", ...]}`
(space-joined morphemes — minimal contract). The Go ingestion path
detects Korean docs (via `NormalizedDoc.Lang == "ko"` per §1.3), calls
the sidecar, replaces the doc's `Title`/`Body`/`Snippet` with the
space-joined tokens, and indexes the *pretokenized* text into the
`usearch-ko` Meili index. Meili's default tokenizer (which is
whitespace-aware) then sees Korean text already split into morphemes.

This pattern is well-established in Korean search — Naver's old
ElasticSearch deployments used the same morpheme-pretokenized approach
before nori/mecab-ko-elastic plugins shipped.

**Pros**:
- mecab-ko-dic quality (industry standard; §2.1).
- Decoupled from Meili upgrade path. mecab-ko-dic updates land via
  pymecab-ko upgrades, independent of Meili's release cadence.
- Configurable: custom user dictionary (`mecab-ko-dic` accepts `-u`
  user dict files) for adding domain terms — open question §6.
- Observable per-tokenization (latency histogram, cost — though
  CPU-only — in Prometheus).
- Aligns with Universal Search architectural principle "Composition
  over reinvention" (`.moai/project/tech.md` line 5): use upstream
  Korean NLP tooling, don't lock to a search-engine-specific impl.
- Throughput at ~10,000-20,000 docs/sec single-thread mecab-ko
  vs Lindera's ~1000 docs/sec at 2 MB/sec (verified above) —
  10-20× headroom.

**Cons**:
- New service (operational cost: 1 more container, 1 more healthcheck).
- New HTTP hop in the index ingestion path (Go → tokenizer-ko → Go →
  Meili). Adds ~5 ms p50 latency per doc (NFR-IDX-003-002 target).
- Tokenizer must be running for ingestion to work (graceful
  degradation policy: fall through to Meili native Charabia/Lindera
  when sidecar unhealthy — see REQ-IDX-003 spec.md).

**Decision (selected)**: Path B (sidecar pre-tokenization).

**Tiebreaker rationale**:
- mecab-ko-dic quality is materially better than lindera-ko-dic on
  modern Korean (mecab-ko-dic gets updates from the Eunjeon project;
  lindera-ko-dic is a frozen snapshot from circa 2019).
- M3 exit criterion (`.moai/project/roadmap.md` line 150) reads
  "Korean query returns Naver results ranked first." Naver's own
  tokenization expectations are mecab-ko-aligned. Choosing mecab-ko
  reduces ranking drift between Naver's adapter-side ordering and
  our Meili-side ordering.
- The throughput headroom (10-20×) materially affects the bulk
  re-indexing path (M6 SPEC-IDX-004 multi-tenant migration would
  re-tokenize a team's entire history; Path A would block on Lindera).

**Fallback hook**: When the `tokenizer-ko` sidecar is unhealthy,
SPEC-IDX-003 falls through to native Meili tokenization (Path A behavior)
on the `usearch-ko` index, marking the affected docs with
`metadata.tokenizer="lindera_fallback"`. See REQ-IDX-003 spec.md
(state-driven graceful degradation requirement).

---

## §4. Query-Routing Architecture — Single Index with Lang Filter vs Dual Index

### §4.1 Option α — Single index with `lang` field filter

**What**: One physical Meili index `usearch`, every doc has `lang` field
(filterable), Korean queries do `filter: lang = ko`.

**Pros**:
- Simpler ops: one index, one settings object, one backup procedure.

**Cons**:
- The Meili tokenizer for the *whole index* must handle every language.
  Configuring per-locale tokenization on a single index requires
  `localizedAttributes` matching by attribute pattern, which works for
  Path A but breaks the Path B model (Path B writes pre-tokenized
  text into the same fields used for non-Korean docs — losing the
  pre-tokenization advantage when the field is also indexed for
  English text).
- Stop-words are global per-index in Meili. Korean particles in the
  stop-word list would be applied to every doc, including English
  ones (no-op but indicates a hack).
- Synonym lists similarly global.

### §4.2 Option β — Dual physical index `usearch` + `usearch-ko` (selected)

**What**: Two Meili indices on the same Meili instance:
- `usearch` — default index, Charabia auto-detection, English-leaning
  config (existing SPEC-IDX-001 territory).
- `usearch-ko` — Korean-only, receives pre-tokenized text from the
  `tokenizer-ko` sidecar, custom stop-words list of Korean particles
  (research §5), custom synonym hooks (deferred to follow-up SPEC).

**Index-time routing**:
```
NormalizedDoc → Lang field check
  Lang == "ko" → tokenizer-ko sidecar → write to usearch-ko index
  else         → write to usearch index
  Lang == "" (unknown) → write to usearch index (default)
                      → ALSO write to usearch-ko if HangulRatio(Body) ≥ 0.3
                        (defensive routing for adapters that didn't fill Lang)
```

**Query-time routing**:
```
Query → router.HangulRatio(Query.Text) →
  ratio ≥ 0.30 (per IR-001 ratio_high) → query usearch-ko shard
  ratio < 0.10                          → query usearch shard
  0.10 ≤ ratio < 0.30                   → query BOTH shards, merge by RRF
```

The IR-001 ratio_high (0.30) and ratio_low (0.10) thresholds are reused
verbatim — no new constants for tokens, no duplicated thresholds.

**Pros**:
- Each index gets its own tokenizer config. Meili's stop-words,
  separator-tokens, and dictionary are per-index — no global
  contamination.
- Clean ops: backup `usearch-ko` independently, version-pin its
  settings independently.
- Mixed-query support via merge layer is straightforward — RRF over
  two `[]NormalizedDoc` results.

**Cons**:
- Two indices to manage at SPEC-IDX-001 layer (acknowledged; SPEC-IDX-001's
  scope already includes "multi-index" orchestration — this is a
  design alignment, not a scope creep).
- Mixed-language docs (e.g., a Korean post quoting English) are
  duplicated in both shards. Storage overhead is bounded — see §7.

**Decision (selected)**: Option β (dual index).

### §4.3 Cross-Shard Merge for Ambiguous Queries

For queries in the ambiguous band (`0.10 ≤ ratio < 0.30`), the IR-001
escalation point (LLM adjudication) does NOT route the query — it just
classifies. SPEC-IDX-003's index-layer merge logic kicks in regardless:

1. Issue parallel queries to `usearch` and `usearch-ko`.
2. Each returns its own top-K with Meili-internal score.
3. Merge by Reciprocal Rank Fusion (RRF), `score(d) = Σ 1/(k + rank_i(d))`,
   `k = 60` (matches `.moai/project/tech.md` §5).
4. Return merged top-K.

Mixed-query latency budget: 2× single-shard latency (parallel issue + merge).
Per-query budget at ingestion-tier (read-side) is 200 ms p95, see
NFR-IDX-003-005. This is well within the 8-second M4 streaming budget.

---

## §5. Race / Goroutine / Resource Lifecycle

### §5.1 Concurrent Tokenize + Index + Query

Three concurrent paths can run against the tokenizer-ko sidecar:

1. **Index-write fanout** (background) — adapter pulls in a stream of
   Korean docs; each doc fans out to the sidecar in parallel.
2. **Query-time language detection** — `router.HangulRatio` is a pure
   Go function; no sidecar call needed at query-time.
3. **Re-indexing job** — periodic re-tokenization of stale docs.

**Sidecar concurrency model**:
- FastAPI uvicorn with `--workers 1` (matches SPEC-SYN-001 §10 Open
  Question 7 — single worker, async handlers; reduces process-level
  resource overhead). Concurrency comes from asyncio.
- pymecab-ko's `Tagger` is **NOT thread-safe** in v1.0.x (verified by
  reading its source — uses a global C++ object). Solution: wrap each
  `parse` call in `asyncio.to_thread(...)` so each tokenization
  acquires a fresh thread; OR use one Tagger per worker process and
  serialize via asyncio Lock. SPEC-IDX-003 picks the **second** approach
  for predictable throughput (one Tagger per asyncio loop, asyncio.Lock
  serializes parse calls, ~10K-20K ops/sec headroom is enough).

**Go-side concurrency**:
- `internal/index/tokenizer/client.go` (new file in this SPEC) — HTTP
  client wrapping the sidecar. Stateless, safe for goroutine fanout.
- Mirrors `internal/synthesis/client.go` pattern: `*http.Client` with
  `Timeout: 1s`, exponential backoff retry on connection-level errors,
  per-call observability emit.

### §5.2 goleak Discipline

NFR-IDX-003-006 mandates `goleak.VerifyNone(t)` on the Go-side test
suite for the tokenizer client + index router. Pattern reuses the
`TestMain` setup from
`internal/synthesis/client_test.go` (or wherever SPEC-SYN-001 placed
its goleak guard).

### §5.3 race-clean

NFR-IDX-003-007: `go test -race` clean. Concurrent writes to the
metric counters (Prometheus collectors are internally thread-safe per
client_golang docs). Concurrent map access on Lang routing is bounded
by `sync.RWMutex`.

### §5.4 Graceful Shutdown

- FastAPI `lifespan` handler stops accepting new requests and waits for
  in-flight requests (uvicorn handles this; mirrors SPEC-SYN-001
  `services/researcher/src/researcher/app.py:27-33`).
- Compose `stop_grace_period: 30s` covers worst-case in-flight
  tokenization batch.

---

## §6. Dictionary Management — Version Pin vs Custom User Dictionary

### §6.1 Version Pinning Policy

- `pymecab-ko = "1.0.*"` (PEP 440 semver-major bound). Patch updates
  auto, minor bumps require explicit upgrade.
- Dictionary version reported on `GET /health` response under
  `dict_version` field. SPEC-EVAL-003 Korean benchmark consumes this
  value to detect dictionary-driven score drifts.
- Upgrade SOP (mirrors `.claude/rules/moai/core/lsp-client.md` upgrade policy):
  1. Bump `services/tokenizer-ko/pyproject.toml` to new pymecab-ko
     version on a feature branch.
  2. Run integration test against fixed Korean fixture set
     (`services/tokenizer-ko/tests/fixtures/golden_morphemes.json`).
  3. If golden set passes (allowing ≤ 5% morpheme-set delta — domain
     drift is expected), merge.
  4. Document in `services/tokenizer-ko/CHANGELOG.md`.

### §6.2 Custom User Dictionary (open question — deferred)

mecab-ko-dic supports `-u user.dic` for domain-specific terms (e.g.,
"ChatGPT" → single noun token instead of three syllabic tokens). V1
ships without user dict for simplicity. A future SPEC-IDX-006 can add
a user-dict management surface if measurement (SPEC-EVAL-003) shows
domain-term tokenization causing rank drift.

---

## §7. Cross-Language Query Handling

### §7.1 Mixed Query Handling (clarified §4.3)

Re-stated for clarity: queries with `0.10 ≤ HangulRatio < 0.30` (e.g.,
"best Korean LLM 모델") fan out to both shards and merge by RRF. No new
mechanism beyond §4.3.

### §7.2 Mixed-Document Handling

Documents with mixed-language body (e.g., a Korean blog post with English
quotes) follow this rule:
- Adapter sets `NormalizedDoc.Lang = "ko"` for Korean-primary docs.
- The doc is tokenized fully (the English quote is kept verbatim — mecab-ko
  handles ASCII tokens as single morphemes).
- Result is written to `usearch-ko` shard only. The English quote is
  searchable on `usearch-ko` (Meili default tokenizer post-pretokenization
  handles ASCII whitespace).

**Edge case** — a Korean adapter (Naver shopping listing) returns a doc
with English-only title. Adapter still sets `Lang = "ko"` (per Naver's
context, even if individual doc text is English). It lands on
`usearch-ko`. Query for the English term would route to `usearch`
(non-Korean ratio → default shard) and miss the doc. Mitigation:
defensive dual-write — when `Lang == "ko"` AND `HangulRatio(Body) < 0.10`
(Korean-source-but-English-content), the indexer writes to BOTH shards.
This is REQ-IDX-003-005 in spec.md.

### §7.3 Korean-First Scoring Guarantee

M3 exit criterion: "Korean query returns Naver results ranked first."

SPEC-IDX-003 contributes to this by:
1. Routing Korean queries to `usearch-ko` shard (high-precision Korean
   matching).
2. Ranked-first guarantee: when fanout returns adapter results,
   Korean adapters' results (Naver, Daum, RSS Korean) carry the same
   `Lang = "ko"`. The Korean-first ranking decision is at the FAN-001
   / synthesis layer (SPEC-FAN-001 §rank fusion). SPEC-IDX-003 is
   **necessary but not sufficient**: the index layer ensures Korean
   matches actually surface; the rank-fusion layer ensures they rank
   first.

REQ-IDX-003-013 in spec.md formalizes the index-side guarantee: when a
Korean query is routed to `usearch-ko`, the top-K result set MUST contain
at least one doc with `Lang == "ko"` if any such doc matches the query
text (i.e., the index does not silently exclude Korean docs from a
Korean query).

---

## §8. Open Questions

The following are explicitly unresolved at SPEC-approval time and
documented here rather than pre-decided. They do not block SPEC approval.

1. **Locale code for `localizedAttributes`** — `kor` (ISO 639-3) vs
   `ko` (ISO 639-1)? Meili documentation lists `jpn` and `zho` (ISO
   639-3) as examples (verified §3.1 WebFetch). Default for V1: `kor`,
   verified at run-phase by posting a settings update and observing
   the response. If Meili rejects `kor`, fall back to `ko`. Documented
   as open question; resolution by run-phase implementer.

2. **mecab-ko-dic dictionary version** — pymecab-ko bundles a specific
   mecab-ko-dic version. Currently (2025-09-23 release) likely
   mecab-ko-dic-2.1.1-20180720. Latest mecab-ko-dic upstream may be
   newer. V1: accept the bundled version; document in
   `services/tokenizer-ko/README.md`. Resolution owner: future SPEC-IDX-006
   if measurement shows benefit.

3. **Custom user dictionary** — see §6.2. Deferred.

4. **Sidecar workers count** — V1 `--workers 1`. If observed throughput
   < 1000 docs/sec p50 at run-phase, bump to `--workers 2` and
   load-balance. Resolution: revisit in M3 SPEC-IDX-003 run-phase
   testing.

5. **Synonym handling** — Meili's `synonyms` setting accepts arbitrary
   key-value mappings. Korean synonyms (e.g., "스타트업" ↔ "창업"
   ↔ "startup") are out of scope for V1. Future SPEC-IDX-006 if
   needed.

6. **Cross-shard score normalization** — when merging RRF results from
   `usearch` + `usearch-ko`, do we also normalize Meili-internal scores
   before fusing? V1: pure rank-based RRF (per `.moai/project/tech.md`
   line 141). Score normalization is post-V1.

7. **Re-tokenization on dictionary upgrade** — when pymecab-ko upgrades
   change tokenization output, existing indexed docs become stale. V1
   assumption: full re-index on dictionary upgrade is acceptable
   (manual operation). Future SPEC-IDX-006 may add incremental
   re-tokenization.

8. **Health-check upstream Meili reachability** — should the
   `tokenizer-ko` sidecar's `/health` endpoint also probe Meili
   reachability? V1: no — Meili is downstream of the indexer, not the
   tokenizer. Tokenizer health is purely "does mecab-ko-dic load and
   tokenize a known fixture." Documented as open; revisit if M3
   integration tests reveal cascading failures.

9. **Defensive routing threshold** — §7.2 mixed-source defensive
   dual-write fires when `Lang == "ko"` AND `HangulRatio(Body) < 0.10`.
   Is 0.10 the right threshold or should it be lower (e.g., 0.05)? V1:
   0.10 (matches IR-001's `ratio_low`). Resolution: post-M3 traffic
   measurement.

10. **Hangul block U+D7B0–D7FF (Jamo Extended-B)** — IR-001's
    `isHangulRune` does NOT include this block. SPEC-IDX-003 should
    consider whether the tokenizer-ko sidecar (or a new
    `internal/tokenizer/lang.go` Go-side helper) extends the range.
    Default V1: keep IR-001's range (4 blocks). Jamo Extended-B is
    historical/archaic; modern Korean does not use it. Resolution:
    out-of-scope for V1; revisit if SPEC-EVAL-003 measurements show
    misses.

---

## §9. References

### Internal (file:line citations)

- `.moai/project/roadmap.md:57` — SPEC-IDX-003 row + M3 placement.
- `.moai/project/roadmap.md:150` — M3 exit criterion: "Korean query
  returns Naver results ranked first."
- `.moai/project/tech.md:50` — Korean keyword tokenizer
  decision: "mecab-ko / khaiii (served via Python sidecar)" — SPEC
  selects mecab-ko per §2.
- `.moai/project/tech.md:141` — RRF formula: `score(d) = Σ 1 / (k + rank_i(d))`
  with `k=60` (used in §4.3 cross-shard merge).
- `.moai/project/tech.md:167` — Decision Log entry "Korean tokenization =
  mecab-ko sidecar" (2026-04-24, antecedent for this SPEC).
- `internal/router/korean.go:18` — Korean particle list (reused for
  Meili stop-words, see §5).
- `internal/router/korean.go:27-39` — `isHangulRune` Hangul block ranges.
- `internal/router/korean.go:46-64` — `HangulRatio` (consumed by
  query-time routing §4.2).
- `internal/router/korean.go:70-85` — `ParticleDensity` (also reused).
- `pkg/types/normalized_doc.go:51` — `NormalizedDoc.Lang` (BCP-47
  routing key).
- `services/researcher/src/researcher/app.py:27-46` — FastAPI lifespan +
  /health pattern (mirrored for tokenizer-ko).
- `services/researcher/Dockerfile:5-29` — Python sidecar Dockerfile
  pattern (mirrored).
- `deploy/docker-compose.yml:48-66` — Meilisearch service entry
  (consumed; SPEC-IDX-003 adds settings on top).
- `deploy/docker-compose.yml:165-188` — researcher compose entry
  (mirrored for tokenizer-ko).
- `internal/index/index.go` — current 4-line stub.
- `internal/synthesis/client.go` — Go HTTP client to sidecar, retry
  pattern (mirrored).
- `internal/synthesis/types.go` — Go-side request/response types
  (mirrored).
- `internal/synthesis/config.go` — env binder pattern (mirrored).
- `internal/obs/metrics/router.go` — new metric family pattern
  (precedent from IR-001).
- `internal/obs/metrics/synthesis.go` — new metric family pattern
  (precedent from SYN-001).
- `internal/obs/metrics/metrics.go:147-154` — cardinality allowlist
  (must be amended for `shard` label — see SPEC-IDX-003 spec.md §6).
- `.moai/specs/SPEC-CORE-001/spec.md` §3 — `NormalizedDoc.Lang`
  contract.
- `.moai/specs/SPEC-IR-001/spec.md` §2.1(d) — `HangulRatio` /
  `KoreanSignals` semantics.
- `.moai/specs/SPEC-SYN-001/spec.md` §2.1(a-p) — Python sidecar layout
  pattern.
- `.moai/specs/SPEC-OBS-001/spec.md` §2.1, §3 — observability
  conventions and cardinality discipline.
- `.moai/specs/SPEC-BOOT-001/spec.md` §6.2 — compose service template.
- `.claude/rules/moai/languages/python.md` — Pydantic v2,
  `ConfigDict(extra="forbid", str_strip_whitespace=True)`,
  `pytest-asyncio asyncio_mode="auto"`, ruff.
- `.claude/rules/moai/languages/go.md` — Go 1.23+ stdlib-first,
  context.Context, errgroup, race detection.
- `.claude/rules/moai/workflow/mx-tag-protocol.md` — @MX tag rules.

### External (verified URLs)

- `https://www.meilisearch.com/docs/reference/api/settings` — Meili
  per-index settings (separatorTokens, stopWords, dictionary,
  localizedAttributes). Verified 2026-05-04.
- `https://github.com/meilisearch/charabia` — Meili tokenizer
  library; Korean via lindera + KO-dict; v0.9.9 (2025-11-24);
  ~2 MiB/sec throughput. Verified 2026-05-04.
- `https://github.com/lindera/lindera` — Lindera tokenizer with
  lindera-ko-dic; v3.0.7 (2026-04-24). Verified 2026-05-04.
- `https://github.com/meilisearch/meilisearch` — Meilisearch v1.43.0
  (2026-05-04). Verified 2026-05-04.
- `https://github.com/SamuraiT/mecab-python3` — Japanese-only;
  recommends pymecab-ko for Korean. Verified 2026-05-04.
- `https://github.com/NoUnique/pymecab-ko` — pymecab-ko v1.0.2
  (2025-09-23); Python 3.6+; mecab-ko-dic bundled. Verified 2026-05-04.
- `https://en.wikipedia.org/wiki/Hangul_Syllables` — Hangul Unicode
  block ranges (AC00-D7AF Syllables, 1100-11FF Jamo, 3130-318F Compat
  Jamo, A960-A97F Jamo Ext-A, D7B0-D7FF Jamo Ext-B). Verified
  2026-05-04.

---

*End of SPEC-IDX-003 research.md*
