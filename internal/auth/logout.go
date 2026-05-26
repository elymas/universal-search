package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// LogoutHandler handles POST /v1/auth/logout requests.
// REQ-AUTH1-009: RP-Initiated Logout + optional Redis revocation.
type LogoutHandler struct {
	middleware         *Middleware
	revocation         *RevocationChecker
	endSessionEndpoint string
	metrics            *AuthMetrics
	postLogoutURI      string
	logger             *slog.Logger
}

// NewLogoutHandler creates a new logout handler.
func NewLogoutHandler(middleware *Middleware, revocation *RevocationChecker, endSessionEndpoint string, postLogoutURI string, metrics *AuthMetrics, logger *slog.Logger) *LogoutHandler {
	return &LogoutHandler{
		middleware:         middleware,
		revocation:         revocation,
		endSessionEndpoint: endSessionEndpoint,
		postLogoutURI:      postLogoutURI,
		metrics:            metrics,
		logger:             logger,
	}
}

// ServeHTTP handles the logout request.
func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract bearer token
	rawToken := extractBearerToken(r)
	if rawToken == "" {
		writeAuthError(w, string(ReasonMissingToken), http.StatusUnauthorized)
		return
	}

	// Validate token
	claims, err := h.middleware.validator.Validate(r.Context(), rawToken)
	if err != nil {
		reason := FailureReasonFromError(err)
		writeAuthError(w, string(reason), http.StatusUnauthorized)
		return
	}

	// Extract jti for revocation
	jti, _ := claims.Raw["jti"].(string)
	expFloat, _ := claims.Raw["exp"].(float64)
	exp := time.Unix(int64(expFloat), 0)

	// Revoke token if revocation is enabled
	if err := h.revocation.RevokeToken(r.Context(), jti, exp); err != nil {
		if h.logger != nil {
			h.logger.Error("failed to revoke token", "jti", jti, "error", err)
		}
	}

	// Increment revoked counter
	h.metrics.TokenRevoked.WithLabelValues("explicit_logout").Inc()

	// Redirect to end_session_endpoint if available
	if h.endSessionEndpoint != "" {
		redirectURL := h.endSessionEndpoint + "?id_token_hint=" + rawToken
		if h.postLogoutURI != "" {
			redirectURL += "&post_logout_redirect_uri=" + h.postLogoutURI
		}
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// No end_session_endpoint: return 204 (server-side revocation only)
	w.WriteHeader(http.StatusNoContent)
}

// _ ensures json import is used (writeAuthError uses json.Encoder)
var _ = json.Marshal
