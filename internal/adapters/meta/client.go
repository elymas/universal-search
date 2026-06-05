// Package meta — HTTP client construction and request helpers.
// REQ-ADP10-002/003: HTTP client with timeout, redirect allowlist, Bearer auth.
package meta

import (
	"net/http"
	"strings"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// doRequest executes the given HTTP request via the adapter's HTTP client,
// setting User-Agent, Accept, and Authorization: Bearer headers.
//
// @MX:WARN: [AUTO] Outbound network call carrying the Bearer token.
// @MX:REASON: removing CheckRedirect re-opens SSRF; token must stay in Authorization
// header only (NFR-ADP10-002).
// @MX:SPEC: SPEC-ADP-010
func (a *Adapter) doRequest(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Accept", "application/json")
	if a.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.accessToken)
	}
	return a.httpClient.Do(req)
}

// isCrossDomainRedirectErr reports whether err originated from the cross-domain redirect guard.
func isCrossDomainRedirectErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "cross-domain redirect")
}

// categorizeStatus maps an HTTP status code to a *types.SourceError with the
// appropriate Category. adapterName is "threads" or "facebook". retryAfter is
// only applied for status 429.
//
// Mapping:
//   - 429 -> CategoryRateLimited (with RetryAfter)
//   - 4xx -> CategoryPermanent
//   - 5xx -> CategoryUnavailable
//   - 0   -> CategoryUnavailable (network-layer error)
//   - other -> CategoryUnknown
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
