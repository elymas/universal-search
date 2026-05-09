---
id: SPEC-SYN-003
version: 0.1.1
status: approved
created: 2026-05-09
updated: 2026-05-09
author: limbowl
priority: P0
issue_number: 0
title: Dedup + clustering (pre-synthesis)
milestone: M4 — Basic Synthesis Hardening
owner: expert-backend
methodology: tdd
coverage_target: 85
depends_on: [SPEC-FAN-001, SPEC-CORE-001, SPEC-IDX-002]
blocks: [SPEC-EVAL-001]
---

# SPEC-SYN-003: Dedup + clustering (pre-synthesis)

## HISTORY

- 2026-05-09 — status draft → approved (plan-auditor PASS iter 1, D1+D2 MINOR fixes applied; D3-D6 deferred). v0.1 → v0.1.1. Counter-semantics rule (D1) added explicitly to REQ-SYN3-002 / REQ-SYN3-003 / new "Counter Semantics" subsection: in hybrid mode ONLY `hybrid_refined` (success) or `embedding_fallback` (degradation) emits; `simhash_clustered` fires ONLY when `mode==simhash_only`. Dedicated embedder-timeout scenario (D2) added to acceptance §3.5b (was implicitly covered only via ctx-cancel mid-embed). plan.md decisions §6.5 records the exclusivity rationale; spec-compact.md mode table notes the counter mapping.

- 2026-05-09 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC for pre-synthesis near-duplicate
  clustering. Inserts a NEW `internal/synthcluster/` Go package
  between SPEC-FAN-001's `Dispatch` output and SPEC-SYN-001's
  `synth.Synthesize` call, at `cmd/usearch/query.go:229-258`. Reuses
  SPEC-IDX-002 BGE-M3 embedder client (`internal/embedder`) read-only
  as an OPTIONAL secondary refinement when the sidecar is reachable;
  default mode is `simhash_only` (no embedding RTT in the query path).
  Uses `Metadata["spec_syn003_cluster"]` as the cluster-membership
  storage namespace — no `NormalizedDoc` schema change. Preserves
  SPEC-FAN-001 exact-URL dedup invariant (runs AFTER `dedupDocs`) and
  SPEC-SYN-002 doc_id traceability invariant (every input doc_id is
  either a representative output ID or a recorded cluster member).
  5 EARS REQs (4 × P0 + 1 × P1) covering all five EARS patterns.
  2 NFRs. Companion research artifact at
  `.moai/specs/SPEC-SYN-003/research.md` — pipeline trace, library
  survey, Korean-tokenization caveat (v0 uses character 3-shingles;
  mecab-ko upgrade gated on SPEC-EVAL-003 M8). Explicitly delegates
  semantic clustering for synthesis structure to STORM (M5,
  SPEC-DEEP-001), cross-query memoization to SPEC-IDX-005 (M6),
  Korean-locale recall benchmark to SPEC-EVAL-003 (M8). No GitHub
  issue tracking on this SPEC (`issue_number: 0`). Ready for
  plan-auditor review and annotation cycle.

---

## 1. Purpose

`.moai/project/roadmap.md` line 65 declares M4 SPEC-SYN-003:

> Dedup + clustering | cluster near-duplicate results pre-synthesis
> (SimHash + embedding cosine) | expert-backend

SPEC-FAN-001 (implemented at commit `04308b8`) delivered **exact-URL
dedup** in `internal/fanout/dedup.go` — same canonical URL or same
content hash collapses. This catches the "Reddit thread + HN comment
linking the same NYT article" case.

But the residual near-duplicate surface that reaches the synthesizer
includes:

- Naver News + Daum News + an English RSS feed, all reporting the same
  incident with different URLs and slightly paraphrased text.
- Reddit comment + HN comment + tweet, all summarizing the same
  external article with different URLs and prose.
- Korean and English coverage of the same event from sibling sources
  (cross-lingual paraphrase).

These cases give the synthesizer redundant context, inflate token
cost, and risk biasing the synthesized paragraph toward the
over-represented event. SPEC-SYN-003 collapses them into clusters
BEFORE synthesis, picks one representative per cluster, and persists
the dropped doc_ids in the representative's `Metadata` for downstream
auditability.

This SPEC is **structural near-dup detection only**. Whether two docs
actually describe the same event (semantic equivalence) is delegated
to SPEC-EVAL-003 (M8 Korean-locale benchmark) for empirical recall
floor and to STORM (SPEC-DEEP-001, M5) for synthesis-time semantic
structuring.

Completion delivers a SimHash-first, optionally embedding-refined
clustering stage that reduces the synthesizer's input redundancy while
preserving every input doc_id under the SPEC-SYN-002 traceability
contract.

