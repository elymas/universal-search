package reddit

// Coverage for the SSRF redirect guard (redirectAllowlist) and the cross-domain
// redirect error classifier (isCrossDomainRedirectErr).
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
	t.Run("allowed host within hop limit passes", func(t *testing.T) {
		req := mustReq(t, "https://old.reddit.com/r/golang")
		if err := redirectAllowlist(req, nil); err != nil {
			t.Errorf("allowed host rejected: %v", err)
		}
	})

	t.Run("disallowed host is rejected", func(t *testing.T) {
		req := mustReq(t, "https://evil.example.com/")
		err := redirectAllowlist(req, nil)
		if err == nil {
			t.Fatal("expected cross-domain rejection, got nil")
		}
		if !isCrossDomainRedirectErr(err) {
			t.Errorf("error %v should be classified as cross-domain", err)
		}
	})

	t.Run("too many hops is rejected", func(t *testing.T) {
		req := mustReq(t, "https://www.reddit.com/")
		via := []*http.Request{{}, {}, {}}
		if err := redirectAllowlist(req, via); err == nil {
			t.Fatal("expected too-many-redirects error, got nil")
		}
	})
}

func TestIsCrossDomainRedirectErr(t *testing.T) {
	if isCrossDomainRedirectErr(nil) {
		t.Error("nil error must not be classified as cross-domain")
	}
	if isCrossDomainRedirectErr(errors.New("some other failure")) {
		t.Error("unrelated error must not be classified as cross-domain")
	}
	if !isCrossDomainRedirectErr(errors.New("reddit: cross-domain redirect rejected: x")) {
		t.Error("matching error must be classified as cross-domain")
	}
}
