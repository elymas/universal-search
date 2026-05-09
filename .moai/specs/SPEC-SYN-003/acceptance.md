# SPEC-SYN-003 Acceptance Criteria

Companion to `.moai/specs/SPEC-SYN-003/spec.md`.
Format: Given / When / Then scenarios + edge-case enumeration + quality
gate criteria.

---

## 1. Scope

This document specifies the testable acceptance criteria for
SPEC-SYN-003 v0.1. Each scenario maps to one or more EARS REQs / NFRs
in `spec.md` §3. Together they constitute the Definition of Done.

---

## 2. Definition of Done (gate summary)

A scenario is "DONE" when ALL of the following hold:

1. The scenario's listed test(s) PASS in the project's standard `go
   test ./...` invocation.
2. The scenario's REQ/NFR ID is referenced in the test name comment
   block (`// REQ-SYN3-NNN: <one-line>`).
3. Coverage for the affected files in `internal/synthcluster/` is
   ≥ 85%.
4. `go vet ./...` is clean.
5. `golangci-lint run` is clean (project default config).
6. `go test -race ./internal/synthcluster/...` PASS.
7. SPEC-SYN-001 / SPEC-SYN-002 / SPEC-FAN-001 acceptance suites remain
   GREEN after SPEC-SYN-003 implementation (no regression).
8. Pre-submission self-review per project workflow-modes rule:
   reviewed full diff, no simpler approach found.

---

## 3. Given/When/Then Scenarios (minimum 3, expanded)

### 3.1 Scenario A — Identical-URL early dedup (already deduped by fanout — SPEC-SYN-003 sees no near-dups)

Maps to: REQ-SYN3-001 (invariant preservation),
REQ-SYN3-005 (no-op when no clusters needed), NFR-SYN3-002 (idempotence).

> **Given** a fanout output of 5 docs from 5 different sources, all
> with distinct canonical URLs and lexically distinct (title + body)
> content (Hamming distance > 4 between every pair),
>
> **When** `Cluster(ctx, docs, Options{Mode: "simhash_only",
> HammingThreshold: 4})` is called,
>
> **Then** the function returns `(reps, stats, nil)` where
> `len(reps) == 5`, every `reps[i].ID == docs[i].ID`,
> NO doc has `Metadata["spec_syn003_cluster"]` written,
> `stats.ClustersFormed == 0`, and the
> `usearch_synthcluster_outcomes_total{outcome="simhash_clustered",
> mode="simhash_only"}` counter is unchanged.
>
> **And** running `Cluster` a second time on `reps` returns the same
> 5 docs (idempotence — NFR-SYN3-002 property holds).

### 3.2 Scenario B — Near-dup paraphrase clustering (English mirror sites)

Maps to: REQ-SYN3-001 (SimHash always computed),
REQ-SYN3-002 (Hamming threshold triggers candidate pair),
REQ-SYN3-004 (representative selection rule).

