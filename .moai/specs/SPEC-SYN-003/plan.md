# SPEC-SYN-003 Implementation Plan

Companion to `.moai/specs/SPEC-SYN-003/spec.md`.
Status: draft (pre-annotation).
Methodology: TDD (RED → GREEN → REFACTOR), per project default
(`.moai/config/sections/quality.yaml` `development_mode: tdd`).

---

## 1. Approach (one-page summary)

SPEC-SYN-003 introduces a NEW Go package `internal/synthcluster/` that
sits between the fanout output (`fanoutResult.Docs`) and the synthesis
input (`synth.Synthesize(..., docs)`) at `cmd/usearch/query.go:229-258`.
The package provides a single public function:

```
synthcluster.Cluster(ctx context.Context, docs []types.NormalizedDoc, opts Options)
    ([]types.NormalizedDoc, Stats, error)
```

It runs in three modes (env-configurable):

- `simhash_only` (default) — Charikar 64-bit SimHash + Hamming-distance
  candidate filter + Union-Find cluster assembly + 4-tier representative
  selection. No external service calls.
- `hybrid` — `simhash_only` + a single batched call to
  `internal/embedder.Client.Embed` for cosine-similarity refinement of
  candidate pairs. Falls back to `simhash_only` on any embedder error
  or context cancellation.
- `off` — pass-through; returns the input slice unchanged.

Output: a clustered `[]NormalizedDoc` slice where each cluster of size
≥ 2 is represented by ONE doc whose `Metadata["spec_syn003_cluster"]`
records the dropped near-duplicates' doc_ids and clustering metadata.
Every input doc_id is preserved exactly once across the union of
representative IDs and cluster member IDs (NFR-SYN3-002).

---

## 2. File Impact

### 2.1 NEW files

| File | Purpose | Approximate LOC |
|---|---|---|
| `internal/synthcluster/types.go` | `Options`, `Stats`, `Cluster` value types, `Mode` enum, error sentinels (`ErrInvalidMode`, etc.) | 80 |
| `internal/synthcluster/options.go` | `OptionsFromEnv()`, defaults, validation | 90 |
| `internal/synthcluster/simhash.go` | `simHash64(string) uint64`, NFC normalization helper, char-3-shingle tokenizer, Hamming distance helper | 120 |
| `internal/synthcluster/cluster.go` | `Cluster()` entry point, candidate-pair filter (O(N²) Hamming), Union-Find assembly, representative selector (4-tier), `Stats` accumulation | 200 |
| `internal/synthcluster/embed_refine.go` | Hybrid-mode cosine refinement: batch embedder call + pairwise cosine over candidate pairs + edge demotion + fallback handling | 150 |
| `internal/synthcluster/metadata.go` | Reader/writer for `Metadata["spec_syn003_cluster"]` versioned schema; member list manipulation | 80 |
| `internal/synthcluster/observability.go` | `emit()` helper wrapping the `obs.SynthClusterOutcomes` and `obs.SynthClusterMembers` collectors + structured logger | 60 |
| `internal/obs/metrics/synthcluster.go` | Two new Prometheus collectors and `registerSynthCluster(pr)` helper | 60 |
| `internal/synthcluster/types_test.go` | Options validation, defaults, env override | 80 |
| `internal/synthcluster/simhash_test.go` | Determinism, NFC behavior, Hamming distance | 100 |
| `internal/synthcluster/cluster_test.go` | Mode dispatch, Union-Find correctness, representative selection, all 5 REQ acceptance tests | 350 |
| `internal/synthcluster/embed_refine_test.go` | Hybrid-mode happy path + 4 fallback paths (unreachable / timeout / ctx-cancelled / model-load) + counter assertions | 220 |
| `internal/synthcluster/metadata_test.go` | Metadata round-trip, schema version handling, namespace collision guard | 80 |
| `internal/synthcluster/property_test.go` | NFR-SYN3-002 property + idempotence test | 100 |
| `internal/synthcluster/bench_test.go` | NFR-SYN3-001 benchmarks (50-doc p95 / p99) | 80 |

Total NEW: ~1,850 LOC (1,080 production + 770 test).

### 2.2 MODIFY files

