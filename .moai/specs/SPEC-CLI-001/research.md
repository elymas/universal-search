# Research — SPEC-CLI-001: `usearch query` subcommand

Date: 2026-04-28
Author: limbowl (via manager-spec)
Phase: plan / research
Status: draft

This artifact captures the codebase analysis, library survey, and design tradeoffs underpinning SPEC-CLI-001. Every claim is file-cited (path + line range) or URL-cited.

---

## 1. Existing CLI Surface (`cmd/usearch/main.go`)

### 1.1 Current behaviour

`cmd/usearch/main.go:1-83` implements a single-binary entrypoint with two flag handlers:

- `--version` / `-v` (declared at `main.go:24-25`, handled at `main.go:30-33`): prints `usearch v<semver>\n` to **stdout** and exits 0. The version string lives in `const Version = "0.1.0-dev"` (`main.go:21`).
- `--no-llm` (declared at `main.go:26`): suppresses LLM client initialisation. Used by tests and offline runs.

Past `--version`/`-v`, `flag.Parse()` runs unconditionally (`main.go:27`). The binary then:

1. Calls `obs.Init` with service metadata + admin port + OTLP endpoint from env (`main.go:36-47`).
2. Optionally constructs an LLM client when `LITELLM_MASTER_KEY` is set (`main.go:53-71`).
3. Prints `"usearch: no command given. Use --version for version info."` to **stderr** and exits 1 (`main.go:73-74`).

The current main is therefore a degenerate dispatcher: there is no subcommand routing layer, no positional-argument parsing past `flag.Parse()`. Adding `query` cleanly requires inserting subcommand dispatch BEFORE `flag.Parse()` (or refactoring to per-subcommand FlagSets).

### 1.2 Test pattern (`cmd/usearch/main_test.go`)

`main_test.go:1-49` uses `os/exec.Command("go", "run", ".", "<args>")` to drive the binary as a black box, captures stdout, and regex-asserts the output. Tests:

- `TestVersionFlag` — `--version` (`main_test.go:14-31`).
- `TestVersionShortFlag` — `-v` (`main_test.go:33-48`).

The chosen test style is end-to-end at the process boundary, NOT in-process function call. SPEC-CLI-001 will mostly continue this pattern for stdout/stderr/exit-code assertions but ALSO factor an in-process `Execute(args, stdout, stderr)` function for table-driven unit testing of pipeline branches.

### 1.3 Integrations already available in `main.go`

- `obs.Init` returns `(*obs.Obs, shutdown, error)` — `cmd/usearch/main.go:36-47`. Available for `query`.
- `llm.New(cfg, o)` constructs a `*llm.Client` — `main.go:59-64`. Available for SYN-001's HTTP client wrapper.
- `LITELLM_MASTER_KEY`, `OTLP_ENDPOINT`, `LOG_LEVEL`, `USEARCH_ADMIN_PORT` env keys — `main.go:39-42, 78-83, 53`.

The `main` function has no adapter registry construction yet. SPEC-CLI-001 must wire the registry alongside the router.

---

## 2. Go CLI Library Survey

The decision is bounded by two constraints from `.moai/project/roadmap.md` §M2 exit criteria and the M1 footprint goal:

- M2 exit (`roadmap.md:149`): `usearch query "hello world"` returns Reddit + HN + synthesized paragraph + citations. Single subcommand for v0.
- Binary size: M1 baseline target ≤ 30MB (carries forward as NFR-CLI-003).

### 2.1 Option A — stdlib `flag` (RECOMMENDED for M2)

- **Pros**: zero new dependencies; binary size unchanged; tests already use stdlib `flag`; the existing pattern at `cmd/usearch/main.go:24-27` directly extends; trivial to test (the `*flag.FlagSet`-per-subcommand pattern is a well-known stdlib idiom — see `https://pkg.go.dev/flag#FlagSet`).
- **Cons**: subcommand routing has to be hand-rolled (`switch os.Args[1]`); no built-in help generator beyond per-FlagSet `Usage`; no auto-completion.
- **Subcommand pattern**:

  ```go
  func main() {
      if len(os.Args) < 2 { /* print usage to stderr */ }
      switch os.Args[1] {
      case "query": runQuery(os.Args[2:])
      case "--version", "-v": printVersion()
      default: printUsage(); os.Exit(2)
      }
  }
  ```

  Each subcommand owns its own `flag.NewFlagSet("query", flag.ContinueOnError)` so flags do not leak between subcommands.
- **Token cost**: zero. Test cost: zero.

