package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elymas/universal-search/internal/security/events"
)

// fakeEmitter records emitted events without touching the audit chain.
type fakeEmitter struct {
	events []events.Event
}

func (f *fakeEmitter) Emit(_ context.Context, ev events.Event) error {
	f.events = append(f.events, ev)
	return nil
}

// fakeMetrics records the tenant_id_class values passed to the recorder.
type fakeMetrics struct {
	classes []string
}

func (f *fakeMetrics) RecordRateLimitExceeded(tenantIDClass string) {
	f.classes = append(f.classes, tenantIDClass)
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

// burst=1 + zero refill (PerTenantPerMinute=1) makes the second immediate
// request a guaranteed breach within the test window.
func tightLimiter() *Limiter {
	return New(Config{PerTenantPerMinute: 1, Burst: 1})
}

func TestRateLimitExceededAlertOnly(t *testing.T) {
	em := &fakeEmitter{}
	mt := &fakeMetrics{}
	mw := Middleware(MiddlewareConfig{
		Limiter:        tightLimiter(),
		Emitter:        em,
		Metrics:        mt,
		Tenant:         func(context.Context) string { return "tenant-a" },
		RejectOnExceed: false, // alert-only
	})
	h := mw(okHandler())

	// First request consumes the single token.
	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr1.Code != http.StatusOK {
		t.Fatalf("first request: got %d, want 200", rr1.Code)
	}

	// Second request breaches — but alert-only means it STILL serves 200.
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr2.Code != http.StatusOK {
		t.Errorf("alert-only breach: got %d, want 200 (no rejection)", rr2.Code)
	}

	// Event + metric must still fire on the breach.
	if len(em.events) != 1 || em.events[0].Type != events.TypeRateLimitExceeded {
		t.Errorf("expected 1 ratelimit.exceeded event, got %+v", em.events)
	}
	if len(mt.classes) != 1 || mt.classes[0] != "known" {
		t.Errorf("expected 1 metric record with class=known, got %+v", mt.classes)
	}
}

func TestRateLimitEnforceReturns429(t *testing.T) {
	em := &fakeEmitter{}
	mt := &fakeMetrics{}
	mw := Middleware(MiddlewareConfig{
		Limiter:        tightLimiter(),
		Emitter:        em,
		Metrics:        mt,
		Tenant:         func(context.Context) string { return "tenant-a" },
		RejectOnExceed: true, // enforcement on
	})
	h := mw(okHandler())

	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr1.Code != http.StatusOK {
		t.Fatalf("first request: got %d, want 200", rr1.Code)
	}

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("enforced breach: got %d, want 429", rr2.Code)
	}
	if len(em.events) != 1 {
		t.Errorf("expected 1 event on breach, got %d", len(em.events))
	}
}

func TestRateLimitRetryAfterHeader(t *testing.T) {
	mw := Middleware(MiddlewareConfig{
		Limiter:           tightLimiter(),
		Tenant:            func(context.Context) string { return "t" },
		RejectOnExceed:    true,
		RetryAfterSeconds: 90,
	})
	h := mw(okHandler())

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", rr.Code)
	}
	if got := rr.Header().Get("Retry-After"); got != "90" {
		t.Errorf("Retry-After = %q, want %q", got, "90")
	}
}

func TestRateLimitRetryAfterDefault(t *testing.T) {
	mw := Middleware(MiddlewareConfig{
		Limiter:        tightLimiter(),
		Tenant:         func(context.Context) string { return "t" },
		RejectOnExceed: true,
		// RetryAfterSeconds unset → default 60.
	})
	h := mw(okHandler())
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rr.Header().Get("Retry-After"); got != "60" {
		t.Errorf("default Retry-After = %q, want %q", got, "60")
	}
}

func TestRateLimitMetricCardinality(t *testing.T) {
	// tenant_id_class must collapse to exactly {known, unknown} regardless of
	// the raw tenant value — the raw tenant_id must NEVER reach the recorder.
	tests := []struct {
		name      string
		tenant    string
		wantClass string
	}{
		{"named tenant -> known", "tenant-xyz-123", "known"},
		{"another named tenant -> known", "tenant-abc", "known"},
		{"empty tenant -> unknown", "", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt := &fakeMetrics{}
			mw := Middleware(MiddlewareConfig{
				Limiter:        tightLimiter(),
				Metrics:        mt,
				Tenant:         func(context.Context) string { return tt.tenant },
				RejectOnExceed: false,
			})
			h := mw(okHandler())
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

			if len(mt.classes) != 1 {
				t.Fatalf("expected exactly 1 metric record, got %d", len(mt.classes))
			}
			if mt.classes[0] != tt.wantClass {
				t.Errorf("class = %q, want %q", mt.classes[0], tt.wantClass)
			}
			if mt.classes[0] != "known" && mt.classes[0] != "unknown" {
				t.Errorf("class %q escapes the bounded {known,unknown} set", mt.classes[0])
			}
		})
	}
}

func TestLimiterAllowsWithinBurst(t *testing.T) {
	l := New(Config{PerTenantPerMinute: 60, Burst: 5})
	for i := 0; i < 5; i++ {
		if !l.Allow("t") {
			t.Fatalf("request %d within burst should be allowed", i)
		}
	}
	if l.Allow("t") {
		t.Error("6th request exceeds burst of 5; should be denied")
	}
}

func TestLimiterIsolatesTenants(t *testing.T) {
	l := New(Config{PerTenantPerMinute: 1, Burst: 1})
	if !l.Allow("tenant-a") {
		t.Fatal("tenant-a first request should be allowed")
	}
	// tenant-b has its own bucket and must not be affected by tenant-a.
	if !l.Allow("tenant-b") {
		t.Error("tenant-b first request should be allowed (separate bucket)")
	}
}

func TestLimiterDefaults(t *testing.T) {
	l := New(Config{}) // all zero → defaults
	if l.cfg.PerTenantPerMinute != defaultPerMinute || l.cfg.Burst != defaultBurst {
		t.Errorf("defaults not applied: %+v", l.cfg)
	}
}
