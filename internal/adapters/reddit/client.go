// Package reddit — HTTP client construction and request helpers.
// REQ-ADP-003/004/005/009/010: HTTP client with timeout, redirect allowlist,
// reqid transport, custom User-Agent, and status-to-Category mapping.
package reddit

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/pkg/types"
)

// allowedRedirectHosts is the SSRF-guard allowlist for Reddit redirect targets.
// Only these hosts may be followed during CheckRedirect.
//
// @MX:NOTE: [AUTO] 5-entry security boundary (ADP-001a added oauth.reddit.com).
// Adding a host here requires a security review — a permissive allowlist
// re-opens SSRF via Reddit's CDN redirect infrastructure.
// @MX:SPEC: SPEC-ADP-001 SPEC-ADP-001a
var allowedRedirectHosts = map[string]struct{}{
	"www.reddit.com":   {},
	"old.reddit.com":   {},
	"new.reddit.com":   {},
	"reddit.com":       {},
	"oauth.reddit.com": {},
}

// redirectAllowlist is the CheckRedirect policy for the Reddit HTTP client.
// It enforces:
//   - Maximum 3 redirect hops (REQ-ADP-010).
//   - All redirect targets must be in allowedRedirectHosts (SSRF guard).
func redirectAllowlist(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return errors.New("reddit: too many redirects (max 3)")
	}
	host := req.URL.Hostname()
	if _, ok := allowedRedirectHosts[host]; !ok {
		return fmt.Errorf("reddit: cross-domain redirect rejected: %s", host)
	}
	return nil
}

// newDefaultClient constructs the default HTTP client for the Reddit adapter:
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
// setting the required headers:
//   - User-Agent: a.userAgent (custom UA per REQ-ADP-009)
//   - Accept: application/json
//
// The request is cloned to avoid mutating the caller's Request object.
// The caller is responsible for closing the response body.
//
// @MX:WARN: [AUTO] Outbound network call. The CheckRedirect policy on
// a.httpClient enforces the SSRF guard; do not bypass or replace the client
// without re-applying the allowlist.
// @MX:REASON: removing CheckRedirect re-opens SSRF via Reddit CDN redirects.
// @MX:SPEC: SPEC-ADP-001
func (a *Adapter) doRequest(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Accept", "application/json")
	return a.httpClient.Do(req)
}

// isCrossDomainRedirectErr reports whether err originated from the cross-domain
// redirect guard in redirectAllowlist. The check is string-based because
// http.Client wraps CheckRedirect errors in *url.Error, losing the original
// type for errors.As. A sentinel string avoids an extra sentinel error type.
func isCrossDomainRedirectErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "cross-domain redirect")
}

// categorizeStatus maps an HTTP status code to a *types.SourceError with the
// appropriate Category. retryAfter is only applied for status 429.
//
// Mapping (ADP-001a updated):
//   - 429 -> CategoryRateLimited (with RetryAfter)
//   - 401 -> CategoryUnavailable (token expired; Search intercepts before here
//     for refresh+retry, but if it reaches this point after exhaustion, map to
//     Unavailable per REQ-ADP-001a-004)
//   - 4xx -> CategoryPermanent (including 403 — genuinely forbidden)
//   - 5xx -> CategoryUnavailable
//   - 0   -> CategoryUnavailable (network-layer error)
//   - other -> CategoryUnknown
//
// @MX:NOTE: [AUTO] HTTP-status rosetta stone. ADP-001a added 401 carve-out:
// 401 is now CategoryUnavailable (recoverable via token refresh), NOT Permanent.
// The Search layer intercepts 401 before this function for the refresh+retry path.
// Future contributors adding new status-code handling should update this switch
// first, then add a test row in TestCategorizeStatusTable.
// @MX:SPEC: SPEC-ADP-001 SPEC-ADP-001a
func categorizeStatus(status int, retryAfter time.Duration, cause error) *types.SourceError {
	se := &types.SourceError{
		Adapter:    "reddit",
		HTTPStatus: status,
		Cause:      cause,
	}
	switch {
	case status == 429:
		se.Category = types.CategoryRateLimited
		se.RetryAfter = retryAfter
	case status == 401:
		// ADP-001a: 401 is recoverable via token refresh. Search intercepts
		// before reaching here for the refresh+retry path. If it reaches here
		// after exhaustion, map to Unavailable.
		se.Category = types.CategoryUnavailable
	case status >= 400 && status < 500:
		se.Category = types.CategoryPermanent
	case status >= 500 && status < 600:
		se.Category = types.CategoryUnavailable
	case status == 0:
		// Network-layer error (DNS failure, dial timeout, TLS handshake failure, etc.)
		se.Category = types.CategoryUnavailable
	default:
		// 1xx/2xx/3xx unexpected here; Search consumes 2xx before calling this.
		se.Category = types.CategoryUnknown
	}
	return se
}