> **Given** 3 docs `A`, `B`, `C` from 3 different sources, where:
>   - `A.URL = "https://reuters.com/world/2026/event-x"`,
>     `A.Title = "Event X happened in Seoul"`,
>     `A.Body = "Major incident X occurred today in Seoul ..."`,
>     `A.Score = 0.8`, `A.PublishedAt = 2026-04-01T10:00:00Z`.
>   - `B.URL = "https://ap.example/2026/event-x"`,
>     `B.Title = "Event X took place in Seoul"`,
>     `B.Body = "Major incident X took place earlier today in Seoul ..."`,
>     `B.Score = 0.6`, `B.PublishedAt = 2026-04-01T10:30:00Z`.
>   - `C.URL = "https://ai-news.example/totally-different"`,
>     `C.Title = "Tech AI breakthrough"`,
>     `C.Body = "Researchers announced a new AI model ..."`,
>     `C.Score = 0.5`.
>
>   Constructed such that `Hamming(A.simhash, B.simhash) <= 4` and
>   `Hamming(A.simhash, C.simhash) > 4` and `Hamming(B.simhash,
>   C.simhash) > 4`,
>
> **When** `Cluster(ctx, [A, B, C], Options{Mode: "simhash_only",
> HammingThreshold: 4})` is called,
>
> **Then** the function returns `(reps, stats, nil)` where
> `len(reps) == 2`, the representative of cluster {A, B} is `A`
> (Score 0.8 > 0.6 wins on the primary tiebreaker),
> `A.Metadata["spec_syn003_cluster"]["members"] == ["B.ID"]`,
> `A.Metadata["spec_syn003_cluster"]["cluster_size"] == 2`,
> `A.Metadata["spec_syn003_cluster"]["dedup_mode"] == "simhash_only"`,
> `A.Metadata["spec_syn003_cluster"]["schema_version"] == 1`,
> `C` appears unchanged (no `Metadata` mutation; size-1 clusters do
> NOT receive the metadata key per acceptance §4.1.4),
> `stats.ClustersFormed == 1`,
> `usearch_synthcluster_outcomes_total{outcome="simhash_clustered",
> mode="simhash_only"} += 1`,
> `usearch_synthcluster_members` histogram observes the value `2`.

### 3.3 Scenario C — Korean paraphrase clustering (cross-source Naver + Daum)

Maps to: REQ-SYN3-001 (Korean text via NFC + char-3-shingles),
REQ-SYN3-002 (Hamming threshold cross-lingual),
REQ-SYN3-004 (representative selection on Korean docs).

> **Given** 2 docs `K1`, `K2` from Naver News and Daum News
> respectively, both reporting the same Korean-language news event:
>   - `K1.SourceID = "naver"`,
>     `K1.URL = "https://n.news.naver.com/article/2026/05/event-y"`,
>     `K1.Title = "오늘 서울에서 사건 Y가 발생했다"`,
>     `K1.Body = "오늘 오전 서울 강남구에서 사건 Y가 발생하였다고 ..."`,
>     `K1.Lang = "ko"`, `K1.Score = 0.7`,
>     `K1.PublishedAt = 2026-05-09T08:00:00Z`.
>   - `K2.SourceID = "daum"`,
>     `K2.URL = "https://news.daum.net/article/2026/event-y-koreaaaa"`,
>     `K2.Title = "서울에서 오늘 사건 Y 발생"`,
>     `K2.Body = "오늘 서울 강남구에서 사건 Y가 발생하였다 ..."`,
>     `K2.Lang = "ko"`, `K2.Score = 0.5`,
>     `K2.PublishedAt = 2026-05-09T08:15:00Z`.
>
>   Such that NFC normalization + char-3-shingle SimHash produces
>   `Hamming(K1.simhash, K2.simhash) <= 4`,
>
> **When** `Cluster(ctx, [K1, K2], Options{Mode: "simhash_only",
> HammingThreshold: 4})` is called,
>
> **Then** the function returns `(reps, stats, nil)` where
> `len(reps) == 1`, the representative is `K1` (Score 0.7 > 0.5),
> `K1.Metadata["spec_syn003_cluster"]["members"] == ["K2.ID"]`,
> `stats.ClustersFormed == 1`,
> NFC normalization is applied (assert via instrumentation hook in
> test mode that the tokenizer received NFC-form input — confirms
> that "사건" composed and decomposed Hangul forms hash identically).

### 3.4 Scenario D — Pass-through mode (mode=off)

Maps to: REQ-SYN3-005 (Optional pattern), NFR-SYN3-001 (overhead).

