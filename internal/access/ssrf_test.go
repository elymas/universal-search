// Package access — unit tests for SSRF guards.
//
// REQ-CACHE-013: Scheme allowlist, private IP deny-list, DNS resolution guards.
package access

import (
	"context"
	"net"
	"net/url"
	"testing"
)

func TestValidateScheme_HTTP(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("http://example.com/path")
	if err := validateScheme(u); err != nil {
		t.Errorf("http scheme must be allowed: %v", err)
	}
}

func TestValidateScheme_HTTPS(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("https://example.com/path")
	if err := validateScheme(u); err != nil {
		t.Errorf("https scheme must be allowed: %v", err)
	}
}

func TestValidateScheme_File_Blocked(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("file:///etc/passwd")
	if err := validateScheme(u); err == nil {
		t.Error("file scheme must be blocked")
	}
}

func TestValidateScheme_FTP_Blocked(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("ftp://ftp.example.com/file")
	if err := validateScheme(u); err == nil {
		t.Error("ftp scheme must be blocked")
	}
}

func TestValidateScheme_Javascript_Blocked(t *testing.T) {
	t.Parallel()
	u := &url.URL{Scheme: "javascript", Host: "evil", Path: "/"}
	if err := validateScheme(u); err == nil {
		t.Error("javascript scheme must be blocked")
	}
}

func TestValidateScheme_Empty_Blocked(t *testing.T) {
	t.Parallel()
	u := &url.URL{Scheme: "", Host: "example.com", Path: "/"}
	if err := validateScheme(u); err == nil {
		t.Error("empty scheme must be blocked")
	}
}

func TestIsPrivateOrLoopback_Loopback(t *testing.T) {
	t.Parallel()
	cases := []string{
		"127.0.0.1",
		"127.255.255.255",
		"::1",
	}
	for _, s := range cases {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("failed to parse IP: %s", s)
		}
		if !isPrivateOrLoopback(ip) {
			t.Errorf("isPrivateOrLoopback(%s) = false, want true", s)
		}
	}
}

func TestIsPrivateOrLoopback_RFC1918(t *testing.T) {
	t.Parallel()
	cases := []string{
		"10.0.0.1",
		"10.255.255.254",
		"172.16.0.1",
		"172.31.255.254",
		"192.168.1.1",
		"192.168.255.255",
	}
	for _, s := range cases {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("failed to parse IP: %s", s)
		}
		if !isPrivateOrLoopback(ip) {
			t.Errorf("isPrivateOrLoopback(%s) = false, want true", s)
		}
	}
}

func TestIsPrivateOrLoopback_LinkLocal(t *testing.T) {
	t.Parallel()
	ip := net.ParseIP("169.254.1.1")
	if !isPrivateOrLoopback(ip) {
		t.Error("link-local 169.254.1.1 must be private")
	}
}

func TestIsPrivateOrLoopback_IPv6_ULA(t *testing.T) {
	t.Parallel()
	ip := net.ParseIP("fc00::1")
	if !isPrivateOrLoopback(ip) {
		t.Error("IPv6 ULA fc00::1 must be private")
	}
}

func TestIsPrivateOrLoopback_IPv6_LinkLocal(t *testing.T) {
	t.Parallel()
	ip := net.ParseIP("fe80::1")
	if !isPrivateOrLoopback(ip) {
		t.Error("IPv6 link-local fe80::1 must be private")
	}
}

func TestIsPrivateOrLoopback_Public_False(t *testing.T) {
	t.Parallel()
	cases := []string{
		"1.1.1.1",
		"8.8.8.8",
		"203.0.113.1",
		"2001:db8::1", // documentation range — should be public
	}
	for _, s := range cases {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("failed to parse IP: %s", s)
		}
		if isPrivateOrLoopback(ip) {
			t.Errorf("isPrivateOrLoopback(%s) = true, want false", s)
		}
	}
}

func TestValidateHost_AllowPrivate_Bypasses(t *testing.T) {
	t.Parallel()
	// With AllowPrivateNetworks, localhost resolves but SSRF check is skipped.
	opts := Options{AllowPrivateNetworks: true}
	fopts := FetchOptions{}
	u, _ := url.Parse("http://127.0.0.1:8080/test")
	ctx := context.Background()
	if err := validateHost(ctx, u, opts, fopts); err != nil {
		t.Errorf("AllowPrivateNetworks should bypass SSRF guard: %v", err)
	}
}

func TestValidateHost_FetchOptions_AllowPrivate(t *testing.T) {
	t.Parallel()
	opts := Options{}
	fopts := FetchOptions{AllowPrivateNetworks: true}
	u, _ := url.Parse("http://localhost/test")
	ctx := context.Background()
	if err := validateHost(ctx, u, opts, fopts); err != nil {
		t.Errorf("FetchOptions.AllowPrivateNetworks should bypass SSRF guard: %v", err)
	}
}
