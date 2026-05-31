package ssrf

import (
	"context"
	"net/url"
	"testing"
)

// validateHostByName runs ValidateHost with the default (cloud-metadata)
// blocklist and private networks ALLOWED, so the test isolates the hostname
// blocklist layer from DNS resolution / private-IP checks.
func validateHostByName(t *testing.T, host string) error {
	t.Helper()
	u, err := url.Parse("http://" + host + "/latest/meta-data/")
	if err != nil {
		t.Fatalf("parse %q: %v", host, err)
	}
	return ValidateHost(context.Background(), u, Options{AllowPrivateNetworks: true}, FetchOptions{})
}

func TestValidateHostBlocksGCPMetadata(t *testing.T) {
	t.Parallel()
	err := validateHostByName(t, "metadata.google.internal")
	if err == nil {
		t.Fatal("GCP metadata hostname must be blocked")
	}
	if ReasonOf(err) != ReasonHostnameBlocked {
		t.Errorf("reason = %q, want %q", ReasonOf(err), ReasonHostnameBlocked)
	}
}

func TestValidateHostBlocksAWSMetadata(t *testing.T) {
	t.Parallel()
	// AWS metadata hostname.
	if err := validateHostByName(t, "instance-data.ec2.internal"); err == nil {
		t.Error("AWS instance-data hostname must be blocked")
	}
	// AWS IMDS IP literal as host.
	if err := validateHostByName(t, "169.254.169.254"); err == nil {
		t.Error("AWS IMDS IP literal must be blocked by hostname blocklist")
	}
	// AWS IMDS IPv6 literal as host (bracketed per RFC 3986 host syntax).
	if err := validateHostByName(t, "[fd00:ec2::254]"); err == nil {
		t.Error("AWS IMDS IPv6 literal must be blocked by hostname blocklist")
	}
}

func TestValidateHostBlocksAzureMetadata(t *testing.T) {
	t.Parallel()
	if err := validateHostByName(t, "metadata.azure.com"); err == nil {
		t.Error("Azure metadata hostname must be blocked")
	}
}

func TestValidateHostCaseInsensitive(t *testing.T) {
	t.Parallel()
	for _, h := range []string{
		"METADATA.GOOGLE.INTERNAL",
		"Metadata.Google.Internal",
		"metadata.google.internal.", // trailing dot
	} {
		if err := validateHostByName(t, h); err == nil {
			t.Errorf("case/dot variant %q must be blocked", h)
		}
	}
}

func TestValidateHostSuffixMatch(t *testing.T) {
	t.Parallel()
	opts := Options{
		AllowPrivateNetworks: true,
		HostnameBlocklist:    []string{"*.metadata.internal"},
	}
	u, _ := url.Parse("http://foo.metadata.internal/")
	if err := ValidateHost(context.Background(), u, opts, FetchOptions{}); err == nil {
		t.Error("*.suffix pattern must block foo.metadata.internal")
	}
	// The bare suffix itself also matches.
	u2, _ := url.Parse("http://metadata.internal/")
	if err := ValidateHost(context.Background(), u2, opts, FetchOptions{}); err == nil {
		t.Error("*.suffix pattern must block the bare suffix metadata.internal")
	}
	// A non-matching host passes the hostname layer (private allowed).
	u3, _ := url.Parse("http://example.com/")
	if err := ValidateHost(context.Background(), u3, opts, FetchOptions{}); err != nil {
		t.Errorf("non-matching host must pass: %v", err)
	}
}

func TestValidateHostDefaultBlocklistAllowsNormalHost(t *testing.T) {
	t.Parallel()
	// A normal public hostname must NOT be blocked by the default blocklist
	// (with private networks allowed to skip DNS).
	if err := validateHostByName(t, "example.com"); err != nil {
		t.Errorf("normal host must pass default blocklist: %v", err)
	}
}
