// Package sse — heartbeat goroutine helper.
//
// REQ-SYN4-003: Emits `: ping\n\n` SSE comments every interval until ctx is done.
//
// @MX:WARN: [AUTO] Goroutine lifecycle — heartbeat ticker runs until ctx cancelled
// @MX:REASON: cancel on client disconnect; caller must pass a context derived from request context
package sse

import (
	"context"
	"time"
)

// RunHeartbeat emits `: ping\n\n` SSE comments every interval via w until ctx
// is done. It returns ctx.Err() on cancellation, or a write error if emission
// fails.
//
// Callers MUST ensure that ctx is cancelled (e.g. via request context or explicit
// cancel) to prevent goroutine leaks. The heartbeat goroutine exits within one
// ticker cycle after ctx cancellation.
func RunHeartbeat(ctx context.Context, w *Writer, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.WriteComment("ping"); err != nil {
				return err
			}
			if err := w.Flush(); err != nil {
				return err
			}
		}
	}
}
