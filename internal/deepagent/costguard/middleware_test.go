package costguard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// Phase E: Middleware + chi Wiring
// ---------------------------------------------------------------------------

// testNextHandler is a simple handler that records whether it was called.
type testNextHandler struct {
	called bool
}

func (h *testNextHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.called = true
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// TestIdentityMiddlewareReadsXUserId verifies that the identity middleware
// reads the X-User-Id header and injects it into context.
// REQ-DEEP4-001.
func TestIdentityMiddlewareReadsXUserId(t *testing.T) {
	t.Parallel()

	mw := NewMiddleware(DefaultConfig(), nil, nil, nil)

	var gotUserID string
	captureHandler := mw.IdentityMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodPost, "/deep", nil)
	req.Header.Set("X-User-Id", "alice@example.com")
	captureHandler.ServeHTTP(httptest.NewRecorder(), req)

	if gotUserID != "alice@example.com" {
		t.Errorf("user_id: got %q, want %q", gotUserID, "alice@example.com")
	}
}

// TestIdentityMiddlewareDefaultsAnonymous verifies that when X-User-Id
// header is absent, the middleware defaults to "anonymous".
// REQ-DEEP4-001.
func TestIdentityMiddlewareDefaultsAnonymous(t *testing.T) {
	t.Parallel()

	mw := NewMiddleware(DefaultConfig(), nil, nil, nil)

	var gotUserID string
	handler := mw.IdentityMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotUserID = UserIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodPost, "/deep", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotUserID != "anonymous" {
		t.Errorf("user_id: got %q, want %q", gotUserID, "anonymous")
	}
}

// TestIdentityMiddlewareDefaultTenantFromConfig verifies that when X-Tenant-Id
// is absent, the middleware uses the default tenant from config.
// REQ-DEEP4-001.
func TestIdentityMiddlewareDefaultTenantFromConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.DefaultTenantID = "my-team"
	mw := NewMiddleware(cfg, nil, nil, nil)

	var gotTenant string
	handler := mw.IdentityMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotTenant = TenantIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodPost, "/deep", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotTenant != "my-team" {
		t.Errorf("tenant_id: got %q, want %q", gotTenant, "my-team")
	}
}