---

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | [NEW] `internal/synthcluster/` package — pure-Go implementation. Public surface: `Cluster(ctx context.Context, docs []types.NormalizedDoc, opts Options) ([]types.NormalizedDoc, Stats, error)`. Internal: `simhash.go` (64-bit Charikar SimHash over character 3-shingles, NFC-normalized), `cluster.go` (candidate-pair filter via Hamming distance, Union-Find cluster assembly, representative selection), `embed_refine.go` (OPTIONAL secondary cosine refinement when embedder is reachable), `types.go` (Options, Stats, Cluster value types), `metadata.go` (`Metadata["spec_syn003_cluster"]` reader/writer with versioned schema). |
| b | [MODIFY] `cmd/usearch/query.go` — between line 229 (`docs := fanoutResult.Docs`) and line 258 (`synth.Synthesize(spanCtx, prompt, decision.Lang, docs)`), invoke `synthcluster.Cluster(spanCtx, docs, opts)` to produce the clustered slice; pass the result (NOT the raw fanout output) to `synth.Synthesize`. The all-adapters-failed guard at line 245 runs BEFORE clustering — clustering MUST NOT execute on an empty slice. |
| c | [NEW] `DEDUPCLUSTER_MODE` env var: `simhash_only` (default), `hybrid`, `off`. `simhash_only` runs SimHash + Hamming filter only. `hybrid` runs SimHash + (when embedder reachable) embedding cosine secondary refinement. `off` bypasses clustering entirely (pass-through). Loaded once per `Cluster` call from `Options.Mode`. |
| d | [NEW] `DEDUPCLUSTER_HAMMING_THRESHOLD` env var (default `4`, range `[0, 64]`) and `DEDUPCLUSTER_COSINE_THRESHOLD` env var (default `0.92`, range `[0.0, 1.0]`). Both surfaced via `Options` for test injection. |
| e | [NEW] `Metadata["spec_syn003_cluster"]` reserved key on representative docs. Schema (versioned, `schema_version: 1`): `{schema_version: int, members: []string (doc IDs of dropped near-dups), simhash: string (16-hex-char of representative's SimHash digest), dedup_mode: string ("simhash_only" / "hybrid"), cosine_min: float (when hybrid; minimum pairwise cosine in cluster), cluster_size: int}`. Adapter metadata namespace `spec_syn003_*` is RESERVED — adapters MUST NOT write keys with this prefix (validation lives in adapter contract tests, but a guard test in `synthcluster_test.go` asserts no input metadata key collides). |
| f | [NEW] Two Prometheus collectors in `internal/obs/metrics/synthcluster.go`: `SynthClusterOutcomes *prometheus.CounterVec{outcome, mode}` (label values for `outcome`: `passthrough`, `simhash_clustered`, `hybrid_refined`, `embedding_fallback`; for `mode`: `simhash_only`, `hybrid`, `off`) and `SynthClusterMembers prometheus.Histogram` (cluster-size distribution; buckets `[1, 2, 3, 5, 10, 20, 50]`). Cardinality allowlist EXTENSION: a new label name `mode` with three pre-declared values is added — this REQUIRES an allowlist amendment per SPEC-OBS-001 discipline (see §6 risk register R5). |
| g | [MODIFY] `internal/obs/metrics/metrics.go` — register the two new collectors via a new `registerSynthCluster(pr)` helper, mirroring the SPEC-SYN-001 `registerSynthesis(pr)` pattern. |
| h | [MODIFY] `internal/obs/obs.go` — re-export the two new collector handles for caller convenience (`obs.SynthClusterOutcomes`, `obs.SynthClusterMembers`). |
| i | [MODIFY] `.env.example` — add `DEDUPCLUSTER_MODE=simhash_only`, `DEDUPCLUSTER_HAMMING_THRESHOLD=4`, `DEDUPCLUSTER_COSINE_THRESHOLD=0.92`, `DEDUPCLUSTER_EMBEDDING_TIMEOUT_MS=1500` with explanatory comments. |
| j | [EXISTING — UNCHANGED] `pkg/types/normalized_doc.go` schema. NO field added; `Metadata` is the extension point. `CanonicalHash()` remains stable (Metadata is excluded from the hash by the existing godoc invariant on line 31). NFR-CORE-001 < 1 µs/op preserved. |
| k | [EXISTING — UNCHANGED] `internal/fanout/dedup.go`. SPEC-SYN-003 runs AFTER fanout returns; the exact-URL/CanonicalHash dedup invariant is preserved verbatim. The `@MX:ANCHOR` at `dedup.go:26` remains intact. |
| l | [EXISTING — UNCHANGED] `internal/embedder/client.go`. Read-only consumer of the existing `client.Embed(ctx, Request)` API. No changes to embedder signatures, error sentinels, or sidecar contract. |
| m | [EXISTING — UNCHANGED] `internal/synthesis/client.go`, `internal/synthesis/types.go`, and `services/researcher/`. The synthesis sidecar's contract is untouched. SPEC-SYN-003 changes only the **content** of the `docs []NormalizedDoc` slice passed to `synth.Synthesize`, not its shape. SPEC-SYN-002's `doc_id`-faithfulness invariant continues to be enforced over the post-cluster slice. |
| n | [NEW] `internal/synthcluster/cluster_test.go`, `internal/synthcluster/simhash_test.go`, `internal/synthcluster/embed_refine_test.go`, `internal/synthcluster/metadata_test.go` — unit tests including the property test required by NFR-SYN3-002. |
| o | [MODIFY] `cmd/usearch/query_test.go` — add integration tests for `dedup_mode=off` pass-through, `dedup_mode=simhash_only` happy path, and `dedup_mode=hybrid` with a mocked embedder client. |

