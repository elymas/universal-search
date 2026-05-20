// Package searxng — HTTP client construction and request helpers.
// REQ-ADP7-003/004/006/007/009: HTTP client with timeout, redirect allowlist,
// reqid transport, custom User-Agent, and status-to-Category mapping.
package searxng

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/pkg/types"
)

// allowedRedirectHosts is the SSRF-guard allowlist for SearXNG redirect targets.
// SearXNG never issues cross-host redirects in normal operation; the allowlist
// is defensive and consistent with the pattern from SPEC-ADP-001.
//
// @MX:NOTE: [AUTO] 3-entry local-only redirect allowlist. The local-only posture
// is intentional; adding a host requires a security review confirming the new
// host is operator-controlled.
// @MX:SPEC: SPEC-ADP-007
var allowedRedirectHosts = map[string]struct{}{
	"searxng":   {},
	"localhost": {},
	"127.0.0.1": {},
}

// redirectAllowlist is the CheckRedirect policy for the SearXNG HTTP client.
// It enforces:
//   - Maximum 3 redirect hops (REQ-ADP7-009).
//   - All redirect targets must be in allowedRedirectHosts (SSRF guard).
func redirectAllowlist(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return errors.New("searxng: too many redirects (max 3)")
	}
	host := req.URL.Hostname()
	if _, ok := allowedRedirectHosts[host]; !ok {
		return fmt.Errorf("searxng: cross-domain redirect rejected: %s", host)
	}
	return nil
}

// newDefaultClient constructs the default HTTP client for the SearXNG adapter:
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
//   - User-Agent: a.userAgent (custom UA per REQ-ADP7-006)
//   - Accept: application/json
//
// The caller is responsible for closing the response body.
//
// @MX:WARN: [AUTO] Outbound network call. The CheckRedirect policy on
// a.httpClient enforces the SSRF guard; do not bypass or replace the client
// without re-applying the allowlist.
// @MX:REASON: removing CheckRedirect re-opens SSRF via any host that the local
// SearXNG instance might unexpectedly redirect to.
// @MX:SPEC: SPEC-ADP-007
func (a *Adapter) doRequest(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Accept", "application/json")
	return a.httpClient.Do(req)
}

// isCrossDomainRedirectErr reports whether err originated from the cross-domain
// redirect guard in redirectAllowlist. The check is string-based because
// http.Client wraps CheckRedirect errors in *url.Error, losing the original
// type for errors.As.
func isCrossDomainRedirectErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "cross-domain redirect")
}

// categorizeStatus maps an HTTP status code to a *types.SourceError with the
// appropriate Category. This extends the ADP-001 rosetta with the
// 403+Retry-After → RateLimited promotion (REQ-ADP7-007).
//
// Mapping:
//   - 429 → CategoryRateLimited (with RetryAfter)
//   - 403 WITH Retry-After → CategoryRateLimited (limiter promotion, REQ-ADP7-007)
//   - 4xx → CategoryPermanent
//   - 5xx → CategoryUnavailable
//   - 0   → CategoryUnavailable (network-layer error)
//   - other → CategoryUnknown
//
// @MX:NOTE: [AUTO] HTTP-status rosetta stone. The 403+Retry-After promotion is
// the SearXNG-specific delta from the ADP-001 baseline; future contributors
// checking limiter behaviour changes should update this function first, then add
// a test row in TestCategorizeStatusTable.
// @MX:SPEC: SPEC-ADP-007
func categorizeStatus(status int, retryAfter time.Duration, hasRetryAfterHeader bool, cause error) *types.SourceError {
	se := &types.SourceError{
		Adapter:    "searxng",
		HTTPStatus: status,
		Cause:      cause,
	}
	switch {
	case status == 429:
		se.Category = types.CategoryRateLimited
		se.RetryAfter = retryAfter
	case status == 403 && hasRetryAfterHeader:
		// REQ-ADP7-007: promote 403 with Retry-After to RateLimited.
		// SearXNG's bot-detection may emit 403 instead of 429 depending on version.
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
		// 1xx/2xx/3xx unexpected here; Search consumes 2xx before calling this.
		se.Category = types.CategoryUnknown
	}
	return se
}
