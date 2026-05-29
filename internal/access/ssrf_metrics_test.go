package access

import (
	"context"
	"net/url"
	"testing"
)

// TestSSRFBlockHook_FiresWithReason verifies that an SSRF block invokes the
// observability hook with the classified reason (SPEC-SEC-001 REQ-SEC-009).
// The hook is reset after the test to avoid leaking state to other tests.
func TestSSRFBlockHook_FiresWithReason(t *testing.T) {
	// NOTE: not parallel — mutates the package-level ssrfBlockHook.
	var got []string
	SetSSRFBlockHook(func(reason string) { got = append(got, reason) })
	defer SetSSRFBlockHook(nil)

	// Blocked scheme → reason "scheme".
	u, _ := url.Parse("file:///etc/passwd")
	if err := validateScheme(u); err == nil {
		t.Fatal("file scheme must block")
	}

	// Blocked private IP → reason "private_ip".
	u2, _ := url.Parse("http://10.0.0.1/x")
	if err := validateHost(context.Background(), u2, Options{}, FetchOptions{}); err == nil {
		t.Fatal("private IP must block")
	}

	// Blocked cloud-metadata hostname → reason "hostname_allowlist".
	u3, _ := url.Parse("http://metadata.google.internal/")
	if err := validateHost(context.Background(), u3, Options{AllowPrivateNetworks: true}, FetchOptions{}); err == nil {
		t.Fatal("cloud metadata hostname must block")
	}

	want := map[string]bool{"scheme": true, "private_ip": true, "hostname_allowlist": true}
	for _, r := range got {
		delete(want, r)
	}
	if len(want) != 0 {
		t.Errorf("missing SSRF block reasons from hook: %v (got %v)", want, got)
	}
}

// TestSSRFBlockHook_NotFiredOnSuccess verifies the hook is silent when no block
// occurs.
func TestSSRFBlockHook_NotFiredOnSuccess(t *testing.T) {
	var fired bool
	SetSSRFBlockHook(func(string) { fired = true })
	defer SetSSRFBlockHook(nil)

	u, _ := url.Parse("https://example.com/")
	if err := validateScheme(u); err != nil {
		t.Fatalf("https must pass: %v", err)
	}
	// Allowed private (test mode) must not block or fire.
	u2, _ := url.Parse("http://127.0.0.1/x")
	if err := validateHost(context.Background(), u2, Options{AllowPrivateNetworks: true}, FetchOptions{}); err != nil {
		t.Fatalf("allow-private must pass: %v", err)
	}
	if fired {
		t.Error("hook must not fire when no SSRF block occurs")
	}
}
