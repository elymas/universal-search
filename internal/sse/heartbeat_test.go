// Package sse_test — RED phase tests for the heartbeat goroutine.
// test(stream): RED — heartbeat tests (SPEC-SYN-004 REQ-SYN4-003)
package sse_test

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/sse"
)

// TestHeartbeatEmitsAtConfiguredInterval verifies REQ-SYN4-003: `: ping\n\n`
// is written every interval until ctx is cancelled.
func TestHeartbeatEmitsAtConfiguredInterval(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rr := &fakeResponseWriter{buf: &buf, header: make(http.Header)}
	w := sse.NewWriter(rr)

	ctx, cancel := context.WithCancel(context.Background())
	interval := 30 * time.Millisecond

	done := make(chan error, 1)
	go func() {
		done <- sse.RunHeartbeat(ctx, w, interval)
	}()

	// Allow ~3 heartbeats then cancel.
	time.Sleep(110 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("heartbeat goroutine did not return within 200ms after cancel")
	}

	count := strings.Count(buf.String(), ": ping\n\n")
	if count < 2 {
		t.Errorf("heartbeat count = %d, want >= 2", count)
	}
}

// TestHeartbeatTerminatesOnCtxDone verifies REQ-SYN4-003: heartbeat returns
// within 100ms of ctx cancellation.
func TestHeartbeatTerminatesOnCtxDone(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rr := &fakeResponseWriter{buf: &buf, header: make(http.Header)}
	w := sse.NewWriter(rr)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- sse.RunHeartbeat(ctx, w, 10*time.Second)
	}()

	cancel()

	select {
	case <-done:
		// good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("heartbeat goroutine did not terminate within 100ms of cancel")
	}
}

// TestHeartbeatDisabledEmitsNothing verifies REQ-SYN4-003: when ctx is immediately
// cancelled (simulating disabled mode), no ping is emitted.
func TestHeartbeatDisabledEmitsNothing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	rr := &fakeResponseWriter{buf: &buf, header: make(http.Header)}
	w := sse.NewWriter(rr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately — simulates disabled

	_ = sse.RunHeartbeat(ctx, w, 10*time.Second)

	if strings.Contains(buf.String(), "ping") {
		t.Errorf("heartbeat emitted ping when disabled: %q", buf.String())
	}
}
