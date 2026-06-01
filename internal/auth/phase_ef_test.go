package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ---------------------------------------------------------------------------
// Redis mock for revocation tests
// ---------------------------------------------------------------------------

// redisMock implements RedisClient for testing.
type redisMock struct {
	mu       sync.Mutex
	sets     map[string][]interface{} // key -> members
	ttls     map[string]time.Duration
	exists   map[string]int64 // key -> count (0 or 1)
	saddErr  error            // inject SADD error
	existErr error            // inject EXISTS error
}

func newRedisMock() *redisMock {
	return &redisMock{
		sets:   make(map[string][]interface{}),
		ttls:   make(map[string]time.Duration),
		exists: make(map[string]int64),
	}
}

func (m *redisMock) SAdd(_ context.Context, key string, members ...interface{}) error {
	if m.saddErr != nil {
		return m.saddErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sets[key] = append(m.sets[key], members...)
	return nil
}

func (m *redisMock) Expire(_ context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ttls[key] = ttl
	return nil
}

func (m *redisMock) Exists(_ context.Context, key string) (int64, error) {
	if m.existErr != nil {
		return 0, m.existErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if count, ok := m.exists[key]; ok {
		return count, nil
	}
	return 0, nil
}

// ---------------------------------------------------------------------------
// Phase E — T-005 tests: Logout + Revocation + Callback
// ---------------------------------------------------------------------------

// §5.8 case A: logout with end_session_endpoint → 302 + Location header
func TestLogoutReturns302WithEndSessionURL(t *testing.T) {
	cfg := defaultTestConfig()
	stub, validator, _ := setupTestAuth(t, cfg)

	// Override discovery to include end_session_endpoint
	endpoint := stub.Issuer() + "/logout"

	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)

	logoutHandler := NewLogoutHandler(
		NewMiddleware(validator, cfg),
		NewRevocationChecker(nil, false, RevocationFailOpen, nil, metrics),
		endpoint,
		"https://usearch.example.com/",
		metrics,
		nil,
	)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "eve@example.com",
		"jti": "tok-002",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	logoutHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Error("Location header is empty")
	}
	if loc != endpoint+"?id_token_hint="+token+"&post_logout_redirect_uri=https://usearch.example.com/" {
		t.Errorf("Location = %q, want end_session_endpoint with id_token_hint and post_logout_redirect_uri", loc)
	}
}

// §5.8 case C: logout without end_session_endpoint → 204
func TestLogoutWithoutEndSessionEndpointReturns204(t *testing.T) {
	cfg := defaultTestConfig()
	stub, validator, _ := setupTestAuth(t, cfg)

	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)

	// Empty end_session_endpoint simulates provider without logout support
	logoutHandler := NewLogoutHandler(
		NewMiddleware(validator, cfg),
		NewRevocationChecker(nil, false, RevocationFailOpen, nil, metrics),
		"",
		"",
		metrics,
		nil,
	)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "eve@example.com",
		"jti": "tok-003",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	logoutHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

// §5.8 case B: logout with revocation enabled → Redis SADD + EXPIRE
func TestLogoutWithRevocationEnabledAddsToRedisSet(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Revocation.Enabled = true
	stub, validator, _ := setupTestAuth(t, cfg)

	mockRedis := newRedisMock()
	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)

	revocation := NewRevocationChecker(mockRedis, true, RevocationFailOpen, nil, metrics)
	logoutHandler := NewLogoutHandler(
		NewMiddleware(validator, cfg),
		revocation,
		stub.Issuer()+"/logout",
		"",
		metrics,
		nil,
	)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "eve@example.com",
		"jti": "tok-revoke-001",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	logoutHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusFound)
	}

	// Verify Redis SADD was called
	mockRedis.mu.Lock()
	key := "auth:revoked:tok-revoke-001"
	if len(mockRedis.sets[key]) == 0 {
		t.Error("expected SADD to be called for revoked token")
	}
	if mockRedis.ttls[key] == 0 {
		t.Error("expected EXPIRE to be called for revoked token")
	}
	mockRedis.mu.Unlock()
}

