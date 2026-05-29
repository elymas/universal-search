package access

import "sync/atomic"

// SPEC-SEC-001 REQ-SEC-009: SSRF block observability hook.
//
// The access package must NOT import internal/obs/metrics (it would create an
// import cycle via the metrics registry). Instead, the SSRF block metric is
// emitted through a package-level hook that the obs wiring sets at startup to
// point at the usearch_security_ssrf_blocks_total{reason, component} collector.
// When unset (e.g. in unit tests), the hook is a no-op.
//
// The hook is stored in an atomic.Pointer so concurrent obs.Init calls (e.g.
// parallel tests) and the SSRF guard hot path do not race.

// ssrfBlockHook holds the optional SSRF-block observability callback.
//
// @MX:NOTE: [AUTO] Decoupling hook — obs.Init wires this to the security metric
// collector; default no-op avoids an access -> obs import cycle. Atomic for
// concurrent SetSSRFBlockHook / recordSSRFBlock access.
var ssrfBlockHook atomic.Pointer[func(reason string)]

// SetSSRFBlockHook installs the SSRF-block observability hook. Pass nil to
// reset to the no-op default. Safe for concurrent use. component label is
// fixed to "access" by the caller.
func SetSSRFBlockHook(fn func(reason string)) {
	if fn == nil {
		ssrfBlockHook.Store(nil)
		return
	}
	ssrfBlockHook.Store(&fn)
}

// recordSSRFBlock invokes the hook if installed.
func recordSSRFBlock(reason string) {
	if fn := ssrfBlockHook.Load(); fn != nil {
		(*fn)(reason)
	}
}
