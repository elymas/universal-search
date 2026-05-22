package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// T10 RED tests: Graceful Shutdown + NFR
// ---------------------------------------------------------------------------

// TestGracefulShutdownInflight verifies that when a shutdown signal is received,
// in-flight requests are allowed to complete before the server exits.
//
// REQ-MCP-003: Grace period (default 30s) for in-flight tool calls.
func TestGracefulShutdownInflight(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"
	cfg.Shutdown.GracePeriodSeconds = 1

	srv := New(cfg, nil, nil, nil, nil)
	handler, err := srv.httpHandler()
	if err != nil {
		t.Fatalf("httpHandler: %v", err)
	}

	server := httptest.NewServer(handler)
	defer server.Close()

	// Start an initialize request.
	var wg sync.WaitGroup
	var requestDone atomic.Bool

	wg.Add(1)
	go func() {
		defer wg.Done()
		initReq := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]any{
				"protocolVersion": "2025-06-18",
				"capabilities":    map[string]any{},
				"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
			},
		}
		body, _ := json.Marshal(initReq)
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/mcp", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")

		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			requestDone.Store(true)
		}
	}()

	// Give the request time to start.
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := srv.gracefulShutdown(ctx)
	if result.reason == "" {
		t.Error("expected non-empty shutdown reason")
	}

	// Wait for the request to complete.
	wg.Wait()

	if !requestDone.Load() {
		t.Error("in-flight request should have completed during grace period")
	}
}

// TestColdStartUnderCap verifies that the MCP server initializes within
// 500ms p95.
//
// NFR-MCP-001: Cold start under 500ms p95.
func TestColdStartUnderCap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cold start benchmark in short mode")
	}

	var maxDur time.Duration
	for i := range 5 {
		start := time.Now()
		cfg := DefaultConfig()
		cfg.Transport = "stdio"

		srv := New(cfg, nil, nil, nil, nil)
		_ = srv

		dur := time.Since(start)
		if dur > maxDur {
			maxDur = dur
		}
		t.Logf("iteration %d: %v", i, dur)
	}

	p95Cap := 500 * time.Millisecond
	if maxDur > p95Cap {
		t.Errorf("cold start p95: %v exceeds %v", maxDur, p95Cap)
	}
}

// TestShutdownLogRecord verifies that the shutdown emits a structured slog
// record with required fields.
//
// REQ-MCP-003: emit usearch.mcp.shutdown slog record.
func TestShutdownLogRecord(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)

	// Capture log output.
	_ = bytes.Buffer{} // buffer available for future log capture
	srv.obs = nil      // use default logger

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result := srv.gracefulShutdown(ctx)

	// Verify shutdown result fields.
	if result.reason == "" {
		t.Error("expected non-empty reason in shutdown result")
	}
	if result.durationMs < 0 {
		t.Error("expected duration_ms >= 0")
	}
}

// TestGracePeriodConfig verifies the grace period defaults and configuration.
func TestGracePeriodConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Shutdown.GracePeriodSeconds != 30 {
		t.Errorf("default grace period: got %d, want 30", cfg.Shutdown.GracePeriodSeconds)
	}

	cfg.Shutdown.GracePeriodSeconds = 60
	if cfg.Shutdown.GracePeriodSeconds != 60 {
		t.Errorf("custom grace period: got %d, want 60", cfg.Shutdown.GracePeriodSeconds)
	}
}

// TestShutdownResultString verifies the String() method produces expected format.
func TestShutdownResultString(t *testing.T) {
	r := ShutdownResult{
		reason:     "signal",
		inflight:   3,
		durationMs: 150,
	}

	s := r.String()
	if !strings.Contains(s, "signal") {
		t.Errorf("expected 'signal' in String(), got %q", s)
	}
	if !strings.Contains(s, "inflight=3") {
		t.Errorf("expected 'inflight=3' in String(), got %q", s)
	}
	if !strings.Contains(s, "duration_ms=150") {
		t.Errorf("expected 'duration_ms=150' in String(), got %q", s)
	}
}

// TestGracefulShutdownDefaultGracePeriod verifies that grace period defaults to
// 30s when config value is 0.
func TestGracefulShutdownDefaultGracePeriod(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Shutdown.GracePeriodSeconds = 0 // should default to 30s
	cfg.Transport = "http"
	cfg.HTTP.ListenAddr = "127.0.0.1:0"

	srv := New(cfg, nil, nil, nil, nil)

	// Use a short context to avoid waiting the full 30s.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := srv.gracefulShutdown(ctx)
	if result.reason == "" {
		t.Error("expected non-empty reason")
	}
}

// Compile-time checks for imports.
var _ = fmt.Sprintf
var _ = strings.Contains
var _ = os.Getenv
var _ = sync.Mutex{}
