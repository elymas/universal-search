# SPEC-CLI-001 Plan — Post-Hoc Implementation Summary

Created: 2026-04-28
Updated: 2026-05-04 (post-implementation reconciliation)
Author: limbowl (via manager-spec)
Status: implemented
Methodology: TDD (RED-GREEN-REFACTOR)
Coverage Target: 80% (per spec.md frontmatter)

## 0. Plan Scope

Reverse-engineered description of how SPEC-CLI-001 was implemented. The
`query` subcommand and its supporting files were delivered on
2026-05-04 as the FIRST user-facing surface of the system. The file
inventory below reflects the state at SPEC-CLI-001 completion;
SPEC-CLI-002 subsequently migrated the dispatcher from stdlib `flag`
to cobra (`root.go`, `query.go::Execute` wrapper preserved). Read
alongside spec.md (requirements) and acceptance.md (Given/When/Then
scenarios).

## 1. Approach Summary

A single linear pipeline implements the user-visible query flow:
`parseQueryFlags → empty-prompt check → ctx.WithTimeout → obs.Init →
router.Classify → runFanout → synthesis.Synthesize → formatText/JSON →
exit code`. The pipeline lives in `cmd/usearch/query.go::Execute`,
which is the testable entry point (separate from `main`). All
side-effecting boundaries (stdout, stderr, exit) are passed as
parameters so `Execute` can run in-process under table-driven tests.
The `internal/synthesis/` package was created with a `Client`
interface and a `nopclient.Client` no-op fallback so the binary
compiles before SPEC-SYN-001 ships; with the nop client, `query`
exits 3 (partial) and degrades to a raw-snippet rendering with a
`[synthesis: unavailable]` stderr warning.

## 2. Reference Implementations (consumed)

| Concern | Reference (file:line) | Pattern reused |
|---------|-----------------------|----------------|
| Adapter registry construction | `internal/adapters/registry.go` (SPEC-CORE-001) | `NewRegistry(obs)`, `Register(adapter)`, `Get(name)`, `List()` |
| Router classification | `internal/router/router.go` (SPEC-IR-001) | `router.New(Options{Registry, LLMClient, Obs})`, `Classify(ctx, RouterQuery) → RoutingDecision` |
| Observability init | `internal/obs/obs.go::Init` (SPEC-OBS-001) | `obs.Init(ctx, Config)` returns `*Obs + shutdown + err`; `obs.Logger`, `obs.Tracer` |
| Request ID | `internal/obs/reqid/reqid.go::New`/`WithContext` (SPEC-OBS-001) | ULID generation; ctx propagation; stderr slog enrichment |
| LLM client | `internal/llm/client.go::New` (SPEC-LLM-001) | conditional construction when `LITELLM_MASTER_KEY` env is set |
| Fanout (interim) | inline `runFanout` in `cmd/usearch/query.go` | errgroup-WithContext + per-adapter goroutine; replaced by SPEC-FAN-001 in M3 |
| Test process-boundary pattern | `cmd/usearch/main_test.go` (SPEC-BOOT-001) | `os/exec.Command("go", "run", ".", ...)` for end-to-end CLI tests |

## 3. Package Layout (as implemented)

```
cmd/usearch/                  # CLI subcommand surface
├── main.go                   # entrypoint (post-CLI-002: cobra root delegation)
├── main_test.go              # BOOT-001 version tests + CLI-001 dispatch tests
├── query.go                  # Execute, runQuery, runFanout, parseQueryFlags, intersectSources
├── query_test.go             # table-driven TestExecute + TestMain(goleak)
├── query_response.go         # queryResponse struct used by formatters
├── output_text.go            # formatText
├── output_text_test.go
├── output_json.go            # formatJSON + schema_version="1"
├── output_json_test.go
├── progress.go               # progressEmitter interface + impls
├── progress_test.go
├── exitcode.go               # ExitOK, ExitUserError, ExitSystemError, ExitPartial constants + classifyError
├── exitcode_test.go
├── integration_test.go       # NFR-CLI-001 build-tagged integration
├── coverage_supplement_test.go  # post-implementation coverage gap closure

internal/synthesis/           # NEW package introduced by CLI-001
├── client.go                 # Client interface, Request, Response, Citation, ErrSynthesisUnavailable
└── nopclient/
    ├── nopclient.go          # no-op fallback returning ErrSynthesisUnavailable
    └── nopclient_test.go
```

Note: subsequent SPEC-CLI-002 added `root.go`, `repl.go`, `deep_cmd.go`,
`config_cmd.go`, `history_cmd.go`, `login_cmd.go`, `sources_cmd.go`,
`format_markdown.go` and migrated `main.go` to delegate via cobra. The
CLI-001 file inventory above is the snapshot at CLI-001 completion.

## 4. Key Implementation Files (file:line refs)

### Pipeline entry
- `cmd/usearch/query.go::Execute(ctx, args, stdout, stderr) int` —
  testable entry point invoked by `main` (or by cobra `RunE` post-CLI-002).
