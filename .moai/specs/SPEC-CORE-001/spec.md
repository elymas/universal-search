---
id: SPEC-CORE-001
title: Adapter Interface and NormalizedDoc Contract
milestone: M2 — Foundation contracts (inserted)
status: implemented
priority: P0
owner: expert-backend
methodology: tdd
coverage_target: 85
created: 2026-04-26
updated: 2026-04-26
author: limbowl
issue_number: null
depends_on: [SPEC-BOOT-001]
blocks: [SPEC-IR-001, SPEC-ADP-001, SPEC-ADP-002, SPEC-ADP-003, SPEC-ADP-004, SPEC-ADP-005, SPEC-ADP-006, SPEC-ADP-007, SPEC-ADP-008, SPEC-ADP-009, SPEC-FAN-001, SPEC-IDX-001]
---

# SPEC-CORE-001: Adapter Interface and NormalizedDoc Contract

## 1. Purpose

SPEC-BOOT-001 reserved `pkg/types/` (`pkg/types/types.go`) and
`internal/adapters/` (`internal/adapters/adapters.go`) as empty stubs.
SPEC-OBS-001 registered the `usearch_adapter_calls_total{adapter,outcome}`
counter and `usearch_adapter_call_duration_seconds{adapter}` histogram
(`internal/obs/metrics/metrics.go:86-101`) with `adapter` and `outcome`
in the cardinality allowlist (`internal/obs/metrics/metrics.go:147-154`).
SPEC-LLM-001 demonstrated the canonical orchestration patterns: provider
priority router with RWMutex (`internal/llm/router.go:148-198`),
per-call observability emission (`internal/llm/client.go:230-252`), and
typed-error classification (`internal/llm/retry.go:14-50`).

SPEC-CORE-001 fills `pkg/types/` and `internal/adapters/` with the
**foundational contract** that all 12+ M3 adapter SPECs depend on:

- A canonical `NormalizedDoc` struct (public, in `pkg/types/`) describing
  every search result regardless of source — the unified shape that
  Reddit, Hacker News, arXiv, GitHub, YouTube, Bluesky, X, SearXNG,
  Naver, Daum, KoreaNewsCrawler, RSS, and Polymarket all converge to.
- An `Adapter` interface (public, in `pkg/types/`) defining the four
  methods every source adapter must implement: `Name`, `Search`,
  `Healthcheck`, `Capabilities`.
- A `Query` value type (public) and `Capabilities` descriptor (public)
  that together let the Intent Router (SPEC-IR-001) make routing
  decisions without consulting individual adapters.
- A unified error taxonomy (`ErrTransient`, `ErrPermanent`,
  `ErrRateLimited`, `ErrSourceUnavailable`) plus a typed `*SourceError`
  carrying HTTP status + RetryAfter context, with a `CategorizeError(err)
  Category` classifier consumed by SPEC-FAN-001 fanout retry logic.
- An adapter registry (`internal/adapters/registry.go`) providing
  concurrency-safe `Register` / `Get` / `List`, mirroring the
  `internal/llm/router.go` Router pattern.
- A wrappedAdapter inside the registry that emits ONE Prometheus counter
  increment + ONE histogram observation + ONE OTel span + ONE slog
  record per Search call — so all 12 adapter implementations stay free
  of observability boilerplate. This single source of truth uses the
  collectors SPEC-OBS-001 already registered; CORE-001 ships zero new
  metric families.
- A reference `noop` adapter (`internal/adapters/noop/`) that satisfies
  the interface, doubles as a compile-time contract check, and serves
  as a test fixture for SPEC-FAN-001 / SPEC-IR-001 / SPEC-IDX-001 development.

Completion unblocks every M3 adapter SPEC (SPEC-ADP-001 through
SPEC-ADP-009 — seven independent SPECs that can now develop in parallel),
the Intent Router SPEC-IR-001 (consumes Capabilities for routing tables),
the Fanout SPEC-FAN-001 (iterates `Adapter.Search` with retry policies
keyed on the error taxonomy), and the Index ingestion SPEC-IDX-001
(consumes `[]NormalizedDoc` for hybrid index population).

