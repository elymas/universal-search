// Package types — Adapter contract.
// REQ-CORE-002.
package types

import "context"

// Adapter is the contract every search source implements.
//
// All twelve-plus M3 adapters (Reddit, Hacker News, arXiv, GitHub, YouTube,
// Bluesky, X, SearXNG, Naver, Daum, KoreaNewsCrawler, RSS, Polymarket)
// implement this exact 4-method interface. The registry wraps each
// implementation in a wrappedAdapter that emits per-call observability so
// adapter authors do not write any metric/log/span boilerplate.
//
// Implementations:
//   - MUST honour ctx cancellation in Search and Healthcheck.
//   - MUST wrap raw errors in *SourceError with the appropriate Category so
//     the wrappedAdapter can classify outcomes uniformly.
//   - MUST return Capabilities deterministically (same value every call).
//   - MUST keep Name() stable across the process lifetime — it is the
//     Prometheus label value and the registry key.
//
// @MX:ANCHOR: [AUTO] Adapter contract; callers: every M3 adapter, registry,
// FAN-001 fanout, IR-001 router, tests
// @MX:REASON: fan_in >= 12; sole boundary between source-specific code and
// orchestration. Method additions are breaking for every adapter.
// @MX:SPEC: SPEC-CORE-001
type Adapter interface {
	// Name returns the stable adapter identifier (e.g., "reddit", "hackernews").
	// MUST equal Capabilities().SourceID. Used as the Prometheus label value.
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
