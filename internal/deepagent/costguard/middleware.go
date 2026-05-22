package costguard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Context keys for identity and costguard data.
type contextKey string

const (
	UserIDKey   contextKey = "costguard.user_id"
	TenantIDKey contextKey = "costguard.tenant_id"
	RequestIDKey contextKey = "costguard.request_id"
)

// Middleware provides chi-compatible middleware functions for costguard.
type Middleware struct {
	cfg     Config
	checker *CapChecker
	screen  *HaikuScreen
	metrics *Metrics
}

// NewMiddleware creates a new Middleware with the given dependencies.
func NewMiddleware(cfg Config, checker *CapChecker, screen *HaikuScreen, metrics *Metrics) *Middleware {
	return &Middleware{
		cfg:     cfg,
		checker: checker,
		screen:  screen,
		metrics: metrics,
	}
}

// IdentityMiddleware reads identity from context (JWT path) or headers (V1 path), falls back to
// defaults, and injects into request context.
// REQ-DEEP4-001: identity middleware reads headers, falls back to anonymous/default.
// REQ-AUTH1-006: source-priority: (a) context UserIDKey (JWT path) > (b) X-User-Id header (DEEP-004 V1 path) > (c) "anonymous".
// @MX:ANCHOR: [AUTO] Identity bridge between AUTH-001 and DEEP-004; callers: main.go chain, integration tests, /deep route
// @MX:REASON: Joint invariant for REQ-AUTH1-006 + REQ-DEEP4-001. Source-priority determines cost_ledger.user_id.
func (m *Middleware) IdentityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Source priority (a): check if UserIDKey was already set by JWT middleware
		existingUserID, _ := ctx.Value(UserIDKey).(string)
		userID := existingUserID

		// Source priority (b): fall back to X-User-Id header (DEEP-004 V1 path)
		if userID == "" {
			userID = r.Header.Get("X-User-Id")
		}

		// Source priority (c): fall back to "anonymous"
		if userID == "" {
			userID = "anonymous"
		}

		// Tenant: check context first (JWT path), then header, then default
		existingTenantID, _ := ctx.Value(TenantIDKey).(string)
		tenantID := existingTenantID
		if tenantID == "" {
			tenantID = r.Header.Get("X-Tenant-Id")
		}
		if tenantID == "" {
			tenantID = m.cfg.DefaultTenantID
		}

		ctx = context.WithValue(ctx, UserIDKey, userID)
		ctx = context.WithValue(ctx, TenantIDKey, tenantID)

		// Propagate request ID from upstream or generate one.
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = fmt.Sprintf("req_%d", time.Now().UnixNano())
		}
		ctx = context.WithValue(ctx, RequestIDKey, reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CapCheckMiddleware enforces the daily cap via Redis atomic evaluation.
// REQ-DEEP4-009: atomic cap-check. REQ-DEEP4-010: 429 on cap exceeded.
// REQ-DEEP4-011: X-Allow-Degrade header for fallback.
// @MX:ANCHOR: [AUTO] Cap-check middleware; callers: synthesis.go, /deep route, integration tests
// @MX:REASON: fan_in >= 3; cap enforcement is the invariant for every /deep call
func (m *Middleware) CapCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		userID, _ := ctx.Value(UserIDKey).(string)
		tenantID, _ := ctx.Value(TenantIDKey).(string)

		// Estimated cost for this call (conservative average).
		estimatedCost := 0.07

		result, err := m.checker.EvaluateAtomic(r.Context(), tenantID, userID, estimatedCost)
		if err != nil {
			// Redis failure path.
			if m.cfg.RedisFailureMode == "fail-open" {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "costguard_unavailable",
				"detail": "redis unreachable",
			})
			return
		}

		if !result.Allowed {
			// Check for degrade header.
			if r.Header.Get("X-Allow-Degrade") == "1" {
				w.Header().Set("X-Deep-Degraded", "cap-exceeded")
				r = r.WithContext(context.WithValue(r.Context(), contextKey("degraded"), true))
				next.ServeHTTP(w, r)
				return
			}

			// Hard reject with 429.
			w.Header().Set("Content-Type", "application/json")
			resetAt := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":     "cap_exceeded",
				"dimension": string(result.Exceeded),
				"remaining": map[string]interface{}{
					"calls": result.RemainingCalls,
					"usd":   result.RemainingUSD,
				},
				"reset_at": resetAt,
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// UserIDFromContext extracts the user ID from the request context.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(UserIDKey).(string)
	return v
}

// TenantIDFromContext extracts the tenant ID from the request context.
func TenantIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(TenantIDKey).(string)
	return v
}

// RequestIDFromContext extracts the request ID from the request context.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(RequestIDKey).(string)
	return v
}
