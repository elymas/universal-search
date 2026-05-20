# SPEC-SYN-003 Research — Dedup + Clustering (Pre-Synthesis)

Author: limbowl (manager-spec, Phase 0.5 Deep Research)
Date: 2026-05-09
Status: Research artifact for `.moai/specs/SPEC-SYN-003/spec.md` Phase 1B planning.

---

## 1. Roadmap anchor and SPEC slot

`.moai/project/roadmap.md:65` declares:

> SPEC-SYN-003 | Dedup + clustering | cluster near-duplicate results
> pre-synthesis (SimHash + embedding cosine) | expert-backend

M4 backlog second SPEC, between SPEC-SYN-002 (Citation faithfulness, just
approved) and SPEC-SYN-004 (Streaming response). Same M4 milestone as
SPEC-SYN-002 — they parallelize per `.moai/project/roadmap.md:124` (M4
3-way: SYN-002 + SYN-003 + SYN-004).

Pipeline coordinate: BETWEEN fanout (`internal/fanout`, SPEC-FAN-001) and
synthesis (`internal/synthesis` Go client + `services/researcher` Python
sidecar, SPEC-SYN-001).

---

## 2. Existing pipeline trace

### 2.1 Fanout output handoff

`cmd/usearch/query.go:228-258` is the fanout → synthesis seam:

```
Line 228: fanoutResult, _ := f.Dispatch(spanCtx, fanoutDecision, types.Query{Text: prompt})
Line 229: docs := fanoutResult.Docs                   // []types.NormalizedDoc
Line 230: adapterErrs := fanoutResult.AdapterErrors
...
Line 257: prog.Emit("synthesis", ...len(docs)...)
Line 258: synthResp, synthErr := synth.Synthesize(spanCtx, prompt, decision.Lang, docs)
```

The natural insertion point for SPEC-SYN-003 cluster/dedup is line 250-ish —
AFTER the all-adapters-failed guard (line 245) and BEFORE
`synth.Synthesize` (line 258). Inputs available: `docs []NormalizedDoc`,
`spanCtx context.Context`, `decision.Lang`. Outputs needed: same
`[]NormalizedDoc` shape with cluster representatives only and
cluster-membership metadata attached.

### 2.2 Existing fanout dedup (PRESERVE — invariant)

`internal/fanout/dedup.go:30-45` — `dedupDocs` is the SPEC-FAN-001 dedup
transform. It is **EXACT-equality only**:

- Primary key: `"url:" + canonicalURL(doc.URL)` (when URL parses)
- Fallback key: `"hash:" + doc.CanonicalHash()` (16-hex SHA-256 of
  SourceID|URL|Title|Body)
- First-occurrence-wins; later equal-key docs silently dropped
- `@MX:ANCHOR` at `dedup.go:26` warns the invariant must not change

SPEC-SYN-003 does NOT replace this. It runs AFTER fanout returns. The
invariant SPEC-SYN-003 preserves: every doc that survives fanout dedup
also survives or is represented by SPEC-SYN-003 clustering — no doc_id
ever fully disappears.

### 2.3 NormalizedDoc schema available fields

`pkg/types/normalized_doc.go:40-56` — 15-field struct. SimHash inputs:

- `Title string` — adapter-normalized title text
- `Body string` — ranking input, longer text
- `URL string` — already canonicalized (tracking params stripped per adapter
  contract); SPEC-FAN-001 already deduped exact equals
- `Lang string` — BCP-47, may be empty (unknown)

Fields SPEC-SYN-003 MUST NOT mutate on surviving docs:

- `ID` — required by Validate() (line 64)
- `SourceID` — required by Validate() (line 67)
- `URL`, `RetrievedAt` — required (lines 70, 73)
- `Citations []string` — SPEC-SYN-002 explicitly consumes this for
  per-claim provenance (line 30 godoc). SPEC-SYN-003 MUST NOT touch it.
- `Hash string` — `CanonicalHash()` cached value; clustering must not
  invalidate it. Reading is fine; writing forbidden on surviving docs.

Fields SPEC-SYN-003 MAY use:

