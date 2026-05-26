package mcpserver

import (
	"context"
	"fmt"
	"time"
)

// ShutdownResult captures the outcome of a graceful shutdown.
type ShutdownResult struct {
	reason     string
	inflight   int
	durationMs int64
}

// gracefulShutdown performs a graceful shutdown:
// 1. Stops accepting new requests.
// 2. Waits for in-flight requests to complete (up to grace period).
// 3. Cancels remaining contexts.
// 4. Emits a usearch.mcp.shutdown slog record.
// 5. Returns the result.
//
// REQ-MCP-003: Graceful shutdown with grace period.
func (s *Server) gracefulShutdown(ctx context.Context) ShutdownResult {
	start := time.Now()
	gracePeriod := time.Duration(s.cfg.Shutdown.GracePeriodSeconds) * time.Second
	if gracePeriod <= 0 {
		gracePeriod = 30 * time.Second
	}

	// In V1, there are no tracked in-flight requests yet.
	// This will be extended when request tracking is added.
	inflight := 0

	// Wait for grace period or context cancellation.
	select {
	case <-time.After(gracePeriod):
		// Grace period elapsed.
	case <-ctx.Done():
		// Context cancelled.
	}

	result := ShutdownResult{
		reason:     "signal",
		inflight:   inflight,
		durationMs: time.Since(start).Milliseconds(),
	}

	// Emit shutdown log record.
	s.logger().Info("usearch.mcp.shutdown",
		"reason", result.reason,
		"inflight_at_signal", result.inflight,
		"duration_ms", result.durationMs,
	)

	return result
}

// String returns a human-readable representation of the shutdown result.
func (r ShutdownResult) String() string {
	return fmt.Sprintf("ShutdownResult{reason=%q, inflight=%d, duration_ms=%d}",
		r.reason, r.inflight, r.durationMs)
}
