package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
)

// Validator performs JWT token validation against OIDC provider.
// REQ-AUTH1-003: signature, iss exact, aud whitelist, exp+skew, nbf+skew, sub non-empty.
type Validator struct {
	provider  *oidc.Provider
	verifier  *oidc.IDTokenVerifier
	clockSkew time.Duration
	audiences []string
}

// NewValidator creates a JWT validator for the given OIDC provider.
func NewValidator(provider *oidc.Provider, audiences []string, clockSkew time.Duration) *Validator {
	cfg := &oidc.Config{
		SupportedSigningAlgs: []string{"RS256"},
		// We handle audience and issuer checks ourselves for explicit error reasons
		SkipClientIDCheck: true,
		SkipIssuerCheck:   false,
		Now:               time.Now,
	}

	return &Validator{
		provider:  provider,
		verifier:  provider.Verifier(cfg),
		clockSkew: clockSkew,
		audiences: audiences,
	}
}

// ValidateResult holds the outcome of token validation.
type ValidateResult struct {
	Claims *Claims
}

// Validate verifies a raw JWT token string and returns the validated claims.
// Returns a FailureReason on validation failure.
func (v *Validator) Validate(ctx context.Context, rawToken string) (*Claims, error) {
	if rawToken == "" {
		return nil, fmt.Errorf("%w: empty token", ErrMalformed)
	}

	// Parse unverified to extract header and claims for detailed error reasons
	_, _, err := jwt.NewParser(jwt.WithoutClaimsValidation()).ParseUnverified(rawToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}

	// Verify signature and standard claims using go-oidc
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		reason := classifyError(err)
		return nil, fmt.Errorf("%w: %v", reason, err)
	}

	// Extract all claims
	var allClaims map[string]interface{}
	if err := token.Claims(&allClaims); err != nil {
		return nil, fmt.Errorf("%w: failed to extract claims: %v", ErrMalformed, err)
	}

	// Validate sub is non-empty (REQ-AUTH1-003)
	sub, _ := allClaims["sub"].(string)
	if sub == "" {
		return nil, fmt.Errorf("%w: empty sub claim", ErrMalformed)
	}

	// Validate audience whitelist (REQ-AUTH1-003)
	if err := v.validateAudience(allClaims); err != nil {
		return nil, err
	}

	// Validate issuer (already done by go-oidc verifier, but double-check)
	iss, _ := allClaims["iss"].(string)
	if iss == "" {
		return nil, fmt.Errorf("%w: empty iss claim", ErrMalformed)
	}

	// Validate nbf with clock skew
	if err := v.validateNbf(allClaims); err != nil {
		return nil, err
	}

	claims := &Claims{
		Subject:  sub,
		Issuer:   iss,
		Audience: extractAudience(allClaims),
		Raw:      allClaims,
	}

	return claims, nil
}

// validateAudience checks that the token audience matches the whitelist.
func (v *Validator) validateAudience(claims map[string]interface{}) error {
	aud := extractAudience(claims)
	if len(aud) == 0 {
		return fmt.Errorf("%w: no audience claim", ErrInvalidAudience)
	}

	for _, a := range aud {
		for _, allowed := range v.audiences {
			if a == allowed {
				return nil
			}
		}
	}

	return fmt.Errorf("%w: audience %v not in whitelist %v", ErrInvalidAudience, aud, v.audiences)
}

// validateNbf checks the nbf claim with clock skew tolerance.
func (v *Validator) validateNbf(claims map[string]interface{}) error {
	nbfVal, ok := claims["nbf"]
	if !ok {
		// nbf is optional; if missing, no check needed
		return nil
	}

	var nbfFloat float64
	switch v := nbfVal.(type) {
	case float64:
		nbfFloat = v
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return nil
		}
		nbfFloat = f
	default:
		return nil
	}

	nbfTime := time.Unix(int64(nbfFloat), 0)
	now := time.Now()

	// Token is not yet valid, accounting for clock skew
	if now.Add(v.clockSkew).Before(nbfTime) {
		return fmt.Errorf("%w: token not yet valid (nbf=%v, now+skew=%v)", ErrInvalidNbf, nbfTime, now.Add(v.clockSkew))
	}

	return nil
}

// classifyError maps go-oidc verification errors to FailureReason.
func classifyError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()

	switch {
	case strings.Contains(msg, "token is expired"):
		return ErrExpired
	case strings.Contains(msg, "audience"):
		return ErrInvalidAudience
	case strings.Contains(msg, "issuer"):
		return ErrInvalidIssuer
	case strings.Contains(msg, "signing method"):
		return ErrInvalidSignature
	case strings.Contains(msg, "key"):
		return ErrInvalidSignature
	case strings.Contains(msg, "nbf"):
		return ErrInvalidNbf
	default:
		return ErrMalformed
	}
}

// extractAudience returns the audience claim as a string slice.
func extractAudience(claims map[string]interface{}) []string {
	aud, ok := claims["aud"]
	if !ok {
		return nil
	}

	switch v := aud.(type) {
	case string:
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, a := range v {
			if s, ok := a.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

// Sentinel errors for each failure reason.
var (
	ErrExpired               = failureReasonError{ReasonExpired}
	ErrInvalidSignature      = failureReasonError{ReasonInvalidSignature}
	ErrInvalidAudience       = failureReasonError{ReasonInvalidAudience}
	ErrInvalidIssuer         = failureReasonError{ReasonInvalidIssuer}
	ErrInvalidNbf            = failureReasonError{ReasonInvalidNbf}
	ErrMalformed             = failureReasonError{ReasonMalformed}
	ErrRevoked               = failureReasonError{ReasonRevoked}
	ErrMissingToken          = failureReasonError{ReasonMissingToken}
	ErrRevocationUnavailable = failureReasonError{ReasonRevocationUnavailable}
)

type failureReasonError struct {
	reason FailureReason
}

func (e failureReasonError) Error() string { return string(e.reason) }

// FailureReasonFromError extracts the FailureReason from a validation error.
func FailureReasonFromError(err error) FailureReason {
	if err == nil {
		return ""
	}
	if fre, ok := err.(failureReasonError); ok {
		return fre.reason
	}
	// Check wrapped errors
	if fre, ok := unwrapFailureReason(err); ok {
		return fre.reason
	}
	return ReasonMalformed
}

func unwrapFailureReason(err error) (failureReasonError, bool) {
	for {
		if fre, ok := err.(failureReasonError); ok {
			return fre, true
		}
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
			if err == nil {
				return failureReasonError{}, false
			}
		} else {
			return failureReasonError{}, false
		}
	}
}