### 2.2 Out-of-Scope (Exclusions — What NOT to Build)

[HARD] This SPEC explicitly excludes the following items. Each has a
known destination SPEC; this list prevents scope creep.

- **Semantic clustering for synthesis structure** — grouping docs by
  topic to drive paragraph organization in the synthesizer's output.
  → SPEC-DEEP-001 (M5 STORM integration); STORM owns synthesis-time
  topic structuring.
- **Cross-query memoization / team-shared answer reuse** — caching
  cluster results across queries or users.
  → SPEC-IDX-005 (M6 team-shared answer reuse).
- **Lossy compression of cluster members** — discarding dropped
  near-dup doc_ids without persistence. → REQ-SYN3-005 mandates
  `Metadata["spec_syn003_cluster"]["members"]` retains every cluster
  member's doc_id; no compression, no truncation, no member-count cap.
- **Replacing or modifying SPEC-FAN-001 exact-URL dedup** — the
  fanout-side `dedupDocs` is preserved verbatim. SPEC-SYN-003 runs
  AFTER it. SPEC-SYN-003 MUST NOT collapse two docs that
  `internal/fanout/dedup.go` already left distinct unless their
  Hamming distance ≤ threshold (i.e. it's not enough to share the
  same SourceID — they must actually be near-duplicates).
- **Modifying SPEC-SYN-002 citation-faithfulness contract** —
  SPEC-SYN-002's `doc_id`-trace invariant (`.moai/specs/SPEC-SYN-002/spec.md:159`)
  is preserved over the post-cluster slice. No changes to
  `services/researcher/` Python side.
- **`pkg/types/normalized_doc.go` schema change** — no new field,
  no field rename, no JSON shape change. `Metadata` is the only
  extension surface.
- **mecab-ko Korean tokenizer integration in the dedup package** —
  v0 uses character 3-shingles (research §4.2). mecab-ko-tokenized
  SimHash is gated on SPEC-EVAL-003 (M8) recall measurements;
  upgrade is a fast-follow SPEC if empirics warrant.
- **Korean-locale recall benchmark fixtures** — adversarial Korean
  paraphrase pairs and recall-floor measurements belong in
  SPEC-EVAL-003 (M8).
- **Cross-cluster member citation propagation in the synthesizer** —
  v0 synthesis sees only representative docs. Allowing the
  synthesizer to cite a cluster member's `doc_id` is a SPEC-SYN-005
  candidate (NOT in M4).
- **Promotion of a cluster member to representative after the
  fact** — once the representative is selected (per §6 of
  research.md), its identity is FROZEN for the call. Re-clustering
  inside the call is NOT supported.
- **Bypassing clustering on a per-doc basis** — `dedup_mode=off` is
  the only bypass; there is no per-source or per-tag opt-out. (A
  hypothetical "always include this doc verbatim" flag is out of
  scope and has no caller demand today.)
- **Persistent cross-call cluster memory** — clustering state is
  per-call, in-process; no Redis, no DB, no shared cache. (Sees
  SPEC-IDX-005 for the team-shared answer reuse story.)
- **Adapting Hamming / cosine thresholds at runtime via feedback
  signals** — thresholds are env-driven static values for v0.
  Auto-tuning is gated on SPEC-EVAL-003 measurement data.
- **GitHub Issue tracking on this SPEC** (`issue_number: 0`).

---

## 3. EARS Requirements

