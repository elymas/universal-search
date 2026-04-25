---
id: SPEC-CORE-001
title: Adapter Interface and NormalizedDoc Contract (Compact)
milestone: M2 — Foundation contracts (inserted)
status: draft
priority: P0
generated_from: spec.md, plan.md, acceptance.md
generated_at: 2026-04-26
---

# SPEC-CORE-001 (Compact)

Condensed reference. For full context see `spec.md`, `plan.md`,
`acceptance.md`, `research.md`.

## EARS Requirements (Summary)

- **REQ-CORE-001 (Ubiquitous, P0)**: `pkg/types.NormalizedDoc` SHALL expose 15
  exported fields with documented JSON tags + `Validate() error` + `CanonicalHash() string`.
- **REQ-CORE-002 (Ubiquitous, P0)**: `pkg/types.Adapter` SHALL define exactly four
  methods (`Name`, `Search`, `Healthcheck`, `Capabilities`); `Query` (6 fields) and
  `Capabilities` (10 fields) shall be public value types.
- **REQ-CORE-003 (Event-Driven, P0)**: WHEN `Registry.Register(a)` is called,
  the registry SHALL store under write-lock, reject duplicate names with
  `*RegistryError` wrapping `ErrDuplicateAdapter`, and validate auth env
  vars per REQ-CORE-006.
- **REQ-CORE-004 (Event-Driven, P0)**: WHEN wrappedAdapter.Search resolves,
  the wrapper SHALL emit exactly 1 counter increment + 1 histogram
  observation + 1 OTel span + 1 slog record; outcome ∈ {success, failure,
  timeout, rate_limited, unavailable}; underlying error preserved via `%w`.
- **REQ-CORE-005 (State-Driven, P0)**: WHILE concurrent goroutines call Get/List/Register,
  the registry SHALL admit unbounded reader concurrency via `sync.RWMutex`
  with `-race` clean; `List()` returns sorted names.
- **REQ-CORE-006 (Optional, P1)**: WHERE `Capabilities.RequiresAuth == true`,
  registry SHALL validate `Capabilities.AuthEnvVars` are set unless
  `RegisterOptions.SkipAuthCheck == true`.
- **REQ-CORE-007 (Unwanted, P0)**: IF `NormalizedDoc.Validate()` is called with
  any of {ID, SourceID, URL, RetrievedAt} empty/zero, THEN return typed
  `*ValidationError` AND do NOT silently coerce.
- **REQ-CORE-008 (Ubiquitous, P0)**: `pkg/types` SHALL expose 4 sentinel errors
  (`ErrTransient`, `ErrPermanent`, `ErrRateLimited`, `ErrSourceUnavailable`),
  `Category` enum, `*SourceError` typed error with `Is`/`Unwrap`,
  `CategorizeError(err) Category`, `OutcomeFromError(err) string`.

NFRs:
- **NFR-CORE-001**: `Validate()` < 1 µs/op on amd64; `CanonicalHash()` < 5 µs/op.
- **NFR-CORE-002**: outcome label values bounded to enumerated set; extends
  SPEC-OBS-001's `TestNoUnboundedLabels` allowlist.
- **NFR-CORE-003**: `pkg/types/` has zero direct imports from `internal/`,
  `prometheus/client_golang`, or `go.opentelemetry.io/otel`.

## Acceptance Criteria (Summary)

- 12 Given/When/Then scenarios (full text in `acceptance.md` §1).
- 6 edge-case behaviors (nil ctx, empty query, oversized result, ctx
  cancel mid-Search, malformed timestamp on unmarshal, Metadata circular
  refs).
- Quality gates: ≥ 85% coverage in pkg/types and internal/adapters; race
  clean; vet/lint clean; benchmarks meet thresholds; @MX:ANCHOR/WARN/NOTE
  placed per plan.md §4.

## Files to Modify

Created (15 files):
- `pkg/types/normalized_doc.go` + `_test.go`
- `pkg/types/adapter.go` + `_test.go`
- `pkg/types/query.go` + `_test.go`
- `pkg/types/capabilities.go` + `_test.go`
- `pkg/types/errors.go` + `_test.go`
- `pkg/types/types_test.go` (import-discipline test)
- `pkg/types/bench_test.go` (NFR-CORE-001)
- `internal/adapters/registry.go` + `_test.go`
- `internal/adapters/noop/noop.go` + `_test.go`

Modified (2 files):
- `pkg/types/types.go` — replace 2-line stub with package doc
- `internal/adapters/adapters.go` — replace 4-line stub with package doc

Unchanged (by design):
- `internal/obs/metrics/metrics.go` (AdapterCalls + AdapterCallDuration
  already registered; allowlist already includes `adapter` + `outcome`)
- `internal/llm/*` (no dependency)
- `cmd/usearch*/main.go` (registry construction is M3 SPEC-IR-001 / SPEC-FAN-001 concern)
- `go.mod` / `go.sum` (zero new direct deps)

## Exclusions (What NOT to Build)

- Per-source adapter implementations → SPEC-ADP-001..009 (M3)
- Fanout / parallel dispatch / dedup → SPEC-FAN-001 (M3)
- Index ingestion (Qdrant/Meili/PG) → SPEC-IDX-001..003 (M3)
- Synthesis consumption (gpt-researcher wrapper) → SPEC-SYN-001 (M2), SPEC-SYN-002 (M4)
- Cache layer / 5-phase access fallback → SPEC-CACHE-001 (M3)
- Auth / RBAC / audit (team-scoped credentials) → SPEC-AUTH-001..003 (M6)
- Embedding generation (BGE-M3 sidecar) → SPEC-IDX-002 (M3)
- Korean tokenization (Meilisearch + mecab-ko) → SPEC-IDX-003 (M3)
- Streaming Search (channel-based) → SPEC-SYN-004 (M4) if measured to need it
- Adapter health-state machine (auto-disable/re-enable) → SPEC-EVAL-002 (M8)
- Per-adapter custom metrics → per-adapter SPECs
- gRPC contract between Go and Python services → separate `proto/` SPEC
- Adapter discovery / plugin loading at runtime → post-V1
- Pre-flight Query validation → FAN-001 owns query preprocessing
- Multi-adapter result fusion (RRF, BGE-reranker) → SPEC-IDX-001 (M3)

## Dependencies

- `depends_on`: [SPEC-BOOT-001]
- Soft dependency on SPEC-OBS-001 (uses obs.Obs DI; nil-safe)
- `blocks`: [SPEC-IR-001, SPEC-ADP-001..009, SPEC-FAN-001, SPEC-IDX-001]

---

*End of spec-compact.md v0.1*
