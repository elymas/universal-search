// Package social — HTTP client construction and request helpers.
// REQ-ADP6-003/009/010: HTTP client with timeout, redirect allowlist,
// reqid transport, custom User-Agent, and status-to-Category mapping.
package social

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/pkg/types"
)

// blueskyAllowedRedirectHosts is the SSRF-guard allowlist for Bluesky redirect targets.
//
// @MX:NOTE: [AUTO] 3-entry security boundary for Bluesky.
// @MX:SPEC: SPEC-ADP-006
var blueskyAllowedRedirectHosts = map[string]struct{}{
	"public.api.bsky.app": {},
	"api.bsky.app":        {},
	"bsky.app":            {},
}

// blueskyRedirectAllowlist is the CheckRedirect policy for the Bluesky HTTP client.
// Enforces max 3 redirect hops and restricts targets to the Bluesky allowlist.
func blueskyRedirectAllowlist(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return errors.New("social: too many redirects (max 3)")
	}
	host := req.URL.Hostname()
	if _, ok := blueskyAllowedRedirectHosts[host]; !ok {
		return fmt.Errorf("social: cross-domain redirect rejected: %s", host)
	}
	return nil
}

// newDefaultBlueskyClient constructs the default HTTP client for the Bluesky adapter:
//   - 10-second request timeout
//   - blueskyRedirectAllowlist CheckRedirect policy (SSRF guard, max 3 hops)
//   - reqid.NewTransport wrapping http.DefaultTransport for request-ID propagation
func newDefaultBlueskyClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		Transport:     reqid.NewTransport(http.DefaultTransport),
		CheckRedirect: blueskyRedirectAllowlist,
	}
}

// doRequest executes the given HTTP request via the adapter's HTTP client,
// setting the required headers:
//   - User-Agent: a.userAgent (custom UA per REQ-ADP6-009)
//   - Accept: application/json
//
// @MX:WARN: [AUTO] Outbound network call. The CheckRedirect policy on
// a.httpClient enforces the SSRF guard; do not bypass or replace the client
// without re-applying the allowlist.
// @MX:REASON: removing CheckRedirect re-opens SSRF via Bluesky CDN redirects.
// @MX:SPEC: SPEC-ADP-006
func (a *Adapter) doRequest(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Accept", "application/json")
	return a.httpClient.Do(req)
}

// isCrossDomainRedirectErr reports whether err originated from the cross-domain
// redirect guard.
func isCrossDomainRedirectErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "cross-domain redirect")
}

// categorizeStatus maps an HTTP status code to a *types.SourceError with the
// appropriate Category. adapterName is "bluesky" or "x". retryAfter is only
// applied for status 429.
//
// Mapping:
//   - 429 -> CategoryRateLimited (with RetryAfter)
//   - 4xx -> CategoryPermanent
//   - 5xx -> CategoryUnavailable
//   - 0   -> CategoryUnavailable (network-layer error)
//   - other -> CategoryUnknown
//
// @MX:NOTE: [AUTO] HTTP-status rosetta stone. Future contributors adding new
// status-code handling should update this switch first, then add a test row.
// @MX:SPEC: SPEC-ADP-006
func categorizeStatus(adapterName string, status int, retryAfter time.Duration, cause error) *types.SourceError {
	se := &types.SourceError{
		Adapter:    adapterName,
		HTTPStatus: status,
		Cause:      cause,
	}
	switch {
	case status == 429:
		se.Category = types.CategoryRateLimited
		se.RetryAfter = retryAfter
	case status >= 400 && status < 500:
		se.Category = types.CategoryPermanent
	case status >= 500 && status < 600:
		se.Category = types.CategoryUnavailable
	case status == 0:
		se.Category = types.CategoryUnavailable
	default:
		se.Category = types.CategoryUnknown
	}
	return se
}