> **Given** 50 docs from a realistic mixed-adapter fanout output
> (Reddit + HN + Naver + arXiv + GitHub),
>
> **When** `Cluster(ctx, docs, Options{Mode: "off"})` is called,
>
> **Then** the function returns `(docs, stats, nil)` where
> `len(reps) == 50`, every `reps[i].ID == docs[i].ID`,
> NO doc has `Metadata["spec_syn003_cluster"]` written,
> `stats.Mode == "off"`,
> the embedder client is NOT invoked (mock records 0 calls),
> the SimHash function is NOT invoked (mock records 0 calls — verified
> via test-only seam, not test the production code-path),
> `usearch_synthcluster_outcomes_total{outcome="passthrough",
> mode="off"} += 1`,
> a 1000-iteration benchmark of the pass-through path adds ≤ 1 ms vs.
> a no-op identity function (NFR-SYN3-001 sub-clause).

### 3.5 Scenario E — Hybrid mode embedder fallback (sidecar unreachable)

Maps to: REQ-SYN3-003 (State-Driven graceful fallback).

> **Given** 4 docs `A`, `B`, `C`, `D` where SimHash produces 2
> candidate pairs (A,B) and (C,D), AND a mocked embedder client
> configured to return `embedder.ErrSidecarUnreachable` on every
> call,
>
> **When** `Cluster(ctx, [A,B,C,D], Options{Mode: "hybrid",
> HammingThreshold: 4, CosineThreshold: 0.92,
> EmbeddingTimeoutMs: 1500})` is called,
>
> **Then** the function returns `(reps, stats, nil)` where
> `len(reps) == 2` (A and C, with B and D as cluster members
> respectively, on the SimHash-only fallback verdict),
> the embedder mock recorded exactly ONE call attempt (not retried),
> `usearch_synthcluster_outcomes_total{outcome="embedding_fallback",
> mode="hybrid"} += 1` (exactly once per call regardless of cluster
> count),
> **`usearch_synthcluster_outcomes_total{outcome="simhash_clustered",
> mode="hybrid"}` delta == 0** (D1 counter-exclusivity: even though
> the fallback uses SimHash-only clustering logic, the `simhash_clustered`
> counter MUST NOT fire in hybrid mode),
> `usearch_synthcluster_outcomes_total{outcome="hybrid_refined",
> mode="hybrid"}` delta == 0,
> exactly ONE WARN-level log record was emitted with attributes
> `{request_id, dedup_mode:"hybrid",
> embedding_error:"embedder: sidecar unreachable",
> fallback_to:"simhash_only"}`,
> `stats.EmbeddingFallback == true`.

### 3.5b Scenario E2 — Hybrid mode embedder timeout (D2 dedicated path)

Maps to: REQ-SYN3-003 (State-Driven graceful fallback via timeout).
Distinct from §3.5 (sidecar unreachable) and from §4.3.5 (ctx-cancelled
mid-embed by external caller). This scenario isolates the
`Options.EmbeddingTimeoutMs` per-call deadline trigger.