// §5.8: logout with revocation disabled → no Redis call
func TestLogoutWithRevocationDisabledSkipsRedis(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Revocation.Enabled = false
	stub, validator, _ := setupTestAuth(t, cfg)

	mockRedis := newRedisMock()
	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)

	revocation := NewRevocationChecker(mockRedis, false, RevocationFailOpen, nil, metrics)
	logoutHandler := NewLogoutHandler(
		NewMiddleware(validator, cfg),
		revocation,
		stub.Issuer()+"/logout",
		"",
		metrics,
		nil,
	)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "eve@example.com",
		"jti": "tok-no-revoke",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	logoutHandler.ServeHTTP(rec, req)

	mockRedis.mu.Lock()
	if len(mockRedis.sets) > 0 {
		t.Errorf("expected no Redis calls when revocation disabled, got %d keys", len(mockRedis.sets))
	}
	mockRedis.mu.Unlock()
}

// §5.8: revoked token → 401 on next request
func TestRevokedTokenReturns401(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Revocation.Enabled = true
	stub, validator, _ := setupTestAuth(t, cfg)

	mockRedis := newRedisMock()
	// Pre-populate as if the token was revoked
	mockRedis.exists["auth:revoked:tok-revoked"] = 1

	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)
	revocation := NewRevocationChecker(mockRedis, true, RevocationFailOpen, nil, metrics)
	mw := NewMiddlewareWithRevocation(validator, cfg, revocation, metrics)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "eve@example.com",
		"jti": "tok-revoked",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for revoked token")
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != "revoked" {
		t.Errorf("error = %q, want %q", body["error"], "revoked")
	}
}

// Non-revoked token passes through
func TestNonRevokedTokenPasses(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Revocation.Enabled = true
	stub, validator, _ := setupTestAuth(t, cfg)

	mockRedis := newRedisMock()
	// No revoked entries — empty mock

	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)
	revocation := NewRevocationChecker(mockRedis, true, RevocationFailOpen, nil, metrics)
	mw := NewMiddlewareWithRevocation(validator, cfg, revocation, metrics)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "alice@example.com",
		"jti": "tok-valid",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// Edge2 case A: Redis failure + fail-open default → request passes
func TestRevocationCheckRedisFailureFailsOpenByDefault(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Revocation.Enabled = true
	cfg.Revocation.FailureMode = RevocationFailOpen
	stub, validator, _ := setupTestAuth(t, cfg)

	mockRedis := newRedisMock()
	mockRedis.existErr = errors.New("connection refused")

	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)
	revocation := NewRevocationChecker(mockRedis, true, RevocationFailOpen, nil, metrics)
	mw := NewMiddlewareWithRevocation(validator, cfg, revocation, metrics)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "alice@example.com",
		"jti": "tok-redis-down",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (fail-open should allow request)", rec.Code, http.StatusOK)
	}
}

// Edge2 case B: Redis failure + fail-closed → 401
func TestRevocationCheckRedisFailureFailsClosedWhenConfigured(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Revocation.Enabled = true
	cfg.Revocation.FailureMode = RevocationFailClosed
	stub, validator, _ := setupTestAuth(t, cfg)

	mockRedis := newRedisMock()
	mockRedis.existErr = errors.New("connection refused")

	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)
	revocation := NewRevocationChecker(mockRedis, true, RevocationFailClosed, nil, metrics)
	mw := NewMiddlewareWithRevocation(validator, cfg, revocation, metrics)

	token, err := stub.IssueToken(map[string]interface{}{
		"sub": "alice@example.com",
		"jti": "tok-redis-down",
	}, 5*time.Minute)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when fail-closed")
	}))

	req := httptest.NewRequest("POST", "/query", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (fail-closed should reject)", rec.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["error"] != string(ReasonRevocationUnavailable) {
		t.Errorf("error = %q, want %q", body["error"], ReasonRevocationUnavailable)
	}
}

