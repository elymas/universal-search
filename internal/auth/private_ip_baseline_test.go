// Package auth — DDD characterization baseline for the OIDC discovery
// private-IP SSRF guard.
//
// SPEC-SEC-001 T01 (DDD ANALYZE/PRESERVE): pins the current observable
// behavior of checkPrivateIP / isPrivateIP / validateIssuerURL BEFORE the
// SPEC-SEC-001 Phase 4 refactor that deduplicates the IP classification into
// internal/security/ssrf/. The auth package's external behavior — HTTPS
// enforcement plus private-IP rejection on the issuer URL — must remain
// identical after the refactor.
package auth

import (
	"strings"
	"testing"
)

func TestCharacterize_IsPrivateIP_Classification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ip      string
		private bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.1.2.3", true},
		{"172.16.5.5", true},
		{"192.168.0.1", true},
		{"169.254.0.1", true}, // link-local
		{"fe80::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"203.0.113.7", false},
		{"not-an-ip", false}, // unparseable → false (not private)
	}
	for _, c := range cases {
		if got := isPrivateIP(c.ip); got != c.private {
			t.Errorf("isPrivateIP(%q) = %v, want %v", c.ip, got, c.private)
		}
	}
}

func TestCharacterize_CheckPrivateIP_LiteralPrivateRejected(t *testing.T) {
	t.Parallel()
	// A literal private IP hostname is rejected with a descriptive error.
	if err := checkPrivateIP("10.0.0.1"); err == nil {
		t.Fatal("checkPrivateIP(10.0.0.1) must return an error")
	}
	// A literal public IP passes.
	if err := checkPrivateIP("8.8.8.8"); err != nil {
		t.Errorf("checkPrivateIP(8.8.8.8) must pass, got %v", err)
	}
	// Empty hostname passes (defensive no-op).
	if err := checkPrivateIP(""); err != nil {
		t.Errorf("checkPrivateIP(\"\") must pass, got %v", err)
	}
}

func TestCharacterize_ValidateIssuerURL_HTTPSEnforced(t *testing.T) {
	t.Parallel()
	// Non-https issuer is rejected even when private networks are allowed —
	// HTTPS enforcement is independent of the private-IP bypass.
	if err := validateIssuerURL("http://issuer.example.com", true); err == nil {
		t.Fatal("non-https issuer must be rejected regardless of allowPrivate")
	} else if !strings.Contains(err.Error(), "https") {
		t.Errorf("error should mention https scheme requirement, got %v", err)
	}
	// https public issuer passes.
	if err := validateIssuerURL("https://issuer.example.com", false); err != nil {
		t.Errorf("https public issuer must pass, got %v", err)
	}
}

func TestCharacterize_ValidateIssuerURL_PrivateBlockedUnlessAllowed(t *testing.T) {
	t.Parallel()
	// https + literal private IP host: blocked when allowPrivate=false,
	// allowed when allowPrivate=true.
	if err := validateIssuerURL("https://10.0.0.1", false); err == nil {
		t.Error("https private-IP issuer must be blocked when allowPrivate=false")
	}
	if err := validateIssuerURL("https://10.0.0.1", true); err != nil {
		t.Errorf("https private-IP issuer must pass when allowPrivate=true, got %v", err)
	}
}
