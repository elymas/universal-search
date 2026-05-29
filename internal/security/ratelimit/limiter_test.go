package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/security/ratelimit"
)

type mockClassifier struct{}

func (m *mockClassifier) ClassifyTenant(_ string) string { return "known" }

func TestRateLimitAllowsUnderLimit(t *testing.T) {
	t.Parallel()
	limiter := ratelimit.NewLimiter(60, &mockClassifier{})

	for i := 0; i < 5; i++ {
		if !limiter.Allow("tenant-1") {
			t.Fatalf("request %d should be allowed under limit", i+1)
		}
	}
}

func TestRateLimitExceededReturns429(t *testing.T) {
	// Not parallel: uses time-sensitive rate limiting
	limiter := ratelimit.NewLimiter(60, &mockClassifier{})
	// Exhaust burst (10) + rate tokens
	for i := 0; i < 12; i++ {
		limiter.Allow("tenant-1")
	}
	// Should be rate limited now
	if limiter.Allow("tenant-1") {
		t.Fatal("expected request to be rate limited after burst exhaustion")
	}
}

func TestRateLimitWrite429(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	ratelimit.Write429(w, 5)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != "5" {
		t.Fatalf("expected Retry-After=5, got %q", got)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	// Not parallel: uses time-sensitive rate limiting
	limiter := ratelimit.NewLimiter(60, &mockClassifier{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := limiter.Middleware(handler)

	// Exhaust burst
	for i := 0; i < 12; i++ {
		req := httptest.NewRequest("GET", "/api/v1/search?q=test", nil)
		req.Header.Set("X-Tenant-ID", "tenant-1")
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/api/v1/search?q=test", nil)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	w := httptest.NewRecorder()
	middleware.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for rate-limited request, got %d", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestRateLimitPerTenantIsolation(t *testing.T) {
	t.Parallel()
	limiter := ratelimit.NewLimiter(60, &mockClassifier{})

	// Exhaust tenant-1 burst
	for i := 0; i < 12; i++ {
		limiter.Allow("tenant-1")
	}

	// tenant-2 should still be allowed (separate bucket)
	if !limiter.Allow("tenant-2") {
		t.Fatal("tenant-2 should have independent rate limit")
	}
}

func TestRateLimitTenantIDClass(t *testing.T) {
	t.Parallel()
	limiter := ratelimit.NewLimiter(60, &mockClassifier{})
	if got := limiter.TenantIDClass("tenant-1"); got != "known" {
		t.Fatalf("expected 'known', got %q", got)
	}

	limiterNoClassifier := ratelimit.NewLimiter(60, nil)
	if got := limiterNoClassifier.TenantIDClass("tenant-1"); got != "unknown" {
		t.Fatalf("expected 'unknown', got %q", got)
	}
}

func TestRateLimitDefaultValues(t *testing.T) {
	t.Parallel()
	limiter := ratelimit.NewLimiter(0, nil) // 0 should use default (60)
	if !limiter.Allow("tenant-1") {
		t.Fatal("should allow with default rate")
	}
}

func TestRateLimitBurstReplenishment(t *testing.T) {
	// Not parallel: uses time-sensitive rate limiting
	limiter := ratelimit.NewLimiter(60, nil)

	// Exhaust burst
	for i := 0; i < 12; i++ {
		limiter.Allow("tenant-1")
	}
	if limiter.Allow("tenant-1") {
		t.Fatal("expected rate limit after burst exhaustion")
	}

	// Wait for one token to replenish (1 second at 60/min rate)
	time.Sleep(1100 * time.Millisecond)

	if !limiter.Allow("tenant-1") {
		t.Fatal("expected request to be allowed after token replenishment")
	}
}