| File | Change | Approximate LOC delta |
|---|---|---|
| `cmd/usearch/query.go` | Insert `synthcluster.Cluster(...)` call between fanout result and synthesis call (lines 229-258 region). Read `Options` from env. Emit progress event. Pass clustered slice to `synth.Synthesize`. | +25 / -2 |
| `cmd/usearch/query_test.go` | Integration tests: mode=off pass-through, mode=simhash_only happy path, mode=hybrid with mocked embedder client | +180 |
| `internal/obs/metrics/metrics.go` | Register the two new collectors via `registerSynthCluster(pr)`; add new label-value pre-declarations to the cardinality allowlist for `mode` label (`simhash_only`, `hybrid`, `off`) and for `outcome` label (`passthrough`, `simhash_clustered`, `hybrid_refined`, `embedding_fallback`) | +20 |
| `internal/obs/obs.go` | Re-export `obs.SynthClusterOutcomes`, `obs.SynthClusterMembers` for caller convenience | +6 |
| `.env.example` | Add `DEDUPCLUSTER_MODE=simhash_only`, `DEDUPCLUSTER_HAMMING_THRESHOLD=4`, `DEDUPCLUSTER_COSINE_THRESHOLD=0.92`, `DEDUPCLUSTER_EMBEDDING_TIMEOUT_MS=1500` with explanatory comments | +12 |

### 2.3 EXISTING — UNCHANGED (verified by integration test suite)

| File | Why touched but verified unchanged |
|---|---|
| `pkg/types/normalized_doc.go` | Schema unchanged; `Metadata` is the only extension surface. `CanonicalHash()` invariants preserved (Metadata excluded by design). NFR-CORE-001 < 1 µs/op preserved. |
| `internal/fanout/dedup.go` | Exact-URL/Hash dedup runs FIRST; SPEC-FAN-001 `@MX:ANCHOR` invariant preserved. SPEC-SYN-003 runs strictly downstream. |
| `internal/fanout/fanout.go` | `Dispatch` signature and semantics unchanged. SPEC-FAN-001 NFR-FAN-* preserved. |
| `internal/embedder/client.go` | Read-only consumer; no signature, no error-sentinel, no contract change. |
| `internal/embedder/types.go` | Unchanged. |
| `internal/synthesis/client.go` | Synthesis sidecar contract unchanged; SPEC-SYN-003 changes only the **content** of the docs slice, not the shape. |
| `internal/synthesis/types.go` | Unchanged. |
| `services/researcher/` | Python sidecar unchanged. SPEC-SYN-002 doc_id-faithfulness contract continues to be enforced over the post-cluster slice. |

---

## 3. Milestone Sequencing (priority-based, no time estimates)

### Milestone P0-A — Skeleton + RED (priority HIGH)

Order: 1 of 5.

- Create `internal/synthcluster/` package directory.
- Add `types.go` with `Options`, `Stats`, `Mode` enum, error
  sentinels.
- Add `options.go` with `OptionsFromEnv()` and validation.
- Add stub `cluster.go` with `Cluster()` returning
  `(nil, Stats{}, errors.New("not implemented"))`.
- Write all tests for REQ-SYN3-001 through REQ-SYN3-005 (RED phase).
  Tests fail on the stub.
- Write property test for NFR-SYN3-002 (RED).
- Write benchmark scaffolding for NFR-SYN3-001 (RED — benchmarks
  fail on a no-op function trivially).

Exit gate: `go test ./internal/synthcluster/...` runs and shows the
expected RED failures across all REQ test groups.

### Milestone P0-B — SimHash + Hamming + Union-Find (GREEN for REQ-SYN3-001, REQ-SYN3-002, REQ-SYN3-005)

Order: 2 of 5.

- Implement `simhash.go`: NFC normalization, char-3-shingle
  tokenizer, 64-bit Charikar SimHash.
- Implement candidate-pair filter in `cluster.go`: O(N²) pairwise
  Hamming with early-exit threshold.
- Implement Union-Find cluster assembly.
- Implement 4-tier representative selector.
- Implement `metadata.go` with `Metadata["spec_syn003_cluster"]`
  versioned schema reader/writer.
- Implement `mode == "off"` pass-through path.

Exit gate: REQ-SYN3-001, REQ-SYN3-002, REQ-SYN3-005 GREEN.
Property test (NFR-SYN3-002) GREEN. Benchmarks
(`simhash_only` p95/p99) GREEN. `hybrid` mode tests still RED.

### Milestone P0-C — Embedder fallback + observability (GREEN for REQ-SYN3-003, REQ-SYN3-004)

Order: 3 of 5.

- Implement `embed_refine.go`: batched embedder call (single
  `client.Embed` per `Cluster` invocation), pairwise cosine over
  candidate-pair docs, threshold-based pair demotion, graceful
  fallback to `simhash_only` on errors / ctx cancellation.
- Implement `observability.go` and wire all five outcome labels:
  `passthrough`, `simhash_clustered`, `hybrid_refined`,
  `embedding_fallback` (plus the WARN log on
  `embedding_fallback` and on representative tiebreaker exhaustion).
