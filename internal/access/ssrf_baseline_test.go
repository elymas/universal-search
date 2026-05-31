// Package access — DDD characterization baseline for the SSRF guards.
//
// SPEC-SEC-001 T01 (DDD ANALYZE/PRESERVE): these tests PIN the *current*
// observable behavior of the unexported SSRF guards (validateScheme,
// validateHost, validateRedirect, isPrivateOrLoopback) and the pinned-IP
// dialer BEFORE the SPEC-SEC-001 Phase 4 extraction into
// internal/security/ssrf/. They must stay byte-identical green both against
// the unchanged CACHE-001 code and after the access package is refactored to
// delegate to the new generic package.
//
// Unlike ssrf_test.go (which is the CACHE-001 unit suite), these tests assert
// the FetchError.Category surfaced to cascade callers — the contract that the
// extraction must preserve, not just err != nil.
package access

import (
	"context"
	"errors"
	"net"
	"net/url"
	"testing"
)

// asFetchError extracts the *FetchError from an error, failing the test if the
// returned error is not a *FetchError. This pins the error-type contract that
// cascade.go / phase3 / phase4 callers depend on.
func asFetchError(t *testing.T, err error) *FetchError {
	t.Helper()
	var fe *FetchError
	if !errors.As(err, &fe) {
		t.Fatalf("expected *FetchError, got %T: %v", err, err)
	}
	return fe
}

func TestCharacterize_ValidateScheme_BlockedCategory(t *testing.T) {
	t.Parallel()
	// Non-http(s) schemes block with CategoryBlocked. Pins the observable
	// category, not just non-nil.
	blocked := []string{
		"file:///etc/passwd",
		"ftp://ftp.example.com/x",
		"gopher://evil/x",
		"data:text/plain;base64,QQ==",
	}
	for _, raw := range blocked {
		u, _ := url.Parse(raw)
		err := validateScheme(u)
		if err == nil {
			t.Fatalf("scheme %q must be blocked", raw)
		}
		if fe := asFetchError(t, err); fe.Category != CategoryBlocked {
			t.Errorf("scheme %q: category = %q, want %q", raw, fe.Category, CategoryBlocked)
		}
	}
}

func TestCharacterize_ValidateScheme_AllowsHTTPAndHTTPS(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"http://example.com/", "https://example.com/"} {
		u, _ := url.Parse(raw)
		if err := validateScheme(u); err != nil {
			t.Errorf("scheme %q must be allowed, got %v", raw, err)
		}
	}
}

func TestCharacterize_ValidateHost_PrivateIPBlockedCategory(t *testing.T) {
	t.Parallel()
	// A literal private IP host blocks with CategoryBlocked when private
	// networks are not allowed. Resolution of an IP literal is deterministic
	// and offline-safe.
	u, _ := url.Parse("http://10.0.0.1/admin")
	err := validateHost(context.Background(), u, Options{}, FetchOptions{})
	if err == nil {
		t.Fatal("private IP host must be blocked")
	}
	if fe := asFetchError(t, err); fe.Category != CategoryBlocked {
		t.Errorf("category = %q, want %q", fe.Category, CategoryBlocked)
	}
}

func TestCharacterize_ValidateHost_AllowPrivateBypass(t *testing.T) {
	t.Parallel()
	// Both the fetcher-level Options override and the per-call FetchOptions
	// override skip the guard. This is the exact dual-override semantics the
	// extraction MUST preserve (REQ-SEC-007).
	u, _ := url.Parse("http://127.0.0.1:8080/x")
	if err := validateHost(context.Background(), u, Options{AllowPrivateNetworks: true}, FetchOptions{}); err != nil {
		t.Errorf("Options.AllowPrivateNetworks must bypass: %v", err)
	}
	if err := validateHost(context.Background(), u, Options{}, FetchOptions{AllowPrivateNetworks: true}); err != nil {
		t.Errorf("FetchOptions.AllowPrivateNetworks must bypass: %v", err)
	}
}

func TestCharacterize_ValidateRedirect_HopCapBlocks(t *testing.T) {
	t.Parallel()
	// Exceeding RedirectMaxHops blocks with CategoryBlocked. Use the test-mode
	// bypass so the host check is skipped and only the hop cap is exercised.
	u, _ := url.Parse("https://example.com/next")
	opts := Options{AllowPrivateNetworks: true, RedirectMaxHops: 5}
	fopts := FetchOptions{AllowPrivateNetworks: true}
	if err := validateRedirect(u, opts, fopts, 6); err == nil {
		t.Fatal("hop count 6 over limit 5 must block")
	} else if fe := asFetchError(t, err); fe.Category != CategoryBlocked {
		t.Errorf("category = %q, want %q", fe.Category, CategoryBlocked)
	}
	// Within the cap, an allowed scheme + bypassed host passes.
	if err := validateRedirect(u, opts, fopts, 1); err != nil {
		t.Errorf("hop count 1 within cap must pass, got %v", err)
	}
}

func TestCharacterize_ValidateRedirect_DefaultHopCapIsFive(t *testing.T) {
	t.Parallel()
	// RedirectMaxHops == 0 falls back to the documented default of 5.
	u, _ := url.Parse("https://example.com/next")
	opts := Options{AllowPrivateNetworks: true} // RedirectMaxHops unset → default 5
	fopts := FetchOptions{AllowPrivateNetworks: true}
	if err := validateRedirect(u, opts, fopts, 5); err != nil {
		t.Errorf("hop 5 within default cap 5 must pass, got %v", err)
	}
	if err := validateRedirect(u, opts, fopts, 6); err == nil {
		t.Error("hop 6 over default cap 5 must block")
	}
}

func TestCharacterize_IsPrivateOrLoopback_Classification(t *testing.T) {
	t.Parallel()
	// Snapshot of the exact private/public classification boundary. This
	// table is the canonical behavior the extracted predicate must reproduce.
	cases := []struct {
		ip      string
		private bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true}, // AWS/GCP/Azure metadata link-local
		{"fc00::1", true},
		{"fe80::1", true},
		{"100.64.0.1", true}, // RFC 6598 shared address space
		{"1.1.1.1", false},
		{"8.8.8.8", false},
		{"203.0.113.1", false},
		{"2001:db8::1", false},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("unparseable IP %q", c.ip)
		}
		if got := isPrivateOrLoopback(ip); got != c.private {
			t.Errorf("isPrivateOrLoopback(%s) = %v, want %v", c.ip, got, c.private)
		}
	}
}

func TestCharacterize_PinnedDialContext_AllowPrivateReturnsPlainDialer(t *testing.T) {
	t.Parallel()
	// In test mode no DNS resolution occurs and a usable dialer is returned.
	dialFn, err := pinnedDialContext(context.Background(), "127.0.0.1", Options{AllowPrivateNetworks: true}, FetchOptions{})
	if err != nil {
		t.Fatalf("pinnedDialContext must succeed in test mode: %v", err)
	}
	if dialFn == nil {
		t.Fatal("dial function must not be nil")
	}
}
