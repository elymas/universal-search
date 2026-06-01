package searxng

// Coverage for the SSRF redirect guard and cross-domain error classifier.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func mustReq(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse %q: %v", rawURL, err)
	}
	return &http.Request{URL: u}
}

func TestRedirectAllowlist(t *testing.T) {
	t.Run("allowed host passes", func(t *testing.T) {
		if err := redirectAllowlist(mustReq(t, "http://localhost:8888/search"), nil); err != nil {
			t.Errorf("allowed host rejected: %v", err)
		}
	})
	t.Run("disallowed host rejected", func(t *testing.T) {
		err := redirectAllowlist(mustReq(t, "http://evil.example.com/"), nil)
		if err == nil || !isCrossDomainRedirectErr(err) {
			t.Errorf("expected cross-domain rejection, got %v", err)
		}
	})
	t.Run("too many hops rejected", func(t *testing.T) {
		via := []*http.Request{{}, {}, {}}
		if err := redirectAllowlist(mustReq(t, "http://localhost/"), via); err == nil {
			t.Fatal("expected too-many-redirects error")
		}
	})
}

func TestIsCrossDomainRedirectErr(t *testing.T) {
	if isCrossDomainRedirectErr(nil) {
		t.Error("nil must be false")
	}
	if isCrossDomainRedirectErr(errors.New("unrelated")) {
		t.Error("unrelated error must be false")
	}
	if !isCrossDomainRedirectErr(errors.New("searxng: cross-domain redirect rejected: x")) {
		t.Error("matching error must be true")
	}
}