### Functional Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-SYN3-001 | Ubiquitous | The `internal/synthcluster.Cluster` function SHALL compute a 64-bit Charikar SimHash digest for every input `NormalizedDoc` whose `Validate()` succeeds, using character 3-shingles over the NFC-normalized concatenation `Title + "\n" + Body` as the token source. The SimHash digest SHALL be deterministic for byte-identical inputs (idempotent across calls within the same binary). The function SHALL preserve the input slice's `[]NormalizedDoc` shape (representative docs are verbatim from input modulo the `Metadata["spec_syn003_cluster"]` key on the representative). The SPEC-FAN-001 `dedupDocs` exact-URL dedup invariant from `internal/fanout/dedup.go:30-45` SHALL be preserved (SPEC-SYN-003 runs strictly AFTER fanout). The SPEC-SYN-002 `doc_id` traceability invariant from `.moai/specs/SPEC-SYN-002/spec.md:159` SHALL be preserved: every input `doc_id` is EITHER (a) the `ID` of a representative output doc, OR (b) recorded in some representative's `Metadata["spec_syn003_cluster"]["members"]`. | P0 | `test_simhash_deterministic`, `test_simhash_computed_for_all_input_docs`, `test_doc_id_invariant_preserved` (every input ID accounted for), `test_fanout_invariant_preserved` (running fanout dedup before clustering remains a no-op for clustering's input). |
| REQ-SYN3-002 | Event-Driven | WHEN the `Cluster` function detects a candidate pair `(d_i, d_j)` whose 64-bit SimHash Hamming distance is `<= Options.HammingThreshold`, THEN the function SHALL emit a candidate-pair event into the cluster-assembly Union-Find structure AND SHALL record the pair in the per-call cluster graph. The function SHALL increment `usearch_synthcluster_outcomes_total{outcome="simhash_clustered", mode=<mode>}` by 1 once per confirmed cluster (not once per pair) **EXCLUSIVELY when `Options.Mode == "simhash_only"`**; the `simhash_clustered` counter SHALL remain at 0 in `hybrid` mode regardless of the number of clusters formed (see counter-exclusivity rule below and REQ-SYN3-003 for the hybrid-mode counters). The candidate-pair detection SHALL be O(N²) over the input slice for N ≤ 200 (acceptable at fanout volumes per research §7) and SHALL NOT block on external services. | P0 | `test_hamming_within_threshold_creates_pair`, `test_hamming_above_threshold_no_pair`, `test_simhash_clustered_counter_increments_per_cluster`, `test_simhash_clustered_counter_zero_in_hybrid_mode`, `test_candidate_pairs_assembled_into_unionfind`. |
| REQ-SYN3-003 | State-Driven | WHILE `Options.Mode == "hybrid"` AND the embedder client returns a successful response within `Options.EmbeddingTimeoutMs` (default 1500 ms), the `Cluster` function SHALL invoke `embedder.Client.Embed` ONCE per call with the batched `Texts` field set to `[doc.Title + "\n" + doc.Body for each candidate-pair-participant doc]`, SHALL compute pairwise cosine similarity over the returned dense vectors, AND SHALL accept ONLY candidate pairs whose cosine similarity `>= Options.CosineThreshold` (default 0.92) into the final cluster graph. Pairs failing the cosine refinement SHALL be DEMOTED — the two docs remain distinct, and the SimHash-derived candidate edge is dropped. The function SHALL increment `usearch_synthcluster_outcomes_total{outcome="hybrid_refined", mode="hybrid"}` per cluster that survives cosine refinement. **In hybrid mode, only `hybrid_refined` (success path) or `embedding_fallback` (degradation path) MAY fire — `simhash_clustered` SHALL NOT fire under any hybrid-mode code path** (counter-exclusivity invariant; see Counter Semantics rule below). IF the embedder returns `ErrSidecarUnreachable`, `ErrTimeout`, `ErrModelLoadFailed`, OR the per-call `context.Context` is cancelled, THEN the function SHALL fall back to SimHash-only clustering (treat all SimHash candidate pairs as confirmed), SHALL increment `usearch_synthcluster_outcomes_total{outcome="embedding_fallback", mode="hybrid"}` ONCE per call, AND SHALL emit a WARN-level structured log record with `{request_id, dedup_mode:"hybrid", embedding_error:<err>, fallback_to:"simhash_only"}`. | P0 | `test_hybrid_calls_embedder_once`, `test_cosine_above_threshold_confirms_pair`, `test_cosine_below_threshold_demotes_pair`, `test_embedder_unreachable_falls_back_to_simhash`, `test_embedder_timeout_falls_back_to_simhash`, `test_embedder_timeout_path_dedicated` (D2 dedicated timeout scenario), `test_embedding_fallback_counter_increments_once_per_call`, `test_embedding_fallback_logs_warn`, `test_hybrid_mode_simhash_clustered_counter_zero` (D1 counter-exclusivity). |
| REQ-SYN3-004 | Unwanted | IF `Cluster` cannot deterministically select a single representative for any cluster of size ≥ 2 — for example because two cluster members tie on `Score` AND `PublishedAt` AND `len(Body)` AND `ID` (a degenerate case requiring two non-Validate-able docs with identical IDs, which `pkg/types/normalized_doc.go:64` rules out at input time, but which is defensively guarded here) — THEN the function SHALL select the **input-order-first** doc as the representative, SHALL NOT drop the cluster, SHALL emit a WARN-level structured log record with `{request_id, cluster_id, member_ids, fallback_reason:"representative_tiebreaker_exhausted"}`, AND SHALL increment a dedicated counter (re-using `usearch_synthcluster_outcomes_total{outcome="simhash_clustered", mode=<mode>}` is acceptable; the WARN log is the primary audit signal). The function SHALL NEVER drop an entire cluster, SHALL NEVER return fewer docs than the count of distinct clusters, AND SHALL NEVER lose a doc_id from the input set. | P0 | `test_representative_selection_uses_score_tiebreaker`, `test_representative_selection_falls_back_to_input_order`, `test_no_cluster_ever_dropped`, `test_doc_id_count_invariant` (count of unique input IDs == count of unique IDs across representatives + their cluster members). |
| REQ-SYN3-005 | Optional | WHERE `Options.Mode == "off"` (set via `DEDUPCLUSTER_MODE=off`), the `Cluster` function SHALL bypass all SimHash, Hamming, embedding, and cluster-assembly logic, SHALL return the input `[]NormalizedDoc` slice **unchanged** (no `Metadata` mutation, no SimHash digest computation, no embedder call), SHALL increment `usearch_synthcluster_outcomes_total{outcome="passthrough", mode="off"}` by 1, AND SHALL emit no WARN-level log records. The pass-through path SHALL add ≤ 1 ms of overhead vs. directly passing `fanoutResult.Docs` to `synth.Synthesize` (single struct field check). | P1 | `test_mode_off_returns_input_unchanged`, `test_mode_off_no_metadata_mutation`, `test_mode_off_no_embedder_call`, `test_mode_off_passthrough_counter`, `test_mode_off_overhead_under_1ms`. |

