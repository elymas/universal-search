# Testing Patterns

**Analysis Date:** 2026-06-04

The repo has three test planes: **Go** (the primary plane — 346 `*_test.go` files across `internal/`, `pkg/`, `cmd/`), **Web** (Vitest + Testing Library in `web/`), and **Python** (pytest in `services/*/tests`). The Go plane is the focus; reported total coverage is **86.1%** (`coverage.out`).

## Test Framework

**Go:**

- Runner: standard library `testing` (`go test`). No `testify`/`go-cmp` as the default — assertions are hand-written `if got != want { t.Errorf(...) }`.
  - `testify` appears in only 16 test files; `go-cmp` is not used; `reflect.DeepEqual` appears in 6 files. Prefer plain stdlib assertions to match the dominant style.
- Race detector and coverage are always on in CI and the Makefile.
- Config: none beyond `go.mod` (Go 1.25.8).

**Web:**

- Runner: **Vitest 4** with `@vitejs/plugin-react` (`web/vitest.config.ts`)
- Assertions: `@testing-library/jest-dom/vitest` matchers + `@testing-library/react` / `user-event`
- Environment: `jsdom`, `globals: true`, setup file `web/src/__tests__/setup.ts`
- Include glob: `src/**/*.{test,spec}.{ts,tsx}`

**Python:**

- `pytest` per service via `uv run --directory services/$svc pytest` (`Makefile` `test-py`)

**Run Commands:**

```bash
# Go — all packages, race + coverage (Makefile test-go and CI)
go test ./... -race -cover

# Go — single package, verbose
go test ./internal/llm/... -v

# Go — coverage profile + total percentage
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1     # -> total: ... 86.1%
go tool cover -html=coverage.out               # HTML report

# Go — integration tests (build-tagged, opt-in)
go test -tags=integration ./internal/index/... -v
go test -tags=integration ./tests/integration/... -v

# Web
pnpm --dir web test            # vitest run (one-shot)
pnpm --dir web test:watch      # vitest watch

# Everything (Go + Python + Node typecheck)
make test
```

## Test File Organization

**Location:**

- **Go:** co-located in the same package as the code under test. Unit tests use the production package name (white-box) — e.g., `package access` in `internal/access/cascade_helpers_test.go`, allowing tests to exercise unexported funcs like `contextOutcome`, `derivePhaseCtx`, `toFloat`.
- **Integration tests** in `tests/integration/` use an external test package (`package integration_test` in `tests/integration/deep_tree_test.go`).
- **Web:** `__tests__/` subfolders adjacent to the code (`web/src/components/__tests__/`, `web/src/app/admin/_components/__tests__/`).

**Naming:**

- Unit: `<subject>_test.go` (`errors_test.go`, `cap_check_test.go`)
- Integration: `<subject>_integration_test.go` or files under `tests/integration/` guarded by a build tag
- Benchmarks: `bench_test.go` (one per adapter package, e.g., `internal/adapters/naver/bench_test.go`)
- Web: `<subject>.test.tsx` / `<subject>.test.ts`

**Structure:**

```
internal/<domain>/
├── <feature>.go
├── <feature>_test.go            # white-box unit tests, package <domain>
├── errors.go
├── errors_test.go
├── testdata/                    # fixtures / golden inputs
└── <subject>_integration_test.go  # //go:build integration
```

## Test Structure

**Test function pattern (Go):** one test function per behavior, descriptive name encoding the scenario, `t.Parallel()` at the top:

```go
func TestDerivePhaseCtx_RespectsParentDeadline(t *testing.T) {
    t.Parallel()
    f := &Fetcher{opts: Options{}}
    f.opts.applyDefaults()

    parent, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
    defer cancel()

    ctx, cancelPhase := f.derivePhaseCtx(parent, 3)
    defer cancelPhase()

    deadline, ok := ctx.Deadline()
    if !ok {
        t.Error("derived ctx must have a deadline")
    }
    if time.Until(deadline) > 60*time.Millisecond {
        t.Errorf("deadline %v should be capped by parent deadline", time.Until(deadline))
    }
}
```

Source: `internal/access/cascade_helpers_test.go:24-44`.

**Patterns observed:**

- **`t.Parallel()`** in 184 test files — the default for independent unit tests.
- **`t.Context()`** (Go 1.24+) used for auto-cancelled test contexts in 16 files — preferred over manual `context.Background()` + cleanup.
- **`t.Helper()`** marks helper functions (46 files) so failures report the caller's line.
- **`t.Run(...)`** subtests for table cases (79 files).
- Naming convention `TestFunc_Scenario` (underscore separates subject from condition).
- Failure messages state `got ... want ...` with the input echoed: `t.Errorf("toFloat(%v): got %f, want %f", ...)`.
- `t.Fatalf` for setup/precondition failures, `t.Errorf`/`t.Error` for assertion failures.

## Table-Driven Tests

The canonical pattern: a `[]struct` slice of named cases iterated with `t.Run`:

```go
func TestToFloatCoversAllBranches(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name string
        val  interface{}
        want float64
    }{
        {"float64", float64(3.14), 3.14},
        {"int64", int64(42), 42.0},
        {"string_valid", "2.5", 2.5},
        {"string_invalid", "notanumber", 0},
        {"nil", nil, 0},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            got := toFloat(tc.val)
            if got != tc.want {
                t.Errorf("toFloat(%v): got %f, want %f", tc.val, got, tc.want)
            }
        })
    }
}
```

Source: `internal/deepagent/costguard/cap_check_test.go:270-292`.

**Conventions:**