- `Metadata map[string]any` (line 54) — adapter-specific extension bag,
  EXCLUDED from `CanonicalHash()` per the godoc invariant. This is the
  natural home for cluster-membership data without schema breakage. We
  use a single reserved key, `spec_syn003_cluster`, holding a struct/map
  describing the cluster (member doc_ids, simhash digest, dedup_mode
  used, optional cosine score).

### 2.4 Embedder service availability

`internal/embedder/client.go` + `services/embedder/` (SPEC-IDX-002,
status: implemented). The Go client is fully usable from the query path —
HTTP `POST /embed` returns dense + optional sparse + ColBERT vectors.

Empirical concern: query-time latency budget. SPEC-SYN-001 acceptance
gate mentions p50 ≤ 8s end-to-end (`.moai/specs/SPEC-SYN-002/spec.md:169`
NFR-SYN2-001 reaffirms). SPEC-IDX-002 NFR target on CPU 4 vCPU is
batched-embedding latency; a 50-doc fanout output embedded in one batched
call is documented to fit comfortably under the synthesis SLA. But the
sidecar may be unreachable (Docker compose service down,
`EMBEDDER_BASE_URL` misconfigured, sidecar still loading model — returns
`ErrModelLoadFailed`). SPEC-SYN-003 MUST degrade gracefully: when the
embedder is unreachable or returns an error, fall back to SimHash-only
clustering. The whole pipeline MUST NOT fail the user query because the
clustering's secondary refinement is unavailable.

Confirmed: query path does NOT call the embedder today (SPEC-IDX-001
hybrid index is the typical embedding consumer; query path goes straight
fanout → synth). SPEC-SYN-003 introduces the FIRST query-time embedding
call. This is a meaningful operational change and MUST be guarded by
config and circuit-breaking.

---

## 3. SimHash for near-duplicate detection (algorithm choice)

### 3.1 Why SimHash (over MinHash, CDC)

- **MinHash**: better for set-similarity over n-grams; well-supported in
  Go (`github.com/dgryski/go-minhash`). But our payload is short-to-
  medium prose (article + comment), not large doc-shingles like web
  crawl corpora. SimHash digests collapse the document to a single
  64-bit fingerprint with cheap Hamming-distance similarity — the
  natural fit for paragraph-scale near-dup detection.
- **CDC (content-defined chunking)**: useful for binary blob dedup; not
  applicable to text near-dup detection.
- **Embedding cosine alone**: sufficient quality, but every pair test
  costs an embedding lookup + cosine → O(N²) is brutal at fanout volumes
  (50–200 docs). SimHash gives O(N) digest computation + a cheap
  candidate-pair filter; cosine refines only the candidate set.

### 3.2 SimHash Go libraries surveyed

(For reference only; library selection deferred to Run phase per Approach
First rule. Run phase will pick one.)

| Library | Last release | Notes |
|---|---|---|
| `github.com/mfonda/simhash` | 2018 | Charikar's algorithm; ASCII tokenizer; small surface; widely cited. |
| `github.com/AndreasBriese/bbloom` (related, not simhash) | n/a | Bloom filter, not relevant; listed only to disambiguate. |
| Hand-rolled (≈80 LOC) | n/a | Charikar SimHash is short; in-tree implementation avoids external dep. |

Recommendation for Run phase: choose between mfonda/simhash and a
hand-rolled implementation based on Korean tokenization fit (see §4).

### 3.3 SimHash threshold tuning

Default 64-bit Hamming distance threshold for "near-duplicate" in
literature: typically 3–6 bits (Manku, Jain, Sarma — WWW 2007). Our
target is paragraph-scale (article body), not web-page-scale. Initial
default: **Hamming distance ≤ 4 bits** (config-overridable). Empirical
tuning belongs to SPEC-EVAL-001 (M4 — DeepEval gate at ≥0.85).

---

## 4. Korean tokenization caveat

### 4.1 Why naive byte-level SimHash fails for Korean

SimHash quality depends on the tokenizer producing semantic tokens. For
Korean text:

- ASCII-only tokenizer (whitespace split): Korean is space-delimited but
  morphologically rich. "검색했다" / "검색하였다" / "검색해" all
  share root "검색" — naive whitespace split treats them as different
  tokens, lowering near-dup recall.
- mecab-ko (SPEC-IDX-003): the project's authoritative Korean
  tokenizer. Available as a sidecar at the index plane; cross-package
  import from query plane is **not** clean.