- Add the two new Prometheus collectors and the
  `registerSynthCluster(pr)` helper. Update the metric label
  allowlist to declare the new `mode` and `outcome` values.
- Re-export collector handles in `internal/obs/obs.go`.

Exit gate: All five EARS REQ tests GREEN. Cardinality allowlist
amendment validated by the existing observability test
(`internal/obs/metrics/metrics_test.go` — mirror SPEC-SYN-002 §2.1(g)
pattern). REQ-SYN3-004 representative-tiebreaker WARN log path
covered with synthetic test-only state.

### Milestone P0-D — Wire-up to query path (REFACTOR + integration)

Order: 4 of 5.

- Modify `cmd/usearch/query.go`: insert `synthcluster.Cluster(...)`
  between line 229 and line 258, AFTER the all-adapters-failed
  guard at line 245.
- Read `Options` from env via `synthcluster.OptionsFromEnv()`.
- Emit a progress event (`prog.Emit("dedupcluster", ...)`).
- Pass the post-cluster slice to `synth.Synthesize`.
- Add integration tests in `cmd/usearch/query_test.go` for all
  three modes.
- Update `.env.example` with the four new env vars.

Exit gate: `go test ./...` GREEN across the full project. Existing
SPEC-SYN-001 / SPEC-SYN-002 acceptance tests remain GREEN. End-to-end
query benchmark shows p50 ≤ 8 s preserved (NFR cross-check).

### Milestone P0-E — Quality gates + sync (REFACTOR + sync)

Order: 5 of 5.

- Pre-submission self-review per
  `.claude/rules/moai/workflow/workflow-modes.md` Pre-submission
  Self-Review section: review full diff for unnecessary abstractions
  / premature generalization.
- @MX tag application:
  - `synthcluster.Cluster` (public entry point, fan_in ≥ 3 expected:
    `cmd/usearch/query.go` + tests + future MCP wrapper) →
    `@MX:ANCHOR` with `@MX:REASON` and `@MX:SPEC: SPEC-SYN-003`.
  - SimHash compute loop (in `cluster.go`) → `@MX:NOTE` documenting
    the char-3-shingle Korean compromise.
  - Embedder fallback path (in `embed_refine.go`) → `@MX:WARN` with
    `@MX:REASON` documenting the budget invariant.
  - Representative selection comparator (in `cluster.go`) →
    `@MX:NOTE` documenting the 4-tier business rule.
- TRUST 5 gate: 85%+ coverage for `internal/synthcluster/` and the
  modified `cmd/usearch/query.go` regions; `go vet` zero;
  `golangci-lint` zero; race-detector PASS.
- Update `CHANGELOG.md` (sync-phase responsibility).

Exit gate: `manager-quality` PASS. Sync phase ready (not part of this
SPEC; deferred to `/moai sync SPEC-SYN-003`).

---

## 4. Test Plan Summary

(Detailed Given/When/Then in `acceptance.md`.)

| REQ / NFR | Test File | Test Count (planned) |
|---|---|---|
| REQ-SYN3-001 | `cluster_test.go`, `simhash_test.go`, `metadata_test.go` | 5 |
| REQ-SYN3-002 | `cluster_test.go` | 4 |
| REQ-SYN3-003 | `embed_refine_test.go` | 7 |
| REQ-SYN3-004 | `cluster_test.go` | 4 |
| REQ-SYN3-005 | `cluster_test.go` | 5 |
| NFR-SYN3-001 | `bench_test.go` | 3 (benchmarks + e2e cross-check) |
| NFR-SYN3-002 | `property_test.go` | 3 (property + 2 derived assertions) |
| Integration | `cmd/usearch/query_test.go` | 3 (one per mode) |
| **Total new tests** | | **~34** |

Coverage target: 85% for `internal/synthcluster/`. Coverage of
modified `cmd/usearch/query.go` regions: ≥ 85% on the diff.

---

## 5. Dependencies

### 5.1 Runtime dependencies

- `internal/embedder` (existing, SPEC-IDX-002) — read-only, opt-in.
- `internal/obs` (existing, SPEC-OBS-001) — collectors + logger.
- `pkg/types` (existing, SPEC-CORE-001) — `NormalizedDoc`,
  `ValidationError`, `Query` types.
- Standard library: `golang.org/x/text/unicode/norm` (NFC),
  `crypto/sha1` or `hash/maphash` (SimHash primitive — final choice
  in Run phase).

### 5.2 SPEC dependencies

- **depends_on**: SPEC-FAN-001 (provides input shape; invariant
  preserved), SPEC-CORE-001 (`NormalizedDoc` schema), SPEC-IDX-002
  (embedder client; opt-in consumer).
