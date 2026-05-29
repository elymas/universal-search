package auth

import "net/http"

// SPEC-SEC-001 REQ-SEC-012: session cookies MUST set Secure, HttpOnly, and
// SameSite=Lax. V1 does not yet implement the OIDC session callback (see
// callback.go — returns 501), so no cookie is set in production paths today.
// This helper is the single, tested construction point for session cookies so
// that when the OIDC session flow lands it inherits the compliant flags rather
// than hand-rolling http.Cookie literals at the call site.

// SessionCookieName is the canonical name for the usearch session cookie.
const SessionCookieName = "usearch_session"

// NewSessionCookie builds a session cookie with the REQ-SEC-012 secure defaults:
// Secure (HTTPS-only), HttpOnly (no JS access), and SameSite=Lax (CSRF
// mitigation while allowing top-level navigation). maxAgeSeconds <= 0 produces
// a session cookie (no Max-Age); > 0 sets an explicit lifetime.
//
// @MX:NOTE: [AUTO] REQ-SEC-012 secure session cookie factory — the only place
// session cookies are constructed; TestCookieFlagsCompliance pins the flags.
// @MX:SPEC: SPEC-SEC-001
func NewSessionCookie(value string, maxAgeSeconds int) *http.Cookie {
	c := &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if maxAgeSeconds > 0 {
		c.MaxAge = maxAgeSeconds
	}
	return c
}