This SPEC is inserted into M2 as the first contract layer; M2's first-
end-to-end-slice exit criterion (`.moai/project/roadmap.md` line 147,
"`usearch query 'hello world'` returns Reddit + HN results with one
synthesized paragraph + citations") becomes achievable only after
CORE-001 is merged: ADP-001 cannot return results without
`NormalizedDoc`, and synthesis cannot consume them without the typed
contract.

## 2. Scope

### 2.1 In-Scope

| Ref | Item |
|-----|------|
| a | `pkg/types/normalized_doc.go`: `NormalizedDoc` struct (15 fields), JSON tags, `Validate() error` method, `CanonicalHash() string` derived field |
| b | `pkg/types/adapter.go`: `Adapter` interface (4 methods: `Name`, `Search`, `Healthcheck`, `Capabilities`) |
| c | `pkg/types/query.go`: `Query` struct (Text, Lang, MaxResults, Filters, Cursor, Deadline) and `Filter` shape |
| d | `pkg/types/capabilities.go`: `Capabilities` struct (10 fields), `DocType` enum (article/post/paper/video/repo/issue/social/other), Filter constants |
| e | `pkg/types/errors.go`: 4 sentinel errors (`ErrTransient`, `ErrPermanent`, `ErrRateLimited`, `ErrSourceUnavailable`), `Category` enum, `*SourceError` typed error with `Unwrap`/`Is`, `CategorizeError(err) Category`, `OutcomeFromError(err) string` returning the Prometheus label value |
| f | `internal/adapters/registry.go`: `Registry` struct (sync.RWMutex), `NewRegistry(o *obs.Obs) *Registry`, `Register(Adapter) error`, `Get(name string) (Adapter, bool)`, `List() []string`, internal `wrappedAdapter` that emits metrics+span+log per Search call |
| g | `internal/adapters/registry_test.go`: contract tests covering REQ-CORE-003/004/005/006/007 + concurrency safety + nil-Obs handling |
| h | `internal/adapters/noop/noop.go`: reference adapter under 50 LoC implementing the interface; static compile-time interface assertion via `var _ types.Adapter = (*Adapter)(nil)`; used by FAN-001 / IR-001 tests as a stable fixture |
| i | `internal/adapters/noop/noop_test.go`: unit tests confirming the noop satisfies the interface, returns deterministic results, and handles ctx cancellation |
| j | JSON (de)serialization round-trip tests for `NormalizedDoc` covering all 15 fields including the `Metadata map[string]any` with mixed-type values |
| k | Five-value `outcome` Prometheus label set for the wrappedAdapter: `success`, `failure`, `timeout`, `rate_limited`, `unavailable` (refines SPEC-OBS-001's outcome enumeration without breaking the existing allowlist) |
| l | Public-package import-boundary regression: extends SPEC-OBS-001's `TestNoUnboundedLabels` cardinality-guard principle — `pkg/types/` MUST have zero imports from `internal/`, `prometheus/client_golang`, or `go.opentelemetry.io/otel` |

### 2.2 Out-of-Scope

- **Per-source adapter implementations** — Reddit, Hacker News, arXiv,
  GitHub, YouTube, Bluesky, X, SearXNG, Naver, Daum, KoreaNewsCrawler,
  RSS, Polymarket each land in their own SPEC-ADP-* (M3 owners listed in
  `.moai/project/roadmap.md` lines 44-56).
- **Fanout / parallel dispatch / partial-result assembly / dedup** —
  belongs to SPEC-FAN-001 (M3). CORE-001 provides the `Adapter` contract;
  FAN-001 provides the goroutine pool that calls into many adapters.
- **Index ingestion** — `Qdrant` / `Meilisearch` / `Postgres` writes of
  `[]NormalizedDoc` belong to SPEC-IDX-001..003 (M3).
- **Synthesis consumption** — gpt-researcher wrapper consuming
  `[]NormalizedDoc` for citation assembly belongs to SPEC-SYN-001 (M2)
  and SPEC-SYN-002 (M4).
- **Cache layer / 5-phase access fallback** — SPEC-CACHE-001 (M3).
- **Auth / RBAC / audit** — SPEC-AUTH-001..003 (M6). The
  `Capabilities.RequiresAuth` field is *declarative* (says "this adapter
  needs an env var"); the *enforcement* of team-scoped credentials is
  M6's concern.
- **Embedding generation** — SPEC-IDX-002 (M3) consumes `NormalizedDoc.Body`
  for BGE-M3 embedding; CORE-001 just defines the field.
- **Korean tokenization** — SPEC-IDX-003 (M3) handles Meilisearch /
  mecab-ko side concerns; CORE-001 defines `NormalizedDoc.Lang` as a
  BCP-47 string and lets downstream consumers route on it.
- **Streaming Search results** (channel-based `Search(ctx, q) <-chan
  NormalizedDoc`) — V1 returns `[]NormalizedDoc`; streaming is reserved
  for SPEC-SYN-004 (M4) if its measurements warrant it.
- **Adapter health-state machine** (auto-disable on
  `ErrSourceUnavailable`, re-enable on Healthcheck pass) — SPEC-EVAL-002
  (M8) owns adapter-reliability dashboard concerns and may consume the
  registry but won't modify its state in V1.
- **Per-adapter custom metrics** (e.g., a Reddit-specific
  `reddit_pagination_pages_total` counter) — adapter SPECs own their
  own additional metrics; CORE-001's wrappedAdapter only emits the
  shared adapter metric family.
- **gRPC contract between Go and Python services** — `proto/` layer is
  separate (`structure.md:159-165`); CORE-001 stays Go-internal.

## 3. EARS Requirements

| ID | Pattern | Requirement | Priority | Acceptance Summary |
|----|---------|-------------|----------|--------------------|
| REQ-CORE-001 | Ubiquitous | The package `pkg/types` SHALL expose a `NormalizedDoc` struct containing exactly these 15 exported fields with JSON tags as specified: `ID string` (`json:"id"`), `SourceID string` (`json:"source_id"`), `URL string` (`json:"url"`), `Title string` (`json:"title"`), `Body string` (`json:"body"`), `Snippet string` (`json:"snippet"`), `PublishedAt time.Time` (`json:"published_at"`), `RetrievedAt time.Time` (`json:"retrieved_at"`), `Author string` (`json:"author"`), `Score float64` (`json:"score"`), `Lang string` (`json:"lang"`), `DocType DocType` (`json:"doc_type"`), `Citations []string` (`json:"citations,omitempty"`), `Metadata map[string]any` (`json:"metadata,omitempty"`), `Hash string` (`json:"hash"`); a `Validate() error` method returning a typed error when any of `{ID, SourceID, URL, RetrievedAt}` is empty/zero; and a `CanonicalHash() string` method computing a deterministic hash of `{SourceID, URL, Title, Body}` (the content-only quartet, excluding mutable metadata). | P0 | `TestNormalizedDocFieldSet` confirms exact field count + names + JSON tags via reflection; `TestNormalizedDocJSONRoundTrip` round-trips a fully-populated doc through `json.Marshal`/`json.Unmarshal` with byte-equal field semantics including a Metadata map of mixed types; `TestNormalizedDocValidateRequiredFields` table-driven over the four required fields; `TestNormalizedDocCanonicalHashStable` confirms deterministic output across two computations of the same doc. |
| REQ-CORE-002 | Ubiquitous | The package `pkg/types` SHALL expose an `Adapter` interface with exactly these four methods: `Name() string`, `Search(ctx context.Context, q Query) ([]NormalizedDoc, error)`, `Healthcheck(ctx context.Context) error`, `Capabilities() Capabilities`; the `Query` struct SHALL contain `Text string`, `Lang string` (BCP-47), `MaxResults int`, `Filters []Filter`, `Cursor string`, `Deadline time.Time`; the `Capabilities` struct SHALL contain `SourceID string`, `DisplayName string`, `DocTypes []DocType`, `SupportedLangs []string`, `SupportsSince bool`, `RequiresAuth bool`, `AuthEnvVars []string`, `RateLimitPerMin int`, `DefaultMaxResults int`, `Notes string`. | P0 | `TestAdapterInterfaceShape` confirms via `reflect.TypeOf((*types.Adapter)(nil)).Elem()` that exactly four methods exist with the named signatures; `TestQueryStructFields` and `TestCapabilitiesStructFields` confirm field counts and types; `TestNoopAdapterImplementsInterface` provides a compile-time check via `var _ types.Adapter = (*noop.Adapter)(nil)`. |
| REQ-CORE-003 | Event-Driven | WHEN `(*Registry).Register(a Adapter)` is called with a previously-unregistered adapter (`a.Name()` not present in the registry), the registry SHALL store the wrappedAdapter under that name in its internal map under write-lock; WHEN `Register` is called with an adapter whose `Name()` collides with an already-registered entry, the registry SHALL return a typed `*RegistryError` wrapping `ErrDuplicateAdapter` and SHALL NOT modify its state; WHEN `Register` is called with `a.Capabilities().RequiresAuth == true` and any value in `a.Capabilities().AuthEnvVars` is unset in the process environment, the registry SHALL return `*RegistryError` wrapping `ErrMissingAuth` (overridable by `RegisterOptions{SkipAuthCheck: true}` for tests). | P0 | `TestRegisterSucceedsForNewAdapter`, `TestRegisterRejectsDuplicateName`, `TestRegisterValidatesAuthEnvVars`, `TestRegisterAllowsSkipAuthCheck`. The duplicate-rejection test confirms the registry state is unchanged after the error returns. |
| REQ-CORE-004 | Event-Driven | WHEN a wrappedAdapter's `Search(ctx, q)` returns (success, failure, timeout, rate-limited, or source-unavailable), the registry's wrappedAdapter SHALL (a) increment `obs.Metrics.AdapterCalls.WithLabelValues(name, outcome)` exactly once where outcome ∈ `{success, failure, timeout, rate_limited, unavailable}`; (b) observe `obs.Metrics.AdapterCallDuration.WithLabelValues(name)` with the elapsed seconds; (c) start and end an OTel span named `adapter.search` with attributes `adapter.name`, `adapter.outcome`, `adapter.result_count`, recording the underlying error via `span.RecordError` on non-success outcomes; (d) emit one slog record at level INFO (success) or WARN (non-success) via `obs.Logger` with attributes `{adapter, outcome, elapsed_seconds, result_count, request_id}`. The wrapper SHALL NOT alter the underlying error returned to the caller (preserve via `%w` wrapping when adding context). | P0 | `TestWrappedAdapterEmitsCounter` × 5 (one per outcome value); `TestWrappedAdapterEmitsHistogram`; `TestWrappedAdapterCreatesOTelSpan`; `TestWrappedAdapterEmitsSlogRecord`; `TestWrappedAdapterPreservesUnderlyingError` confirms `errors.Is(returned, original)` holds; `TestWrappedAdapterSafeOnNilObs` confirms passing `nil` for `obs.Metrics` and/or `obs.Logger` does not panic — the wrapper degrades gracefully (per the existing nil-guard pattern at `internal/llm/client.go:244-251`). |
| REQ-CORE-005 | State-Driven | WHILE multiple goroutines invoke `(*Registry).Get` and `(*Registry).List` concurrently with one goroutine invoking `(*Registry).Register`, the registry SHALL admit unbounded reader concurrency via `sync.RWMutex` with no torn reads, no panics, and no missed writes; List SHALL return adapter names in sorted (lexicographic) order to provide deterministic iteration for downstream consumers (FAN-001's fanout dispatch, IR-001's routing table dump). | P0 | `TestRegistryConcurrentReadWrite` runs 100 readers + 1 writer for 1 second under `-race`; asserts no race detector alarms and final List length is consistent with successful Register calls; `TestListReturnsSortedNames` registers names in non-alphabetical order and asserts the result is sorted. |
| REQ-CORE-006 | Optional | WHERE an adapter declares `Capabilities.RequiresAuth == true`, the registry SHALL validate at registration time that every value in `Capabilities.AuthEnvVars` is set in the process environment via `os.LookupEnv`; the registry MAY accept a `RegisterOptions{SkipAuthCheck: true}` to bypass this validation (for tests, dev environments, or pre-flight registration before secrets are loaded); WHERE the adapter declares `RequiresAuth == false`, the registry SHALL skip the env validation regardless of `AuthEnvVars` content. | P1 | `TestRegisterRequiresAuthEnvVars` covers the four states (RequiresAuth × AuthEnvVars-set) × (SkipAuthCheck on/off) = 4 cases. |
| REQ-CORE-007 | Unwanted | IF `NormalizedDoc.Validate()` is called on a doc with any of `{ID, SourceID, URL, RetrievedAt}` empty or zero, THEN the function SHALL return a typed `*ValidationError` containing the offending field name, AND SHALL NOT silently coerce, default, or auto-fill any field; IF `Adapter.Search` returns docs whose `Validate()` would fail, the wrappedAdapter SHALL still increment counters and log the call, but FAN-001 / downstream consumers MAY filter invalid docs (FAN-001 owns this policy, not CORE-001). | P0 | `TestValidateRejectsMissingID`, `TestValidateRejectsMissingSourceID`, `TestValidateRejectsMissingURL`, `TestValidateRejectsZeroRetrievedAt`, `TestValidateAcceptsCompleteDoc`, `TestValidationErrorWrapsFieldName` confirms `errors.As(err, &ve); ve.Field` resolves correctly. |
| REQ-CORE-008 | Ubiquitous | The package `pkg/types` SHALL expose four sentinel errors (`ErrTransient`, `ErrPermanent`, `ErrRateLimited`, `ErrSourceUnavailable`); a `Category` enum (`CategoryTransient`, `CategoryPermanent`, `CategoryRateLimited`, `CategoryUnavailable`, `CategoryUnknown`); a typed `*SourceError` struct with fields `{Adapter string, Category Category, HTTPStatus int, Cause error, RetryAfter time.Duration}` implementing `Error() string`, `Unwrap() error`, and `Is(target error) bool` such that `errors.Is(srcErr, ErrTransient)` returns true when `srcErr.Category == CategoryTransient`; a `CategorizeError(err error) Category` function that returns the Category by inspecting `*SourceError` via `errors.As`, with `context.DeadlineExceeded` mapping to `CategoryTransient`; an `OutcomeFromError(err error) string` returning the Prometheus label value (`"success"` for nil, `"timeout"` for `context.DeadlineExceeded`, `"rate_limited"` / `"unavailable"` / `"failure"` for the categorised err). | P0 | `TestSentinelErrorsExist`, `TestSourceErrorIsMatchesSentinels`, `TestSourceErrorUnwrapsCause`, `TestCategorizeErrorTable` (table-driven over 7 input shapes), `TestOutcomeFromErrorTable` (5 outputs). |

## 4. Non-Functional Requirements

| ID | Name | Requirement |
|----|------|-------------|
| NFR-CORE-001 | Performance | `NormalizedDoc.Validate()` SHALL execute in less than 1 microsecond per doc on amd64 (`go test -bench`) when called on a fully-populated doc with all 15 fields set. The benchmark `BenchmarkNormalizedDocValidate` lives at `pkg/types/bench_test.go` and runs in CI on a scheduled weekly job, matching the cadence of SPEC-OBS-001 NFR-OBS-001. Runtime cost is bounded by 4 string-empty checks + 1 time-zero check; no allocation per call. |
| NFR-CORE-002 | Cardinality Safety | The five `outcome` label values emitted by the wrappedAdapter (`success`, `failure`, `timeout`, `rate_limited`, `unavailable`) plus an internal `transient` value used only when classification cannot be more precise SHALL be the complete enumeration accepted by the wrappedAdapter; no other value MAY be passed to `obs.Metrics.AdapterCalls.WithLabelValues`. The static-scan `TestAdapterOutcomeLabels` extends SPEC-OBS-001's `TestNoUnboundedLabels` allowlist with this enumeration. The `adapter` label is bounded by the V1 registry size (≤14 adapters per `.moai/project/roadmap.md`). |
| NFR-CORE-003 | Public-API Stability | The package `pkg/types` SHALL declare zero direct imports from `internal/`, `github.com/prometheus/client_golang`, `go.opentelemetry.io/otel`, or any third-party SDK. The package SHALL depend only on Go stdlib (`context`, `time`, `errors`, `crypto`, etc.). Verified via `TestPkgTypesNoInternalImports` walking `go list -deps -json` output. Rationale: `pkg/types` is the SDK boundary (`.moai/project/structure.md:159-164`); breaking changes here require major-version bump for any external Go consumer. |

## 5. Acceptance Criteria

### REQ-CORE-001 — NormalizedDoc

- File `pkg/types/normalized_doc.go` declares `NormalizedDoc` with exactly the
  15 fields specified in §3, in the documented order, with the documented
  JSON tags, and with a single line of godoc per field explaining its
  purpose and any invariant (e.g., URL must be canonical, Hash is
  content-only).
- `Validate() error` returns nil for a complete doc; returns a typed
  `*ValidationError` containing the offending field name when any of
  `ID`, `SourceID`, `URL`, or `RetrievedAt` is empty/zero.
- `CanonicalHash() string` returns a 16-character lowercase hex string
  computed from `{SourceID, URL, Title, Body}`; calling it twice on the
  same doc yields byte-equal output.
- `TestNormalizedDocFieldSet`, `TestNormalizedDocJSONRoundTrip`,
  `TestNormalizedDocValidateRequiredFields`, `TestNormalizedDocCanonicalHashStable`,
  `TestCanonicalHashIgnoresMetadata` all pass.

### REQ-CORE-002 — Adapter / Query / Capabilities

- File `pkg/types/adapter.go` declares the `Adapter` interface with the
  four methods specified in §3.
- File `pkg/types/query.go` declares `Query` with the six fields
  specified.
- File `pkg/types/capabilities.go` declares `Capabilities` with the ten
  fields specified plus a `DocType` enum
  (`DocTypeArticle`, `DocTypePost`, `DocTypePaper`, `DocTypeVideo`,
  `DocTypeRepo`, `DocTypeIssue`, `DocTypeSocial`, `DocTypeOther`).
- `TestAdapterInterfaceShape`, `TestQueryStructFields`,
  `TestCapabilitiesStructFields`, `TestDocTypeEnumComplete` all pass.

### REQ-CORE-003 — Registry Register

- File `internal/adapters/registry.go` declares `Registry` with `sync.RWMutex`-
  protected internal `map[string]Adapter`.
- `NewRegistry(o *obs.Obs) *Registry` constructs an empty registry and
  retains the obs bundle for wrappedAdapter use.
- `Register(a Adapter) error` succeeds for a new name; returns
  `*RegistryError{Op: "register", Name: name, Cause: ErrDuplicateAdapter}`
  for a colliding name; returns `*RegistryError{Op: "register", Name:
  name, Cause: ErrMissingAuth}` when auth env vars missing (per
  REQ-CORE-006).
- `RegisterOptions{SkipAuthCheck bool}` available via `RegisterWithOptions`.
- Internal storage stores a `wrappedAdapter` (not the raw adapter), so all
  subsequent `Get`-returned adapters are observability-instrumented.
- `TestRegisterSucceedsForNewAdapter`, `TestRegisterRejectsDuplicateName`,
  `TestRegisterValidatesAuthEnvVars`, `TestRegisterAllowsSkipAuthCheck`,
  `TestRegisterStateUnchangedOnError` all pass.

### REQ-CORE-004 — wrappedAdapter Observability

- The wrappedAdapter is internal to `internal/adapters/`; its `Search`
  method calls into the wrapped adapter and emits exactly: 1 counter
  increment, 1 histogram observation, 1 OTel span, 1 slog record per
  call.
- `outcome` is computed by `OutcomeFromError(err)` (REQ-CORE-008).
- Underlying error is preserved via `%w` wrapping; `errors.Is(returned,
  originalErr)` holds.
- Nil-guards on `obs.Metrics`, `obs.Metrics.AdapterCalls`,
  `obs.Metrics.AdapterCallDuration`, `obs.Logger` per the existing
  pattern at `internal/llm/client.go:244-251`.
- `TestWrappedAdapterEmitsCounterSuccess`,
  `TestWrappedAdapterEmitsCounterFailure`,
  `TestWrappedAdapterEmitsCounterTimeout`,
  `TestWrappedAdapterEmitsCounterRateLimited`,
  `TestWrappedAdapterEmitsCounterUnavailable`,
  `TestWrappedAdapterEmitsHistogram`,
  `TestWrappedAdapterCreatesOTelSpan`,
  `TestWrappedAdapterEmitsSlogRecord`,
  `TestWrappedAdapterPreservesUnderlyingError`,
  `TestWrappedAdapterSafeOnNilObs` all pass.

### REQ-CORE-005 — Concurrency

- `TestRegistryConcurrentReadWrite` spawns 100 readers and 1 writer over
  1 second; `-race` returns clean.
- `TestListReturnsSortedNames` confirms registration in mixed order
  yields sorted output.

### REQ-CORE-006 — Auth Env Validation

- `TestRegisterRequiresAuthEnvVars` covers the 4-cell truth table:
  - `RequiresAuth=true`, env set, `SkipAuthCheck=false` → success
  - `RequiresAuth=true`, env unset, `SkipAuthCheck=false` → ErrMissingAuth
  - `RequiresAuth=true`, env unset, `SkipAuthCheck=true` → success
  - `RequiresAuth=false`, env unset, `SkipAuthCheck=false` → success

### REQ-CORE-007 — Validation Rejection

- `TestValidateRejectsMissingID`, `TestValidateRejectsMissingSourceID`,
  `TestValidateRejectsMissingURL`, `TestValidateRejectsZeroRetrievedAt`,
  `TestValidateAcceptsCompleteDoc`, `TestValidationErrorWrapsFieldName`
  all pass.

### REQ-CORE-008 — Error Taxonomy

- `pkg/types/errors.go` declares the four sentinels, Category enum, and
  `*SourceError` struct.
- `*SourceError.Is(target)` returns true when `target` is the sentinel
  matching `Category`.
- `*SourceError.Unwrap()` returns the inner `Cause`.
- `CategorizeError(err)` correctly classifies all 7 input shapes in the
  table-driven test (nil, ErrTransient, ErrPermanent, ErrRateLimited,
  ErrSourceUnavailable, context.DeadlineExceeded, arbitrary error).
- `OutcomeFromError(err)` returns one of the 5 enumerated label values.

### NFR-CORE-001 — Validate Performance

- `BenchmarkNormalizedDocValidate` reports < 1 µs/op on amd64.
- `BenchmarkNormalizedDocCanonicalHash` reports < 5 µs/op on amd64.

### NFR-CORE-002 — Cardinality Safety

- `TestAdapterOutcomeLabels` extends the SPEC-OBS-001 allowlist test;
  all 5 emitted values are in the allowlist.
- A canary test attempts `wrappedAdapter` with a synthetic adapter whose
  classification slips past the enum; the test fails loudly (test-only
  assertion, panicking on disallowed label values).

### NFR-CORE-003 — pkg/types Import Discipline

- `TestPkgTypesNoInternalImports` walks `go list -deps -json` for
  `github.com/elymas/universal-search/pkg/types/...`; no listed import
  starts with `internal/`, `prometheus/`, or `go.opentelemetry.io/`.

## 6. Technical Approach

### 6.1 Package Layout

```
pkg/types/
├── normalized_doc.go    # NormalizedDoc, Validate, CanonicalHash, ValidationError
├── normalized_doc_test.go
├── adapter.go           # Adapter interface
├── adapter_test.go      # interface shape verification
├── query.go             # Query, Filter
├── query_test.go
├── capabilities.go      # Capabilities, DocType enum
├── capabilities_test.go
├── errors.go            # sentinels, Category, SourceError, CategorizeError, OutcomeFromError
├── errors_test.go
└── bench_test.go        # NFR-CORE-001 benchmarks

internal/adapters/
├── adapters.go          # (replace stub) package doc, ANCHOR placement
├── registry.go          # Registry, NewRegistry, Register, Get, List, wrappedAdapter
├── registry_test.go     # REQ-CORE-003/004/005/006 tests
└── noop/
    ├── noop.go          # reference Adapter under 50 LoC; compile-time interface check
    └── noop_test.go
```

### 6.2 Type Sketch (`pkg/types/adapter.go`)

```go
package types

import (
    "context"
    "time"
)

// Adapter is the contract every search source implements.
//
// @MX:ANCHOR: [AUTO] Adapter contract; callers: 12+ M3 adapters, registry, FAN-001, IR-001, tests
// @MX:REASON: fan_in >= 12; sole boundary between source-specific code and orchestration
type Adapter interface {
    // Name is the stable adapter identifier (e.g., "reddit", "hackernews").
    // Must match Capabilities.SourceID. Used as the Prometheus label value.
    Name() string

    // Search executes a query and returns normalized results.
    // Implementations MUST honour ctx cancellation. Errors SHOULD be wrapped
    // in *SourceError carrying the appropriate Category.
    Search(ctx context.Context, q Query) ([]NormalizedDoc, error)

    // Healthcheck probes the adapter's external dependency. Returns nil when
    // the source is reachable. Cheap; called by SPEC-EVAL-002 dashboard.
    Healthcheck(ctx context.Context) error

    // Capabilities returns adapter-static metadata. Called once at startup
    // by the Intent Router (SPEC-IR-001). MUST be deterministic.
    Capabilities() Capabilities
}

// Query is a normalized search request.
type Query struct {
    Text       string
    Lang       string    // BCP-47 (e.g., "ko", "en", "ja"); empty = no preference
    MaxResults int       // upper bound; adapter may return fewer
    Filters    []Filter  // adapter-specific filters; opaque to fanout
    Cursor     string    // adapter-specific pagination cursor; opaque
    Deadline   time.Time // soft deadline; honour via ctx.WithDeadline upstream
}

// Filter is one adapter-specific constraint (e.g., {"date_from": "2026-01-01"}).
type Filter struct {
    Key   string
    Value string
}
```

### 6.3 NormalizedDoc Sketch (`pkg/types/normalized_doc.go`)

```go
type NormalizedDoc struct {
    ID          string         `json:"id"`            // unique within (SourceID, URL)
    SourceID    string         `json:"source_id"`     // matches Adapter.Name()
    URL         string         `json:"url"`           // canonical URL
    Title       string         `json:"title"`
    Body        string         `json:"body"`          // full text (ranking input)
    Snippet     string         `json:"snippet"`       // short excerpt (UI display)
    PublishedAt time.Time      `json:"published_at"`  // zero if unknown
    RetrievedAt time.Time      `json:"retrieved_at"`  // when this adapter saw it
    Author      string         `json:"author"`
    Score       float64        `json:"score"`         // normalized [0,1]; 0 = unscored
    Lang        string         `json:"lang"`          // BCP-47
    DocType     DocType        `json:"doc_type"`
    Citations   []string       `json:"citations,omitempty"`  // doc IDs referenced
    Metadata    map[string]any `json:"metadata,omitempty"`   // adapter extras
    Hash        string         `json:"hash"`                  // CanonicalHash() output cached
}
```

### 6.4 Registry Sketch (`internal/adapters/registry.go`)

```go
package adapters

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "os"
    "sort"
    "sync"
    "time"

    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    oteltrace "go.opentelemetry.io/otel/trace"

    "github.com/elymas/universal-search/internal/obs"
    "github.com/elymas/universal-search/pkg/types"
)

// Registry stores registered adapters and provides concurrent-safe lookup.
//
// @MX:ANCHOR: [AUTO] Adapter registry; callers: cmd mains, FAN-001, IR-001, tests
// @MX:REASON: fan_in >= 3; the only sanctioned source of Adapter instances at runtime
type Registry struct {
    mu       sync.RWMutex
    adapters map[string]types.Adapter // values are wrappedAdapter
    obs      *obs.Obs
}

func NewRegistry(o *obs.Obs) *Registry {
    return &Registry{adapters: make(map[string]types.Adapter), obs: o}
}

// Register adds an adapter under its Name(). Returns *RegistryError on
// duplicate name or missing auth env var.
func (r *Registry) Register(a types.Adapter) error {
    return r.RegisterWithOptions(a, RegisterOptions{})
}

// @MX:WARN: [AUTO] Allows duplicate-name detection; returns typed error
// @MX:REASON: silent overwrite would invalidate FAN-001's adapter routing table
func (r *Registry) RegisterWithOptions(a types.Adapter, opts RegisterOptions) error {
    name := a.Name()
    caps := a.Capabilities()

    if !opts.SkipAuthCheck && caps.RequiresAuth {
        for _, ev := range caps.AuthEnvVars {
            if _, ok := os.LookupEnv(ev); !ok {
                return &RegistryError{Op: "register", Name: name, Cause: ErrMissingAuth}
            }
        }
    }

    r.mu.Lock()
    defer r.mu.Unlock()
    if _, exists := r.adapters[name]; exists {
        return &RegistryError{Op: "register", Name: name, Cause: ErrDuplicateAdapter}
    }
    r.adapters[name] = &wrappedAdapter{inner: a, obs: r.obs}
    return nil
}

func (r *Registry) Get(name string) (types.Adapter, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    a, ok := r.adapters[name]
    return a, ok
}

func (r *Registry) List() []string {
    r.mu.RLock()
    names := make([]string, 0, len(r.adapters))
    for n := range r.adapters {
        names = append(names, n)
    }
    r.mu.RUnlock()
    sort.Strings(names)
    return names
}

type RegisterOptions struct {
    SkipAuthCheck bool
}

var (
    ErrDuplicateAdapter = errors.New("adapters: duplicate adapter name")
    ErrMissingAuth      = errors.New("adapters: required auth env var not set")
)

type RegistryError struct {
    Op    string
    Name  string
    Cause error
}

func (e *RegistryError) Error() string {
    return fmt.Sprintf("registry %s %q: %v", e.Op, e.Name, e.Cause)
}
func (e *RegistryError) Unwrap() error { return e.Cause }
```

### 6.5 wrappedAdapter Sketch

```go
// wrappedAdapter delegates Search/Healthcheck/Capabilities to inner while
// emitting metrics, span, and slog per Search call.
type wrappedAdapter struct {
    inner types.Adapter
    obs   *obs.Obs
}

func (w *wrappedAdapter) Name() string                    { return w.inner.Name() }
func (w *wrappedAdapter) Capabilities() types.Capabilities { return w.inner.Capabilities() }
func (w *wrappedAdapter) Healthcheck(ctx context.Context) error {
    return w.inner.Healthcheck(ctx)
}

func (w *wrappedAdapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
    name := w.inner.Name()
    var tracer oteltrace.Tracer
    if w.obs != nil {
        tracer = w.obs.Tracer("adapter")
    } else {
        // Mirrors internal/obs/trace/trace.go:20,43 which imports
        // go.opentelemetry.io/otel/trace/noop for the no-op provider.
        tracer = noop.NewTracerProvider().Tracer("adapter")
    }
    ctx, span := tracer.Start(ctx, "adapter.search",
        oteltrace.WithAttributes(attribute.String("adapter.name", name)))
    defer span.End()

    start := time.Now()
    docs, err := w.inner.Search(ctx, q)
    elapsed := time.Since(start).Seconds()

    outcome := types.OutcomeFromError(err)
    span.SetAttributes(
        attribute.String("adapter.outcome", outcome),
        attribute.Int("adapter.result_count", len(docs)),
    )
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, outcome)
    }

    if w.obs != nil {
        if reg := w.obs.Metrics; reg != nil {
            if reg.AdapterCalls != nil {
                reg.AdapterCalls.WithLabelValues(name, outcome).Inc()
            }
            if reg.AdapterCallDuration != nil {
                reg.AdapterCallDuration.WithLabelValues(name).Observe(elapsed)
            }
        }
        if w.obs.Logger != nil {
            level := slog.LevelInfo
            if err != nil {
                level = slog.LevelWarn
            }
            w.obs.Logger.Log(ctx, level, "adapter call",
                slog.String("adapter", name),
                slog.String("outcome", outcome),
                slog.Float64("elapsed_seconds", elapsed),
                slog.Int("result_count", len(docs)),
            )
        }
    }

    return docs, err
}
```

### 6.6 Reference Noop Adapter (`internal/adapters/noop/noop.go`)

```go
package noop

import (
    "context"
    "time"

    "github.com/elymas/universal-search/pkg/types"
)

// Adapter is a no-op reference adapter satisfying types.Adapter.
// Used as a compile-time contract check and as a stable test fixture
// for SPEC-FAN-001, SPEC-IR-001, and SPEC-IDX-001 development.
type Adapter struct {
    name string
}

func New(name string) *Adapter { return &Adapter{name: name} }

func (a *Adapter) Name() string                                   { return a.name }
func (a *Adapter) Healthcheck(_ context.Context) error            { return nil }
func (a *Adapter) Search(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
    return nil, nil
}
func (a *Adapter) Capabilities() types.Capabilities {
    return types.Capabilities{
        SourceID:          a.name,
        DisplayName:       "Noop",
        DocTypes:          []types.DocType{types.DocTypeOther},
        SupportedLangs:    []string{"en"},
        DefaultMaxResults: 10,
        Notes:             "Reference noop adapter; no external calls.",
    }
}

// Compile-time interface assertion.
var _ types.Adapter = (*Adapter)(nil)
```

### 6.7 Files to Modify (Summary)

Created (12 files):
- `pkg/types/normalized_doc.go`
- `pkg/types/normalized_doc_test.go`
- `pkg/types/adapter.go`
- `pkg/types/adapter_test.go`
- `pkg/types/query.go`
- `pkg/types/query_test.go`
- `pkg/types/capabilities.go`
- `pkg/types/capabilities_test.go`
- `pkg/types/errors.go`
- `pkg/types/errors_test.go`
- `pkg/types/bench_test.go`
- `internal/adapters/registry.go`
- `internal/adapters/registry_test.go`
- `internal/adapters/noop/noop.go`
- `internal/adapters/noop/noop_test.go`

Modified (2 files):
- `pkg/types/types.go` — replace 2-line stub with package doc + re-exports if needed
- `internal/adapters/adapters.go` — replace 4-line stub with package doc + ANCHOR

Unchanged (by design):
- `internal/obs/metrics/metrics.go` — already declares AdapterCalls + AdapterCallDuration; allowlist already includes `adapter` and `outcome`
- `internal/llm/*` — no dependency
- `cmd/usearch*/main.go` — registry construction is M3 SPEC-IR-001 / SPEC-FAN-001 concern; CORE-001 just provides the type

### 6.8 Harness Level

Per `.moai/config/sections/harness.yaml` auto-routing: 8 REQs (7 × P0 + 1 × P1)
+ 3 NFRs touching 2 packages (5 sub-files in pkg/types, 2 in
internal/adapters, 2 in internal/adapters/noop) + zero compose / env / config
deltas = **standard** harness level. Sprint Contract is optional but
recommended; evaluator profile `default` applies.

## 7. What NOT to Build (Exclusions)

[HARD] This SPEC explicitly excludes the following items. Each has a known
destination SPEC; this list prevents scope creep into CORE-001.

- **Per-source adapter implementations** (Reddit, HN, arXiv, GitHub, YouTube,
  Bluesky, X, SearXNG, Naver, Daum, KoreaNewsCrawler, RSS, Polymarket) →
  SPEC-ADP-001 through SPEC-ADP-009 (M3).
- **Fanout / parallel dispatch / partial-result assembly / dedup** → SPEC-FAN-001 (M3).
- **Index ingestion** (`Qdrant`, `Meilisearch`, `Postgres` writes of
  `[]NormalizedDoc`) → SPEC-IDX-001..003 (M3).
- **Synthesis consumption** (gpt-researcher wrapper consuming docs for
  citation assembly) → SPEC-SYN-001 (M2), SPEC-SYN-002 (M4).
- **Cache layer / 5-phase access fallback** → SPEC-CACHE-001 (M3).
- **Auth / RBAC / audit** (team-scoped credentials, per-tenant adapter
  visibility) → SPEC-AUTH-001..003 (M6). CORE-001 declares
  `Capabilities.RequiresAuth` (declarative); enforcement of which TEAM
  has access to which adapter is M6.
- **Embedding generation** (BGE-M3 sidecar consuming `NormalizedDoc.Body`) → SPEC-IDX-002 (M3).
- **Korean tokenization** (Meilisearch + mecab-ko) → SPEC-IDX-003 (M3).
- **Streaming Search results** (channel-based `Search`) → SPEC-SYN-004 (M4) if measured to need it; V1 returns `[]NormalizedDoc`.
- **Adapter health-state machine** (auto-disable on `ErrSourceUnavailable`,
  re-enable on Healthcheck pass) → SPEC-EVAL-002 (M8).
- **Per-adapter custom metrics** (e.g., adapter-specific cache-hit gauges) →
  the per-adapter SPEC owns its extra metrics.
- **gRPC contract between Go and Python services** → `proto/` layer in a
  separate SPEC.
- **Adapter discovery / plugin loading at runtime** (loading adapters from
  `.so` files or via reflection) → not on the V1 roadmap; would land in a
  post-V1 SPEC if needed.
- **Pre-flight Query validation** (rejecting malformed queries before they
  reach an adapter) — Query is opaque to the registry; FAN-001 owns query
  preprocessing.
- **Multi-adapter result fusion** (RRF, BGE-reranker) → SPEC-IDX-001 RRF
  fusion (M3); CORE-001 just produces the doc list.

## 8. TDD Plan

Development follows RED-GREEN-REFACTOR per `quality.development_mode: tdd`
(quality.yaml). Representative RED-phase tests, written before
implementation, grouped by REQ. Total: ~38 tests covering every REQ and
NFR acceptance criterion; coverage target 85%.

| # | Test | Layer | REQ | Assertion |
|---|------|-------|-----|-----------|
| 1 | `TestNormalizedDocFieldSet` | `normalized_doc_test.go` | REQ-CORE-001 | Reflect over struct; assert exactly 15 fields with named tags |
| 2 | `TestNormalizedDocJSONRoundTrip` | `normalized_doc_test.go` | REQ-CORE-001 | Marshal → Unmarshal → field-equal on a fully-populated doc with mixed-type Metadata map |
| 3 | `TestNormalizedDocValidateRequiredFields` | `normalized_doc_test.go` | REQ-CORE-001, REQ-CORE-007 | Table-driven 5 cases (missing each required + complete) |
| 4 | `TestNormalizedDocCanonicalHashStable` | `normalized_doc_test.go` | REQ-CORE-001 | Two calls on identical doc yield byte-equal hash |
| 5 | `TestCanonicalHashIgnoresMetadata` | `normalized_doc_test.go` | REQ-CORE-001 | Two docs differing only in Metadata produce identical hash |
| 6 | `TestAdapterInterfaceShape` | `adapter_test.go` | REQ-CORE-002 | reflect.Type confirms 4 methods with named signatures |
| 7 | `TestQueryStructFields` | `query_test.go` | REQ-CORE-002 | Reflect over Query; assert 6 fields |
| 8 | `TestCapabilitiesStructFields` | `capabilities_test.go` | REQ-CORE-002 | Reflect over Capabilities; assert 10 fields |
| 9 | `TestDocTypeEnumComplete` | `capabilities_test.go` | REQ-CORE-002 | All 8 DocType constants exist |
| 10 | `TestNoopAdapterImplementsInterface` | `noop/noop_test.go` | REQ-CORE-002 | Compile-time: `var _ types.Adapter = (*Adapter)(nil)` |
| 11 | `TestRegisterSucceedsForNewAdapter` | `registry_test.go` | REQ-CORE-003 | Register noop A → no error; Get("a") returns wrapped instance |
| 12 | `TestRegisterRejectsDuplicateName` | `registry_test.go` | REQ-CORE-003 | Two registers same name → second returns *RegistryError wrapping ErrDuplicateAdapter |
| 13 | `TestRegisterStateUnchangedOnError` | `registry_test.go` | REQ-CORE-003 | Pre-error List vs post-error List byte-equal |
| 14 | `TestRegisterValidatesAuthEnvVars` | `registry_test.go` | REQ-CORE-003, REQ-CORE-006 | RequiresAuth=true + env unset → ErrMissingAuth |
| 15 | `TestRegisterAllowsSkipAuthCheck` | `registry_test.go` | REQ-CORE-003, REQ-CORE-006 | RegisterWithOptions(.., SkipAuthCheck=true) bypasses |
| 16 | `TestWrappedAdapterEmitsCounterSuccess` | `registry_test.go` | REQ-CORE-004 | Successful Search → AdapterCalls{name="noop",outcome="success"} +1 |
| 17 | `TestWrappedAdapterEmitsCounterFailure` | `registry_test.go` | REQ-CORE-004 | Search returns ErrPermanent → outcome="failure" |
| 18 | `TestWrappedAdapterEmitsCounterTimeout` | `registry_test.go` | REQ-CORE-004 | Search returns context.DeadlineExceeded → outcome="timeout" |
| 19 | `TestWrappedAdapterEmitsCounterRateLimited` | `registry_test.go` | REQ-CORE-004 | Search returns *SourceError{Category=RateLimited} → outcome="rate_limited" |
| 20 | `TestWrappedAdapterEmitsCounterUnavailable` | `registry_test.go` | REQ-CORE-004 | Search returns *SourceError{Category=Unavailable} → outcome="unavailable" |
| 21 | `TestWrappedAdapterEmitsHistogram` | `registry_test.go` | REQ-CORE-004 | Histogram count +1; observed value > 0 |
| 22 | `TestWrappedAdapterCreatesOTelSpan` | `registry_test.go` | REQ-CORE-004 | In-memory exporter captures span "adapter.search" with attrs |
| 23 | `TestWrappedAdapterEmitsSlogRecord` | `registry_test.go` | REQ-CORE-004 | Buffered handler captures one record with named attrs |
| 24 | `TestWrappedAdapterPreservesUnderlyingError` | `registry_test.go` | REQ-CORE-004 | errors.Is(returnedErr, originalErr) holds |
| 25 | `TestWrappedAdapterSafeOnNilObs` | `registry_test.go` | REQ-CORE-004 | Pass nil Metrics / Logger; no panic |
| 26 | `TestRegistryConcurrentReadWrite` | `registry_test.go` | REQ-CORE-005 | 100 readers + 1 writer × 1s; -race clean |
| 27 | `TestListReturnsSortedNames` | `registry_test.go` | REQ-CORE-005 | Register {"z","a","m"} → List = ["a","m","z"] |
| 28 | `TestRegisterRequiresAuthEnvVarsTable` | `registry_test.go` | REQ-CORE-006 | 4-cell truth table |
| 29 | `TestValidateRejectsMissingID` | `normalized_doc_test.go` | REQ-CORE-007 | Returns *ValidationError with Field=="ID" |
| 30 | `TestValidateRejectsMissingSourceID` | `normalized_doc_test.go` | REQ-CORE-007 | Field=="SourceID" |
| 31 | `TestValidateRejectsMissingURL` | `normalized_doc_test.go` | REQ-CORE-007 | Field=="URL" |
| 32 | `TestValidateRejectsZeroRetrievedAt` | `normalized_doc_test.go` | REQ-CORE-007 | Field=="RetrievedAt" |
| 33 | `TestValidationErrorWrapsFieldName` | `normalized_doc_test.go` | REQ-CORE-007 | errors.As resolves *ValidationError; .Field correct |
| 34 | `TestSentinelErrorsExist` | `errors_test.go` | REQ-CORE-008 | All 4 sentinels declared |
| 35 | `TestSourceErrorIsMatchesSentinels` | `errors_test.go` | REQ-CORE-008 | errors.Is(srcErr, ErrTransient) etc. |
| 36 | `TestSourceErrorUnwrapsCause` | `errors_test.go` | REQ-CORE-008 | errors.Unwrap returns the cause |
| 37 | `TestCategorizeErrorTable` | `errors_test.go` | REQ-CORE-008 | Table over 7 input shapes |
| 38 | `TestOutcomeFromErrorTable` | `errors_test.go` | REQ-CORE-008 | Table over 5 outputs |
| 39 | `BenchmarkNormalizedDocValidate` | `bench_test.go` | NFR-CORE-001 | < 1 µs/op on amd64 |
| 40 | `BenchmarkNormalizedDocCanonicalHash` | `bench_test.go` | NFR-CORE-001 | < 5 µs/op on amd64 |
| 41 | `TestAdapterOutcomeLabels` | `registry_test.go` | NFR-CORE-002 | Allowlist enforcement; rejection of disallowed values |
| 42 | `TestPkgTypesNoInternalImports` | `pkg/types/types_test.go` | NFR-CORE-003 | go list -deps -json scan; pkg/types depends only on stdlib |

Coverage target: 85% per `quality.test_coverage_target`. Benchmarks do not
count toward coverage.

RED-GREEN-REFACTOR per requirement:

1. RED: Write failing test for REQ-CORE-N.
2. GREEN: Implement the minimal code to pass.
3. REFACTOR: Tidy, extract shared helpers if they remove duplication.

Brownfield note: `pkg/types/types.go` and `internal/adapters/adapters.go`
exist as 2-4 line stubs. Per workflow-modes.md §Brownfield Enhancement,
the existing stubs have no behavior to preserve, so characterization
tests are not needed; RED tests for REQ-CORE-* are written against the
planned package surface.

## 9. Dependencies

### 9.1 Upstream SPEC Dependencies

- **SPEC-BOOT-001 (approved, merged to main)**: provides `pkg/types/types.go`
  stub, `internal/adapters/adapters.go` stub, `cmd/usearch*` binaries
  (registry construction lands later in M3).
- **SPEC-OBS-001 (approved, merged to main)**: provides
  `internal/obs/metrics/metrics.go::AdapterCalls`,
  `internal/obs/metrics/metrics.go::AdapterCallDuration`, `obs.Obs`
  bundle, cardinality allowlist already including `adapter` and
  `outcome`. Soft dependency — CORE-001 imports `internal/obs` but is
  nil-safe if obs collectors are absent.

### 9.2 Parallelizable

- **SPEC-DEP-001** (approved): no new external dependencies; CORE-001
  uses only Go stdlib + the existing prometheus/otel deps already pinned.
  No interaction with the dependency manifest.
- **SPEC-IR-001** (M2): CORE-001 publishes the contract; IR-001
  consumes Capabilities. Parallelizable starting after CORE-001's
  spec.md is approved (IR-001 can begin its research phase before
  CORE-001 lands code).

### 9.3 Downstream Blocked SPECs

- **SPEC-IR-001** (M2 Intent Router): consumes `Capabilities` from
  registered adapters to build the routing table.
- **SPEC-ADP-001** (M2 Reddit, reference): implements `Adapter`,
  registers via `Registry.Register`, returns `[]NormalizedDoc`.
- **SPEC-ADP-002 through SPEC-ADP-009** (M3): each implements
  `Adapter` per the contract.
- **SPEC-FAN-001** (M3 fanout): iterates `registry.List()`, calls
  `Search` on each via goroutine pool, classifies errors via
  `CategorizeError`, retries based on `errors.Is(err, ErrTransient)`.
- **SPEC-IDX-001** (M3 hybrid index): consumes `[]NormalizedDoc` for
  embedding + BM25 + dense indexing.
- **SPEC-SYN-001** (M2 basic synthesis): consumes docs for citation
  assembly via gpt-researcher wrapper.

### 9.4 External Dependencies (run-phase pins)

No new Go module dependencies. `pkg/types/` is pure stdlib. The
registry imports `internal/obs` (already pinned) and
`go.opentelemetry.io/otel/{attribute,codes,trace}` (already pinned via
SPEC-OBS-001).

## 10. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| NormalizedDoc field-set drift across M3 adapters (e.g., Reddit wants `Karma`, X wants `Likes`) | High | High (forces public-API breaking change) | Lock canonical 15-field set in V1; per-adapter extras flow to `Metadata map[string]any`; documented policy in field godoc |
| `Metadata map[string]any` becomes a free-for-all, defeating typed-contract benefits | Medium | Medium | Document allowed key conventions in adapter SPECs; defer lint test to M8 if drift is observed |
| Registry's wrappedAdapter swallows or re-classifies adapter-returned errors incorrectly | Medium | High (FAN-001 makes wrong retry decisions) | Wrapper preserves underlying err via `%w`; classification is `errors.Is`/`errors.As`-based, no string parsing; REQ-CORE-004 acceptance test asserts preservation |
| `NormalizedDoc.Hash` collisions across adapters (Reddit's permalink and HN's storyURL hashing to the same value) | Low | High (dedup falsely merges distinct docs) | Hash includes `SourceID` prefix in canonicalisation; documented in normalized_doc.go godoc |
| Registry locking contention if adapter list grows past 50 (post-V1) | Low | Medium | RWMutex; reads dominate (List is hot, Register is cold); benchmark in CI if M9 adapter count exceeds 30 |
| Five-outcome label set under-specified (some err categories don't fit any of the five) | Low | Low | `transient` is a sixth catch-all internal value when classification cannot be more precise; documented in errors.go |
| Adapter authors bypass the registry and call wrappedAdapter directly, missing observability | Medium | Medium | wrappedAdapter is unexported; only Register-time path produces it; documented in registry.go godoc |
| Auth env-var validation at registration time is too strict for dev environments | Medium | Low | RegisterOptions{SkipAuthCheck: true} escape hatch; documented in capabilities.go godoc |
| Capabilities struct grows monotonically as adapters request new fields | Medium | Low | Field additions are non-breaking for the SDK boundary; documented growth policy |
| `outcome` label cardinality breaks SPEC-OBS-001's NFR-OBS-002 promise | Low | High (Prometheus storage blow-up) | Static enum at compile time; NFR-CORE-002 test extends OBS-001's allowlist test; CI gate ensures no drift |
| Time.Time JSON serialization not RFC-3339 by default in some Go versions | Low | Low | Go stdlib uses RFC-3339 since 1.0; verified in REQ-CORE-001 round-trip test |

## 11. Open Questions

The following are explicitly unresolved at SPEC-approval time and
documented here rather than pre-decided. They do not block SPEC approval.

1. **Hash algorithm: SHA-256 (collision-safe, 32 bytes) vs xxhash64 (8
   bytes, faster, industry-standard non-crypto dedup).** Default xxhash64
   (existing project transitive dep `cespare/xxhash/v2 v2.3.0` per
   `go.mod`); revisit if dedup precision becomes a measured concern in
   M3-M4. Resolution owner: run-phase implementer.

2. **Should `NormalizedDoc.PublishedAt` be required (rejected when zero)
   or optional (zero permitted)?** Default optional — Naver shopping
   listings and some social posts have no extractable date; forcing
   non-zero would push adapters to fabricate timestamps. Documented in
   field godoc; downstream consumers (ranking) may treat zero as
   "unknown" and tiebreak with RetrievedAt.

3. **Should the registry expose a `HealthcheckAll(ctx) map[string]error`
   operation that probes every registered adapter in parallel?** Useful
   for SPEC-EVAL-002 dashboard. Default: deferred — EVAL-002 can
   iterate registry.List() and call Healthcheck itself; registry stays
   minimal. Resolution owner: M8 SPEC author.

4. **Should `wrappedAdapter` enforce a default per-call timeout if the
   caller's context has none?** Risk of a runaway adapter blocking
   forever. Default: no — adapters get the caller's context as-is;
   FAN-001 owns per-adapter timeout policy via context.WithDeadline.
   Revisit if observed in M3 testing.

5. **Should `errors.Is(err, ErrSourceUnavailable)` automatically mark
   the adapter unhealthy in the registry (auto-disable until Healthcheck
   passes)?** Powerful but risky — a transient outage could disable the
   adapter for the rest of the process lifetime. Default: registry
   tracks no health state in V1; FAN-001 / EVAL-002 own the
   unhealthy/healthy transition policy externally.

6. **Cursor pagination shape: opaque `string` (V1) vs typed struct
   (`type Cursor any`)?** Opaque string lets each adapter encode its own
   format; typed struct forces a uniform pagination model that doesn't
   match all sources (HN's `numericFilters` cursor is structurally
   different from Reddit's `after`). Default: opaque string;
   adapter-specific format documented per-adapter SPEC.

7. **Should the noop adapter return synthetic results (so FAN-001 tests
   have non-empty data to fuse) or always return nil?** Default: always
   nil; FAN-001 owns its own fixture adapter that returns synthetic data
   with controlled cardinality/timing. CORE-001's noop is a
   contract-conformance fixture, not a behavior fixture.

## 12. References

External (cited in research.md):

- gpt-researcher retriever convention — Context7 `/assafelovic/gpt-researcher`
- SearXNG engine plugin pattern — Context7 `/searxng/searxng`
- Perplexica Source interface — Context7 `/itzcrazykns/perplexica`
- Stanford STORM knowledge_graph — https://github.com/stanford-oval/storm
- last30days-skill engagement scoring — https://github.com/mvanhorn/last30days-skill
- Go stdlib error patterns — https://pkg.go.dev/errors
- prometheus/client_golang CounterVec — pinned at v1.23.2 (`go.mod`)

Internal (project files):

- SPEC-BOOT-001 — `.moai/specs/SPEC-BOOT-001/spec.md`
- SPEC-OBS-001 — `.moai/specs/SPEC-OBS-001/spec.md`
- SPEC-LLM-001 — `.moai/specs/SPEC-LLM-001/spec.md` (router/observability pattern source)
- internal/llm/router.go — registry pattern reference
- internal/llm/client.go — observability emit pattern reference
- internal/llm/retry.go — error classification pattern reference
- internal/obs/obs.go — Obs bundle DI reference
- internal/obs/metrics/metrics.go — existing AdapterCalls/AdapterCallDuration registration
- pkg/types/types.go — current stub
- internal/adapters/adapters.go — current stub
- .moai/project/structure.md §4-§5 — NormalizedDoc data model + pkg/types stability commitment
- .moai/project/roadmap.md §M2-M3 — milestone placement and parallelization plan

## 13. HISTORY

- 2026-04-26 — Implemented in commit f728aa2. Tests: pkg/types 85.5% / internal/adapters 90.8% / noop 100%. All 8 EARS REQ entries verified by green tests. @MX tags applied (5 ANCHOR + 1 WARN + 6 NOTE).
- 2026-04-26 (initial draft v0.1, limbowl via manager-spec):
  First EARS-formatted SPEC drafted after research phase. Scope derived
  from `.moai/project/roadmap.md` M2-M3 dependency analysis: extracted
  NormalizedDoc + Adapter contract from SPEC-ADP-001's original scope to
  unblock 7-way M3 adapter parallelization. Built on SPEC-BOOT-001
  (`pkg/types/types.go` + `internal/adapters/adapters.go` stubs) and
  SPEC-OBS-001 (existing `AdapterCalls`/`AdapterCallDuration` collectors,
  `adapter` and `outcome` already in cardinality allowlist). Mirrors
  SPEC-LLM-001 patterns: provider router (`internal/llm/router.go`) →
  adapter registry; per-call observability emit
  (`internal/llm/client.go::emitObservability`) → wrappedAdapter; typed
  error classification (`internal/llm/retry.go::isNonRetryable`) →
  CategorizeError. 8 EARS REQs (7 × P0 + 1 × P1) covering all five EARS
  patterns (Ubiquitous, Event-Driven, State-Driven, Optional, Unwanted),
  3 NFRs, 42 representative RED tests, 7 Open Questions. Zero new Go
  module dependencies; pure stdlib in pkg/types/, stdlib + existing
  prometheus/otel + obs in internal/adapters/. Research artifact at
  `.moai/specs/SPEC-CORE-001/research.md` captures existing-pattern
  citation, reference design survey (gpt-researcher, SearXNG, Perplexica,
  STORM, last30days-skill), type-system trade-offs, and rejected
  alternatives. Inserted into M2 as "Foundation contracts (inserted)";
  blocks 12 downstream SPECs (IR-001, ADP-001..009, FAN-001, IDX-001).
  Ready for plan-auditor review and annotation cycle.

---

*End of SPEC-CORE-001 v0.1*
