package auth

import (
	"context"
)

// contextKey is the internal type for auth-related context keys.
type contextKey string

const (
	// ClaimsKey stores the full JWT claim map in request context.
	// @MX:NOTE: [AUTO] auth.ClaimsKey is SPEC-AUTH-001 specific; downstream code needing
	// only user_id should use costguard.UserIDKey instead.
	ClaimsKey contextKey = "auth.claims"
)

// Claims holds the validated JWT token claims extracted during authentication.
type Claims struct {
	// Subject is the "sub" claim — the authenticated user identifier.
	Subject string
	// Issuer is the "iss" claim — the token issuer URL.
	Issuer string
	// Audience is the "aud" claim — the intended audience(s).
	Audience []string
	// Raw is the full claim map for tenant extraction and audit logging.
	Raw map[string]interface{}
}

// ClaimsFromContext extracts auth.Claims from the request context.
// Returns nil when no JWT was validated (anonymous or disabled mode).
func ClaimsFromContext(ctx context.Context) *Claims {
	v, _ := ctx.Value(ClaimsKey).(*Claims)
	return v
}

// FailureReason enumerates all possible JWT validation failure causes.
// NFR-AUTH1-006: bounded enum used as Prometheus label values.
type FailureReason string

const (
	ReasonExpired               FailureReason = "expired"
	ReasonInvalidSignature      FailureReason = "invalid_signature"
	ReasonInvalidAudience       FailureReason = "invalid_aud"
	ReasonInvalidIssuer         FailureReason = "invalid_iss"
	ReasonInvalidNbf            FailureReason = "invalid_nbf"
	ReasonMalformed             FailureReason = "malformed"
	ReasonRevoked               FailureReason = "revoked"
	ReasonMissingToken          FailureReason = "missing_token"
	ReasonRevocationUnavailable FailureReason = "revocation_check_unavailable"
)

// AuthMode enumerates the authentication enforcement modes.
type AuthMode string

const (
	ModeStrict     AuthMode = "strict"
	ModePermissive AuthMode = "permissive"
	ModeDisabled   AuthMode = "disabled"
)

// TenantMode enumerates tenant resolution strategies.
type TenantMode string

const (
	TenantModeStatic TenantMode = "static"
	TenantModeHeader TenantMode = "header"
	TenantModeClaim  TenantMode = "claim"
)

// RevocationFailureMode controls behavior when Redis is unavailable.
type RevocationFailureMode string

const (
	RevocationFailOpen   RevocationFailureMode = "fail-open"
	RevocationFailClosed RevocationFailureMode = "fail-closed"
)

// JWKSRefreshOutcome enumerates JWKS refresh result types for metrics.
type JWKSRefreshOutcome string

const (
	JWKSRefreshScheduled    JWKSRefreshOutcome = "scheduled"
	JWKSRefreshUnknownKID   JWKSRefreshOutcome = "unknown_kid_fetch"
	JWKSRefreshParseError   JWKSRefreshOutcome = "parse_error"
	JWKSRefreshNetworkError JWKSRefreshOutcome = "network_error"
)
