package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/elymas/universal-search/internal/deepagent/costguard"
)

// Middleware provides JWT validation middleware for HTTP handlers.
type Middleware struct {
	validator  *Validator
	config     Config
	revocation *RevocationChecker
	metrics    *AuthMetrics
}

// NewMiddleware creates a new auth middleware.
func NewMiddleware(validator *Validator, config Config) *Middleware {
	return &Middleware{
		validator: validator,
		config:    config,
	}
}

// NewMiddlewareWithRevocation creates a middleware with revocation checking.
func NewMiddlewareWithRevocation(validator *Validator, config Config, revocation *RevocationChecker, metrics *AuthMetrics) *Middleware {
	return &Middleware{
		validator:  validator,
		config:     config,
		revocation: revocation,
		metrics:    metrics,
	}
}

// JWTValidationMiddleware validates JWT bearer tokens and injects identity into context.
// @MX:ANCHOR: [AUTO] JWT validation middleware; callers: main.go, integration tests, future endpoints
// @MX:REASON: This middleware is the source of cost_ledger.user_id values.
// Changing it may break DEEP-004 forward-compat invariant (REQ-AUTH1-006).
func (m *Middleware) JWTValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if endpoint is in allowlist (REQ-AUTH1-008)
		if m.isAllowlisted(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Check mode
		switch m.config.Mode {
		case ModeDisabled:
			// Bypass entirely — let costguard.IdentityMiddleware handle it
			next.ServeHTTP(w, r)
			return
		case ModeStrict, ModePermissive:
			// Continue with JWT validation below
		}

		// Extract bearer token
		rawToken := extractBearerToken(r)
		if rawToken == "" {
			if m.config.Mode == ModePermissive {
				// Permissive mode: inject anonymous identity (REQ-AUTH1-004)
				ctx := r.Context()
				ctx = context.WithValue(ctx, costguard.UserIDKey, "anonymous")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			// Strict mode: reject
			writeAuthError(w, "missing_token", http.StatusUnauthorized)
			return
		}

		// Validate token
		claims, err := m.validator.Validate(r.Context(), rawToken)
		if err != nil {
			reason := FailureReasonFromError(err)
			writeAuthError(w, string(reason), http.StatusUnauthorized)
			return
		}

		// Check revocation (REQ-AUTH1-010)
		if m.revocation != nil {
			jti, _ := claims.Raw["jti"].(string)
			revoked, rerr := m.revocation.CheckRevoked(r.Context(), jti)
			if rerr != nil {
				reason := FailureReasonFromError(rerr)
				writeAuthError(w, string(reason), http.StatusUnauthorized)
				return
			}
			if revoked {
				writeAuthError(w, string(ReasonRevoked), http.StatusUnauthorized)
				return
			}
		}

		// Inject claims and user ID into context
		// REQ-AUTH1-003: inject auth.ClaimsKey + costguard.UserIDKey + costguard.TenantIDKey
		ctx := r.Context()
		ctx = context.WithValue(ctx, ClaimsKey, claims)
		ctx = context.WithValue(ctx, costguard.UserIDKey, claims.Subject)

		// Resolve tenant (REQ-AUTH1-007)
		tenantID := resolveTenant(m.config.Tenant, claims, r)
		ctx = context.WithValue(ctx, costguard.TenantIDKey, tenantID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isAllowlisted checks if the given path bypasses auth middleware.
// REQ-AUTH1-008: /healthz, /metrics, /v1/auth/callback, /v1/auth/login
func (m *Middleware) isAllowlisted(path string) bool {
	for _, p := range m.config.AllowEndpoints {
		if p == path {
			return true
		}
	}
	return false
}

// extractBearerToken extracts the Bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

// resolveTenant determines the tenant ID based on tenant mode configuration.
// REQ-AUTH1-007: claim, header, or static mode.
func resolveTenant(tenantCfg TenantConfig, claims *Claims, r *http.Request) string {
	switch tenantCfg.Mode {
	case TenantModeClaim:
		if claims != nil && claims.Raw != nil && tenantCfg.ClaimPath != "" {
			if v, ok := claims.Raw[tenantCfg.ClaimPath]; ok {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
		return tenantCfg.DefaultTenantID

	case TenantModeHeader:
		if header := r.Header.Get("X-Tenant-Id"); header != "" {
			return header
		}
		return tenantCfg.DefaultTenantID

	case TenantModeStatic:
		return tenantCfg.DefaultTenantID

	default:
		return tenantCfg.DefaultTenantID
	}
}

// writeAuthError writes a JSON error response for authentication failures.
func writeAuthError(w http.ResponseWriter, reason string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": reason,
	})
}

// emitAuthEvent logs a structured authentication event.
// AUTH-003 forward-compat: additive-only schema.
func emitAuthEvent(logger *slog.Logger, outcome string, subjectHash string, issuer string, audience string, tokenAge time.Duration) {
	if logger == nil {
		return
	}
	logger.Info("auth.validation",
		"event_type", "auth.validation",
		"outcome", outcome,
		"subject_hash", subjectHash,
		"issuer", issuer,
		"audience", audience,
		"token_age_seconds", tokenAge.Seconds(),
	)
}