> **Given** 4 docs `A`, `B`, `C`, `D` where SimHash produces 2
> candidate pairs (A,B) and (C,D), AND a mocked embedder client
> configured to **block** on every `Embed` call for a duration
> strictly greater than `Options.EmbeddingTimeoutMs` (e.g. block
> 3000 ms when timeout is 1500 ms — guaranteeing the per-call
> deadline fires before any response is returned),
>
> **When** `Cluster(ctx, [A,B,C,D], Options{Mode: "hybrid",
> HammingThreshold: 4, CosineThreshold: 0.92,
> EmbeddingTimeoutMs: 1500})` is called with a parent `ctx` that is
> NOT externally cancelled (the per-call deadline is the sole trigger),
>
> **Then** the function returns `(reps, stats, nil)` (NOT an error —
> the timeout triggers graceful fallback, not propagation),
> the wall-clock duration of the call is approximately
> `EmbeddingTimeoutMs ± 100 ms` jitter (asserts the per-call deadline
> fires, not the parent ctx),
> `len(reps) == 2` (A and C, with B and D as cluster members
> respectively, via the SimHash-derived fallback verdict — all clusters
> preserved with simhash-derived members),
> `usearch_synthcluster_outcomes_total{outcome="embedding_fallback",
> mode="hybrid"} += 1`,
> **`usearch_synthcluster_outcomes_total{outcome="simhash_clustered",
> mode="hybrid"}` delta == 0** (D1 counter-exclusivity assertion),
> `usearch_synthcluster_outcomes_total{outcome="hybrid_refined",
> mode="hybrid"}` delta == 0,
> exactly ONE WARN-level log record was emitted with attributes
> `{request_id, dedup_mode:"hybrid",
> embedding_error:<wraps context.DeadlineExceeded>,
> fallback_to:"simhash_only"}` — the `embedding_error` value MUST
> contain the substring `"deadline exceeded"` or unwrap to
> `context.DeadlineExceeded` (distinct from §3.5's
> `ErrSidecarUnreachable` and from §4.3.5's `context.Canceled`),
> `stats.EmbeddingFallback == true`,
> `stats.EmbeddingFallbackReason == "timeout"` (or equivalent
> machine-readable discriminator distinguishing this path from
> unreachable/cancelled).

### 3.6 Scenario F — Hybrid mode cosine demotion (SimHash false positive)

Maps to: REQ-SYN3-003 (cosine refinement demotes pair).

> **Given** 2 docs `D1`, `D2` where SimHash produces a candidate pair
> (Hamming distance ≤ 4) BUT the cosine similarity over the embedder
> dense vectors is 0.78 (< 0.92 threshold) — a SimHash false positive,
>
> **When** `Cluster(ctx, [D1, D2], Options{Mode: "hybrid",
> HammingThreshold: 4, CosineThreshold: 0.92})` is called with a
> mocked embedder returning realistic vectors that produce cosine 0.78,
>
> **Then** the function returns `(reps, stats, nil)` where
> `len(reps) == 2` (the candidate pair was DEMOTED — D1 and D2 remain
> distinct), neither has a `Metadata["spec_syn003_cluster"]` key,
> `stats.ClustersFormed == 0`,
> `stats.PairsDemotedByCosine == 1`,
> `usearch_synthcluster_outcomes_total{outcome="hybrid_refined",
> mode="hybrid"}` is unchanged (no cluster confirmed),
> **`usearch_synthcluster_outcomes_total{outcome="simhash_clustered",
> mode="hybrid"}` delta == 0** (D1 counter-exclusivity: even when
> hybrid refinement demotes every pair, the SimHash counter MUST stay
> at 0 in hybrid mode),
> `usearch_synthcluster_outcomes_total{outcome="embedding_fallback",
> mode="hybrid"}` delta == 0 (embedder succeeded, no fallback),
> NO WARN log emitted.

### 3.7 Scenario G — Representative tiebreaker exhaustion (defensive)

Maps to: REQ-SYN3-004 (Unwanted pattern fallback).

> **Given** a synthetic test-only construction of 2 docs in a cluster
> that tie on Score, PublishedAt, len(Body), AND ID (a degenerate
> case requiring identical-ID input docs, which `Validate()`
> guarantees against at the input boundary; this scenario uses an
> internal test seam to inject the degenerate case),
>
> **When** `Cluster` is called with `Options{Mode: "simhash_only"}`
> and the cluster is selected internally,
>
> **Then** the function returns `(reps, stats, nil)` where the
> representative is the input-order-first doc, the cluster is NOT
> dropped, exactly ONE WARN-level log record is emitted with
> attributes `{request_id, cluster_id, member_ids,
> fallback_reason:"representative_tiebreaker_exhausted"}`, and
> `stats.RepresentativeFallbacks == 1`.

### 3.8 Scenario H — Empty input

Maps to: REQ-SYN3-001 (defensive boundary), REQ-SYN3-005 (no-op).

> **Given** an empty `docs` slice (`len(docs) == 0`),
>
> **When** `Cluster(ctx, []NormalizedDoc{}, anyOptions)` is called in
> any of the three modes,
>
> **Then** the function returns `([]NormalizedDoc{}, Stats{}, nil)`
> immediately, no embedder call, no SimHash compute, no log records,
> no counter increments.
>
> **Note**: in production, this scenario does not occur because
> `cmd/usearch/query.go:245` short-circuits all-adapters-failed before
> the SPEC-SYN-003 call site. The test exists for the package-level
> contract.

---

## 4. Edge Cases

### 4.1 Cluster size handling

1. **Cluster size 1** (no near-dup): NO `Metadata["spec_syn003_cluster"]`
   key written. The doc passes through unchanged.
2. **Cluster size 2**: representative + one member. Member's doc_id in
   `members` array.
3. **Cluster size ≥ 3** (transitive via Union-Find): representative +
   N-1 members. All non-representative IDs in `members`.
4. **All N docs in one cluster** (degenerate Korean repeat-news case):
   `len(reps) == 1`, `members` contains N-1 IDs.

### 4.2 SimHash + Hamming edge cases

1. **Empty Title and Body**: SimHash digest is the digest of the empty
   shingle set (deterministic; same digest for all empty docs). Test
   asserts that 2 empty docs cluster together (acceptable
   degenerate case; defensive — Validate() does NOT require non-empty
   Title/Body).
2. **Very short Body** (< 3 chars): char-3-shingles produces a very
   small shingle set or none. SimHash digest is computed over the
   minimal shingle set (Title + Body has ≥ 3 chars in practice for
   any real adapter output).
3. **Title equal, Body differs**: depends on shingle weighting. Test
   asserts that two docs with identical Title and very different Body
   are NOT collapsed (Hamming > 4).
4. **HammingThreshold = 0**: only byte-identical SimHash digests
   cluster. Effectively a duplicate detection; acceptance test
   confirms.
5. **HammingThreshold = 64**: every pair clusters. Pathological mode;
   acceptance test confirms `stats.ClustersFormed == 1` (all docs in
   one cluster).

### 4.3 Hybrid-mode edge cases

1. **Embedder returns vectors with zero norm** (degenerate case):
   cosine is undefined; treat as 0.0; pair is demoted (cosine < 0.92).
2. **Embedder returns fewer vectors than requested texts** (sidecar
   contract violation): treat as `embedder.ErrInvalidRequest`,
   trigger fallback path, increment `embedding_fallback` counter.
3. **No candidate pairs** (Hamming filter rejected everything): the
   embedder is NOT called (optimization — no pairs to refine).
   Counter `outcome="hybrid_refined"` does NOT increment.
4. **All candidate pairs demoted by cosine**: returns same docs
   verbatim, `stats.ClustersFormed == 0`,
   `stats.PairsDemotedByCosine == <num candidate pairs>`.
5. **Context cancelled mid-embed**: graceful fallback, counter
   `embedding_fallback += 1`, WARN log includes
   `embedding_error: "context: cancelled"` or similar.

### 4.4 Metadata edge cases

1. **Input doc already has `Metadata["spec_syn003_cluster"]` key**
   (re-clustering case or adapter contract violation): test asserts
   the key is OVERWRITTEN if the doc becomes a representative, or
   IGNORED if the doc becomes a member (the input value is
   informational only — production correctness is the
   producer's responsibility).
2. **Input doc has `Metadata == nil`**: representative selection
   initializes the map before writing the cluster key.
3. **`Metadata` round-trip through JSON**: cluster key survives
   marshal/unmarshal via `json.Marshal(NormalizedDoc)` (acceptance
   test asserts).

### 4.5 NormalizedDoc validation edge cases

1. **Input doc fails `Validate()`** (e.g. empty ID): SPEC-SYN-003
   does NOT re-validate. Validation is a producer-side responsibility
   (adapter contract). The function processes the doc as-is, but the
   property test asserts that a failing `Validate()` doc, if present,
   does NOT corrupt the doc_id traceability invariant for the
   surviving docs.
2. **Two input docs with identical IDs** (adapter contract violation):
   the function's behavior is undefined; production correctness
   requires unique IDs from upstream. A defensive test asserts the
   function does not panic; the result may have unexpected
   representative selection (input-order-first wins per REQ-SYN3-004
   fallback).

### 4.6 NFR-SYN3-001 latency test method

| Test | Configuration | Iterations | Target |
|---|---|---|---|
| `BenchmarkCluster_SimhashOnly_50docs` | mode=simhash_only, 50 docs (mixed Korean + English, varying lengths 50-500 chars) | 100 | p95 ≤ 50 ms, p99 ≤ 100 ms |
| `BenchmarkCluster_Hybrid_50docs_FastEmbed` | mode=hybrid, 50 docs, mocked embedder uniformly distributed 300-800 ms | 100 | p95 ≤ 1500 ms |
| `BenchmarkCluster_Off_50docs` | mode=off, 50 docs | 1000 | mean overhead vs. no-op identity ≤ 1 ms |
| `TestSynthE2E_P50_Preserved_With_SYN003` | mode=simhash_only enabled in query path | 20 (e2e) | SPEC-SYN-001 NFR-SYN-001 p50 ≤ 8 s holds |

Reporting: `go test -bench=. -benchmem ./internal/synthcluster/...`
output captured in `progress.md` post-Run-phase.

---

## 5. Quality Gate Criteria

A SPEC-SYN-003 implementation passes the quality gate when:

| Gate | Tool | Threshold |
|---|---|---|
| Unit tests | `go test ./internal/synthcluster/...` | 100% pass |
| Integration tests | `go test ./cmd/usearch/...` | 100% pass; SPEC-SYN-001 / SPEC-SYN-002 / SPEC-FAN-001 acceptance tests remain GREEN |
| Coverage | `go test -coverprofile=cover.out` | ≥ 85% line coverage on `internal/synthcluster/` |
| Race | `go test -race` | PASS |
| Vet | `go vet ./...` | 0 findings |
| Lint | `golangci-lint run` | 0 findings |
| Property | `testing/quick` or fuzz harness | ≥ 1000 generated-input cases PASS |
| Benchmarks | `go test -bench=.` | NFR-SYN3-001 thresholds met |
| Self-review | Pre-submission review per workflow-modes rule | One pass; documented in `progress.md` |
| @MX tags | manual placement | 1 ANCHOR (`Cluster`) + ≥ 1 NOTE (representative comparator, char-shingle compromise) + 1 WARN (embedder fallback path) all with REASON for ANCHOR/WARN |

---

## 6. Definition of Done — checklist

- [ ] All 5 EARS REQs (SYN3-001 through SYN3-005) have GREEN tests
      with `// REQ-SYN3-NNN:` comment headers.
- [ ] Both NFRs (SYN3-001 perf, SYN3-002 property) have GREEN
      benchmarks / property tests.
- [ ] `internal/synthcluster/` package coverage ≥ 85%.
- [ ] `cmd/usearch/query.go` integration covered for all three modes.
- [ ] SPEC-SYN-001 / SPEC-SYN-002 / SPEC-FAN-001 acceptance suites
      remain GREEN — no regression.
- [ ] `go vet` clean. `golangci-lint` clean. `go test -race` PASS.
- [ ] Pre-submission self-review documented in `progress.md`.
- [ ] @MX tags applied per `plan.md` §3 Milestone P0-E.
- [ ] `.env.example` updated with the four new env vars + comments.
- [ ] `internal/obs/metrics/` cardinality allowlist amended to
      declare new `mode` and `outcome` label values.
- [ ] No NEW dependency on `pkg/types/normalized_doc.go` schema.
- [ ] No NEW dependency on `internal/fanout/` internals (only the
      public `Result.Docs` field is consumed).
- [ ] No modification to `internal/synthesis/` Go side or
      `services/researcher/` Python side.
- [ ] `Metadata["spec_syn003_cluster"]` schema_version 1 round-trips
      through JSON and is consumable by future SPECs (SPEC-SYN-005).

---

*End of SPEC-SYN-003 acceptance.md.*
