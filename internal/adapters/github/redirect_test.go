package github

// Coverage for the SSRF redirect guard (redirectAllowlist).
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"net/http"
	"net/url"
	"testing"
)

func ghReq(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %q: %v", rawURL, err)
	}
	return &http.Request{URL: u}
}

func TestRedirectAllowlist(t *testing.T) {
	t.Run("allowed host passes", func(t *testing.T) {
		if err := redirectAllowlist(ghReq(t, "https://api.github.com/repos"), nil); err != nil {
			t.Errorf("allowed host rejected: %v", err)
		}
	})
	t.Run("disallowed host rejected", func(t *testing.T) {
		if err := redirectAllowlist(ghReq(t, "https://evil.example.com/"), nil); err == nil {
			t.Fatal("expected cross-domain rejection, got nil")
		}
	})
	t.Run("too many hops rejected", func(t *testing.T) {
		via := []*http.Request{{}, {}, {}}
		if err := redirectAllowlist(ghReq(t, "https://github.com/"), via); err == nil {
			t.Fatal("expected too-many-redirects error")
		}
	})
}