### 2.2 Option B — `github.com/spf13/cobra` (DEFERRED to SPEC-CLI-002)

- **Pros**: industry standard for multi-subcommand Go CLIs (`gh`, `kubectl`, `helm`); rich help, completion, suggestions; nested command trees scale gracefully.
- **Cons**: pulls in `cobra` + `pflag` + transitively `spf13/viper` if config integration is desired. `cobra` adds ~500KB to binary size and ~30KB of code generation overhead. `cobra v1.8.1` go.sum delta is ~12 transitive dependencies (per `https://pkg.go.dev/github.com/spf13/cobra` + `go mod why`).
- **When justified**: M7 (`SPEC-CLI-002` — `usearch deep`, `usearch team list/members`, `usearch team members`, TUI) where the subcommand tree branches to ≥ 5 commands and structured help becomes visible value.
- **Migration cost**: moving from `flag` to `cobra` later is mechanical (rewrite `runQuery` as `*cobra.Command`'s `RunE`). The test surface (process-boundary `exec.Command`) is unchanged, so SPEC-CLI-001's tests carry forward verbatim.

### 2.3 Option C — `github.com/urfave/cli/v2`

- **Pros**: lighter than cobra (~200KB binary delta); single dependency; `App.Action` API is concise.
- **Cons**: less Go-idiomatic; smaller ecosystem; struct-tag-based flags differ from stdlib conventions used elsewhere in the project.
- **Verdict**: rejected for v0. If we ever migrate, cobra is the better landing zone given community size.

### 2.4 Option D — `github.com/alecthomas/kong`

- **Pros**: declarative struct-tag-based; tiny binary delta (~80KB); type-safe; embraces composition.
- **Cons**: niche; the project has no other `kong` usage; learning curve for new contributors.
- **Verdict**: rejected for v0.

### 2.5 Decision

**Option A — stdlib `flag`** for SPEC-CLI-001. Rationale:

1. M2 only adds ONE subcommand (`query`). The argument count (3 flags + 1 positional) is well below cobra's break-even point.
2. NFR-CLI-003 caps binary size at 30MB. Adding cobra here is premature complexity in violation of `moai-constitution.md` "Enforce Simplicity".
3. Migration to cobra in SPEC-CLI-002 is mechanical and low-risk; the cost of starting with cobra now is paid for naught if the design changes.
4. `cmd/usearch-api/` and `cmd/usearch-mcp/` (which already exist as empty cmd dirs per `cmd/` listing) may impose their own CLI conventions; locking in cobra at M2 pre-commits all three binaries.

**Constraint** for the run phase: SPEC-CLI-001's implementation MUST NOT prevent a future cobra migration. Specifically, the in-process `Execute(args []string, stdout, stderr io.Writer) int` function signature is the boundary that cobra would re-wrap, so it must remain stable across the SPEC-CLI-001 → SPEC-CLI-002 transition.

---

## 3. Intent Router (SPEC-IR-001) Integration Surface

### 3.1 Public types

The router exposes (`internal/router/`):

- `router.Options` (`router.go:48-71`) — config struct: `Rules`, `LLMClient llm.Client`, `Registry *adapters.Registry`, `Obs *obs.Obs`, `LLMModelOverride string`, `LLMDeadline time.Duration`, `ConfidenceThreshold float64`.
- `router.New(opts Options) (*Router, error)` (`router.go:93-134`) — returns `ErrAdapterRegistryEmpty` if registry is nil or empty.
- `router.Router.Classify(ctx, RouterQuery) (RoutingDecision, error)` (`router.go:151-208`).
- `router.RouterQuery` (`query_input.go:16-18`) — embeds `pkg/types.Query`; `Validate()` enforces non-empty Text.
- `router.RoutingDecision` (`routing_decision.go:23-37`) — `Category`, `Confidence`, `AdapterSet []string`, `Lang`, `Source`, `Metadata`.

### 3.2 CLI wiring requirements

To call `Classify`, the CLI must:

1. Construct an `*adapters.Registry` populated with at least one adapter (Reddit per SPEC-ADP-001; HN per SPEC-ADP-002 — currently `draft`, see §6).
2. Build the `obs.Obs` bundle (already done in `main.go:36-47`).
3. Construct an `llm.Client` if available (or pass nil; `Classify` handles nil-LLM via degraded path per REQ-IR-002 / `router.go:184-194`).
4. Call `router.New(...)` and bail with exit code 2 on error.
5. Construct `RouterQuery` from `pkg/types.Query{Text: <prompt>, Deadline: time.Now().Add(timeout)}`.
6. Call `Classify(ctx, q)` with a `context.WithTimeout` derived from `--timeout`.

### 3.3 AdapterSet → fanout mapping

The router returns `AdapterSet []string` (sorted lexicographically per `router.go:292`). For SPEC-CLI-001 (without SPEC-FAN-001 in M3), the CLI must implement a **basic fanout** itself. The minimum viable shape:

```go
type adapterResult struct {
    adapter string
    docs    []types.NormalizedDoc
    err     error
}
```

A `sync.WaitGroup` + per-adapter goroutine + per-result channel is the simplest pattern that honours the timeout and surfaces partial results. Per Go convention (`.claude/rules/moai/languages/go.md` "Concurrency Patterns"), `errgroup.WithContext` is the canonical primitive.

This basic fanout in CLI-001 is **deliberately minimal**:

- No retry (FAN-001 owns retry orchestration).
- No deduplication beyond what the adapter pipeline already produces.
- No partial-result ranking — results are concatenated in adapter-name order.
- No circuit-breaker per adapter — adapter-level errors are logged and ignored unless ALL adapters fail (exit code 3 partial / 2 system).

When SPEC-FAN-001 lands in M3, the CLI's basic fanout block is replaced wholesale by `fanout.Dispatch(ctx, decision, registry)` returning `[]types.NormalizedDoc`. The interface boundary is preserved by extracting `runFanout(ctx, decision, registry) ([]NormalizedDoc, []error)` into a private function from day one.

---

## 4. Reddit Adapter (SPEC-ADP-001) Usage Pattern

### 4.1 Construction

`internal/adapters/reddit/reddit.go:63-91`:

```go
red, err := reddit.New(reddit.Options{
    UserAgentVersion: Version,  // "0.1.0-dev"
})
if err != nil { /* fatal */ }
err = registry.Register(red)
```

Reddit's `Capabilities()` (`reddit.go:99-118`):
- `SourceID: "reddit"`, `DocTypes: [DocTypePost]`, `SupportedLangs: nil` (language-agnostic), `RequiresAuth: false`, `RateLimitPerMin: 10`, `DefaultMaxResults: 25`.

The router will classify a generic English query into `social` category (per IR-001 §2.3 keyword tables) and the AdapterSet will include `reddit` (DocTypePost ∈ social-eligible per `router.CategoryEligibleDocTypes`).

### 4.2 Search invocation

`pkg/types.Adapter.Search(ctx, q types.Query) ([]types.NormalizedDoc, error)`. Returns `*types.SourceError` on rate-limit (429) / network failure / parse error per ADP-001 §1.6. The CLI treats any `*SourceError` as a non-fatal partial-result error.

### 4.3 HN adapter (SPEC-ADP-002)

The HN adapter SPEC is currently `draft` (per `.moai/specs/SPEC-ADP-002/` empty directory). SPEC-CLI-001's `depends_on` list assumes both ADP-001 and ADP-002 will be implemented before CLI-001's run phase begins. This is a forward-looking dependency — the SPEC can be authored and audited in parallel with ADP-002.

The CLI does NOT hard-code Reddit + HN by name. It constructs the registry, registers all adapters available (Reddit always, HN when its SPEC implements), and lets the router compute `AdapterSet`. If only Reddit is registered, the CLI returns Reddit-only results (M2 exit criterion is "Reddit + HN" but partial Reddit-only is acceptable for early integration testing).

---

## 5. Synthesis Client (SPEC-SYN-001 — draft)

### 5.1 Status

SPEC-SYN-001 is `draft`. The skeleton package `internal/synthesis/` does NOT yet exist (verified by absence in `internal/` listing). The synthesis service is planned as a Python sidecar (`services/researcher/`) per `.moai/project/roadmap.md` M2 row, exposed over HTTP and consumed via a Go HTTP client.

### 5.2 Expected interface

Based on the M2 exit criterion and the gpt-researcher wrapper pattern, SYN-001's Go-side interface will be:

```go
type Client interface {
    Synthesize(ctx context.Context, req Request) (*Response, error)
}
type Request struct {
    Query    string
    Decision router.RoutingDecision  // for adapter selection hints
    Docs     []types.NormalizedDoc   // fanout results
}
type Response struct {
    Summary    string             // synthesized paragraph
    Citations  []Citation         // [1] doc_id → URL/Title mapping
    CostUSD    float64
}
```

**Key implication for SPEC-CLI-001**: the CLI's pipeline ends at `synthesis.Client.Synthesize(...)`. When SYN-001 is not yet runnable (e.g., the Python sidecar is offline), the CLI must still produce useful output — see §7 streaming/output formatting.

### 5.3 Fallback behaviour when SYN-001 is unavailable

Two reasonable policies:

1. **Hard-fail with exit code 2** — synthesis is a contract requirement of `query`.
2. **Soft-fail with degraded output** — print the raw `NormalizedDoc.Snippet` list with manual numbering and a warning that synthesis was unavailable.

Recommendation: **soft-fail** (option 2). Aligns with `--source` filtering being a "best-effort" tool; users get partial value when the sidecar is down. Exit code 3 (partial) signals the degradation. This decision is captured in REQ-CLI-009.

---

## 6. Streaming and Output Formatting

### 6.1 stdout / stderr separation

Per Unix convention and `.moai/config/sections/observability.yaml`:

- **stdout** is for the answer (text or JSON).
- **stderr** is for progress, status, and structured logs (via `obs.Logger` slog handler).

This separation enables `usearch query "..." | jq` (with `--format json`) and `usearch query "..." > answer.txt 2> debug.log`.

### 6.2 Text format (default)

```
<synthesized paragraph>

Citations:
[1] <Title> — <URL>
[2] <Title> — <URL>
```

Wrapping at 80 columns is OUT OF SCOPE for v0 (terminals handle reflow).

### 6.3 JSON format (`--format json`)

```json
{
  "query": "<prompt>",
  "category": "social",
  "lang": "en",
  "adapters": ["hackernews", "reddit"],
  "summary": "...",
  "citations": [
    {"index": 1, "title": "...", "url": "...", "source": "reddit"}
  ],
  "stats": {
    "elapsed_ms": 4200,
    "doc_count": 28,
    "cost_usd": 0.0123,
    "request_id": "01HX..."
  }
}
```

JSON is line-buffered to stdout. No streaming JSON within a single response (incremental SSE-style streaming is OUT OF SCOPE; deferred to SPEC-SYN-004 in M4).

### 6.4 Streaming progress (stderr)

When `--format text`, stderr emits human-readable progress lines like:

```
[router] classified as 'social' (confidence 0.92, adapters=hackernews,reddit)
[fanout] reddit: 25 docs in 380ms
[fanout] hackernews: 12 docs in 510ms
[synthesis] generating summary...
```

When `--format json`, stderr emits **structured JSON slog records** (default `obs.Logger` already produces JSON per SPEC-OBS-001 REQ-OBS-001 / `internal/obs/log/`). The user gets a stream of one-line JSON events for piping into log aggregators.

The "streaming" in SPEC-CLI-001 is **progress events**, not **incremental answer tokens**. True LLM-streaming output (token-by-token synthesis) is deferred to SPEC-SYN-004 (M4).

---

## 7. Exit Code Conventions

Standard Unix-ish convention adopted:

| Code | Meaning | Trigger |
|------|---------|---------|
| 0 | Success | Synthesis succeeded; ≥ 1 adapter returned ≥ 1 doc; non-empty summary |
| 1 | User error | Empty query, invalid `--source` value, invalid `--format`, bad timeout, missing required arg |
| 2 | System error | `obs.Init` failed, `router.New` failed (e.g., `ErrAdapterRegistryEmpty`), context cancelled before any adapter returned, all adapters failed |
| 3 | Partial result | At least one adapter returned docs but synthesis failed OR some adapters errored but at least one succeeded |

Exit code 3 is the "soft success with caveats" signal; CI scripts can choose to treat it as success or failure depending on policy.

---

## 8. Observability Wiring

### 8.1 Top-level span

The `query` command creates a top-level OTel span `usearch.cli.query` covering the entire pipeline:

```
usearch.cli.query (root, 5.2s)
├── router.classify (12ms)
├── fanout.basic
│   ├── adapter.search (reddit, 380ms)
│   └── adapter.search (hackernews, 510ms)
└── synthesis.synthesize (4.2s)
```

The `router.classify` and `adapter.search` spans are emitted by their respective packages already (`router.go:152-156`, `registry.go:198-201`). The CLI just opens the root span.

### 8.2 Request ID

A new request ID is generated at the start of each `query` invocation via `reqid.WithContext(ctx, reqid.New())` (`internal/obs/reqid/reqid.go:25-29`). All downstream slog records carry the `request_id` attribute.

### 8.3 Cost emission

The `llm.Client.Complete` call inside `router.classify` emits cost metrics already (per SPEC-LLM-001). The CLI is responsible for one additional cost emission: the SYN-001 synthesis call's USD cost (when SYN-001 implements). This is exposed via `synthesis.Response.CostUSD` and accumulated into the JSON output's `stats.cost_usd`.

---

## 9. Risks Table

| Risk | Severity | Mitigation |
|------|----------|------------|
| SPEC-SYN-001 not ready by run phase | High | Soft-fail with exit code 3 (REQ-CLI-009); CLI ships with synthesis-disabled mode behind a feature flag if needed |
| SPEC-ADP-002 (HN) not ready | Medium | CLI runs with Reddit-only; M2 exit criterion partially met; `--source reddit` still works end-to-end |
| Basic fanout introduces goroutine leak | High | NFR-CLI-002 enforces context-cancellation; `errgroup.WithContext` pattern; tests assert via `goleak` (already in go.mod per ADP-001 NFR-ADP-003) |
| Binary size creep above 30MB | Medium | Stdlib `flag` only; defer cobra to CLI-002 |
| Stdout pollution by accidental `fmt.Println` from libraries | Medium | Channel all status through `obs.Logger` (slog → stderr); reserve stdout strictly for answer payload |
| Empty `AdapterSet` from router | Medium | Router REQ-IR-008 fallback to lang-agnostic web set; CLI prints empty-result message + exit code 2 if STILL empty after fallback |
| `--source` filter intersects to empty set | Low | CLI prints user-error + exit code 1; suggests `--source ''` to use all adapters |
| LLM timeout cascades into total CLI timeout | Medium | Router enforces its own 2s LLM deadline (REQ-IR-007); CLI's `--timeout` is the parent ctx; deadline arithmetic uses min(router.deadline, cli.timeout - elapsed) |
| Test reliance on real Reddit endpoint | High | All tests use `httptest.Server` per ADP-001 D4; CLI tests use mock router + mock adapter + mock synthesis client |

---

## 10. Open Questions (carried into spec.md §8)

1. Should `--source` accept adapter names or capability tags (e.g., `--source social` to mean "all social adapters")? Recommendation: adapter names only for v0; tag-based filtering deferred to CLI-002.
2. Should we support `usearch query` (no positional, prompt from stdin) for piping? Recommendation: NO for v0; positional argument only. Stdin-mode deferred to CLI-002.
3. Default `--timeout` value: 30s feels correct for M2 single-adapter runs; revisit when SPEC-FAN-001 lands and fanout p95 latency is measurable.
4. JSON schema versioning: do we need `schema_version` in the JSON output? Recommendation: YES, set to `"1"` from day one; cheap insurance against future breakage.
5. Locale of error messages: `language.yaml` says `error_messages: en`. v0 honours this; future SPEC-LANG-001 may add Korean error messages.
6. Should `--no-llm` (existing flag) be propagated to the router? Currently `--no-llm` only affects main.go's startup-time LLM client construction. With `--no-llm`, the router still runs but the LLM-fallback path returns `ErrLLMUnavailable` immediately, which is a graceful degrade per REQ-IR-003. Recommendation: leave the existing flag semantics intact.

---

## 11. References

External:

- Go stdlib `flag` package — `https://pkg.go.dev/flag`
- `errgroup` for fanout patterns — `https://pkg.go.dev/golang.org/x/sync/errgroup`
- Cobra (deferred): `https://pkg.go.dev/github.com/spf13/cobra`
- Unix exit code conventions — `sysexits.h` (BSD style)

Internal:

- `cmd/usearch/main.go:1-83` — current entrypoint
- `cmd/usearch/main_test.go:1-49` — test pattern
- `internal/router/router.go:48-208` — Router API surface
- `internal/router/query_input.go:16-27` — RouterQuery type
- `internal/router/routing_decision.go:23-48` — RoutingDecision type
- `internal/adapters/registry.go:75-167` — Registry API
- `internal/adapters/reddit/reddit.go:1-136` — adapter usage example
- `internal/obs/obs.go:1-148` — Obs bundle, Init lifecycle
- `internal/obs/reqid/reqid.go:1-30` — request ID generator
- `internal/llm/client.go` — LLM client (SPEC-LLM-001)
- `pkg/types/adapter.go:1-46` — Adapter contract
- `pkg/types/normalized_doc.go:1-107` — NormalizedDoc shape
- `pkg/types/query.go:1-44` — Query shape
- `.moai/specs/SPEC-IR-001/spec.md` — Intent Router contract
- `.moai/specs/SPEC-ADP-001/spec.md` — Reddit adapter contract
- `.moai/project/roadmap.md:35-149` — M2 SPEC backlog and exit criterion

---

End of research.md for SPEC-CLI-001.
