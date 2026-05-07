// Package fanout — Options struct with documented zero-value defaults.
// SPEC-FAN-001 §2.1 item c.
package fanout

import (
	"time"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/obs"
)

const (
	// defaultMaxParallel is the default bounded goroutine pool size.
	// OQ §11.2: 8 is the M3 recommended value (12+ adapters, leaves headroom for LLM+synthesis).
	defaultMaxParallel = 8
	// defaultPerAdapterTimeout is the default per-adapter context deadline.
	// OQ §11.1: 8s default; adapters should respond well within this for web search.
	defaultPerAdapterTimeout = 8 * time.Second
	// defaultDeadline is the overall default dispatch deadline.
	// Matches CLI defaultTimeout at cmd/usearch/query.go:37.
	defaultDeadline = 30 * time.Second
)

// Options configures the Fanout dispatcher. Zero-valued fields are normalised
// to documented defaults by New.
//
// @MX:NOTE: [AUTO] PerAdapterTimeout default (8s) and DefaultDeadline (30s) are
// magic constants derived from §2.5 per-adapter ctx derivation rules. The timeout
// is capped by parent ctx deadline — a caller with a 2s deadline sees adapters
// time out at 2s regardless of this setting.
// @MX:SPEC: SPEC-FAN-001
type Options struct {
	// Registry is the adapter registry. Must be non-nil and non-empty.
	Registry *adapters.Registry
	// Obs is the observability bundle. May be nil; fanout degrades gracefully.
	Obs *obs.Obs
	// MaxParallel is the maximum number of concurrent adapter goroutines.
	// Zero defaults to 8.
	MaxParallel int
	// PerAdapterTimeout is the maximum time allowed per adapter Search call.
	// Zero defaults to 8s. Capped by parent ctx deadline per §2.5.
	PerAdapterTimeout time.Duration
	// DefaultDeadline is the overall dispatch deadline when the parent ctx has none.
	// Zero defaults to 30s.
	DefaultDeadline time.Duration
}

// firstNonZeroInt returns a if a > 0, else b.
func firstNonZeroInt(a, b int) int {
	if a > 0 {
		return a
	}
	return b
}

// firstNonZeroDuration returns a if a > 0, else b.
func firstNonZeroDuration(a, b time.Duration) time.Duration {
	if a > 0 {
		return a
	}
	return b
}
