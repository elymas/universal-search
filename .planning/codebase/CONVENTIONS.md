# Coding Conventions

**Analysis Date:** 2026-06-04

This project is a polyglot monorepo. The primary plane is **Go 1.25.8** (`internal/`, `pkg/`, `cmd/`), with a **Next.js 16 / React 19 / TypeScript** frontend in `web/`, and several **Python** sidecar services in `services/`. This document focuses on Go and TypeScript conventions, plus the cross-cutting `@MX` annotation and SPEC-traceability systems that apply repo-wide.

## Naming Patterns

**Files (Go):**

- snake_case for all `.go` files: `cascade_helpers.go`, `phase5_browser.go`, `longform_source.go`
- Test files: `*_test.go` co-located in the same package (e.g., `internal/access/errors_test.go`)
- Integration tests: `*_integration_test.go` (e.g., `internal/index/index_integration_test.go`)
- Benchmarks: `bench_test.go` (e.g., `internal/fanout/bench_test.go`)
- One concept per file; helpers split out (`internal/access/cascade_helpers.go`, `internal/llm/retry.go`, `internal/llm/cost.go`)

**Files (Web):**

- kebab-case for components and modules: `web/src/components/search-input.tsx`, `web/src/lib/sse-client.ts`
- Route segments follow Next.js App Router: `web/src/app/page.tsx`, `web/src/app/admin/page.tsx`
- Private route-local components in `_components/` folders: `web/src/app/admin/_components/audit-viewer.tsx`
- Tests in `__tests__/` subfolders: `web/src/components/__tests__/sidebar-nav.test.tsx`

**Packages (Go):**

- Short, lowercase, single-word package names matching the directory: `package access`, `package llm`, `package streamsynth`
- Package doc comment on a representative file (`internal/llm/llm.go` lines 1-16) describing scope and listing REQ IDs

**Functions / Types (Go):**

- Exported identifiers in PascalCase (`Client`, `New`, `StreamSynthesize`, `ModelClass`)
- Unexported in camelCase (`contextOutcome`, `derivePhaseCtx`, `newDefaultClient`, `toFloat`)
- Constructors named `New` or `New<Thing>` returning the interface type, not the concrete struct (`func New(cfg config.Config, o *obs.Obs) (Client, error)` in `internal/llm/llm.go:128`)
- Interfaces are consumer-defined where it enables decoupling (see `internal/streamsynth/longform_source.go` `LongFormSource`)

**Constants / Enums (Go):**

- Typed string enums: `type ModelClass string` with grouped `const` block (`internal/llm/llm.go:28-39`)
- Each enum value carries a godoc line explaining its routing/mapping

**Variables (Web):**

- camelCase for variables/functions, PascalCase for React components and types
- Path alias `@/*` maps to `web/src/*` (`web/tsconfig.json` paths, mirrored in `web/vitest.config.ts`)

## Code Style

**Formatting (Go):**

- `gofmt` + `goimports` enforced via golangci-lint `formatters` block (`.golangci.yml`)
- Tabs, indent width 4 (`.editorconfig` `[*.go]`)
- LF line endings, UTF-8, final newline, trimmed trailing whitespace repo-wide (`.editorconfig` `[*]`)

**Linting (Go):**

- golangci-lint v2 config at `.golangci.yml`, `default: none` with an explicit allowlist:
  - `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`
- `errcheck` is disabled for `_test.go` files (exclusion rule in `.golangci.yml`)
- 5-minute timeout; CI runs `go vet ./...` then `golangci-lint run` (`.github/workflows/go.yml`)

**Formatting (Web):**

- Prettier (`web/.prettierrc.json`): `semi: true`, `singleQuote: true`, `tabWidth: 2`, `trailingComma: "es5"`
- ESLint flat config (`web/eslint.config.mjs`): `eslint-config-next/core-web-vitals` + `eslint-config-prettier`
- TypeScript `strict: true` (`web/tsconfig.json`); run `pnpm --dir web typecheck` (= `tsc --noEmit`)
- 2-space indent for `.ts/.tsx/.js/.jsx` (`.editorconfig`)

**Formatting (Python services):**

- ruff (each service has `.ruff_cache`); 4-space indent (`.editorconfig` `[*.py]`)

## Import Organization

**Go:**

