package ssrf

import (
	"context"
	"net"
	"net/url"
	"testing"
)

func TestSSRFValidateScheme_HTTPAndHTTPS(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"http://example.com/", "https://example.com/"} {
		u, _ := url.Parse(raw)
		if err := ValidateScheme(u); err != nil {
			t.Errorf("scheme %q must be allowed: %v", raw, err)
		}
	}
}

func TestSSRFValidateScheme_BlockedSchemes(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"file:///etc/passwd", "ftp://x/y", "gopher://x", "data:text/plain,A"} {
		u, _ := url.Parse(raw)
		err := ValidateScheme(u)
		if err == nil {
			t.Errorf("scheme %q must be blocked", raw)
			continue
		}
		if ReasonOf(err) != ReasonScheme {
			t.Errorf("scheme %q: reason = %q, want %q", raw, ReasonOf(err), ReasonScheme)
		}
	}
}

func TestSSRFIsPrivateOrLoopback(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ip      string
		private bool
	}{
		{"127.0.0.1", true},
		{"127.255.255.255", true},
		{"::1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.1.1", true},
		{"fc00::1", true},
		{"fe80::1", true},
		{"100.64.0.1", true},
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
		if got := IsPrivateOrLoopback(ip); got != c.private {
			t.Errorf("IsPrivateOrLoopback(%s) = %v, want %v", c.ip, got, c.private)
		}
	}
}

func TestSSRFValidateHost_AllowPrivateBypass(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("http://127.0.0.1:8080/x")
	// Options-level override.
	if err := ValidateHost(context.Background(), u, Options{AllowPrivateNetworks: true}, FetchOptions{}); err != nil {
		t.Errorf("Options.AllowPrivateNetworks must bypass: %v", err)
	}
	// Per-call FetchOptions override (REQ-SEC-007).
	if err := ValidateHost(context.Background(), u, Options{}, FetchOptions{AllowPrivateNetworks: true}); err != nil {
		t.Errorf("FetchOptions.AllowPrivateNetworks must bypass: %v", err)
	}
}

func TestSSRFValidateHost_PrivateIPBlocked(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("http://10.0.0.1/admin")
	err := ValidateHost(context.Background(), u, Options{}, FetchOptions{})
	if err == nil {
		t.Fatal("private IP host must be blocked")
	}
	if ReasonOf(err) != ReasonPrivateIP {
		t.Errorf("reason = %q, want %q", ReasonOf(err), ReasonPrivateIP)
	}
}

func TestSSRFValidateRedirect_HopCap(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("https://example.com/next")
	opts := Options{AllowPrivateNetworks: true, RedirectMaxHops: 5}
	fopts := FetchOptions{AllowPrivateNetworks: true}
	if err := ValidateRedirect(u, opts, fopts, 6); err == nil {
		t.Fatal("hop 6 over limit 5 must block")
	} else if ReasonOf(err) != ReasonRedirectHop {
		t.Errorf("reason = %q, want %q", ReasonOf(err), ReasonRedirectHop)
	}
	if err := ValidateRedirect(u, opts, fopts, 1); err != nil {
		t.Errorf("hop 1 within cap must pass: %v", err)
	}
}

func TestSSRFValidateRedirect_DefaultHopCapIsFive(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("https://example.com/next")
	opts := Options{AllowPrivateNetworks: true} // RedirectMaxHops unset
	fopts := FetchOptions{AllowPrivateNetworks: true}
	if err := ValidateRedirect(u, opts, fopts, 5); err != nil {
		t.Errorf("hop 5 within default cap 5 must pass: %v", err)
	}
	if err := ValidateRedirect(u, opts, fopts, 6); err == nil {
		t.Error("hop 6 over default cap must block")
	}
}

func TestSSRFValidateRedirect_BlockedScheme(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("file:///etc/passwd")
	if err := ValidateRedirect(u, Options{}, FetchOptions{}, 0); err == nil {
		t.Error("redirect to file:// must be blocked")
	}
}

func TestSSRFPinnedIPDialer_AllowPrivateReturnsPlainDialer(t *testing.T) {
	t.Parallel()
	dialFn, err := PinnedIPDialer(context.Background(), "127.0.0.1", Options{AllowPrivateNetworks: true}, FetchOptions{})
	if err != nil {
		t.Fatalf("PinnedIPDialer must succeed in test mode: %v", err)
	}
	if dialFn == nil {
		t.Fatal("dial function must not be nil")
	}
}

func TestSSRFPinnedIPDialer_FetchOptionsAllowPrivate(t *testing.T) {
	t.Parallel()
	dialFn, err := PinnedIPDialer(context.Background(), "localhost", Options{}, FetchOptions{AllowPrivateNetworks: true})
	if err != nil {
		t.Fatalf("PinnedIPDialer with FetchOptions.AllowPrivate: %v", err)
	}
	if dialFn == nil {
		t.Fatal("dial function must not be nil")
	}
}