### Counter Semantics (Mode-Exclusive Outcomes — D1 Resolution)

[HARD] The `usearch_synthcluster_outcomes_total{outcome, mode}` counter
follows a **single-emission-per-cluster, mode-exclusive** rule:

| Mode | Counter that MAY fire | Counters that MUST stay 0 |
|---|---|---|
| `simhash_only` | `simhash_clustered` (per cluster) | `hybrid_refined`, `embedding_fallback`, `passthrough` |
| `hybrid` (success) | `hybrid_refined` (per surviving cluster) | `simhash_clustered`, `embedding_fallback`, `passthrough` |
| `hybrid` (degraded) | `embedding_fallback` (once per call) | `simhash_clustered`, `hybrid_refined`, `passthrough` |
| `off` | `passthrough` (once per call) | `simhash_clustered`, `hybrid_refined`, `embedding_fallback` |

Rationale: in hybrid mode the SimHash stage produces only **candidate**
pairs; clusters are not confirmed until either cosine refinement
succeeds (`hybrid_refined`) or the embedder degrades and the function
falls back (`embedding_fallback`). Emitting `simhash_clustered`
alongside `hybrid_refined` in a successful hybrid run would
**double-count** clusters; emitting it on the fallback path would
**conflate** modes. The exclusivity rule keeps each cluster counted
exactly once and lets operators read `hybrid_refined`/`embedding_fallback`
as a clean degradation signal.

This invariant is enforced by acceptance §3.5/§3.6 assertions
(`simhash_clustered == 0` in every hybrid-mode scenario) and by
`test_hybrid_mode_simhash_clustered_counter_zero` /
`test_simhash_clustered_counter_zero_in_hybrid_mode`.

### Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-SYN3-001 | Performance (clustering latency budget) | The `Cluster` function in `simhash_only` mode SHALL complete with p95 ≤ 50 ms over a 50-doc input, p99 ≤ 100 ms (single-threaded, char-3-shingle SimHash + O(N²) Hamming pairwise + Union-Find assembly + representative selection). The `Cluster` function in `hybrid` mode SHALL complete with p95 ≤ 1500 ms over a 50-doc input WHEN the embedder is reachable (one batched `client.Embed` call dominates the budget per SPEC-IDX-002 NFR). The total added end-to-end latency SHALL preserve SPEC-SYN-001 NFR-SYN-001 (p50 ≤ 8 s end-to-end query) and SPEC-SYN-002 NFR-SYN2-001 — measured by adding the SPEC-SYN-003 `Cluster` call into the existing query e2e benchmark. Detailed test method (iteration counts, percentile assertions) is specified in `acceptance.md` §4.6. |
| NFR-SYN3-002 | Property: doc_id traceability invariant | For all inputs `docs []NormalizedDoc` with `len(docs) >= 0` and `for all d in docs, d.Validate() == nil`, the `Cluster` function output `(reps, stats, err)` SHALL satisfy: `err == nil` OR `err` is a wrapped `context.Canceled`/`context.DeadlineExceeded`. When `err == nil`: (a) every `d.ID` from input appears EXACTLY ONCE across the union of `[r.ID for r in reps]` and `[m for r in reps for m in metadata.MembersOf(r)]`; (b) `metadata.MembersOf(r)` for any representative `r` returns a slice that does NOT contain `r.ID` (a representative is not its own member); (c) running `Cluster` again on `reps` (idempotence) produces a result where every cluster has size 1 (no further clustering is possible — the representatives are themselves not near-duplicates of each other under the same threshold). Property test via `testing/quick` or hand-rolled fuzz over a generator producing realistic adapter outputs (Korean + English mix, varying lengths, varying score distributions). |

---

## 4. Acceptance Criteria

Detailed Given/When/Then scenarios with edge cases live in
`.moai/specs/SPEC-SYN-003/acceptance.md`. This section enumerates the
acceptance gate per requirement.

### REQ-SYN3-001 — Ubiquitous: SimHash always computed + invariants preserved

- File `internal/synthcluster/simhash.go` exists and exposes
  `simHash64(text string) uint64`. NFC normalization is applied
  before tokenization.
- `test_simhash_deterministic`: `simHash64("foo")` == `simHash64("foo")`
  byte-equal across 100 invocations.
- `test_simhash_computed_for_all_input_docs`: `Cluster` over 50 docs
  produces a per-call internal map of `len == 50` SimHash digests
  (asserted via instrumentation hook in test mode only).
- `test_doc_id_invariant_preserved`: post-`Cluster` set of
  `union(rep.ID, members(rep))` == set of `input_doc.ID`. Every input
  doc_id is present exactly once.
- `test_fanout_invariant_preserved`: input slice already deduped by
  `internal/fanout/dedup.go` is consumed by `Cluster` without
  re-running URL/Hash dedup; assertion: when input has no
  near-duplicates by SimHash threshold, `Cluster` returns the input
  verbatim (modulo `Metadata` annotation if any cluster of size 1
  receives one — but size-1 clusters MUST NOT have the
  `spec_syn003_cluster` key written).

### REQ-SYN3-002 — Event-Driven: SimHash candidate pair → cluster

- `test_hamming_within_threshold_creates_pair`: feed two docs
  constructed to have SimHash Hamming distance ≤ 4; assert they end
  up in the same cluster.
- `test_hamming_above_threshold_no_pair`: feed two docs with Hamming
  distance ≥ 5; assert they are in different clusters.
- `test_simhash_clustered_counter_increments_per_cluster`: 3 clusters
  detected over input → counter increments by 3, not 6 (per pair) or
  10 (per doc).
- `test_candidate_pairs_assembled_into_unionfind`: 3 docs A/B/C where
  Hamming(A,B) ≤ 4, Hamming(B,C) ≤ 4, Hamming(A,C) > 4; assert A/B/C
  end up in the SAME cluster (transitive via Union-Find).

### REQ-SYN3-003 — State-Driven: hybrid mode embedding refinement

- `test_hybrid_calls_embedder_once`: mock embedder records call count;
  with 4 candidate-pair docs, `client.Embed` is invoked exactly once
  with `Texts = [4 strings]`.
- `test_cosine_above_threshold_confirms_pair`: mocked embedder returns
  vectors where pair (A,B) cosine = 0.95 (> 0.92); assert A/B in same
  cluster post-refinement.
- `test_cosine_below_threshold_demotes_pair`: mocked embedder returns
  pair (C,D) cosine = 0.80 (< 0.92); assert C/D in DIFFERENT clusters
  post-refinement (SimHash-derived candidate edge demoted).