- `goimports` groups imports: stdlib first, then third-party + first-party (module path `github.com/elymas/universal-search/...`)
- Example grouping in `internal/llm/llm.go:18-24`: stdlib (`context`, `errors`) then internal packages
- **Architectural import rule:** domain packages MUST NOT import `github.com/openai/openai-go` directly — all LLM I/O flows through `internal/llm` (`internal/llm/llm.go:8`). Treat such "MUST NOT import X" package-doc directives as hard boundaries.

**Web:**

- Path alias `@/*` for intra-`src` imports; relative imports otherwise

## Error Handling

**Go — the dominant pattern:**

- **Sentinel errors** declared as package-level `var Err...` with package-prefixed messages. Group them in a dedicated `errors.go` per package:
  - `internal/access/errors.go`, `internal/fanout/errors.go`, `internal/llm/llm.go:108-121`
  - Message convention: `"<package>: <lowercase description>"` (e.g., `errors.New("llm: per-request budget exceeded")`)
- **Wrapping** with `fmt.Errorf("...: %w", err)` to preserve the chain (`internal/streamsynth/streamsynth.go:126`, `internal/llm/cost.go:103`)
- Wrap messages are short, lowercase, action-first context prefixes: `"marshal sentence: %w"`, `"emit agent event %s: write: %w"`
- **Comparison** via `errors.Is` / `errors.As`, never string matching (`internal/llm/client.go:81,116`; `internal/llm/retry.go:46`)
- A sentinel can be combined into a wrapped error: `fmt.Errorf("%w: cost %.6f exceeds cap %.6f", ErrBudgetExceeded, ...)` (`internal/llm/cost.go:103`)
- Some calls return both a value and a sentinel error (documented contract): `ErrBudgetExceeded` is returned alongside a populated `Response` (`internal/llm/llm.go:109-110`)
- Errors never ignored with `_` (golangci `errcheck` enforces this in non-test code)

**Go — context:**

- `context.Context` is the first parameter of every blocking method (`Complete`, `Stream`, `Embed` in `internal/llm/llm.go:101-103`)
- Context cancellation is explicitly honored in goroutines and loops; these sites are flagged with `@MX:WARN` (see below)

**Web:**

- `web/src/lib/api.ts` and `web/src/lib/sse-client.ts` centralize fetch/SSE error handling

## Logging & Observability

- Telemetry flows through the `internal/obs` bundle (`*obs.Obs`), passed explicitly into constructors (`internal/llm/llm.go:128`) rather than via globals
- Prometheus client (`github.com/prometheus/client_golang`) is a direct dependency (`go.mod`)
- Secrets are never logged — documented as a hard requirement (`internal/llm/llm.go:12`, REQ-LLM-005)

## Comments & Documentation

**Go:**

- Every exported type, function, and interface has a godoc comment (enforced culturally, not by linter)
- Package-level doc comment lists the REQ/NFR IDs the package satisfies (`internal/llm/llm.go:10-15`)
- Inline comments cite the requirement they implement: `// Resolve [N] markers; skip uncited sentences (REQ-SYN4-001c).` (`internal/streamsynth/streamsynth.go:111`)
- Comments are English per `.moai/config/sections/language.yaml` `code_comments: en`

## Function & Module Design

- Constructors return interfaces, keeping concrete structs unexported (`newDefaultClient` is private; `New` is the only public entry)
- Helper extraction: complex flows are decomposed into single-purpose files (`retry.go`, `cost.go`, `stream.go`, `cascade_helpers.go`)
- `defer cancel()` / `defer cancelPhase()` for cleanup of contexts and resources (`internal/access/cascade_helpers_test.go:32,35`)

---

## The @MX Tag Annotation System

`@MX` tags are machine-readable, agent-authored code annotations that persist context, invariants, and danger zones across development sessions. They are used heavily here (≈697 tags across `internal/`, `pkg/`, `cmd/`). Source protocol: `.claude/rules/moai/workflow/mx-tag-protocol.md`.

**Syntax** (Go uses `//`):

```go
// @MX:ANCHOR: [AUTO] Primary LLM interface; callers: cmd/usearch, SPEC-SYN-001, SPEC-DEEP-*, tests
// @MX:REASON: fan_in >= 3; all LLM I/O in Go plane flows through this interface
```

**Tag types and observed frequency:**