// TestCapExceededReturns429 verifies that cap exceeded returns HTTP 429.
// REQ-DEEP4-010.
func TestCapExceededReturns429(t *testing.T) {
	t.Parallel()

	_, client := setupMiniredis(t)
	cfg := DefaultConfig()
	cfg.Tenant.MaxCallsPerDay = 1

	checker := NewCapChecker(adaptRedisClient{client}, cfg)
	metrics := RegisterMetrics(prometheus.NewRegistry())
	mw := NewMiddleware(cfg, checker, nil, metrics)

	// Exhaust the cap.
	checker.EvaluateAtomic(context.Background(), "default", "anonymous", 0.01)

	next := &testNextHandler{}
	handler := mw.IdentityMiddleware(mw.CapCheckMiddleware(next))

	req := httptest.NewRequest(http.MethodPost, "/deep", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	if rw.Code != http.StatusTooManyRequests {
		t.Errorf("status: got %d, want %d", rw.Code, http.StatusTooManyRequests)
	}
	if next.called {
		t.Error("next handler should NOT have been called on cap exceeded")
	}
}

// TestCapExceededRetryAfterHeader verifies the Retry-After header on 429.
// REQ-DEEP4-010.
func TestCapExceededRetryAfterHeader(t *testing.T) {
	t.Parallel()

	_, client := setupMiniredis(t)
	cfg := DefaultConfig()
	cfg.Tenant.MaxCallsPerDay = 1

	checker := NewCapChecker(adaptRedisClient{client}, cfg)
	metrics := RegisterMetrics(prometheus.NewRegistry())
	mw := NewMiddleware(cfg, checker, nil, metrics)

	// Exhaust the cap.
	checker.EvaluateAtomic(context.Background(), "default", "anonymous", 0.01)

	handler := mw.IdentityMiddleware(mw.CapCheckMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodPost, "/deep", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	retryAfter := rw.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header is missing")
	}
}

// TestCapExceededResponseBodyShape verifies the JSON response body shape.
// REQ-DEEP4-010: {error, dimension, remaining, reset_at}.
func TestCapExceededResponseBodyShape(t *testing.T) {
	t.Parallel()

	_, client := setupMiniredis(t)
	cfg := DefaultConfig()
	cfg.Tenant.MaxCallsPerDay = 1

	checker := NewCapChecker(adaptRedisClient{client}, cfg)
	metrics := RegisterMetrics(prometheus.NewRegistry())
	mw := NewMiddleware(cfg, checker, nil, metrics)

	// Exhaust the cap.
	checker.EvaluateAtomic(context.Background(), "default", "anonymous", 0.01)

	handler := mw.IdentityMiddleware(mw.CapCheckMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodPost, "/deep", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	var body map[string]interface{}
	if err := json.NewDecoder(rw.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["error"] != "cap_exceeded" {
		t.Errorf("error: got %v, want %q", body["error"], "cap_exceeded")
	}
	if _, ok := body["dimension"]; !ok {
		t.Error("dimension field missing")
	}
	if _, ok := body["remaining"]; !ok {
		t.Error("remaining field missing")
	}
	if _, ok := body["reset_at"]; !ok {
		t.Error("reset_at field missing")
	}
}

// TestDegradeHeaderFallsBackToBasic verifies that X-Allow-Degrade: 1
// causes a 200 with X-Deep-Degraded header instead of 429.
// REQ-DEEP4-011.
func TestDegradeHeaderFallsBackToBasic(t *testing.T) {
	t.Parallel()

	_, client := setupMiniredis(t)
	cfg := DefaultConfig()
	cfg.Tenant.MaxCallsPerDay = 1

	checker := NewCapChecker(adaptRedisClient{client}, cfg)
	metrics := RegisterMetrics(prometheus.NewRegistry())
	mw := NewMiddleware(cfg, checker, nil, metrics)

	// Exhaust the cap.
	checker.EvaluateAtomic(context.Background(), "default", "anonymous", 0.01)

	nextCalled := false
	handler := mw.IdentityMiddleware(mw.CapCheckMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodPost, "/deep", nil)
	req.Header.Set("X-Allow-Degrade", "1")
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rw.Code)
	}
	if !nextCalled {
		t.Error("next handler should have been called (degraded path)")
	}
	if rw.Header().Get("X-Deep-Degraded") != "cap-exceeded" {
		t.Errorf("X-Deep-Degraded: got %q, want %q", rw.Header().Get("X-Deep-Degraded"), "cap-exceeded")
	}
}

// TestRedisOutageFailsClosed verifies that Redis failure returns 503
// when redis_failure_mode is "fail-closed" (default).
// REQ-DEEP4-014.
func TestRedisOutageFailsClosed(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.RedisFailureMode = "fail-closed"

	metrics := RegisterMetrics(prometheus.NewRegistry())
	brokenChecker := NewCapChecker(&brokenRedisClient{}, cfg)
	mw := NewMiddleware(cfg, brokenChecker, nil, metrics)

	handler := mw.IdentityMiddleware(mw.CapCheckMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodPost, "/deep", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	if rw.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want %d", rw.Code, http.StatusServiceUnavailable)
	}
}

// TestRedisOutageFailOpenOverride verifies that Redis failure allows
// requests through when redis_failure_mode is "fail-open".
// REQ-DEEP4-014.
func TestRedisOutageFailOpenOverride(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.RedisFailureMode = "fail-open"

	metrics := RegisterMetrics(prometheus.NewRegistry())
	brokenChecker := NewCapChecker(&brokenRedisClient{}, cfg)
	mw := NewMiddleware(cfg, brokenChecker, nil, metrics)

	nextCalled := false
	handler := mw.IdentityMiddleware(mw.CapCheckMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodPost, "/deep", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	if rw.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rw.Code)
	}
	if !nextCalled {
		t.Error("next handler should have been called (fail-open)")
	}
}

// ---------------------------------------------------------------------------
// Helpers for Phase E tests
// ---------------------------------------------------------------------------

// brokenRedisClient always returns errors to simulate Redis outage.
type brokenRedisClient struct{}

func (b *brokenRedisClient) Eval(_ context.Context, _ string, _ []string, _ ...interface{}) *redis.Cmd {
	cmd := redis.NewCmd(context.Background())
	cmd.SetErr(fmt.Errorf("redis: connection refused"))
	return cmd
}
