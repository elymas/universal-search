# @MX Tag Validation Report — SPEC-LLM-001

Generated: 2026-04-26
Commit: 5005eb0

## Summary

| Tag Type | Count | Compliance |
|----------|-------|------------|
| @MX:ANCHOR | 5 | All carry @MX:REASON ✓ |
| @MX:WARN | 4 | All carry @MX:REASON ✓ |
| @MX:NOTE | 0 | — |
| @MX:TODO | 0 | All RED tests promoted to GREEN |

Total: 18 tag lines across 7 files (5 ANCHOR × 2 lines + 4 WARN × 2 lines = 18 lines; cost.go @MX:REASON spans 3 lines but counts as 1 WARN instance)

## Tag Inventory

### @MX:ANCHOR — 5 tags

**1. internal/llm/llm.go:98**

```
// @MX:ANCHOR: [AUTO] Primary LLM interface; callers: cmd/usearch, SPEC-SYN-001, SPEC-DEEP-*, tests
// @MX:REASON: fan_in >= 3; all LLM I/O in Go plane flows through this interface
```

Anchored to: `type Client interface`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**2. internal/llm/llm.go:126**

```
// @MX:ANCHOR: [AUTO] LLM client constructor; callers: cmd/usearch, cmd/usearch-api, cmd/usearch-mcp, tests
// @MX:REASON: fan_in >= 3; sole entry point for creating a production Client
```

Anchored to: `func New(cfg config.Config, o *obs.Obs) (Client, error)`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**3. internal/llm/client.go:33**

```
// @MX:ANCHOR: [AUTO] Default LLM client implementation; callers: llm.New, cmd/usearch, tests
// @MX:REASON: fan_in >= 3; all production LLM calls flow through this struct
```

Anchored to: `type defaultClient struct`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**4. internal/llm/config/config.go:15**

```
// @MX:ANCHOR: [AUTO] LLM client config; callers: llm.New, cmd/usearch, config_test
// @MX:REASON: fan_in >= 3; single struct holds all LLM gateway parameters
```

Anchored to: `type Config struct`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**5. internal/llm/router.go:149**

```
// @MX:ANCHOR: [AUTO] Provider selection + circuit breaker; callers: client.go, router_test.go, tests
// @MX:REASON: fan_in >= 3; all retry/fallthrough logic flows through Route
```

Anchored to: `type Router struct`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

### @MX:WARN — 3 tags

**1. internal/llm/stream.go:26**

```
// @MX:WARN: [AUTO] Goroutine launched per Stream call; cancellable via ctx
// @MX:REASON: goroutine lifetime is bounded by ctx + backpressure timeout
```

Located on: `func runStream(...)`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**2. internal/llm/cost.go:26**

```
// @MX:WARN: [AUTO] Context mutation via pointer; request context is replaced
// @MX:REASON: openai-go middleware does not allow replacing the request pointer directly;
//
//	we use a context pointer trick — see implementation note below.
```

Located on: `func newCostMiddlewareRoundTripper(next http.RoundTripper, costPtr *float64, logger *slog.Logger) http.RoundTripper`

- [AUTO] prefix: present
- @MX:REASON: present (multi-line)
- @MX:SPEC: absent (optional)
- Comments in English: yes

**3. internal/llm/retry.go:64**

```
// @MX:WARN: [AUTO] Retry loop with sleeps; must respect ctx cancellation
// @MX:REASON: failure to check ctx.Done allows goroutine to outlive request lifetime
```

Located on: `func withRetry(ctx context.Context, fn func() error) error`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**4. internal/llm/router.go:34**

```
// @MX:WARN: [AUTO] Concurrent state machine; protected by mu
// @MX:REASON: Multiple goroutines may call Record/Allow concurrently
```

Located on: `type breaker struct`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

Note: router.go carries 1 ANCHOR (line 149) + 1 WARN (line 34) = 2 tags total. Under per-file limits.

## Defects: None

All 18 tag lines across 7 source files pass compliance checks:

- Every @MX:ANCHOR carries @MX:REASON
- Every @MX:WARN carries @MX:REASON
- All agent-generated tags carry [AUTO] prefix
- All tag descriptions and sub-lines are in English (consistent with `code_comments: en` in language.yaml)
- @MX:TODO count is zero — all RED-phase requirements promoted to GREEN
- @MX:SPEC is absent on all tags; per mx-tag-protocol.md, @MX:SPEC is OPTIONAL. Absence is not a defect.

## Notes

- `llm.go` carries 2 ANCHOR tags (lines 98 + 126). Under the per-file limit of 3.
- `router.go` carries 1 ANCHOR (line 149) + 1 WARN (line 34). Under all per-file limits.
- `cost.go:26` @MX:WARN @MX:REASON spans multiple lines (Go comment continuation); this is compliant — the protocol requires @MX:REASON to be present, not to be single-line.
- Summary counts: the grep output shows 4 WARN instances total (stream.go, cost.go, retry.go, router.go). The summary table above is corrected to show 4 WARN tags (not 3). Total tag instances = 5 ANCHOR + 4 WARN = 9 distinct tag instances; 18 lines because each tag instance has a main line + @MX:REASON line.
