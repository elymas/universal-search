# SPEC-CORE-001 Acceptance — Given/When/Then Scenarios

Created: 2026-04-26
Author: limbowl (via manager-spec)
Status: draft (with spec.md, plan.md)

## 0. Document Purpose

This document specifies the acceptance criteria for SPEC-CORE-001 in
Given/When/Then format, complementing the requirement-level acceptance
table in spec.md §5. The scenarios here are the externally-observable
behaviors that the run phase MUST verify before declaring CORE-001 complete.

Scope: Twelve Given/When/Then scenarios covering REQ-CORE-001 through
REQ-CORE-008, plus three Edge-Case sections, plus a Definition of Done
checklist.

## 1. Given/When/Then Scenarios

### Scenario 1 — JSON round-trip preserves all NormalizedDoc fields including Metadata

Maps to REQ-CORE-001.

**Given**: A `pkg/types.NormalizedDoc` value with all 15 fields populated:

```
ID:          "reddit:abc123"
SourceID:    "reddit"
URL:         "https://reddit.com/r/golang/comments/abc123"
Title:       "Why Go's stdlib slog is great"
Body:        "<full post text, 2KB>"
Snippet:     "stdlib slog gives us..."
PublishedAt: 2026-04-20T12:34:56Z (RFC-3339)
RetrievedAt: 2026-04-26T09:00:00Z
Author:      "u/gopher42"
Score:       0.87
Lang:        "en"
DocType:     DocTypePost
Citations:   ["arxiv:2401.12345", "github:golang/go#67890"]
Metadata:    {"upvotes": 1234, "subreddit": "golang", "is_oc": true}
Hash:        (computed via CanonicalHash)
```

**When**: The doc is marshaled with `json.Marshal(doc)` and the resulting
bytes are unmarshaled back with `json.Unmarshal(data, &round)`.

**Then**:
- All 15 fields in `round` are byte-equal (or value-equal for non-string
  types) to the original.
- `round.PublishedAt.Equal(doc.PublishedAt)` returns true (RFC-3339 round-
  trip is lossless).