- `cmd/usearch/query.go::parseQueryFlags(args) (queryFlags, prompt, err)` —
  uses `*flag.FlagSet` named `"query"` so flags do not leak between
  subcommands (preserved in CLI-002 cobra migration via cobra flag
  isolation).
- `cmd/usearch/query.go::runQuery` — orchestrates the classify →
  fanout → synthesize → format → exit chain.

### Fanout (interim, replaced by SPEC-FAN-001)
- `cmd/usearch/query.go::runFanout(ctx, decision, registry, prompt) ([]NormalizedDoc, []error)`:
  uses `errgroup.WithContext`; one goroutine per `decision.AdapterSet`
  member; results merged in adapter-name order. @MX:WARN — temporary
  scaffolding pending SPEC-FAN-001.

### Formatters
- `cmd/usearch/output_text.go::formatText(w, resp)` — human-readable
  summary + `Citations:\n[N] <Title> — <URL>`.
- `cmd/usearch/output_json.go::formatJSON(w, resp)` — single JSON object
  with `schema_version="1"` + keys
  `{schema_version, query, category, lang, adapters, summary,
  citations[], stats}`.

### Exit codes
- `cmd/usearch/exitcode.go` — constants:
  - `ExitOK = 0`
  - `ExitUserError = 1`
  - `ExitSystemError = 2`
  - `ExitPartial = 3`
- `classifyError(err) int` maps known errors to exit codes.

### Progress emission
- `cmd/usearch/progress.go::progressEmitter` interface; two impls:
  - `humanProgress` — writes `[router]`, `[fanout]`, `[synthesis]`
    markers to stderr in text mode.
  - `jsonProgress` — no-op (because `obs.Logger` already emits
    structured slog JSON to stderr).

### Synthesis interface (NEW package)
- `internal/synthesis/client.go::Client` interface with `Synthesize(ctx,
  Request) (*Response, error)`.
- `internal/synthesis/client.go::Request{Query, Decision, Docs}`.
- `internal/synthesis/client.go::Response{Summary, Citations, CostUSD}`.
- `internal/synthesis/client.go::Citation{Index, Title, URL, Source, DocID}`.
- `internal/synthesis/client.go::ErrSynthesisUnavailable` sentinel.
- `internal/synthesis/nopclient/nopclient.go::Client` returns
  `ErrSynthesisUnavailable` from `Synthesize`; used by CLI-001 when
  SPEC-SYN-001 is not yet implemented or `LITELLM_MASTER_KEY` is unset.

## 5. Integration Points

| Upstream SPEC | Consumed via |
|---------------|--------------|
| SPEC-OBS-001 | `obs.Init(ctx, Config)` returns the `*Obs` bundle; `obs.Logger(ctx)` enriches stderr; `obs.Tracer("usearch.cli")` creates the root span |
| SPEC-OBS-001 reqid | `reqid.New()` generates ULID; `reqid.WithContext(ctx, id)` propagates; visible in JSON `stats.request_id` and every stderr slog record |
| SPEC-LLM-001 | `llm.New(cfg, obs)` constructs the client when `LITELLM_MASTER_KEY` is set; consumed by router's LLM fallback path |
| SPEC-IR-001 | `router.New(Options{Registry, LLMClient, Obs})` + `router.Classify(ctx, RouterQuery)` returns `RoutingDecision{Category, Lang, AdapterSet}` |
| SPEC-CORE-001 | `adapters.NewRegistry(obs)` + `Register(adapter)` + `Get(name)` + `List()`; `types.Query{Text, Deadline}`; `types.NormalizedDoc` |
| SPEC-ADP-001 | Reddit adapter registered when its env credentials are present |
| SPEC-ADP-002 (forward) | HN adapter registered when its env credentials are present; CLI runs Reddit-only if absent |
| SPEC-SYN-001 (forward) | `synthesis.Client` interface; nopclient soft-fail when absent |

| Downstream SPEC | Provides |
|-----------------|----------|
| SPEC-CLI-002 | Replaces stdlib `flag` with cobra; preserves `Execute` boundary verbatim; adds subcommands |
| SPEC-MCP-001 | Reuses `synthesis.Client` interface |
| SPEC-SYN-004 | Builds streaming on top of the `formatText`/`formatJSON` layer |
| SPEC-FAN-001 | Replaces `runFanout` with the proper goroutine pool |

## 6. Data Structures and Interfaces

### CLI surface
```go
type queryFlags struct {
    Source  []string       // parsed comma-separated from --source
    Format  string         // "text" | "json"
    Timeout time.Duration  // default 30s, max 300s
}

const (
    ExitOK         = 0  // full success
    ExitUserError  = 1  // bad flag / empty prompt / invalid format
    ExitSystemError = 2 // all adapters failed / no adapters / timeout
    ExitPartial    = 3  // partial result (some adapters failed OR synthesis failed)
)

// Execute is the testable CLI entry point.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int
```

