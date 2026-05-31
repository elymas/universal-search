package ratelimit

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/elymas/universal-search/internal/security/events"
)

// EventEmitter is the minimal slice of *events.Emitter the middleware needs.
// Kept local so the middleware depends on a behaviour, not a concrete type.
type EventEmitter interface {
	Emit(ctx context.Context, ev events.Event) error
}

// MetricRecorder is the minimal slice of *metrics.SecurityCollectors the
// middleware needs. tenantIDClass is the bounded {known, unknown} bucket.
type MetricRecorder interface {
	RecordRateLimitExceeded(tenantIDClass string)
}

// TenantExtractor pulls the tenant identifier from a request context. The
// caller supplies this so ratelimit need not import costguard (whose context
// keys are unexported strings). An empty result is classified as "unknown".
type TenantExtractor func(ctx context.Context) string

// MiddlewareConfig wires the limiter to its collaborators and enforcement mode.
type MiddlewareConfig struct {
	Limiter *Limiter
	Emitter EventEmitter
	Metrics MetricRecorder
	Tenant  TenantExtractor

	// RejectOnExceed gates HTTP 429 enforcement. V1 default is false
	// (alert-only): breaches emit an event + metric but the request proceeds.
	// Set true (security.yaml ratelimit.reject_on_exceed) only after tuning.
	RejectOnExceed bool

	// RetryAfterSeconds is the Retry-After header value sent on a 429 (default
	// 60, matching the per-minute refill window).
	RetryAfterSeconds int
}

const defaultRetryAfterSeconds = 60

// Middleware returns a stdlib net/http middleware enforcing per-tenant rate
// limits. On breach it always emits ratelimit.exceeded + increments the metric;
// it returns HTTP 429 + Retry-After ONLY when RejectOnExceed is true.
//
// @MX:NOTE: [AUTO] Alert-only by default (REQ-SEC-014 / plan R9). The 429 path
// is config-gated via RejectOnExceed; the 429 response models the
// costguard CapCheckMiddleware shape (Retry-After + JSON body).
func Middleware(cfg MiddlewareConfig) func(http.Handler) http.Handler {
	retryAfter := cfg.RetryAfterSeconds
	if retryAfter <= 0 {
		retryAfter = defaultRetryAfterSeconds
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := ""
			if cfg.Tenant != nil {
				tenant = cfg.Tenant(r.Context())
			}

			if cfg.Limiter != nil && !cfg.Limiter.Allow(tenant) {
				cfg.onExceeded(r.Context(), tenant)

				if cfg.RejectOnExceed {
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
					w.WriteHeader(http.StatusTooManyRequests)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"error":       "rate_limit_exceeded",
						"retry_after": retryAfter,
					})
					return
				}
				// Alert-only: fall through and serve the request.
			}

			next.ServeHTTP(w, r.WithContext(r.Context()))
		})
	}
}

// onExceeded emits the ratelimit.exceeded security event and increments the
// metric. tenant_id_class is "known" for a non-empty tenant, else "unknown" —
// the raw tenant_id is never recorded as a label or audit-free field here.
func (cfg MiddlewareConfig) onExceeded(ctx context.Context, tenant string) {
	class := tenantClass(tenant)

	if cfg.Metrics != nil {
		cfg.Metrics.RecordRateLimitExceeded(class)
	}
	if cfg.Emitter != nil {
		_ = cfg.Emitter.Emit(ctx, events.Event{
			Type:     events.TypeRateLimitExceeded,
			Severity: events.SeverityMedium,
		})
	}
}

func tenantClass(tenant string) string {
	if tenant == "" {
		return "unknown"
	}
	return "known"
}