| Tag          | Count | Meaning                          | When to add                                                                |
| ------------ | ----- | -------------------------------- | -------------------------------------------------------------------------- |
| `@MX:SPEC`   | 233   | Links code/test to a SPEC ID     | Test or code implementing a specific SPEC (`internal/sse/close_test.go:5`) |
| `@MX:REASON` | 162   | Mandatory justification sub-line | Required on every WARN and ANCHOR                                          |
| `@MX:NOTE`   | 134   | Context / intent delivery        | Magic constants, unexplained business rules                                |
| `@MX:ANCHOR` | 116   | Invariant contract               | Functions with fan_in ≥ 3 or public API boundaries                         |
| `@MX:WARN`   | 47    | Danger zone                      | Goroutine lifecycle, ctx-cancellation, retry-with-sleep                    |
| `@MX:TODO`   | 5     | Incomplete work                  | Untested public fn, unimplemented SPEC requirement                         |

**Rules observed in this codebase:**

- Agent-generated tags carry the `[AUTO]` prefix (`internal/llm/llm.go:98`)
- `@MX:ANCHOR` and `@MX:WARN` always pair with a `@MX:REASON` sub-line stating the invariant or danger (`internal/streamsynth/streamsynth.go:8-9`, `internal/llm/stream.go:26-27`)
- `@MX:WARN` is applied to goroutine coordination and ctx-cancellation sites (`internal/streamsynth/streamsynth.go:91`, `internal/sse/heartbeat.go:5`, `internal/llm/retry.go:64`)
- `@MX:ANCHOR` records the actual caller list, justifying the `fan_in >= 3` threshold
- `@MX:SPEC` tags attach release-gate / SPEC linkage to tests (`internal/streamsynth/emit_marshal_error_test.go:6`)

**When writing new code:** add `@MX:ANCHOR` (+`@MX:REASON`) for any new function reaching 3+ callers or forming a public boundary; add `@MX:WARN` (+`@MX:REASON`) when introducing goroutines, channels, or retry loops; never auto-delete an existing `@MX:ANCHOR`.

---

## SPEC-Based Traceability

Requirements are tracked through the code with stable IDs that link source, tests, and SPEC documents under `.moai/specs/`.

**ID forms observed:**

- `SPEC-<AREA>-<NNN>`: a specification (e.g., `SPEC-SYN-002`, `SPEC-LLM-001`, `SPEC-REL-001`, `SPEC-DEEP-001`)
- `REQ-<AREA><N>-<NNN>`: a functional requirement (e.g., `REQ-SYN4-002`, `REQ-DEEP1-005`, `REQ-IDX-001`)
- `NFR-<AREA>-<NNN>`: a non-functional requirement (e.g., `NFR-LLM-002` per-provider circuit breaker)

**How traceability is expressed:**

- Package doc comments enumerate implemented REQs (`internal/llm/llm.go:10-15`)
- Inline comments cite the requirement at the implementation site (`internal/streamsynth/streamsynth.go:3-6`)
- Type doc comments reference the SPEC section: `// Fields match SPEC-LLM-001 §5 acceptance criteria for REQ-LLM-002` (`internal/llm/llm.go:59`)
- Tests name the requirement they verify (`TestSSEEmitsSectionStartPerSection verifies REQ-DEEP1-005...` in `internal/streamsynth/longform_test.go:46`)
- Integration test headers list all covered REQ IDs (`internal/index/index_integration_test.go`)

**When adding code:** carry the originating SPEC/REQ ID into both the implementation comment and the test name so the requirement remains traceable end-to-end.

---

## Conventional Commits

Commit messages follow the Conventional Commits spec (`git log` history):

**Format:** `type(scope): subject`

**Types observed:** `fix`, `feat`, `chore`, `test`, `release`, `merge`

**Scopes observed:** component or area, e.g. `searxng`, `web`, `deploy`, `spec`, `release`, `security`, `coverage`

**SPEC linkage in commits:** the subject frequently embeds the SPEC ID and PR number:

- `test(coverage): SPEC-REL-001 — reach 85% for G1 release gate (#53)`
- `fix(security): SPEC-SEC-001 — reconcile ratelimit/ssrf test↔impl API mismatch`
- `release: v1.0.0 — integrate M8+M9 (SEC/EVAL/DEPLOY/DOC/REL) (#50)`

**Conventions:**

- Subject in imperative mood, lowercase after the colon
- Em-dash (`—`) separates the SPEC ID from the human description
- Milestone markers (`M8`, `M9`) and gate IDs (`G1`, `G11`) appear where relevant
- Per `.moai/config/sections/language.yaml`, commit messages are in English

---

_Convention analysis: 2026-06-04_