### 4.2 v0 strategy: character-shingling

For SPEC-SYN-003 v0, use **character n-gram shingling** (n=3) over
NFC-normalized text as the SimHash token source. This:

- Is morphology-blind but recall-stable across "검색했다" / "검색하였다"
  via shared 3-char shingles "검색했" / "색했다" vs "검색하" / "색하였" —
  similarity drops gracefully, not catastrophically.
- Avoids cross-package mecab-ko dependency in `internal/synthcluster/`.
- Works equally for English (3-char shingles = trigrams over chars) and
  Korean.
- Is documented to be a known compromise; mecab-ko upgrade is a
  fast-follow gated on SPEC-EVAL-003 Korean-locale benchmark
  (M8, `.moai/project/roadmap.md:103`).

### 4.3 mecab-ko upgrade path (deferred)

If SPEC-EVAL-003 Korean Hamming-distance recall measurements indicate
char-shingling is insufficient, a follow-up SPEC adds an optional
mecab-ko adapter: `internal/synthcluster/tokenizer/` with two
implementations (char-shingle default, mecab-ko opt-in) selectable via
`DEDUPCLUSTER_TOKENIZER ∈ {char_shingle, mecab_ko}`. Out of scope for
SPEC-SYN-003 v0.

---

## 5. Cross-source dedup semantics

### 5.1 Already handled by fanout

SPEC-FAN-001 `dedupDocs` already drops docs with the same canonical URL
(first occurrence wins regardless of source). Example: Reddit thread
linking `nytimes.com/article` and HN submission linking the same
`nytimes.com/article` → only one survives.

### 5.2 What SPEC-SYN-003 actually catches

The remaining near-dup cases that escape exact-URL dedup:

- **Mirror sites / news syndication**: Naver News + Daum News + an
  English-source RSS all reporting on the same incident with different
  URLs and slightly different wording.
- **Paraphrased social discussion**: Reddit comment summarizing an
  article, HN comment doing the same, tweet doing the same — different
  URLs, different SourceIDs, similar prose.
- **Translation pairs**: a Korean Naver article and an English HN
  submission about the same event — text similarity is moderate, URL
  and SourceID differ. SimHash may or may not catch this; embedding
  cosine is the more reliable signal here. (Cosine threshold default
  0.92 is empirically defensible for cross-lingual BGE-M3 embeddings;
  documented in BGE-M3 paper §5.)

### 5.3 What SPEC-SYN-003 explicitly does NOT do

- Does NOT merge a Reddit thread and an HN comment that link to the
  same URL — fanout already collapsed them.
- Does NOT merge a Reddit submission and one of its top comments —
  they have different URLs and are independently informative; treating
  them as duplicates would lose evidence.
- Does NOT alter the doc_ids of cluster representatives. The
  representative keeps its own ID; cluster members' IDs are recorded in
  metadata, not lost.

---

## 6. Cluster representative selection

### 6.1 Rule (locked decision for v0)

For each cluster of size ≥ 2:

1. **Primary**: highest `Score` field (already normalized to [0,1] per
   `pkg/types/normalized_doc.go:48` godoc; 0 = unscored, NOT zero
   engagement).
2. **Secondary**: most recent `PublishedAt` (when both Score == 0 or
   ties on score).
3. **Tertiary**: longer `Body` (more information for synthesizer).
4. **Tiebreaker**: lexicographic `ID` (deterministic).

Rationale: Score reflects adapter-side ranking confidence; PublishedAt
breaks ties toward fresher content; Body length favors source with more
detail; ID makes the function deterministic for tests and replay.

### 6.2 Failure mode: representative selection cannot decide

