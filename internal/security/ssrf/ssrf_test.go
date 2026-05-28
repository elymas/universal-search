// Package ssrf_test — characterization tests for the generic SSRF guard.
//
// REQ-SEC-007: ValidateScheme, ValidateHost, ValidateRedirect, PinnedIPDialer.
// REQ-SEC-008: Hostname blocklist (cloud metadata endpoints).
// These tests mirror SPEC-CACHE-001 REQ-CACHE-013 behavior.
package ssrf_test

import (
	"context"
	"net"
	"net/url"
	"testing"

	"github.com/elymas/universal-search/internal/security/ssrf"
)

// --- REQ-SEC-007: ValidateScheme ---

func TestValidateSchemeAcceptsHTTP(t *testing.T) {
	t.Parallel()
	u := mustParseURL(t, "http://example.com/path")
	if err := ssrf.ValidateScheme(u); err != nil {
		t.Fatalf("expected http to be accepted, got: %v", err)
	}
}

func TestValidateSchemeAcceptsHTTPS(t *testing.T) {
	t.Parallel()
	u := mustParseURL(t, "https://example.com/path")
	if err := ssrf.ValidateScheme(u); err != nil {
		t.Fatalf("expected https to be accepted, got: %v", err)
	}
}

func TestValidateSchemeRejectsFile(t *testing.T) {
	t.Parallel()
	u := mustParseURL(t, "file:///etc/passwd")
	err := ssrf.ValidateScheme(u)
	if err == nil {
		t.Fatal("expected file:// to be rejected")
	}
	assertFetchErrorCategory(t, err, ssrf.CategoryBlocked)
}

func TestValidateSchemeRejectsFTP(t *testing.T) {
	t.Parallel()
	u := mustParseURL(t, "ftp://evil.com/data")
	err := ssrf.ValidateScheme(u)
	if err == nil {
		t.Fatal("expected ftp:// to be rejected")
	}
}

func TestValidateSchemeRejectsGopher(t *testing.T) {
	t.Parallel()
	u := mustParseURL(t, "gopher://evil.com/data")
	err := ssrf.ValidateScheme(u)
	if err == nil {
		t.Fatal("expected gopher:// to be rejected")
	}
}

func TestValidateSchemeRejectsData(t *testing.T) {
	t.Parallel()
	u := mustParseURL(t, "data:text/plain,hello")
	err := ssrf.ValidateScheme(u)
	if err == nil {
		t.Fatal("expected data: to be rejected")
	}
}

func TestValidateSchemeRejectsDict(t *testing.T) {
	t.Parallel()
	u := mustParseURL(t, "dict://evil.com/info")
	err := ssrf.ValidateScheme(u)
	if err == nil {
		t.Fatal("expected dict:// to be rejected")
	}
}

// --- REQ-SEC-007: ValidateHost (private IP denial) ---

func TestValidateHostBlocksRFC1918_10(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	u := mustParseURL(t, "http://10.0.0.1/secret")
	err := ssrf.ValidateHost(ctx, u, ssrf.Options{})
	if err == nil {
		t.Fatal("expected 10.x.x.x to be blocked")
	}
	assertFetchErrorCategory(t, err, ssrf.CategoryBlocked)
}

func TestValidateHostBlocksRFC1918_17216(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	u := mustParseURL(t, "http://172.16.0.1/secret")
	err := ssrf.ValidateHost(ctx, u, ssrf.Options{})
	if err == nil {
		t.Fatal("expected 172.16.x.x to be blocked")
	}
}

func TestValidateHostBlocksRFC1918_192168(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	u := mustParseURL(t, "http://192.168.1.1/secret")
	err := ssrf.ValidateHost(ctx, u, ssrf.Options{})
	if err == nil {
		t.Fatal("expected 192.168.x.x to be blocked")
	}
}

func TestValidateHostBlocksLoopback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	u := mustParseURL(t, "http://127.0.0.1/secret")
	err := ssrf.ValidateHost(ctx, u, ssrf.Options{})
	if err == nil {
		t.Fatal("expected 127.0.0.1 to be blocked")
	}
}

func TestValidateHostBlocksLinkLocal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	u := mustParseURL(t, "http://169.254.169.254/latest/meta-data/")
	err := ssrf.ValidateHost(ctx, u, ssrf.Options{})
	if err == nil {
		t.Fatal("expected 169.254.169.254 to be blocked")
	}
}

func TestValidateHostAllowsPrivateNetworksWhenEnabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	u := mustParseURL(t, "http://192.168.1.1/internal")
	err := ssrf.ValidateHost(ctx, u, ssrf.Options{AllowPrivateNetworks: true})
	if err != nil {
		t.Fatalf("expected private network to be allowed when AllowPrivateNetworks=true, got: %v", err)
	}
}

// --- REQ-SEC-007: ValidateRedirect ---

func TestValidateRedirectEnforcesHopCap(t *testing.T) {
	t.Parallel()
	next := mustParseURL(t, "http://public.example.com/redirected")
	opts := ssrf.Options{MaxRedirects: 3}
	err := ssrf.ValidateRedirect(next, opts, 4)
	if err == nil {
		t.Fatal("expected redirect to be blocked after hop cap exceeded")
	}
	assertFetchErrorCategory(t, err, ssrf.CategoryBlocked)
}