- **blocks**: SPEC-EVAL-001 (citation faithfulness benchmark consumes
  clustered output; faithfulness measurement requires deduplicated
  input).
- **does NOT depend on**: SPEC-SYN-002 (orthogonal — SYN-002 enforces
  citation faithfulness; SYN-003 reduces input redundancy. They share
  the doc_id traceability invariant but operate at different stages.
  M4 parallelization plan permits SYN-002 + SYN-003 + SYN-004 to
  develop concurrently).

### 5.3 Module dependencies

To be added to `go.mod`:

- `golang.org/x/text` (already present per `services/researcher/`
  Python side reference; verify Go side).
- Optional `github.com/mfonda/simhash` if not hand-rolled (decision
  in Run phase per Approach-First rule).

No Python dependencies. No new sidecars.

---

## 6. Risk Mitigations (linked to research §8)

| Risk ID | Mitigation in plan |
|---|---|
| R1 (representative info loss) | `Metadata["spec_syn003_cluster"]["members"]` persistence + member-id audit logging. Cross-member citation propagation deferred to SPEC-SYN-005. |
| R2 (Korean recall) | Tunable `DEDUPCLUSTER_HAMMING_THRESHOLD`; recall-floor measurement via SPEC-EVAL-003 (M8). |
| R3 (embedder latency) | `DEDUPCLUSTER_EMBEDDING_TIMEOUT_MS=1500` per-call deadline; fallback path tested in `test_embedder_timeout_falls_back_to_simhash`. |
| R4 (embedder unreachable) | `simhash_only` is first-class default; fallback is observable via counter and WARN log. |
| R5 (`Metadata` namespace collision) | Reserved-namespace test in `metadata_test.go` asserts no input doc has a `spec_syn003_*` key. Adapter contract tests (out of this SPEC's scope) enforce on the producer side. |
| R6 (concurrency races) | Public API takes/returns plain slices; internal goroutines (if any) write to per-index slices, mirroring SPEC-FAN-001 H1 pattern. `go test -race ./internal/synthcluster/...` PASS in exit gate. |

---

## 6.5 Decisions (D1 — Counter Exclusivity Rule)

Recorded 2026-05-09 in response to plan-auditor review-1 D1 finding.

**Decision**: The `usearch_synthcluster_outcomes_total` counter is
**mode-exclusive per cluster**, not additive across stages.

| Mode | Emits | Does NOT emit |
|---|---|---|
| `simhash_only` | `simhash_clustered` per cluster | `hybrid_refined`, `embedding_fallback`, `passthrough` |
| `hybrid` (success) | `hybrid_refined` per surviving cluster | `simhash_clustered`, `embedding_fallback`, `passthrough` |
| `hybrid` (degraded) | `embedding_fallback` once per call | `simhash_clustered`, `hybrid_refined`, `passthrough` |
| `off` | `passthrough` once per call | `simhash_clustered`, `hybrid_refined`, `embedding_fallback` |

**Rationale**: In hybrid mode the SimHash stage produces *candidate*
pairs; a cluster is not "confirmed" until cosine refinement runs OR
the function falls back. Emitting `simhash_clustered` on top of
`hybrid_refined` would double-count clusters; emitting it on the
fallback path would conflate the two modes' health signals and break
operator dashboards.

**Implementation impact**: `internal/synthcluster/observability.go`
`emit()` helper MUST guard the `simhash_clustered` increment with
`if mode == ModeSimhashOnly { ... }`. Fallback path increments
`embedding_fallback` only — does NOT also increment `simhash_clustered`
even though the underlying clustering is SimHash-derived.

**Test enforcement**:
- `test_simhash_clustered_counter_zero_in_hybrid_mode` (cluster_test.go)
- `test_hybrid_mode_simhash_clustered_counter_zero` (embed_refine_test.go)
- Acceptance §3.5/§3.6 assertions (`simhash_clustered` counter delta == 0).

## 7. Open questions (deferred to Run phase or out of scope)

- SimHash library choice (external `github.com/mfonda/simhash` vs.
  hand-rolled). Decision criterion: Korean tokenization fit. Resolve
  in Run phase Approach-First step.
- Whether to add a third mode `embedding_only` (skip SimHash filter,
  embed all pairs). Out of scope for v0; counter-indication is the
  O(N²) cost without SimHash filtering at N=200 docs. If
  benchmark data later supports it, add as a follow-up SPEC.
- Whether to expose `Stats` in the user-facing CLI output
  (`usearch query --debug` could surface cluster counts). Out of
  scope for v0; CLI output is a SPEC-CLI-002 concern (M7).

---

*End of SPEC-SYN-003 plan.md.*
