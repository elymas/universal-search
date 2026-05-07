// Package naver — HTTP client construction and request helpers.
// REQ-ADP8-003/004/009/010: HTTP client with timeout, redirect allowlist,
// reqid transport, auth headers, and status-to-Category mapping.
package naver

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/pkg/types"
)

// allowedRedirectHosts is the SSRF-guard allowlist for Naver redirect targets.
// Only openapi.naver.com may be followed during CheckRedirect (REQ-ADP8-010).
//
// @MX:NOTE: [AUTO] 1-entry security boundary. Adding a host requires a security
// review — a permissive allowlist re-opens SSRF via Naver CDN redirect infrastructure.
// @MX:SPEC: SPEC-ADP-008
var allowedRedirectHosts = map[string]struct{}{
	"openapi.naver.com": {},
}

// redirectAllowlist is the CheckRedirect policy for the Naver HTTP client.
// It enforces:
//   - Maximum 3 redirect hops (REQ-ADP8-010).
//   - All redirect targets must be in allowedRedirectHosts (SSRF guard).
func redirectAllowlist(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return errors.New("naver: too many redirects (max 3)")
	}
	host := req.URL.Hostname()
	if _, ok := allowedRedirectHosts[host]; !ok {
		return fmt.Errorf("naver: cross-domain redirect rejected: %s", host)
	}
	return nil
}

// newDefaultClient constructs the default HTTP client for the Naver adapter:
//   - 10-second request timeout
//   - redirectAllowlist CheckRedirect policy (SSRF guard, max 3 hops)
//   - reqid.NewTransport wrapping http.DefaultTransport for request-ID propagation
func newDefaultClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		Transport:     reqid.NewTransport(http.DefaultTransport),
		CheckRedirect: redirectAllowlist,
	}
}

// doRequest executes the given HTTP request via the adapter's HTTP client,
// setting the required Naver API auth headers:
//   - X-Naver-Client-Id: a.clientID
//   - X-Naver-Client-Secret: a.clientSecret
//
// The caller is responsible for closing the response body.
//
// @MX:WARN: [AUTO] Outbound network call with auth credentials in headers.
// The CheckRedirect policy on a.httpClient enforces the SSRF guard; do not
// bypass or replace the client without re-applying the allowlist.
// @MX:REASON: removing CheckRedirect re-opens SSRF via Naver CDN redirects;
// credentials exposed in headers must only travel to allowlisted hosts.
// @MX:SPEC: SPEC-ADP-008
func (a *Adapter) doRequest(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Naver-Client-Id", a.clientID)
	req.Header.Set("X-Naver-Client-Secret", a.clientSecret)
	return a.httpClient.Do(req)
}

// categorizeStatus maps an HTTP status code to a *types.SourceError with the
// appropriate Category. retryAfter is only applied for status 429.
//
// Mapping:
//   - 401/403 -> CategoryPermanent (auth failure)
//   - 429     -> CategoryRateLimited (with RetryAfter)
//   - 4xx     -> CategoryPermanent
//   - 5xx     -> CategoryUnavailable
//   - 0       -> CategoryUnavailable (network-layer error)
//   - other   -> CategoryUnknown
//
// @MX:NOTE: [AUTO] HTTP-status rosetta stone for Naver. Future contributors adding
// new status-code handling should update this switch first, then add a test row
// in TestCategorizeStatusTable.
// @MX:SPEC: SPEC-ADP-008
func categorizeStatus(status int, retryAfter time.Duration, cause error) *types.SourceError {
	se := &types.SourceError{
		Adapter:    "naver",
		HTTPStatus: status,
		Cause:      cause,
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		se.Category = types.CategoryPermanent
	case status == http.StatusTooManyRequests:
		se.Category = types.CategoryRateLimited
		se.RetryAfter = retryAfter
	case status >= 400 && status < 500:
		se.Category = types.CategoryPermanent
	case status >= 500 && status < 600:
		se.Category = types.CategoryUnavailable
	case status == 0:
		// Network-layer error (DNS failure, dial timeout, TLS handshake failure, etc.)
		se.Category = types.CategoryUnavailable
	default:
		se.Category = types.CategoryUnknown
	}
	return se
}