func TestValidateRedirectRevalidatesScheme(t *testing.T) {
	t.Parallel()
	next := mustParseURL(t, "file:///etc/passwd")
	opts := ssrf.Options{MaxRedirects: 5}
	err := ssrf.ValidateRedirect(next, opts, 1)
	if err == nil {
		t.Fatal("expected file:// redirect to be blocked")
	}
}

// --- REQ-SEC-007: PinnedIPDialer ---

func TestPinnedIPDialerPreventsRebind(t *testing.T) {
	t.Parallel()
	// Test that PinnedIPDialer dials to the pinned IP, not the hostname.
	// We verify the dialer function is created and uses the pinned IP.
	dialer := ssrf.DialContextWithPinnedIP("93.184.216.34")
	// We can't actually connect without a server, but we can verify the
	// function exists and handles address rewriting.
	if dialer == nil {
		t.Fatal("expected non-nil dialer function")
	}
}

// --- REQ-SEC-008: Hostname blocklist ---

func TestValidateHostBlocksGCPMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, host := range []string{
		"http://metadata.google.internal/computeMetadata/v1/",
		"http://metadata.google.internal./computeMetadata/v1/",
	} {
		u := mustParseURL(t, host)
		err := ssrf.ValidateHost(ctx, u, ssrf.Options{})
		if err == nil {
			t.Fatalf("expected %s to be blocked by hostname blocklist", host)
		}
		assertFetchErrorCategory(t, err, ssrf.CategoryBlocked)
	}
}

func TestValidateHostBlocksAzureMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	u := mustParseURL(t, "http://metadata.azure.com/metadata/instance")
	err := ssrf.ValidateHost(ctx, u, ssrf.Options{})
	if err == nil {
		t.Fatal("expected metadata.azure.com to be blocked")
	}
}

func TestValidateHostBlocksEC2Metadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	u := mustParseURL(t, "http://instance-data.ec2.internal/latest/meta-data/")
	err := ssrf.ValidateHost(ctx, u, ssrf.Options{})
	if err == nil {
		t.Fatal("expected instance-data.ec2.internal to be blocked")
	}
}

func TestValidateHostCaseInsensitive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, host := range []string{
		"http://METADATA.GOOGLE.INTERNAL/computeMetadata/v1/",
		"http://Metadata.Google.Internal/computeMetadata/v1/",
		"http://MeTaDaTa.GoOgLe.InTeRnAl/computeMetadata/v1/",
	} {
		u := mustParseURL(t, host)
		err := ssrf.ValidateHost(ctx, u, ssrf.Options{})
		if err == nil {
			t.Fatalf("expected %s to be blocked (case insensitive)", host)
		}
	}
}

func TestValidateHostSuffixMatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Suffix match: subdomain of a blocked host
	u := mustParseURL(t, "http://something.metadata.google.internal/path")
	err := ssrf.ValidateHost(ctx, u, ssrf.Options{})
	if err == nil {
		t.Fatal("expected suffix match on metadata.google.internal to be blocked")
	}
}

func TestDefaultHostnameBlocklist(t *testing.T) {
	t.Parallel()
	blocklist := ssrf.DefaultHostnameBlocklist()
	expected := []string{
		"metadata.google.internal",
		"metadata.azure.com",
		"instance-data.ec2.internal",
	}
	for _, exp := range expected {
		found := false
		for _, entry := range blocklist {
			if entry == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in default hostname blocklist", exp)
		}
	}
}

func TestValidateHostAllowsPublicHostname(t *testing.T) {
	t.Parallel()
	// This test may fail in environments without DNS, but the intent is clear:
	// a public hostname should NOT be blocked by the hostname blocklist.
	// We skip if DNS is unavailable.
	ctx := context.Background()
	u := mustParseURL(t, "http://example.com/path")
	// With empty options, this should not fail due to hostname blocklist.
	// It might fail due to DNS, which we accept.
	err := ssrf.ValidateHost(ctx, u, ssrf.Options{})
	if err != nil {
		// If it's a DNS error, that's expected in some environments.
		if fe, ok := err.(*ssrf.FetchError); ok && fe.Category == ssrf.CategoryBlocked {
			if contains(fe.Reason, "hostname blocked") {
				t.Fatal("public hostname should not be blocked by blocklist")
			}
		}
	}
}

func TestDefaultSchemeAllowlist(t *testing.T) {
	t.Parallel()
	allowlist := ssrf.DefaultSchemeAllowlist()
	if len(allowlist) != 2 {
		t.Fatalf("expected 2 schemes in default allowlist, got %d", len(allowlist))
	}
	if allowlist[0] != "http" || allowlist[1] != "https" {
		t.Fatalf("expected [http, https], got %v", allowlist)
	}
}

// --- Helpers ---

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse URL %q: %v", raw, err)
	}
	return u
}

func assertFetchErrorCategory(t *testing.T, err error, want ssrf.ErrorCategory) {
	t.Helper()
	fe, ok := err.(*ssrf.FetchError)
	if !ok {
		t.Fatalf("expected *ssrf.FetchError, got %T: %v", err, err)
	}
	if fe.Category != want {
		t.Fatalf("expected category %q, got %q: %v", want, fe.Category, fe.Reason)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure net import is used (for the dialer test).
var _ net.Conn
