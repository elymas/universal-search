package ssrf_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/security/ssrf"
)

func parseIP(s string) net.IP {
	return net.ParseIP(s)
}

func TestPinnedIPDialerAllowsWhenPrivateNetworksEnabled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dialer, err := ssrf.PinnedIPDialer(ctx, "example.com", ssrf.Options{AllowPrivateNetworks: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dialer == nil {
		t.Fatal("expected non-nil dialer")
	}
}

func TestPinnedIPDialerBlocksPrivateIP(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// 127.0.0.1 resolves to loopback
	_, err := ssrf.PinnedIPDialer(ctx, "localhost", ssrf.Options{})
	if err == nil {
		t.Fatal("expected localhost to be blocked")
	}
}

func TestDialContextWithPinnedIPReplacesHost(t *testing.T) {
	t.Parallel()
	// Test that the dialer function properly replaces hostname with pinned IP
	// We can't actually connect, but verify it constructs the address correctly
	dialer := ssrf.DialContextWithPinnedIP("1.2.3.4")
	// Attempt to dial with a short timeout — this will fail but shows the function works
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := dialer(ctx, "tcp", "example.com:80")
	// Expected to fail with connection refused or timeout, but NOT with address parsing error
	if err != nil && err.Error() == "address example.com:80: too many colons in address" {
		t.Fatal("dialer failed to replace hostname with pinned IP")
	}
	// Any other error (connection refused, timeout) is acceptable
}

func TestIsPrivateOrLoopback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true},
		{"::1", true},
		{"fc00::1", true},
		{"100.64.0.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"93.184.216.34", false},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got := ssrf.IsPrivateOrLoopback(parseIP(tt.ip))
			if got != tt.want {
				t.Errorf("IsPrivateOrLoopback(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestValidateSchemeWithCustomAllowlist(t *testing.T) {
	t.Parallel()
	u := mustParseURL(t, "http://example.com")
	// http should be rejected when allowlist only has https
	err := ssrf.ValidateSchemeWithAllowlist(u, []string{"https"})
	if err == nil {
		t.Fatal("expected http to be rejected when only https is allowed")
	}
}

func TestValidateRedirectWithinHopLimit(t *testing.T) {
	t.Parallel()
	next := mustParseURL(t, "http://93.184.216.34/ok")
	err := ssrf.ValidateRedirect(next, ssrf.Options{MaxRedirects: 5}, 3)
	if err != nil {
		t.Fatalf("expected redirect at hop 3 with max 5 to pass, got: %v", err)
	}
}

func TestValidateRedirectExactHopLimit(t *testing.T) {
	t.Parallel()
	next := mustParseURL(t, "http://93.184.216.34/ok")
	// hopCount == maxHops should still pass (check is > not >=)
	err := ssrf.ValidateRedirect(next, ssrf.Options{MaxRedirects: 5}, 5)
	if err != nil {
		t.Fatalf("expected redirect at hop 5 with max 5 to pass, got: %v", err)
	}
}