### Response shape (consumed by formatters)
```go
type queryResponse struct {
    Query     string
    Category  string
    Lang      string
    Adapters  []string                // effective adapter set
    Summary   string                  // from synthesis.Response.Summary
    Citations []synthesis.Citation
    Stats     responseStats           // request_id, latency, cost
}
```

### Synthesis interface
```go
package synthesis

type Client interface {
    Synthesize(ctx context.Context, req Request) (*Response, error)
}

type Request struct {
    Query    string
    Decision router.RoutingDecision
    Docs     []types.NormalizedDoc
}

type Response struct {
    Summary   string
    Citations []Citation
    CostUSD   float64
}

type Citation struct {
    Index  int    `json:"index"`
    Title  string `json:"title"`
    URL    string `json:"url"`
    Source string `json:"source"`
    DocID  string `json:"doc_id"`
}

var ErrSynthesisUnavailable = errors.New("synthesis: client unavailable")
```

## 7. Test Coverage Notes

- `cmd/usearch/query_test.go` — table-driven `TestExecute` covering
  14 case rows (success_text, success_json, empty_prompt,
  whitespace_prompt, invalid_format, invalid_source, timeout_exceeded,
  timeout_max, no_adapters, source_filter, source_unknown,
  partial_failure, synthesis_failure, nop_synthesis,
  all_adapters_fail).
- `cmd/usearch/query_test.go::TestMain` invokes
  `goleak.VerifyTestMain(m)` (NFR-CLI-002).
- `cmd/usearch/output_text_test.go`,
  `cmd/usearch/output_json_test.go` — formatter shape tests
  (REQ-CLI-004).
- `cmd/usearch/exitcode_test.go` — classifyError table.
- `cmd/usearch/progress_test.go` — emitter selection by format.
- `cmd/usearch/integration_test.go` (`// +build integration`) —
  stub Reddit + HN + LiteLLM servers; build binary; invoke; assert
  exit ∈ {0, 3}.
- `cmd/usearch/coverage_supplement_test.go` — post-implementation
  tests added to close coverage gaps; brings coverage to 80.1%
  (target 80%).
- `internal/synthesis/nopclient/nopclient_test.go` — round-trip with
  `ErrSynthesisUnavailable`.

Coverage at completion: 80.1% (per spec.md HISTORY).

## 8. MX Tag Plan (applied)

| File::Symbol | Tag | Reason |
|--------------|-----|--------|
| `query.go::Execute` | @MX:ANCHOR | fan_in ≥ 3 (main + tests + future cobra RunE); sole CLI entry boundary |
| `query.go::runFanout` | @MX:WARN | Temporary scaffolding pending SPEC-FAN-001; lacks retry/dedup/ranking |
| `query.go::runFanout` | @MX:ANCHOR | fan_in ≥ 2; clean function boundary for replacement |
| `exitcode.go::classifyError` | @MX:NOTE | Exit-code UX contract; CI scripts depend on the mapping |
| `output_json.go::schemaVersion` | @MX:NOTE | Constant `"1"`; bump on breaking schema changes |
| `progress.go::progressEmitter` | @MX:NOTE | Interface boundary between text and JSON progress modes |

All tags: `[AUTO]` prefix, `@MX:SPEC: SPEC-CLI-001`, `@MX:REASON:`
mandatory for ANCHOR/WARN, `code_comments: en`.

## 9. Risks Realised

| Original Risk | Outcome |
|---------------|---------|
| SPEC-SYN-001 not ready | Mitigated by nopclient soft-fail (REQ-CLI-009); exit code 3 with degraded rendering |
| SPEC-ADP-002 not ready | CLI runs Reddit-only; `--source reddit` works end-to-end |
| Goroutine leak in runFanout | NFR-CLI-002 + `goleak.VerifyTestMain` clean across all test cases |
| Binary size creep | NFR-CLI-003 CI gate at 30 MB held (until SPEC-CLI-002 cobra migration) |
| Stdout pollution | All status via `obs.Logger` to stderr; stdout strictly payload-only |
| LLM timeout cascades | Router's internal 2 s LLM deadline isolates CLI parent ctx |
| Tests bind to live services | All tests use `httptest.Server` stubs; integration test runs against per-test stubs |

## 10. Self-Review Outcome

Resolved questions:
- Could `Execute` be inlined into `main`?
  → No; tests need the in-process testable boundary.
- Could `formatText` / `formatJSON` share more logic?
  → Already minimal; further extraction would obscure each formatter's
  intent.
- Could the nop synthesis fallback be removed?
  → No; SPEC-SYN-001 timing was uncertain; the soft-fail proved
  necessary for end-to-end testability before SYN-001 shipped.
- Why a separate `progressEmitter` interface?
  → JSON mode wants `obs.Logger` to own stderr; text mode wants
  human-readable markers; the interface keeps both call sites
  consistent.

---

*End of plan.md (post-hoc).*
