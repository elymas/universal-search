# @MX Tag Validation Report — SPEC-OBS-001

Generated: 2026-04-26
Commit: 0234b71

## Summary

| Tag Type | Count | Compliance |
|----------|-------|------------|
| @MX:ANCHOR | 8 | All carry @MX:REASON ✓ |
| @MX:WARN | 1 | Carries @MX:REASON ✓ |
| @MX:NOTE | 0 | — |
| @MX:TODO | 0 | All RED tests promoted to GREEN |

Total: 18 tag lines across 5 files (8 ANCHOR × 2 lines + 1 WARN × 2 lines = 18 lines)

Note on counting: `metrics.go:185–188` is a compound block carrying both @MX:ANCHOR and @MX:WARN on the same function; the protocol counts each tag type instance separately. This report counts 8 ANCHOR instances and 1 WARN instance.

## Tag Inventory

### @MX:ANCHOR — 7 tags

**1. internal/obs/obs.go:49**

```
// @MX:ANCHOR: [AUTO] Central obs bundle; callers: cmd mains, HTTP handlers, tests
// @MX:REASON: fan_in >= 3; single struct passed to all instrumentation call sites
```

Anchored to: `type Obs struct`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional for ANCHOR per protocol — defect only if @MX:SPEC is mandated; protocol says OPTIONAL)
- Comments in English: yes

**2. internal/obs/obs.go:72**

```
// @MX:ANCHOR: [AUTO] Obs lifecycle entry point; callers: cmd/usearch, cmd/usearch-api, cmd/usearch-mcp, tests
// @MX:REASON: fan_in >= 3; wires slog+prometheus+otel in a single call
```

Anchored to: `func Init(ctx context.Context, cfg Config) (*Obs, func(context.Context) error, error)`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**3. internal/obs/metrics/metrics.go:26**

```
// @MX:ANCHOR: [AUTO] Central metrics registry; callers: obs.Init, HTTPMiddleware, StartAdminServer, tests
// @MX:REASON: fan_in >= 3; registry is the single point of truth for all metric families
```

Anchored to: `type Registry struct`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**4. internal/obs/metrics/metrics.go:185**

```
// @MX:ANCHOR: [AUTO] Admin server lifecycle; callers: obs.Init, cmd/usearch, tests
// @MX:REASON: fan_in >= 3; localhost binding is a security requirement (NFR)
```

Anchored to: `func StartAdminServer(ctx context.Context, addr string, reg *Registry) (string, func(context.Context) error, error)`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**5. internal/obs/trace/trace.go:39**

```
// @MX:ANCHOR: [AUTO] OTel global state mutation; callers: obs.Init, cmd mains, tests
// @MX:REASON: fan_in >= 3; sets package-level OTel globals (provider + propagator)
```

Anchored to: `func Init(ctx context.Context, cfg Config) (func(context.Context) error, error)`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**6. internal/obs/log/log.go:20**

```
// @MX:ANCHOR: [AUTO] Level resolution; callers: obs.Init, New, tests
// @MX:REASON: fan_in >= 3; changing default affects all log emission decisions
```

Anchored to: `func LevelFromEnv(raw string) slog.Level`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**7. internal/obs/reqid/reqid.go:25**

```
// @MX:ANCHOR: [AUTO] Central ID generator; callers: reqid.Middleware, reqid_test, obs.Init
// @MX:REASON: fan_in >= 3; format change here affects all request tracing
```

Anchored to: `func New() string`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

**8. internal/obs/reqid/reqid.go:46**

```
// @MX:ANCHOR: [AUTO] HTTP ingress boundary; callers: cmd/usearch-api, obs.Init, tests
// @MX:REASON: fan_in >= 3; must maintain X-Request-ID contract for all ingress traffic
```

Anchored to: `func Middleware(next http.Handler) http.Handler`

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

Note: reqid.go carries 2 ANCHOR tags; the per-file limit is 3 (from mx.yaml defaults). No demotion needed.

### @MX:WARN — 1 tag

**1. internal/obs/metrics/metrics.go:187**

```
// @MX:WARN: [AUTO] Goroutine launched; context-cancellable via serverCtx
// @MX:REASON: goroutine is bounded by ctx lifetime; errgroup pattern used implicitly via server.Shutdown
```

Located on: `func StartAdminServer` (same function as ANCHOR at line 185)

- [AUTO] prefix: present
- @MX:REASON: present
- @MX:SPEC: absent (optional)
- Comments in English: yes

## Defects: None

All 18 tag lines across 5 source files pass compliance checks:

- Every @MX:ANCHOR carries @MX:REASON
- The single @MX:WARN carries @MX:REASON
- All agent-generated tags carry [AUTO] prefix
- All tag descriptions and sub-lines are in English (consistent with `code_comments: en` in language.yaml)
- @MX:TODO count is zero — all RED-phase requirements promoted to GREEN
- @MX:SPEC is absent on all tags; per mx-tag-protocol.md, @MX:SPEC is OPTIONAL (not mandatory). Absence is not a defect.

## Notes

- `metrics.go` carries the per-file ANCHOR limit of 3 (lines 26 + 185 = 2 ANCHOR tags). No demotion needed.
- `reqid.go` carries 2 ANCHOR tags (lines 25 + 46). Under the per-file limit of 3.
- `obs.go` carries 2 ANCHOR tags (lines 49 + 72). Under the per-file limit of 3.
- The compound ANCHOR + WARN block at `metrics.go:185–188` is valid; the protocol permits both tag types on the same function.
