package ssrf

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestErrorImplementsError(t *testing.T) {
	t.Parallel()
	e := &Error{Reason: ReasonScheme, Message: "boom"}
	if e.Error() != "boom" {
		t.Errorf("Error() = %q, want boom", e.Error())
	}
}

func TestReasonOf_NonSSRFError(t *testing.T) {
	t.Parallel()
	if got := ReasonOf(errors.New("plain")); got != "" {
		t.Errorf("ReasonOf(plain) = %q, want empty", got)
	}
	if got := ReasonOf(nil); got != "" {
		t.Errorf("ReasonOf(nil) = %q, want empty", got)
	}
}

func TestReasonOf_WrappedSSRFError(t *testing.T) {
	t.Parallel()
	wrapped := errWrap{inner: &Error{Reason: ReasonPrivateIP, Message: "x"}}
	if got := ReasonOf(wrapped); got != ReasonPrivateIP {
		t.Errorf("ReasonOf(wrapped) = %q, want private_ip", got)
	}
}

type errWrap struct{ inner error }

func (w errWrap) Error() string { return "wrap: " + w.inner.Error() }
func (w errWrap) Unwrap() error { return w.inner }

func TestDialContextWithPinnedIP_ConnectsToRealServer(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	host := srv.URL[len("http://"):] // 127.0.0.1:PORT
	dialFn := DialContextWithPinnedIP("127.0.0.1")
	conn, err := dialFn(context.Background(), "tcp", host)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()
}

func TestDialContextWithPinnedIP_BadAddr(t *testing.T) {
	t.Parallel()
	dialFn := DialContextWithPinnedIP("127.0.0.1")
	if _, err := dialFn(context.Background(), "tcp", "no-port-here"); err == nil {
		t.Error("malformed addr without port must error")
	}
}

func TestPinnedIPDialer_ResolvesPublicHost(t *testing.T) {
	t.Parallel()
	// localhost resolves to loopback; with private NOT allowed the pinned IP is
	// private → blocked with reason private_ip. Exercises the resolution path.
	_, err := PinnedIPDialer(context.Background(), "localhost", Options{}, FetchOptions{})
	if err == nil {
		t.Fatal("localhost (loopback) must be blocked when private not allowed")
	}
	if ReasonOf(err) != ReasonPrivateIP {
		t.Errorf("reason = %q, want private_ip", ReasonOf(err))
	}
}

func TestValidateRedirect_CustomSchemeAllowlist(t *testing.T) {
	t.Parallel()
	// Custom allowlist that excludes http → an http redirect target blocks.
	opts := Options{AllowPrivateNetworks: true, SchemeAllowlist: []string{"https"}}
	u, _ := url.Parse("http://example.com/x")
	if err := ValidateRedirect(u, opts, FetchOptions{AllowPrivateNetworks: true}, 0); err == nil {
		t.Error("http redirect must block under https-only allowlist")
	}
	// https passes the same allowlist.
	u2, _ := url.Parse("https://example.com/x")
	if err := ValidateRedirect(u2, opts, FetchOptions{AllowPrivateNetworks: true}, 0); err != nil {
		t.Errorf("https redirect must pass under https-only allowlist: %v", err)
	}
}

func TestValidateHost_DNSFailureBlocks(t *testing.T) {
	t.Parallel()
	// An unresolvable hostname blocks fail-closed (reason private_ip).
	u, _ := url.Parse("http://nonexistent.invalid.example.test./")
	err := ValidateHost(context.Background(), u, Options{}, FetchOptions{})
	if err == nil {
		t.Fatal("unresolvable host must block fail-closed")
	}
	if ReasonOf(err) != ReasonPrivateIP {
		t.Errorf("reason = %q, want private_ip", ReasonOf(err))
	}
}

func TestReasonDNSRebindConstant(t *testing.T) {
	t.Parallel()
	// The dns_rebind reason is part of the bounded label set even though the
	// pinned dialer surfaces rebind attempts as private_ip blocks at connect
	// time; assert the constant value for the metric allowlist contract.
	if ReasonDNSRebind != "dns_rebind" {
		t.Errorf("ReasonDNSRebind = %q, want dns_rebind", ReasonDNSRebind)
	}
}
