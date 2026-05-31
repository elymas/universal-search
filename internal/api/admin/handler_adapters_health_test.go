package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/pkg/types"
)

// newHealthTestObs builds a fully-initialised obs bundle so the registry can
// read its in-process AdapterCalls counter.
func newHealthTestObs(t *testing.T) *obs.Obs {
	t.Helper()
	o, shutdown, err := obs.Init(context.Background(), obs.Config{
		ServiceName: "admin-health-test",
		LogLevel:    "ERROR",
		LogWriter:   io.Discard,
	})
	if err != nil {
		t.Fatalf("obs.Init: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	return o
}

// seedCalls writes success/fail counts directly to the AdapterCalls counter so
// a deterministic success_rate is produced for the health classifier.
func seedCalls(o *obs.Obs, adapter string, success, fail int) {
	for i := 0; i < success; i++ {
		o.Metrics.AdapterCalls.WithLabelValues(adapter, "success").Inc()
	}
	for i := 0; i < fail; i++ {
		o.Metrics.AdapterCalls.WithLabelValues(adapter, "failure").Inc()
	}
}

func registerAdapter(t *testing.T, reg *adapters.Registry, name string) {
	t.Helper()
	a := &testAdapter{name: name, caps: types.Capabilities{SourceID: name, DisplayName: name}}
	if err := reg.RegisterWithOptions(a, adapters.RegisterOptions{SkipAuthCheck: true}); err != nil {
		t.Fatalf("Register(%q): %v", name, err)
	}
}

// TestAdaptersHealthMixedStatusReturns503 verifies REQ-EVAL2-010b / AC-009:
// with a mixed fixture (healthy + degraded + unhealthy) the endpoint returns
// 503 and a correctly-mapped per-adapter status array.
func TestAdaptersHealthMixedStatusReturns503(t *testing.T) {
	o := newHealthTestObs(t)
	reg := adapters.NewRegistry(o)
	registerAdapter(t, reg, "healthy_a")  // 99/1 -> 0.99 healthy
	registerAdapter(t, reg, "degraded_a") // 90/10 -> 0.90 degraded
	registerAdapter(t, reg, "unhealthy_a")
	seedCalls(o, "healthy_a", 99, 1)
	seedCalls(o, "degraded_a", 90, 10)
	seedCalls(o, "unhealthy_a", 50, 50) // 0.50 unhealthy

	handler := LoopbackOnly(NewAdaptersHealthHandler(reg))
	req := httptest.NewRequest(http.MethodGet, "/api/admin/adapters/health", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", rec.Code)
	}

	var resp AdaptersHealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := map[string]string{}
	for _, a := range resp.Adapters {
		got[a.Name] = a.Status
		// Schema check: required fields present and rate in range.
		if a.SuccessRate24h < 0 || a.SuccessRate24h > 1 {
			t.Errorf("adapter %q success_rate_24h out of range: %v", a.Name, a.SuccessRate24h)
		}
		if a.CircuitState != "closed" {
			t.Errorf("adapter %q circuit_state = %q, want closed (deferred)", a.Name, a.CircuitState)
		}
	}
	if got["healthy_a"] != "healthy" {
		t.Errorf("healthy_a status = %q, want healthy", got["healthy_a"])
	}
	if got["degraded_a"] != "degraded" {
		t.Errorf("degraded_a status = %q, want degraded", got["degraded_a"])
	}
	if got["unhealthy_a"] != "unhealthy" {
		t.Errorf("unhealthy_a status = %q, want unhealthy", got["unhealthy_a"])
	}
}

// TestAdaptersHealthAllHealthyReturns200 verifies the 200 path.
func TestAdaptersHealthAllHealthyReturns200(t *testing.T) {
	o := newHealthTestObs(t)
	reg := adapters.NewRegistry(o)
	registerAdapter(t, reg, "a1")
	registerAdapter(t, reg, "a2")
	seedCalls(o, "a1", 100, 0)
	seedCalls(o, "a2", 96, 4) // 0.96 healthy

	handler := LoopbackOnly(NewAdaptersHealthHandler(reg))
	req := httptest.NewRequest(http.MethodGet, "/api/admin/adapters/health", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
}

// TestAdaptersHealthLoopbackOnly verifies the endpoint is loopback-gated.
func TestAdaptersHealthLoopbackOnly(t *testing.T) {
	o := newHealthTestObs(t)
	reg := adapters.NewRegistry(o)
	registerAdapter(t, reg, "a1")

	handler := LoopbackOnly(NewAdaptersHealthHandler(reg))
	req := httptest.NewRequest(http.MethodGet, "/api/admin/adapters/health", nil)
	req.RemoteAddr = "203.0.113.7:443"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("non-loopback status: got %d, want 403", rec.Code)
	}
}

// TestAdaptersHealthMethodNotAllowed verifies non-GET is rejected.
func TestAdaptersHealthMethodNotAllowed(t *testing.T) {
	o := newHealthTestObs(t)
	reg := adapters.NewRegistry(o)
	registerAdapter(t, reg, "a1")

	handler := LoopbackOnly(NewAdaptersHealthHandler(reg))
	req := httptest.NewRequest(http.MethodPost, "/api/admin/adapters/health", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", rec.Code)
	}
}

// TestAdaptersListPopulatedCounts verifies REQ-EVAL2-010a / AC-009: the
// existing /api/admin/adapters endpoint now returns populated success_count /
// fail_count / success_rate for adapters with telemetry.
func TestAdaptersListPopulatedCounts(t *testing.T) {
	o := newHealthTestObs(t)
	reg := adapters.NewRegistry(o)
	registerAdapter(t, reg, "reddit")
	seedCalls(o, "reddit", 80, 20) // 0.80

	handler := LoopbackOnly(NewAdaptersHandler(reg))
	req := httptest.NewRequest(http.MethodGet, "/api/admin/adapters", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	var views []adapters.AdapterAdminView
	if err := json.Unmarshal(rec.Body.Bytes(), &views); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var reddit *adapters.AdapterAdminView
	for i := range views {
		if views[i].ID == "reddit" {
			reddit = &views[i]
		}
	}
	if reddit == nil {
		t.Fatal("reddit adapter missing from snapshot")
	}
	if reddit.SuccessCount != 80 {
		t.Errorf("SuccessCount: got %d, want 80", reddit.SuccessCount)
	}
	if reddit.FailCount != 20 {
		t.Errorf("FailCount: got %d, want 20", reddit.FailCount)
	}
	if reddit.SuccessRate < 0.79 || reddit.SuccessRate > 0.81 {
		t.Errorf("SuccessRate: got %v, want ~0.80", reddit.SuccessRate)
	}
}