- Slice variable named `tests`, `cases`, or `testCases`; loop variable `tc` (most common) or `tt`.
- First struct field is `name string`, used as the `t.Run` subtest label.
- Fields ordered: `name`, inputs, then `want...` expectations.
- Each case is a one-line literal — readability via alignment.
- Used both for happy-path coverage and explicit branch-coverage tests (the `..._CoversAllBranches` naming signals intentional coverage targeting for the 85% gate).

## Mocking

**No mock-generation framework** (no `mock_*.go`, no gomock, no mockery). Mocking is done by **hand-written fakes implementing small interfaces**, defined inside the `_test.go` file:

```go
// fakeResponseWriter is a minimal http.ResponseWriter + http.Flusher.
type fakeResponseWriter struct {
    buf    *bytes.Buffer
    header http.Header
}

func newFakeRW() *fakeResponseWriter {
    return &fakeResponseWriter{buf: &bytes.Buffer{}, header: make(http.Header)}
}
```

Source: `internal/streamsynth/streamsynth_test.go:18-26`. Similar in-test fakes appear in `internal/sse/writer_test.go`, `internal/deepagent/agents_test.go`, `internal/access/phase3_test.go`, `internal/security/prompt/sanitize_test.go`.

**What to mock:**

- External I/O boundaries: HTTP `ResponseWriter`/`Flusher` for SSE, LLM clients (the `llm.Client` interface in `internal/llm/llm.go:100` is the seam — production code depends on the interface, tests substitute a fake).
- Dependencies are injected through constructors (`New(cfg, o)`), so a fake is passed in place of the real impl.

**What NOT to mock:**

- Real `context.Context` behavior — tests use genuine `context.WithTimeout`/`t.Context()` to exercise cancellation and deadlines rather than faking it (`internal/access/cascade_helpers_test.go`).
- Standard library types you can construct directly (`bytes.Buffer`, `httptest`).

**Network-style fakes:** for integration tests, `httptest.NewServer` is used (`tests/integration/deep_tree_test.go` imports `net/http/httptest`) instead of mocking transport.

## Fixtures and Factories

- **`testdata/` directories** hold golden/fixture inputs (Go's conventional ignored-by-build dir): `internal/access/testdata`, `internal/auth/testdata/oidc_stub`, `internal/router/testdata`, `internal/adapters/*/testdata`.
- Fixtures are typically JSON adapter responses (each adapter package has a `testdata/` of captured payloads).
- Some `testdata` holds compilable stubs (`internal/auth/testdata/oidc_stub/oidc_stub.go`).
- Factories are lightweight constructor helpers in-test (`newFakeRW()`), not a shared factory package.

## Coverage

**Target:** ~85% — this is a release gate (G1 / SPEC-REL-001). Current reported total: **86.1%** (`go tool cover -func=coverage.out`).

**Profile artifact:** `coverage.out` at repo root (`mode: atomic`, committed-ignored per `chore(spec): ... ignore coverage.out`). Generated whenever `-coverprofile` is passed.

**Branch-targeted tests:** files/functions named `..._CoversAllBranches` and `coverage_gap_test.go` (`internal/deepagent/coverage_gap_test.go`) exist specifically to push branch coverage to the gate threshold — mirror this naming when filling coverage gaps.

**View Coverage:**

```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out          # per-function + total
go tool cover -html=coverage.out          # browser report
```

## Test Types

**Unit Tests:**

- White-box, co-located, `t.Parallel()`, table-driven where multiple cases apply.
- Cover pure functions, context/deadline logic, error wrapping/sentinels, SSE wire formatting.
- No external services required.

**Integration Tests:**

- Gated by `//go:build integration` (first line of file): `tests/integration/deep_tree_test.go`, `internal/index/index_integration_test.go`.
- Excluded from the default `go test ./...` run — only execute with `-tags=integration`.
- Require live infrastructure. `internal/index/index_integration_test.go` header documents the dependencies: live **Qdrant, Meilisearch, PostgreSQL** via `docker compose -f deploy/docker-compose.yml up -d`.
- Use `httptest` servers and real JSON marshalling rather than mocks.

**Benchmarks:**

- `bench_test.go` per adapter package, with `func TestMain` for benchmark setup (`internal/fanout/bench_test.go`, `internal/embedder/bench_test.go`).
- Run with `go test -bench . ./internal/adapters/...`.

**E2E / Web:**

- Web component + a11y + security-regression tests via Vitest + Testing Library (`web/src/app/admin/_components/__tests__/a11y.test.tsx`, `security-regression.test.tsx`).
- No browser E2E runner (Playwright/Cypress) configured in this repo.

## Common Patterns

**Build-tag header (integration):**

```go
//go:build integration

// Package index — integration tests requiring live Qdrant, Meilisearch, PostgreSQL.
// Run with: go test -tags=integration ./internal/index/... -v
package index
```

Source: `internal/index/index_integration_test.go:1-11`.

**Context / deadline testing:**

```go
parent, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
defer cancel()
// ... assert derived ctx respects parent deadline
```

**Error testing (sentinel + wrapping):**

```go
_, err := parseFloat("abc")
if err == nil {
    t.Error("parseFloat: expected error for invalid input, got nil")
}
// For sentinels, assert with errors.Is(err, pkg.ErrSomething) rather than string match.
```

Source: `internal/deepagent/costguard/cap_check_test.go:307-314`.

**SPEC linkage in tests:**

- Test names cite the requirement: `TestSSEEmitsSectionStartPerSection verifies REQ-DEEP1-005...` (`internal/streamsynth/longform_test.go:46`)
- `@MX:SPEC` comments tie release-gate tests to their SPEC (`internal/streamsynth/emit_marshal_error_test.go:6`, `internal/sse/close_test.go:5`)

---

_Testing analysis: 2026-06-04_