In practice the 4-tier comparator is total over `NormalizedDoc` (ID is
guaranteed unique per `Validate()`'s requirement). The "fallback" REQ
(EARS Unwanted pattern) covers a defensive case: if a future schema
change somehow makes ID empty, the cluster is still emitted — the
**first** doc in the cluster (by input order) wins. Never drop a whole
cluster.

---

## 7. Latency budget vs synthesis SLA

| Stage | Target | Rationale |
|---|---|---|
| SimHash digest of N docs | < 5ms (N=50, char-shingle, single goroutine) | O(N · L) where L = avg doc bytes; SHA-1 over shingles is hot. |
| Hamming-distance candidate pair filter | < 2ms (N=50) | Pairwise XOR-popcount; can be parallelized but unnecessary at N≤200. |
| Embedding refinement (when reachable) | ≤ 1.5s p95 (N=50, batched single call) | Budget per SPEC-IDX-002 NFR; embedder sidecar is the bottleneck. |
| Cluster representative selection + metadata write | < 1ms (N=50) | In-memory, sort-then-pick. |
| **Total p95 added** | **≤ 1.5s** with embedding, **≤ 10ms** without | |

NFR target for SPEC-SYN-003: p95 added latency ≤ 1500 ms END-TO-END,
which preserves the SPEC-SYN-001 / SPEC-SYN-002 p50 ≤ 8s gate (8s − 1.5s
= 6.5s budget remaining for synthesis itself; comfortable margin).

---

## 8. Risks

### Risk R1 — Cluster representative selection loses information

Severity: **MEDIUM**.
Description: When two near-duplicate docs come from different sources
(e.g. Reddit + Naver), choosing one representative drops the other from
the synthesizer's view. The synthesizer cannot cite the dropped doc.
Mitigation: cluster members' doc_ids are persisted in
`Metadata["spec_syn003_cluster"]["members"]`. Synthesis does NOT use
this v0 (SPEC-SYN-002 cites only top-level docs in `docs[]`). Cross-
member citation propagation is a SPEC-SYN-005 candidate (NOT in M4).
Acceptance posture: log the dropped doc_ids; counters expose the count;
operators can disable clustering via `dedup_mode=off` for debugging.

### Risk R2 — Threshold tuning regresses recall on Korean queries

Severity: **MEDIUM**.
Description: Hamming threshold 4 + char-3-shingles on Korean text may
miss paraphrase pairs that mecab-ko-tokenized SimHash would catch.
Mitigation: SPEC-EVAL-003 (M8) Korean-locale benchmark establishes the
empirical recall floor. Until then, the
`DEDUPCLUSTER_HAMMING_THRESHOLD` env var allows operator tuning.
Acceptance posture: NFR-SYN3-002 mandates a property test ensuring two
identical-text docs always cluster; near-dup recall is benchmark
territory (SPEC-EVAL-003), out of acceptance scope for v0.

### Risk R3 — Embedding sidecar latency violation

Severity: **MEDIUM**.
Description: When the embedder is reachable but slow (model still
warming up, GC pause, network blip), a batched embed call may exceed
the 1.5s NFR. Mitigation: per-call context deadline derived from the
synthesis SLA budget. On context deadline exceeded, the clustering
falls back to SimHash-only result (already-computed candidate pairs are
emitted as-is). Counter `usearch_dedupcluster_embedding_fallback_total`
exposes the rate.

### Risk R4 — Embedder unreachable on every query

Severity: **LOW**.
Description: Sidecar down, query path exercises the fallback path
constantly. Mitigation: `dedup_mode=simhash_only` is a first-class mode,
not a degraded mode — it's a supported configuration. Operators can
also set `dedup_mode=off` to bypass clustering entirely. Behavior is
deterministic and observable.

### Risk R5 — `Metadata["spec_syn003_cluster"]` collides with adapter metadata

Severity: **LOW**.
Description: Adapters use `Metadata` for adapter-specific fields (Reddit
upvote count, HN comment count, ...). The reserved key prefix
`spec_syn003_*` is namespaced and unlikely to collide with adapter
keys. Validation: a unit test asserts no adapter in `internal/adapters`
writes a key starting with `spec_syn003_`.

### Risk R6 — Concurrency: parallel clustering corrupts shared state

Severity: **LOW** (mitigated by design).
Description: If clustering uses goroutines for SimHash compute or for
embedder calls, races on shared maps could corrupt state. Mitigation:
the package's public API takes `[]NormalizedDoc` and returns
`[]NormalizedDoc` — no shared state crosses the boundary. Internal
goroutines (if any) write to per-index slices, mirror SPEC-FAN-001
H1 pattern (SPEC-FAN-001 spec.md HISTORY 2026-05-05 cycle 1).

---

## 9. doc_id preservation invariant (SYN-002 contract)

`pkg/types/normalized_doc.go:30` godoc declares:
> `Citations`: doc IDs referenced by this doc; SPEC-SYN-002 consumes for
> per-claim provenance.

SPEC-SYN-002 REQ-SYN2-001 (`.moai/specs/SPEC-SYN-002/spec.md:159`)
mandates every synthesized claim resolves to a `doc_id` in the input
`docs[]`. SPEC-SYN-003 sits BEFORE synthesis, so the `docs[]` it
produces becomes the `docs[]` SPEC-SYN-002 sees.

**Invariant SPEC-SYN-003 MUST preserve**:
For every output `doc d` in the cluster representative set,
`d.ID == input_d.ID` for some `input_d` (representative is verbatim
from input, modulo `Metadata["spec_syn003_cluster"]` annotation).
For every cluster member `m` (non-representative), `m.ID` is recorded
in `representative.Metadata["spec_syn003_cluster"]["members"]`.

This is the **traceability invariant**: every input doc_id is either
(a) a representative output doc_id, or (b) a recorded member of some
output doc's cluster. No doc_id is ever silently dropped.

---

## 10. Cross-cutting decisions (locked for v0)

| Decision | Choice | Rationale |
|---|---|---|
| SimHash hash size | 64 bits | Standard; sufficient for N≤200 docs per query. |
| Tokenizer | character 3-shingles over NFC-normalized text | Korean-friendly without mecab-ko dep. |
| Default Hamming threshold | 4 bits | Manku 2007 paper; tunable via env. |
| Default cosine threshold (when embedder reachable) | 0.92 | BGE-M3 §5 cross-lingual recommendation. |
| Embedder fallback | graceful (SimHash-only result) | Sidecar may be unreachable; query path must not fail. |
| Cluster member storage | `Metadata["spec_syn003_cluster"]` | Schema-non-breaking; namespaced. |
| Representative selection | Score → PublishedAt → Body length → ID | Deterministic, explained in §6. |
| Default mode | `dedup_mode=simhash_only` (not hybrid) | Conservative; embedder is opt-in for query path. |

---

## 11. References

Internal:

- `.moai/project/roadmap.md:65` — SPEC-SYN-003 row.
- `.moai/project/roadmap.md:124` — M4 3-way parallelization plan.
- `.moai/project/roadmap.md:151` — M4 exit criterion (citation
  faithfulness ≥ 0.85, p50 ≤ 8s).
- `.moai/specs/SPEC-FAN-001/spec.md:1-100` — fanout dedup contract.
- `.moai/specs/SPEC-SYN-001/spec.md` — synthesis sidecar contract.
- `.moai/specs/SPEC-SYN-002/spec.md:159` — REQ-SYN2-001 doc_id
  invariant.
- `.moai/specs/SPEC-IDX-002/spec.md:1-80` — embedder service contract.
- `.moai/specs/SPEC-IDX-003/spec.md` — Korean tokenization service
  (mecab-ko sidecar).
- `internal/fanout/fanout.go:21-97` — Dispatch entry point.
- `internal/fanout/dedup.go:30-54` — exact-URL dedup, SPEC-FAN-001 invariant.
- `pkg/types/normalized_doc.go:40-106` — schema and CanonicalHash.
- `internal/embedder/types.go:9-44` — Request/Response shapes.
- `internal/embedder/client.go` — query-path-callable embedder client.
- `internal/synthesis/types.go:38-58` — synthesis Result + Citation.
- `cmd/usearch/query.go:228-258` — fanout → synthesis seam (insertion
  point for SPEC-SYN-003).

External (verification deferred to Run phase per anti-hallucination
policy; URLs listed for context only):

- Charikar, M. (2002). Similarity estimation techniques from rounding
  algorithms — original SimHash paper.
- Manku, Jain, Das Sarma (WWW 2007). Detecting near-duplicates for web
  crawling. Threshold tuning empirics for 64-bit SimHash.
- BGE-M3 paper (BAAI, 2024) §5 — cross-lingual cosine threshold guidance.
- `https://github.com/mfonda/simhash` — Go SimHash library (last release
  2018).
- `https://github.com/dgryski/go-minhash` — Go MinHash library (alternative
  considered, rejected for paragraph-scale payload).

---

*End of SPEC-SYN-003 research artifact (Phase 0.5).*