- `round.Metadata["upvotes"]` resolves to a numeric type (specifically
  `float64` per Go's default JSON number parsing); test casts and
  asserts the integer value.
- `round.Metadata["is_oc"]` is `true` (bool round-trip).
- `round.Citations` is a slice of length 2 with the two cited doc IDs.
- `round.CanonicalHash() == doc.CanonicalHash()` (the Hash field is
  cached but the method must be re-derivable).

**Verification test**: `TestNormalizedDocJSONRoundTrip` in
`pkg/types/normalized_doc_test.go`.

---

### Scenario 2 — Registry rejects duplicate adapter names with typed error

Maps to REQ-CORE-003.

**Given**: A registry instance:

```
obs := obs.Obs{} // minimal bundle for test
r := adapters.NewRegistry(&obs)
```

And two adapters whose `Name()` both return `"reddit"`:

```
a1 := noop.New("reddit")
a2 := noop.New("reddit")
```

**When**: The first is registered successfully:

```
err1 := r.Register(a1)  // err1 == nil
```

And then the second is registered:

```
err2 := r.Register(a2)
```

**Then**:
- `err2` is non-nil.
- `errors.Is(err2, adapters.ErrDuplicateAdapter)` returns true.
- `var regErr *adapters.RegistryError; errors.As(err2, &regErr)` returns
  true.
- `regErr.Op == "register"`, `regErr.Name == "reddit"`,
  `errors.Is(regErr.Cause, adapters.ErrDuplicateAdapter)`.
- `r.List()` returns `["reddit"]` (length 1; the second register did NOT
  add anything).
- `r.Get("reddit")` returns the wrapped form of `a1`, not `a2` (verified by
  comparing the result of a side-effect on a1: the test stub increments a
  counter when Search is called; calling `r.Get("reddit").Search(...)`
  increments a1's counter, not a2's).

**Verification tests**: `TestRegisterRejectsDuplicateName`,
`TestRegisterStateUnchangedOnError` in
`internal/adapters/registry_test.go`.

---

### Scenario 3 — Validate catches missing required fields

Maps to REQ-CORE-001 + REQ-CORE-007.

**Given**: A `NormalizedDoc` with one of the four required fields empty
or zero. Four sub-scenarios (table-driven):

3a. ID empty.
3b. SourceID empty.
3c. URL empty.
3d. RetrievedAt zero (`time.Time{}`).

**When**: `doc.Validate()` is called.

**Then**:
- The returned error is non-nil.
- `var ve *types.ValidationError; errors.As(err, &ve)` returns true.
- `ve.Field` equals the name of the missing field ("ID", "SourceID",
  "URL", "RetrievedAt" respectively).
- `*types.ValidationError` is recoverable via `errors.As` so callers can
  inspect `Field` and `Cause` without coupling to a sentinel value.

And, conversely, **Given** a fully-populated doc and **When**
`doc.Validate()` is called, **Then** the result is `nil`.

**Verification tests**: `TestValidateRejectsMissingID`,
`TestValidateRejectsMissingSourceID`, `TestValidateRejectsMissingURL`,
`TestValidateRejectsZeroRetrievedAt`, `TestValidateAcceptsCompleteDoc`,
`TestValidationErrorWrapsFieldName` in
`pkg/types/normalized_doc_test.go`.

---

### Scenario 4 — Noop adapter satisfies the interface (compile-time check)

Maps to REQ-CORE-002.

**Given**: The `internal/adapters/noop` package.

**When**: The compile-time assertion line is present:

```go
var _ types.Adapter = (*Adapter)(nil)
```

**Then**:
- The package compiles without error (i.e., `(*noop.Adapter)` satisfies
  every method in the `types.Adapter` interface).
- A runtime fixture confirms behavior:
  - `noop.New("test").Name() == "test"`
  - `noop.New("test").Healthcheck(ctx) == nil`
  - `noop.New("test").Search(ctx, types.Query{}) == (nil, nil)`
  - `noop.New("test").Capabilities().SourceID == "test"`

**And**: If the run-phase implementer accidentally renames a method (e.g.,
`Search` → `Find`), `go build ./internal/adapters/noop/` fails because
the compile-time assertion no longer holds. This is the gate.

**Verification tests**: `TestNoopAdapterImplementsInterface` (compile-time
+ runtime) in `internal/adapters/noop/noop_test.go`.

---

### Scenario 5 — wrappedAdapter emits Prometheus counter increment per outcome

Maps to REQ-CORE-004 + NFR-CORE-002.

**Given**: A registry with one registered noop-like adapter, configured
with a programmable Search behavior:

```
fake := &programmableAdapter{name: "fake"}
r := adapters.NewRegistry(o) // o is a freshly-initialized obs.Obs
_ = r.Register(fake)
```

Five sub-scenarios, one per outcome value:

5a. Programmable adapter returns `nil` error → expected outcome: `success`.
5b. Programmable adapter returns `&types.SourceError{Category: types.CategoryPermanent}` → outcome: `failure`.
5c. Programmable adapter returns `context.DeadlineExceeded` → outcome: `timeout`.
5d. Programmable adapter returns `&types.SourceError{Category: types.CategoryRateLimited}` → outcome: `rate_limited`.
5e. Programmable adapter returns `&types.SourceError{Category: types.CategoryUnavailable}` → outcome: `unavailable`.

**When**: `(adapter from r.Get("fake")).Search(ctx, types.Query{})` is
called once.

**Then**, in each sub-scenario:
- `o.Metrics.AdapterCalls.WithLabelValues("fake", <outcome>).Get()`
  has incremented by exactly 1 since the test's baseline read.
- `o.Metrics.AdapterCallDuration.WithLabelValues("fake")` shows count
  +1 with sample sum > 0.
- The returned error from `Search` is `errors.Is(returned, original)` —
  the wrapper preserves the underlying error untouched.
- An OTel span named `adapter.search` was created (verifiable via
  in-memory exporter) with `adapter.name=fake`,
  `adapter.outcome=<outcome>`, `adapter.result_count=0`.
- For non-success outcomes, `span.Status.Code == codes.Error`.
- One slog record was emitted with attributes `{adapter, outcome,
  elapsed_seconds, result_count, request_id}`. Level is INFO for success,
  WARN for non-success outcomes.

**Verification tests**: `TestWrappedAdapterEmitsCounterSuccess`,
`TestWrappedAdapterEmitsCounterFailure`,
`TestWrappedAdapterEmitsCounterTimeout`,
`TestWrappedAdapterEmitsCounterRateLimited`,
`TestWrappedAdapterEmitsCounterUnavailable`,
`TestWrappedAdapterEmitsHistogram`,
`TestWrappedAdapterCreatesOTelSpan`,
`TestWrappedAdapterEmitsSlogRecord`,
`TestWrappedAdapterPreservesUnderlyingError` in
`internal/adapters/registry_test.go`.

---

### Scenario 6 — Concurrent Register and Get under -race

Maps to REQ-CORE-005.

**Given**: A registry instance with no registered adapters.

**When**: 100 goroutines run for 1 second each, each on its own ticker:
- 99 goroutines call `r.Get(<random name>)` and `r.List()` repeatedly.
- 1 goroutine calls `r.Register(noop.New(<unique name>))` repeatedly with
  monotonically-increasing names (`"a-0"`, `"a-1"`, ...).
- After 1 second, all goroutines stop.

**Then**:
- `go test -race` reports zero race-condition warnings.
- The final `r.List()` returns the count of successfully-registered names
  (no torn writes, no missed writes).
- The List output is sorted lexicographically.

**Verification test**: `TestRegistryConcurrentReadWrite` (with
`t.Parallel()`) and `TestListReturnsSortedNames` in
`internal/adapters/registry_test.go`.

---

### Scenario 7 — Auth env-var validation at registration time

Maps to REQ-CORE-003 + REQ-CORE-006.

**Given**: An adapter with `Capabilities().RequiresAuth == true` and
`Capabilities().AuthEnvVars == []string{"FAKE_API_KEY"}`.

Four sub-scenarios (4-cell truth table):

7a. Env unset, `SkipAuthCheck=false` → expect `*RegistryError` wrapping
    `ErrMissingAuth`. Adapter NOT registered.
7b. Env unset, `SkipAuthCheck=true` → expect `nil` error. Adapter
    registered.
7c. Env set, `SkipAuthCheck=false` → expect `nil` error. Adapter
    registered.
7d. Env set, `SkipAuthCheck=true` → expect `nil` error. Adapter
    registered.

**When**: For each cell:

```
os.Setenv("FAKE_API_KEY", "...") // or os.Unsetenv per cell
err := r.RegisterWithOptions(adapter, RegisterOptions{SkipAuthCheck: ...})
```

**Then**:
- The expected error condition holds.
- `len(r.List())` reflects whether the adapter was added.

**And**: If `Capabilities().RequiresAuth == false`, the env vars are
not checked regardless of `SkipAuthCheck`.

**Verification test**: `TestRegisterRequiresAuthEnvVarsTable` in
`internal/adapters/registry_test.go`.

---

### Scenario 8 — Error taxonomy and CategorizeError mapping

Maps to REQ-CORE-008.

**Given**: A table of error inputs and expected category outputs:

| Input | Expected Category |
|-------|-------------------|
| `nil` | `CategoryUnknown` (no error means no category to classify) |
| `types.ErrTransient` | `CategoryTransient` |
| `types.ErrPermanent` | `CategoryPermanent` |
| `types.ErrRateLimited` | `CategoryRateLimited` |
| `types.ErrSourceUnavailable` | `CategoryUnavailable` |
| `&types.SourceError{Category: types.CategoryPermanent, Cause: io.EOF}` | `CategoryPermanent` |
| `context.DeadlineExceeded` | `CategoryTransient` (timeouts are retryable) |
| `errors.New("random error")` | `CategoryUnknown` |

**When**: `types.CategorizeError(input)` is called for each row.

**Then**: The result equals the expected Category.

**And**, additionally:
- `errors.Is(srcErr, types.ErrTransient)` returns true when
  `srcErr.Category == types.CategoryTransient` (Is method match).
- `errors.Unwrap(srcErr)` returns `srcErr.Cause`.
- `srcErr.Error()` includes `srcErr.Adapter`, the category name, and the
  inner cause's `Error()`.

**Verification tests**: `TestCategorizeErrorTable`,
`TestSourceErrorIsMatchesSentinels`, `TestSourceErrorUnwrapsCause` in
`pkg/types/errors_test.go`.

---

### Scenario 9 — OutcomeFromError returns the canonical Prometheus label

Maps to REQ-CORE-008 + NFR-CORE-002.

**Given**: A table of error inputs and expected outcome label values:

| Input | Expected Outcome |
|-------|------------------|
| `nil` | `"success"` |
| `types.ErrPermanent` | `"failure"` |
| `context.DeadlineExceeded` | `"timeout"` |
| `&types.SourceError{Category: types.CategoryRateLimited}` | `"rate_limited"` |
| `&types.SourceError{Category: types.CategoryUnavailable}` | `"unavailable"` |
| `errors.New("random")` | `"failure"` (catch-all for uncategorised errors) |

**When**: `types.OutcomeFromError(input)` is called.

**Then**: The returned string matches the expected outcome.

**And**: Every value in the codomain is a member of the 5-value enum
declared by NFR-CORE-002 (with `transient` reserved internally for the
wrappedAdapter's catch-all branch).

**Verification test**: `TestOutcomeFromErrorTable` in
`pkg/types/errors_test.go`.

---

### Scenario 10 — pkg/types has zero internal/prometheus/otel imports

Maps to NFR-CORE-003.

**Given**: The `pkg/types` package.

**When**: `go list -deps -json github.com/elymas/universal-search/pkg/types/...`
is executed and parsed.

**Then**:
- No item in the resulting `Imports` array begins with
  `github.com/elymas/universal-search/internal/`.
- No item begins with `github.com/prometheus/`.
- No item begins with `go.opentelemetry.io/`.
- All listed direct imports are stdlib paths (`context`, `time`,
  `errors`, `encoding/json`, `hash`, etc.) or `github.com/cespare/xxhash/v2`
  (the existing transitive dep promoted to direct, if Open Question 1 is
  resolved in xxhash's favor).

**Verification test**: `TestPkgTypesNoInternalImports` in
`pkg/types/types_test.go`.

---

### Scenario 11 — wrappedAdapter is nil-safe under partial Obs

Maps to REQ-CORE-004 (graceful degradation).

**Given**: A registry constructed with a partially-populated obs bundle:

```
obs := &obs.Obs{Logger: nil, Metrics: nil}  // both nil
r := adapters.NewRegistry(obs)
_ = r.Register(noop.New("test"))
```

**When**: `r.Get("test").Search(ctx, types.Query{})` is called.

**Then**:
- The call does NOT panic.
- The call returns `(nil, nil)` (the noop's behavior).
- No metric increment occurs (no panic from nil dereference).
- No log line is emitted (Logger is nil).
- An OTel span is still created via the no-op tracer fallback (because
  the test asserts it doesn't crash, not that a span exists).

**Verification test**: `TestWrappedAdapterSafeOnNilObs` in
`internal/adapters/registry_test.go`.

---

### Scenario 12 — Validate performance under 1µs/doc

Maps to NFR-CORE-001.

**Given**: A fully-populated `NormalizedDoc` with all 15 fields set.

**When**: `BenchmarkNormalizedDocValidate(b)` runs `b.N` iterations of
`doc.Validate()`.

**Then**:
- `b.ReportAllocs()` shows zero allocations per call.
- The reported `ns/op` is less than 1000 (i.e., < 1 µs per call) on
  amd64 hardware (CI runner).

**And**: `BenchmarkNormalizedDocCanonicalHash(b)` reports < 5 µs/op on
amd64 (not zero allocations, since it constructs an internal byte buffer
for the hash).

**Verification test**: `BenchmarkNormalizedDocValidate`,
`BenchmarkNormalizedDocCanonicalHash` in `pkg/types/bench_test.go`.

## 2. Edge Cases

### 2.1 Nil context

When any `Adapter.Search(ctx, q)` is called with `ctx == nil`, the
wrappedAdapter MUST treat it as an immediate `context.Canceled`
(internally, `context.Background()` is substituted, but the call is
gated). This prevents the adapter from spawning goroutines tied to a
nil context that has no `Done()` channel.

Test: `TestWrappedAdapterRejectsNilContext` returns an error wrapping
`context.Canceled` (the test does not require this to be the underlying
adapter's error — wrappedAdapter pre-checks).

### 2.2 Empty query

When `Adapter.Search(ctx, types.Query{Text: ""})` is called, the wrapper
does not pre-validate — it lets the underlying adapter decide. Many
adapters legitimately accept empty queries (e.g., "list trending posts"
on Reddit). This behavior is OUT OF CORE-001 scope; FAN-001 may add
pre-flight validation.

Test: `TestWrappedAdapterAllowsEmptyQuery` confirms no pre-check
rejection at the wrapper layer.

### 2.3 Oversized result set

If `Adapter.Search` returns more than `Query.MaxResults` docs, the
wrappedAdapter does NOT truncate. Truncation belongs to FAN-001 (the
fanout policy may want to keep "extra" docs for dedup tiebreakers).

Test: `TestWrappedAdapterDoesNotTruncate` confirms returned slice
length matches what the adapter returned, regardless of MaxResults.

### 2.4 Context cancellation during Search

If ctx is cancelled mid-Search, the wrappedAdapter:
- Returns whatever (docs, err) the underlying adapter returned.
- Classifies via OutcomeFromError; if err is `context.Canceled`, outcome
  is `failure` (Canceled is treated as a permanent error from the
  query's perspective — the user gave up).

Test: `TestWrappedAdapterClassifiesCancelAsFailure`.

Note: `context.DeadlineExceeded` is distinct and maps to `timeout`.

### 2.5 NormalizedDoc with non-RFC-3339 PublishedAt JSON input

When unmarshaling JSON whose `published_at` field is malformed (e.g.,
`"published_at": "yesterday"`), `json.Unmarshal` returns an error per Go
stdlib behavior. The Validate method does NOT need to handle this — it
runs after Unmarshal succeeds.

Test: `TestNormalizedDocRejectsMalformedTimestampOnUnmarshal`.

### 2.6 Metadata map with circular references

`json.Marshal` rejects circular references with an error. Validate does
not check Metadata structure. This is acceptable — adapters are
trusted to construct sensible Metadata maps.

No specific test required; covered by stdlib behavior.

## 3. Quality Gate Criteria

These thresholds must be met before SPEC-CORE-001 is declared complete.

| Criterion | Threshold | Source |
|-----------|-----------|--------|
| Coverage (pkg/types/) | ≥ 85% | `quality.yaml` |
| Coverage (internal/adapters/) | ≥ 85% | `quality.yaml` |
| `go vet ./...` | clean | `.claude/rules/moai/languages/go.md` |
| `golangci-lint run` | zero issues | ditto |
| `go test -race ./...` | clean (no race warnings) | ditto |
| `BenchmarkNormalizedDocValidate` | < 1 µs/op on amd64 | NFR-CORE-001 |
| `BenchmarkNormalizedDocCanonicalHash` | < 5 µs/op on amd64 | NFR-CORE-001 |
| `TestNoUnboundedLabels` (extended for outcome) | passes | NFR-CORE-002 |
| `TestPkgTypesNoInternalImports` | passes | NFR-CORE-003 |
| All 38 functional tests | pass | spec.md §8 |
| @MX:ANCHOR present on Adapter, Registry, Register, Get, NormalizedDoc, CategorizeError | yes | plan.md §4 |
| @MX:WARN present on RegisterWithOptions duplicate-name detection | yes | plan.md §4 |
| @MX:NOTE on each NormalizedDoc field with non-obvious invariant | yes | plan.md §4 |
| Zero new metric collectors introduced | confirmed | plan.md §3 P6 |
| Zero new direct go.mod dependencies | confirmed | spec.md §9.4 |
| Pre-submission self-review checklist completed | all items checked | plan.md §7 |

## 4. Definition of Done

SPEC-CORE-001 is **done** when ALL of the following hold:

- [ ] All 12 Given/When/Then scenarios in §1 pass via automated tests.
- [ ] All 6 Edge-Case behaviors in §2 are covered by tests.
- [ ] All 16 Quality Gate Criteria in §3 are met.
- [ ] `pkg/types/{normalized_doc,adapter,query,capabilities,errors}.go`
      and their `*_test.go` siblings exist with the field/method shapes
      specified in spec.md §6.
- [ ] `internal/adapters/registry.go` exists with the Registry,
      wrappedAdapter, RegisterOptions, RegistryError shapes specified.
- [ ] `internal/adapters/noop/noop.go` is under 50 LoC and has the
      compile-time interface assertion.
- [ ] `pkg/types/types.go` and `internal/adapters/adapters.go` stub
      replacements have package-level godoc and SPEC reference.
- [ ] `docs/dependencies.md` shows zero net change (per spec.md §9.4).
- [ ] `internal/obs/metrics/metrics.go` is unchanged (per plan.md §3 P6).
- [ ] PR includes the spec.md, plan.md, acceptance.md, research.md, and
      spec-compact.md updates referenced from the SPEC frontmatter `id`.
- [ ] `git log --oneline -1` shows a Conventional Commit referencing
      `SPEC-CORE-001` per the project's commit format.
- [ ] CI green: `go test`, `golangci-lint`, race detector, coverage
      threshold all met.
- [ ] Bench job (scheduled weekly per SPEC-OBS-001 NFR-OBS-001 cadence)
      passes the < 1 µs/op gate.
- [ ] No `TODO` comments remain in the implementation files (only in
      tests if explicitly justified).
- [ ] Plan-auditor (per CLAUDE.md §4) signs off on the implementation.
- [ ] Sync phase (`/moai sync SPEC-CORE-001`) generates
      `docs/api-reference/types.md` documenting the public `pkg/types`
      surface.

## 5. Out-of-Scope Confirmations

These are NOT acceptance criteria for SPEC-CORE-001 (they are listed in
spec.md §7 Exclusions and are restated here for the run-phase reviewer):

- The implementation MUST NOT include any per-source adapter (Reddit,
  HN, etc.) — those land in SPEC-ADP-001 through SPEC-ADP-009.
- The implementation MUST NOT include fanout / parallel dispatch — that
  is SPEC-FAN-001's territory.
- The implementation MUST NOT include index ingestion — SPEC-IDX-001.
- The implementation MUST NOT register new Prometheus collectors —
  reuses SPEC-OBS-001's `AdapterCalls` and `AdapterCallDuration`.
- The implementation MUST NOT modify SPEC-OBS-001's allowlist (only
  enumerate the 5 outcome values; the allowlist already accepts
  `outcome` as a label).
- The implementation MUST NOT add streaming (channel-based) Search —
  V1 returns `[]NormalizedDoc`.

If any of these scope violations are observed during code review, the PR
is rejected and the responsible scope item is migrated to its
destination SPEC.

---

*End of acceptance.md v0.1*
