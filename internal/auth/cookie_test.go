package auth

import (
	"net/http"
	"testing"
)

// TestCookieFlagsCompliance verifies REQ-SEC-012: every session cookie produced
// by the canonical factory sets Secure, HttpOnly, and SameSite=Lax.
func TestCookieFlagsCompliance(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		maxAge int
	}{
		{"session cookie (no max-age)", 0},
		{"persistent cookie", 3600},
		{"negative max-age treated as session", -1},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := NewSessionCookie("opaque-session-token", tc.maxAge)

			if !c.Secure {
				t.Errorf("Secure = false, want true (REQ-SEC-012)")
			}
			if !c.HttpOnly {
				t.Errorf("HttpOnly = false, want true (REQ-SEC-012)")
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Errorf("SameSite = %v, want SameSiteLaxMode (REQ-SEC-012)", c.SameSite)
			}
			if c.Name != SessionCookieName {
				t.Errorf("Name = %q, want %q", c.Name, SessionCookieName)
			}
			if c.Path != "/" {
				t.Errorf("Path = %q, want \"/\"", c.Path)
			}
		})
	}
}

// TestSessionCookieMaxAge confirms the max-age semantics (positive sets a
// lifetime; <= 0 leaves it a session cookie).
func TestSessionCookieMaxAge(t *testing.T) {
	t.Parallel()

	if got := NewSessionCookie("v", 3600); got.MaxAge != 3600 {
		t.Errorf("MaxAge = %d, want 3600", got.MaxAge)
	}
	if got := NewSessionCookie("v", 0); got.MaxAge != 0 {
		t.Errorf("MaxAge = %d, want 0 (session cookie)", got.MaxAge)
	}
	if got := NewSessionCookie("v", -1); got.MaxAge != 0 {
		t.Errorf("MaxAge = %d, want 0 for negative input", got.MaxAge)
	}
}
