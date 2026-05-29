// Package ratelimit provides per-tenant rate limiting using token buckets.
//
// REQ-SEC-014: Per-tenant token bucket using golang.org/x/time/rate.
// Default: 60 queries/min per tenant.
// @MX:NOTE: [AUTO] V1 is alert-only; no auto-blocking per D6.
// @MX:SPEC: SPEC-SEC-001
package ratelimit

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// DefaultRate is the default per-tenant rate limit (60 queries/min).
const DefaultRate = 60

// DefaultBurst is the default burst size.
const DefaultBurst = 10

// TenantClassifier determines the class of a tenant for metric labeling.
type TenantClassifier interface {
	ClassifyTenant(tenantID string) string
}

// Limiter provides per-tenant rate limiting.
type Limiter struct {
	mu         sync.Mutex
	limiters   map[string]*rate.Limiter
	rateLimit  rate.Limit
	burst      int
	classifier TenantClassifier
}

// NewLimiter creates a new rate limiter with the specified queries per minute.
// If queriesPerMinute <= 0, DefaultRate (60/min) is used.
func NewLimiter(queriesPerMinute int, classifier TenantClassifier) *Limiter {
	if queriesPerMinute <= 0 {
		queriesPerMinute = DefaultRate
	}
	r := rate.Every(time.Minute / time.Duration(queriesPerMinute))
	return &Limiter{
		limiters:   make(map[string]*rate.Limiter),
		rateLimit:  r,
		burst:      DefaultBurst,
		classifier: classifier,
	}
}

// Allow checks if the tenant is within rate limits.
func (l *Limiter) Allow(tenantID string) bool {
	limiter := l.getLimiter(tenantID)
	return limiter.Allow()
}

// TenantIDClass returns the classification of a tenant for metric labeling.
func (l *Limiter) TenantIDClass(tenantID string) string {
	if l.classifier != nil {
		return l.classifier.ClassifyTenant(tenantID)
	}
	return "unknown"
}

// getLimiter returns (or creates) the rate limiter for a tenant.
func (l *Limiter) getLimiter(tenantID string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	if limiter, ok := l.limiters[tenantID]; ok {
		return limiter
	}

	limiter := rate.NewLimiter(l.rateLimit, l.burst)
	l.limiters[tenantID] = limiter
	return limiter
}

// Write429 writes an HTTP 429 Too Many Requests response with Retry-After header.
func Write429(w http.ResponseWriter, retryAfterSeconds int) {
	w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfterSeconds))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	fmt.Fprintf(w, `{"error":"rate_limit_exceeded","retry_after":%d}`, retryAfterSeconds)
}

// Middleware returns an HTTP middleware that enforces per-tenant rate limiting.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			tenantID = "anonymous"
		}

		if !l.Allow(tenantID) {
			Write429(w, 1)
			return
		}

		next.ServeHTTP(w, r)
	})
}