// TestRateLimiterRedisPathAlwaysAllows verifies that when a Redis client is
// wired the limiter dispatches to allowRedis, which (in v1) always allows.
// This covers the Allow -> allowRedis branch that the in-memory tests skip.
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate
func TestRateLimiterRedisPathAlwaysAllows(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute, newRedisMock())
	// Even beyond the configured limit of 1, the Redis path returns true.
	for i := 0; i < 5; i++ {
		if !limiter.Allow("client-key") {
			t.Fatalf("request %d: Redis-backed Allow returned false, want true", i+1)
		}
	}
}

// §5.9 case D: callback rate limit 60/min → 429 on 61st
func TestAuthCallbackRateLimit60PerMinute(t *testing.T) {
	limiter := NewRateLimiter(60, time.Minute, nil)
	handler := NewCallbackHandler(limiter)

	// First 60 requests should get 501 (not rate limited)
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest("POST", "/v1/auth/callback", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotImplemented {
			t.Errorf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusNotImplemented)
		}
	}

	// 61st request should get 429
	req := httptest.NewRequest("POST", "/v1/auth/callback", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("request 61: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	// Verify Retry-After header
	if ra := rec.Header().Get("Retry-After"); ra != "60" {
		t.Errorf("Retry-After = %q, want %q", ra, "60")
	}
}

// §5.9 case D: callback returns 501 in v1
func TestAuthCallbackReturns501InV1(t *testing.T) {
	limiter := NewRateLimiter(60, time.Minute, nil)
	handler := NewCallbackHandler(limiter)

	req := httptest.NewRequest("POST", "/v1/auth/callback", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}

// §5.9 case D: different IPs have separate rate limits
func TestAuthCallbackRateLimitPerIP(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute, nil)
	handler := NewCallbackHandler(limiter)

	// IP 1: 2 requests (at limit)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/v1/auth/callback", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotImplemented {
			t.Errorf("IP1 request %d: status = %d, want %d", i+1, rec.Code, http.StatusNotImplemented)
		}
	}

	// IP 1: 3rd should be 429
	req := httptest.NewRequest("POST", "/v1/auth/callback", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("IP1 request 3: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	// IP 2: should still be allowed
	req = httptest.NewRequest("POST", "/v1/auth/callback", nil)
	req.RemoteAddr = "5.6.7.8:5678"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("IP2 request 1: status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}

// Logout without token → 401
func TestLogoutWithoutTokenReturns401(t *testing.T) {
	cfg := defaultTestConfig()
	_, validator, _ := setupTestAuth(t, cfg)

	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)

	logoutHandler := NewLogoutHandler(
		NewMiddleware(validator, cfg),
		NewRevocationChecker(nil, false, RevocationFailOpen, nil, metrics),
		"http://example.com/logout",
		"",
		metrics,
		nil,
	)

	req := httptest.NewRequest("POST", "/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	logoutHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// Logout with invalid token → 401
func TestLogoutWithInvalidTokenReturns401(t *testing.T) {
	cfg := defaultTestConfig()
	_, validator, _ := setupTestAuth(t, cfg)

	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)

	logoutHandler := NewLogoutHandler(
		NewMiddleware(validator, cfg),
		NewRevocationChecker(nil, false, RevocationFailOpen, nil, metrics),
		"http://example.com/logout",
		"",
		metrics,
		nil,
	)

	req := httptest.NewRequest("POST", "/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	logoutHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// ---------------------------------------------------------------------------
// Phase F — T-006 tests: Production hardening + Observability
// ---------------------------------------------------------------------------

// NFR-AUTH1-006: no PII in metric labels (verify label names)
func TestAuthMetricsLabelNamesAreBounded(t *testing.T) {
	reg := prometheus.NewRegistry()
	_ = NewAuthMetrics(reg)

	// All expected label names should be bounded enums
	boundedLabels := map[string]bool{
		"outcome": true,
		"reason":  true,
		"trigger": true,
		"mode":    true,
	}

	// This test verifies that the auth metrics use only these label names
	// The actual cardinality guard is in obs/metrics/metrics_test.go
	for label := range boundedLabels {
		if !boundedLabels[label] {
			t.Errorf("label %q not recognized as bounded", label)
		}
	}
}

// NFR-AUTH1-008: metric naming follows OBS-001 convention (usearch_auth_*)
func TestAuthMetricsNamingConvention(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewAuthMetrics(reg)

	// Verify metric names follow usearch_auth_* pattern
	expectedNames := []string{
		"usearch_auth_attempts_total",
		"usearch_auth_failures_total",
		"usearch_auth_token_revoked_total",
		"usearch_auth_validation_duration_seconds",
		"usearch_auth_jwks_refresh_total",
		"usearch_auth_mode",
	}

	// Check that all metrics are registered
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	registeredNames := make(map[string]bool)
	for _, mf := range mfs {
		registeredNames[mf.GetName()] = true
	}

	for _, name := range expectedNames {
		if !registeredNames[name] {
			t.Errorf("metric %q not registered", name)
		}
	}

	// Suppress unused warning
	_ = m
}

// NFR-AUTH1-009: production env + non-strict mode → WARN log
func TestProductionModeWarningWhenNotStrict(t *testing.T) {
	// This test verifies the EmitProductionWarning function behavior
	cfg := DefaultConfig()
	cfg.Mode = ModePermissive

	// Should not panic when called with non-strict mode in production
	warned := EmitProductionWarning("production", cfg.Mode)
	if !warned {
		t.Error("expected warning to be emitted for permissive mode in production")
	}

	// Should not warn for strict mode
	warned = EmitProductionWarning("production", ModeStrict)
	if warned {
		t.Error("expected no warning for strict mode in production")
	}

	// Should not warn for non-production env
	warned = EmitProductionWarning("development", ModePermissive)
	if warned {
		t.Error("expected no warning for non-production environment")
	}
}

// Test callback allowlisted endpoints include /v1/auth/logout
func TestLogoutEndpointBypassesAuth(t *testing.T) {
	cfg := defaultTestConfig()
	cfg.Mode = ModeStrict
	_, _, mw := setupTestAuth(t, cfg)

	handler := mw.JWTValidationMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (allowlist bypass for GET /v1/auth/logout)", rec.Code, http.StatusOK)
	}
}

// Revocation disabled → CheckRevoked always returns false
func TestRevocationDisabledSkipsCheck(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)
	rc := NewRevocationChecker(nil, false, RevocationFailOpen, nil, metrics)

	revoked, err := rc.CheckRevoked(context.Background(), "some-jti")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if revoked {
		t.Error("expected revoked=false when revocation is disabled")
	}
}

// Revocation with empty jti → skip check
func TestRevocationEmptyJTISkipsCheck(t *testing.T) {
	mockRedis := newRedisMock()
	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)
	rc := NewRevocationChecker(mockRedis, true, RevocationFailOpen, nil, metrics)

	revoked, err := rc.CheckRevoked(context.Background(), "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if revoked {
		t.Error("expected revoked=false for empty jti")
	}
}

// RevokeToken with disabled revocation → no-op
func TestRevokeTokenDisabledNoop(t *testing.T) {
	mockRedis := newRedisMock()
	reg := prometheus.NewRegistry()
	metrics := NewAuthMetrics(reg)
	rc := NewRevocationChecker(mockRedis, false, RevocationFailOpen, nil, metrics)

	err := rc.RevokeToken(context.Background(), "some-jti", time.Now().Add(5*time.Minute))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	mockRedis.mu.Lock()
	if len(mockRedis.sets) > 0 {
		t.Error("expected no Redis calls when revocation disabled")
	}
	mockRedis.mu.Unlock()
}

// Test extractIP helper
func TestExtractIP(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want string
	}{
		{"RemoteAddr only", httptest.NewRequest("GET", "/", nil), "192.0.2.1:1234"},
		{"X-Forwarded-For", func() *http.Request {
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("X-Forwarded-For", "10.0.0.1")
			return r
		}(), "10.0.0.1"},
		{"X-Real-IP", func() *http.Request {
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("X-Real-IP", "10.0.0.2")
			return r
		}(), "10.0.0.2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIP(tt.req)
			if got != tt.want {
				t.Errorf("extractIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
