// Package access — unit tests for validateRedirect SSRF protection.
//
// REQ-CACHE-013: Redirects re-validated against SSRF guard, hop cap enforced.
package access

import (
	"net/url"
	"testing"
)

func TestValidateRedirect_HTTPSTarget_Allowed(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("https://example.com/redirect-target")
	opts := Options{AllowPrivateNetworks: true}
	fopts := FetchOptions{AllowPrivateNetworks: true}
	if err := validateRedirect(u, opts, fopts, 0); err != nil {
		t.Errorf("HTTPS redirect must be allowed: %v", err)
	}
}

func TestValidateRedirect_FileScheme_Blocked(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("file:///etc/passwd")
	opts := Options{}
	fopts := FetchOptions{}
	if err := validateRedirect(u, opts, fopts, 0); err == nil {
		t.Error("redirect to file:// must be blocked")
	}
}

func TestValidateRedirect_FTPScheme_Blocked(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("ftp://attacker.com/evil")
	opts := Options{}
	fopts := FetchOptions{}
	if err := validateRedirect(u, opts, fopts, 0); err == nil {
		t.Error("redirect to ftp:// must be blocked")
	}
}

func TestValidateRedirect_PrivateIP_Blocked(t *testing.T) {
	t.Parallel()
	u, _ := url.Parse("http://192.168.1.1/admin")
	opts := Options{} // AllowPrivateNetworks = false
	fopts := FetchOptions{}
	if err := validateRedirect(u, opts, fopts, 0); err == nil {
		t.Error("redirect to private IP must be blocked when AllowPrivateNetworks=false")
	}
}

func TestValidateRedirect_PrivateIP_AllowedInTestMode(t *testing.T) {
	t.Parallel()
	// Even 192.168.x.x is allowed when AllowPrivateNetworks=true (test mode).
	u, _ := url.Parse("http://192.168.1.1/page")
	opts := Options{AllowPrivateNetworks: true}
	fopts := FetchOptions{}
	if err := validateRedirect(u, opts, fopts, 0); err != nil {
		t.Errorf("redirect to private IP should be allowed in test mode: %v", err)
	}
}
