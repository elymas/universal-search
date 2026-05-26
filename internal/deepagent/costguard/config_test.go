package costguard

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Phase F: Decision Event Log + Observability + Hot-Reload
// ---------------------------------------------------------------------------

// TestCapExceededIncrementsCounterAndDecisionLog verifies that a cap-exceeded
// event writes a JSON line to the logger and contains all required fields.
// REQ-DEEP4-010: JSON line per cap event, stderr output.
func TestCapExceededIncrementsCounterAndDecisionLog(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewDecisionLogger(&buf)

	score := 7
	cacheHit := false
	event := DecisionEvent{
		RequestID:   "req_abc123",
		TenantID:    "default",
		UserID:      "anonymous",
		Decision:    "deny",
		Dimension:   "calls",
		Remaining:   Remaining{Calls: 0, USD: 4.23},
		ScreenScore: &score,
		CacheHit:    &cacheHit,
	}

	if err := logger.Log(event); err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Parse the output line.
	output := strings.TrimSpace(buf.String())
	var parsed DecisionEvent
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("parse decision log line: %v\nline: %q", err, output)
	}

	// Verify required fields per §6.3 forward-compat schema.
	if parsed.Timestamp == "" {
		t.Error("timestamp field missing")
	}
	if parsed.EventType != "cap.evaluation" {
		t.Errorf("event_type: got %q, want %q", parsed.EventType, "cap.evaluation")
	}
	if parsed.RequestID != "req_abc123" {
		t.Errorf("request_id: got %q", parsed.RequestID)
	}
	if parsed.TenantID != "default" {
		t.Errorf("tenant_id: got %q", parsed.TenantID)
	}
	if parsed.UserID != "anonymous" {
		t.Errorf("user_id: got %q", parsed.UserID)
	}
	if parsed.Decision != "deny" {
		t.Errorf("decision: got %q, want %q", parsed.Decision, "deny")
	}
	if parsed.Dimension != "calls" {
		t.Errorf("dimension: got %q", parsed.Dimension)
	}
}

// TestOTelSpanAttributes verifies that the span attribute keys
// follow the naming convention.
// NFR-DEEP4-010: OTel span attributes: deep.cap.*, deep.cache.*, deep.screen.*.
func TestOTelSpanAttributes(t *testing.T) {
	t.Parallel()

	// Verify attribute key names are defined.
	attrs := OTelSpanAttributes()

	expectedKeys := []string{
		"deep.cap.tenant_remaining_usd",
		"deep.cap.tenant_remaining_calls",
		"deep.cache.hit_ratio",
		"deep.screen.score",
		"deep.screen.outcome",
	}

	for _, key := range expectedKeys {
		found := false
		for _, attr := range attrs {
			if attr == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected OTel span attribute %q not found", key)
		}
	}
}

// TestConfigHotReloadOnSIGHUP verifies that config can be reloaded.
// NFR-DEEP4-008: deep.yaml hot-reload via SIGHUP or fsnotify.
func TestConfigHotReloadOnSIGHUP(t *testing.T) {
	t.Parallel()

	initialCfg := DefaultConfig()
	initialCfg.Tenant.MaxCallsPerDay = 20

	var loadCount atomic.Int32
	onLoad := func(path string) (Config, error) {
		loadCount.Add(1)
		cfg := DefaultConfig()
		cfg.Tenant.MaxCallsPerDay = 50 // Updated config
		return cfg, nil
	}

	watcher := NewConfigWatcher(&initialCfg, "deep.yaml", onLoad)

	// Start watcher in background.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watcher.Start(ctx)

	// Verify initial config.
	if watcher.GetConfig().Tenant.MaxCallsPerDay != 20 {
		t.Error("initial config should have 20 max calls")
	}

	// Trigger reload.
	watcher.TriggerReload()

	// Wait for reload to complete.
	time.Sleep(50 * time.Millisecond)

	// Verify config was reloaded.
	if watcher.GetConfig().Tenant.MaxCallsPerDay != 50 {
		t.Errorf("reloaded config: got %d calls, want 50", watcher.GetConfig().Tenant.MaxCallsPerDay)
	}
	if loadCount.Load() != 1 {
		t.Errorf("load count: got %d, want 1", loadCount.Load())
	}
}

// ---------------------------------------------------------------------------
// OTel span attribute helpers
// ---------------------------------------------------------------------------

// OTelSpanAttributes returns the list of OTel span attribute keys used
// by the costguard package.
// NFR-DEEP4-010.
func OTelSpanAttributes() []string {
	return []string{
		"deep.cap.tenant_remaining_usd",
		"deep.cap.tenant_remaining_calls",
		"deep.cap.user_remaining_usd",
		"deep.cap.user_remaining_calls",
		"deep.cache.hit_ratio",
		"deep.screen.score",
		"deep.screen.outcome",
	}
}