- `test_embedder_unreachable_falls_back_to_simhash`: mock embedder
  returns `ErrSidecarUnreachable`; assert all SimHash candidate
  pairs are confirmed as clusters; `outcome="embedding_fallback"`
  counter == 1; WARN log emitted with `embedding_error` attribute.
- `test_embedder_timeout_falls_back_to_simhash`: mock embedder
  blocks > `Options.EmbeddingTimeoutMs`; assert per-call ctx
  cancellation triggers fallback; same assertions as unreachable
  test.
- `test_embedding_fallback_counter_increments_once_per_call`: even
  with 5 distinct clusters, fallback counter == 1 per call.
- `test_embedding_fallback_logs_warn`: exactly one WARN log per
  fallback event with required attributes.

### REQ-SYN3-004 — Unwanted: representative selection fallback

- `test_representative_selection_uses_score_tiebreaker`: cluster of 3
  with scores [0.9, 0.7, 0.5]; representative == doc with score 0.9.
- `test_representative_selection_falls_back_to_input_order`:
  defensively constructed cluster where Score / PublishedAt /
  len(Body) / ID all tie (synthetic test-only state); representative
  == input-order-first; WARN log emitted with
  `fallback_reason:"representative_tiebreaker_exhausted"`.
- `test_no_cluster_ever_dropped`: input of 10 docs forming 3 clusters
  → output has exactly 3 representatives, 0 dropped clusters.
- `test_doc_id_count_invariant`: |input doc_ids| == |union of rep IDs
  and cluster member IDs|; no doc_id loss under any code path.

### REQ-SYN3-005 — Optional: mode=off pass-through

- `test_mode_off_returns_input_unchanged`: input slice identity-equal
  to output slice (test asserts the SAME slice header is returned;
  no copy made).
- `test_mode_off_no_metadata_mutation`: post-call, no input doc's
  `Metadata` map contains the `spec_syn003_cluster` key.
- `test_mode_off_no_embedder_call`: mock embedder records zero calls.
- `test_mode_off_passthrough_counter`: counter
  `outcome="passthrough", mode="off"` == 1.
- `test_mode_off_overhead_under_1ms`: 1000-iteration benchmark
  asserts `Cluster` adds ≤ 1 ms vs. a no-op identity function.

### NFR-SYN3-001 — Latency

- `test_cluster_simhash_only_p95_under_50ms`: 50-doc input, 100
  iterations, `simhash_only` mode; assert p95 ≤ 50 ms, p99 ≤ 100 ms.
- `test_cluster_hybrid_p95_under_1500ms`: 50-doc input, 100
  iterations with mocked embedder returning realistic latency
  (300-800 ms uniform); assert p95 ≤ 1500 ms.
- `test_synth_e2e_p50_preserved`: end-to-end query benchmark with
  SPEC-SYN-003 enabled (`simhash_only`) vs. baseline (no clustering);
  assert SPEC-SYN-001 NFR-SYN-001 p50 ≤ 8 s holds in both
  configurations.

### NFR-SYN3-002 — Property: doc_id traceability + idempotence

- `test_property_doc_id_traceability` (fuzz/quick): for generated
  `(docs)` inputs (10–200 docs, mixed Korean + English, varying
  scores), assert every input `doc.ID` appears exactly once in
  `union(reps, members)` post-`Cluster`.
- `test_property_representative_not_member_of_self`: for any rep `r`
  in output, `r.ID NOT IN metadata.MembersOf(r)`.
- `test_property_idempotence`: `Cluster(Cluster(docs))` produces
  representatives where every cluster has size 1 (no further
  collapse possible).

---

## 5. Technical Approach (high-level, no implementation code)

Detailed plan, file impact, and test plan live in
`.moai/specs/SPEC-SYN-003/plan.md`. High-level approach:

- **Insertion point**: `cmd/usearch/query.go:229-258`. Between the
  fanout result and the synthesis call. The all-adapters-failed
  guard (line 245) runs FIRST; clustering is skipped on empty input.
- **Package boundary**: `internal/synthcluster/` is a new top-level
  internal package, not a sub-package of `fanout` or `synthesis`.
  This keeps the fanout invariant (`@MX:ANCHOR` at
  `internal/fanout/dedup.go:26`) untouched and signals at the
  package-tree level that clustering is a distinct pipeline stage.
- **No schema change**: `NormalizedDoc` is unchanged. Cluster
  membership lives in `Metadata["spec_syn003_cluster"]` —
  schema-non-breaking, namespaced, excluded from `CanonicalHash()`
  by design.
