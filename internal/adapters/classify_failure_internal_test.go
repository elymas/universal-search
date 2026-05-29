// Package adapters — white-box test for classifyFailure (SPEC-EVAL-002
// REQ-EVAL2-005). classifyFailure is unexported, so this test lives in package
// adapters (not adapters_test).
package adapters

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// TestClassifyFailure is a table-driven test for the 7-class failure taxonomy
// plus the nil case. REQ-EVAL2-005, AC-003, EC-003.
func TestClassifyFailure(t *testing.T) {
	t.Parallel()

	// Force a real json.SyntaxError via the standard library.
	var jsonSyntaxErr error
	if e := json.Unmarshal([]byte("{bad"), &struct{}{}); e != nil {
		jsonSyntaxErr = e
	}

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"5xx via SourceError", &types.SourceError{Adapter: "x", Category: types.CategoryTransient, HTTPStatus: 503, Cause: errors.New("bad gateway")}, "5xx"},
		{"4xx via SourceError", &types.SourceError{Adapter: "x", Category: types.CategoryPermanent, HTTPStatus: 404, Cause: errors.New("not found")}, "4xx"},
		{"dns", &net.DNSError{Err: "no such host", Name: "example.invalid", IsNotFound: true}, "dns"},
		{"tls record header", tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}, "tls"},
		{"tls x509 unknown authority", x509.UnknownAuthorityError{}, "tls"},
		{"parse json syntax", jsonSyntaxErr, "parse"},
		{"parse xml heuristic", errors.New("xml: syntax error on line 3"), "parse"},
		{"transcript heuristic", errors.New("yt-dlp: transcript unavailable for video"), "transcript"},
		{"unknown fallthrough", errors.New("totally novel error"), "unknown"},
		// Wrapped SourceError must still classify by HTTPStatus.
		{"wrapped 5xx", fmt.Errorf("call failed: %w", &types.SourceError{Adapter: "y", HTTPStatus: 500, Cause: errors.New("boom")}), "5xx"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyFailure(tc.err); got != tc.want {
				t.Errorf("classifyFailure(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// TestClassifyFailureWrappedDNS verifies wrapped DNS errors classify correctly.
func TestClassifyFailureWrappedDNS(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("dial failed: %w", &net.DNSError{Err: "server misbehaving", Name: "host"})
	if got := classifyFailure(wrapped); got != "dns" {
		t.Errorf("classifyFailure(wrapped dns) = %q, want dns", got)
	}
}

// TestClassifyHealth exercises the SPEC-EVAL-002 REQ-EVAL2-010 health threshold
// mapping directly, including the degraded band and the zero-call case.
func TestClassifyHealth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		success, fail int64
		want          string
	}{
		{"no calls -> healthy", 0, 0, "healthy"},
		{"1.0 -> healthy", 100, 0, "healthy"},
		{"0.95 boundary -> healthy", 95, 5, "healthy"},
		{"0.90 -> degraded", 90, 10, "degraded"},
		{"0.85 boundary -> degraded", 85, 15, "degraded"},
		{"0.84 -> unhealthy", 84, 16, "unhealthy"},
		{"0.0 -> unhealthy", 0, 10, "unhealthy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyHealth(adapterCallStats{success: tc.success, fail: tc.fail})
			if got != tc.want {
				t.Errorf("classifyHealth(%d/%d) = %q, want %q", tc.success, tc.fail, got, tc.want)
			}
		})
	}
}
