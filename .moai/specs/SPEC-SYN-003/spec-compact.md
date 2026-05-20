# SPEC-SYN-003 Compact Reference

Compact (one-page) summary of `.moai/specs/SPEC-SYN-003/spec.md`.
For full context, EARS detail, and acceptance scenarios see the
companion files in this directory.

---

## What

A new Go package `internal/synthcluster/` clusters near-duplicate
search results between fanout (`SPEC-FAN-001`) and synthesis
(`SPEC-SYN-001`), reducing redundancy in the synthesizer's input
without losing any input doc_id.

Public API:

```
synthcluster.Cluster(ctx, docs, opts) ([]NormalizedDoc, Stats, error)
```

Insertion: `cmd/usearch/query.go` between line 229 (`docs :=
fanoutResult.Docs`) and line 258 (`synth.Synthesize(..., docs)`),
after the all-adapters-failed guard at line 245.

---

## Why

Fanout-side `internal/fanout/dedup.go` collapses **exact-URL** /
**exact-content-hash** duplicates only. The residual near-duplicates
that escape (mirror sites, paraphrased social commentary, cross-source
news syndication, Korean+English coverage of the same event) inflate
synthesis token cost and bias the synthesized paragraph toward
over-represented events. SPEC-SYN-003 collapses them to one
representative per cluster, persisting dropped doc_ids in the
representative's `Metadata`.

---

## How

| Stage | Mechanism |
|---|---|
| 1. SimHash digest | 64-bit Charikar SimHash over NFC-normalized **char-3-shingles** of `Title + "\n" + Body`. Korean-friendly without mecab-ko dependency. |
| 2. Candidate-pair filter | O(N²) pairwise Hamming-distance, threshold `DEDUPCLUSTER_HAMMING_THRESHOLD` (default 4). |
| 3. Cluster assembly | Union-Find over confirmed pairs (transitivity preserved). |
| 4. Cosine refinement (hybrid mode only) | One batched `embedder.Client.Embed` call over candidate-pair docs; cosine threshold `DEDUPCLUSTER_COSINE_THRESHOLD` (default 0.92); below-threshold pairs DEMOTED. |
| 5. Representative selection | 4-tier comparator: Score → PublishedAt → len(Body) → ID. |
| 6. Metadata write | Versioned struct under `Metadata["spec_syn003_cluster"]` with schema_version, members, simhash, dedup_mode, cluster_size. |

---

## Modes

| `DEDUPCLUSTER_MODE` | Behavior | Counter emitted (mode-exclusive — see D1 rule) |
|---|---|---|
| `simhash_only` (default) | Stages 1-3 + 5-6. No external service calls. p95 ≤ 50 ms (50 docs). | `simhash_clustered` per cluster |
| `hybrid` (success) | Stages 1-6. One embedder RTT. p95 ≤ 1500 ms when reachable. | `hybrid_refined` per surviving cluster |
| `hybrid` (degraded) | Graceful fallback to SimHash-only logic on any embedder error/timeout/ctx-cancel. | `embedding_fallback` once per call |
| `off` | Pass-through; input slice returned unchanged. ≤ 1 ms overhead. | `passthrough` once per call |

**Counter exclusivity invariant** (D1): In hybrid mode `simhash_clustered`
SHALL stay at 0 — only `hybrid_refined` (success) or `embedding_fallback`
(degradation) emits. See spec.md "Counter Semantics" subsection and
plan.md §6.5.

---

## EARS REQ index (5 patterns covered)

| ID | Pattern | Summary |
|---|---|---|
| REQ-SYN3-001 | Ubiquitous | SimHash always computed for every input doc; doc_id traceability + fanout invariant preserved. |
| REQ-SYN3-002 | Event-Driven | WHEN Hamming distance ≤ threshold → candidate pair into Union-Find. |
| REQ-SYN3-003 | State-Driven | WHILE mode=hybrid AND embedder reachable → cosine refinement; graceful fallback otherwise. |
| REQ-SYN3-004 | Unwanted | IF representative selection ties exhausted → input-order-first fallback, never drop a cluster. |
| REQ-SYN3-005 | Optional | WHERE mode=off → pass-through, no Metadata mutation, no external calls. |