- **SimHash mechanics**: 64-bit Charikar SimHash. Tokenizer: NFC
  normalization → character 3-shingles. Hash function for shingles:
  SHA-1 first 8 bytes (or library-provided primitive). The choice
  between an external library (`github.com/mfonda/simhash`) and a
  hand-rolled implementation is deferred to Run phase per
  Approach-First rule, with Korean tokenization fit as the
  tiebreaker (research §3.2 and §4).
- **Mode selection**: `DEDUPCLUSTER_MODE` env var. Default
  `simhash_only` is conservative — no embedding RTT in the query
  path. `hybrid` is opt-in for operators willing to pay the embedder
  call. `off` is pass-through for debugging or emergency rollback.
- **Embedder fallback**: graceful, observable. Sidecar errors and
  context cancellation BOTH route to the SimHash-only result. The
  fallback counter and WARN log make rate visible to operators.
- **Cluster representative selection**: 4-tier comparator
  (Score → PublishedAt → len(Body) → ID); deterministic for replay;
  defensive input-order fallback per REQ-SYN3-004.

---

## 6. Risks (top-level summary)

Detailed risk register lives in `.moai/specs/SPEC-SYN-003/research.md`
§8. Top three for SPEC-author attention:

1. **Korean paraphrase recall** (R2) — char-3-shingle SimHash may
   miss Korean paraphrase pairs that mecab-ko-tokenized SimHash
   would catch. Mitigated via tunable `DEDUPCLUSTER_HAMMING_THRESHOLD`
   env var; empirical recall floor measured by SPEC-EVAL-003 (M8).
2. **Cluster member information loss to synthesizer** (R1) — the
   synthesizer sees only representatives. Cross-member citation
   propagation is a SPEC-SYN-005 candidate (NOT in M4). Mitigated by
   persisting cluster membership in `Metadata` for downstream
   auditing.
3. **Embedder query-path latency** (R3) — first time the query path
   calls the embedder sidecar. Mitigated by `dedup_mode=simhash_only`
   default and by graceful fallback on sidecar errors. Operators can
   set `dedup_mode=off` to bypass entirely.

---

## 7. References

Internal:

- `cmd/usearch/query.go:228-258` — fanout → synthesis seam
  (insertion point for SPEC-SYN-003).
- `internal/fanout/fanout.go:21-97` — `Fanout.Dispatch` (upstream
  producer of `[]NormalizedDoc`).
- `internal/fanout/dedup.go:30-54` — exact-URL dedup, SPEC-FAN-001
  invariant SPEC-SYN-003 preserves verbatim.
- `pkg/types/normalized_doc.go:40-106` — `NormalizedDoc` schema
  (unchanged) and `CanonicalHash()` (unchanged; `Metadata` excluded
  by design).
- `internal/embedder/types.go:9-44`, `internal/embedder/client.go` —
  embedder client (read-only consumer, optional refinement only).
- `internal/synthesis/types.go:38-58`, `internal/synthesis/client.go`
  — synthesis Go client (consumes the post-cluster slice; contract
  unchanged).
- `.moai/specs/SPEC-FAN-001/spec.md` — fanout dedup contract
  SPEC-SYN-003 sits downstream of.
- `.moai/specs/SPEC-SYN-001/spec.md` — synthesis sidecar contract
  SPEC-SYN-003 feeds into.
- `.moai/specs/SPEC-SYN-002/spec.md:159` — REQ-SYN2-001
  doc_id-trace invariant SPEC-SYN-003 preserves.
- `.moai/specs/SPEC-IDX-002/spec.md:1-80` — embedder service contract
  (status: implemented).
- `.moai/specs/SPEC-IDX-003/spec.md` — Korean tokenization service
  (informs the v0 char-shingle decision; mecab-ko upgrade deferred).
- `.moai/project/roadmap.md:65` — SPEC-SYN-003 row.
- `.moai/project/roadmap.md:124` — M4 3-way parallelization plan.
- `.moai/project/roadmap.md:151` — M4 exit criterion.
- `.moai/specs/SPEC-SYN-003/research.md` — companion research artifact.

External:

- Charikar, M. (2002). Similarity estimation techniques from rounding
  algorithms. SimHash original.
- Manku, Jain, Das Sarma (WWW 2007). Detecting near-duplicates for
  web crawling. SimHash threshold tuning empirics. (To be verified
  via WebFetch in Run phase per anti-hallucination policy.)
- BGE-M3 paper (BAAI, 2024) §5 — cross-lingual cosine threshold
  guidance. (To be verified via WebFetch in Run phase.)
- `https://github.com/mfonda/simhash` — Go SimHash library candidate.
- Unicode UAX #15 — NFC normalization (text-tokenizer pre-processing).

---

*End of SPEC-SYN-003 v0.1 (draft).*