NFRs:

- **NFR-SYN3-001** Latency: simhash_only p95 ≤ 50 ms, hybrid p95 ≤
  1500 ms, off ≤ 1 ms overhead. SPEC-SYN-001 p50 ≤ 8 s e2e preserved.
- **NFR-SYN3-002** Property: doc_id traceability + idempotence
  (`Cluster(Cluster(x))` produces no further collapse).

---

## What NOT to build (top exclusions)

- Semantic clustering for synthesis structure → SPEC-DEEP-001 (M5
  STORM).
- Cross-query memoization / shared-answer reuse → SPEC-IDX-005 (M6).
- Lossy member-id compression — every dropped doc_id is persisted.
- mecab-ko Korean tokenizer integration — char-3-shingles for v0;
  mecab-ko upgrade gated on SPEC-EVAL-003 (M8).
- `NormalizedDoc` schema change — `Metadata` extension only.
- Cross-member citation propagation in synthesizer → SPEC-SYN-005
  (NOT in M4).
- Replacement of SPEC-FAN-001 exact-URL dedup — preserved verbatim.
- Modifications to SPEC-SYN-002 citation-faithfulness contract.

---

## File impact (delta markers)

- `[NEW]` `internal/synthcluster/` (8 files, ~1080 production LOC)
- `[NEW]` `internal/obs/metrics/synthcluster.go`
- `[MODIFY]` `cmd/usearch/query.go` (~25 LOC inserted between :229 and
  :258)
- `[MODIFY]` `internal/obs/metrics/metrics.go` (collector
  registration + cardinality allowlist amendment)
- `[MODIFY]` `internal/obs/obs.go` (re-export collector handles)
- `[MODIFY]` `.env.example` (4 new env vars)
- `[EXISTING]` `pkg/types/normalized_doc.go` UNCHANGED
- `[EXISTING]` `internal/fanout/` UNCHANGED (consumed read-only via
  public API)
- `[EXISTING]` `internal/embedder/` UNCHANGED (consumed read-only)
- `[EXISTING]` `internal/synthesis/` UNCHANGED (downstream consumer
  unaware of clustering — only docs slice content changes)

---

## Quality gate

- 85% coverage on `internal/synthcluster/`
- `go vet` clean, `golangci-lint` clean, `go test -race` PASS
- Property test (NFR-SYN3-002) ≥ 1000 generated cases PASS
- Benchmarks meet NFR-SYN3-001 thresholds
- Pre-submission self-review documented in `progress.md`
- @MX tags: 1 ANCHOR on `Cluster`, ≥ 1 NOTE (char-shingle, comparator
  rule), 1 WARN on embedder fallback (with REASON)

---

## Cross-SPEC contracts preserved

- **SPEC-FAN-001** (`@MX:ANCHOR` at `internal/fanout/dedup.go:26`):
  exact-URL dedup runs FIRST; SPEC-SYN-003 runs strictly downstream.
- **SPEC-CORE-001** (`pkg/types/normalized_doc.go`): no schema
  change; `Metadata` excluded from `CanonicalHash()` so cluster
  annotation does not invalidate cached hashes.
- **SPEC-SYN-001** NFR-SYN-001 (p50 ≤ 8 s): preserved (e2e
  benchmark cross-check in NFR-SYN3-001).
- **SPEC-SYN-002** REQ-SYN2-001 (every claim resolves to a `doc_id`
  in input docs): preserved — every input doc_id is either a
  representative output ID or a recorded cluster member.
- **SPEC-IDX-002** embedder client contract: read-only consumer; no
  signature, error-sentinel, or sidecar contract change.

---

*End of SPEC-SYN-003 spec-compact.md.*
