// Package youtube — HTTP client construction and request helpers.
// REQ-ADP5-006: custom User-Agent, Accept header.
// REQ-ADP5-003/004: categorizeStatus maps HTTP status codes to SourceError.
package youtube

import (
	"fmt"
	"net/http"
	"time"

	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/pkg/types"
)

// newDefaultClient constructs the default HTTP client for the YouTube adapter:
//   - 30-second request timeout (longer than HN/Reddit's 10s because the
//     sidecar's yt-dlp call may legitimately take 5-15 seconds with transcripts)
//   - No CheckRedirect override (sidecar URL is operator-configured and trusted)
//   - reqid.NewTransport wrapping http.DefaultTransport for request-ID propagation
func newDefaultClient() *http.Client {
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: reqid.NewTransport(http.DefaultTransport),
	}
}

// doRequest executes the given HTTP request via the adapter's HTTP client,
// setting the required headers:
//   - User-Agent: a.userAgent (custom UA per REQ-ADP5-006)
//   - Accept: application/json
//
// The caller is responsible for closing the response body.
//
// @MX:WARN: [AUTO] Outbound network call to a configurable sidecar URL.
// No redirect allowlist — sidecar is operator-configured and trusted.
// @MX:REASON: removing the request-context propagation would invalidate
// REQ-ADP5-009 ctx cancellation discipline and NFR-ADP5-003 goroutine-leak guarantee.
// @MX:SPEC: SPEC-ADP-005
func (a *Adapter) doRequest(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", a.userAgent)
	req.Header.Set("Accept", "application/json")
	return a.httpClient.Do(req)
}

// categorizeStatus maps an HTTP status code to a *types.SourceError with the
// appropriate Category. retryAfter is only applied for status 429.
//
// Mapping:
//   - 429 -> CategoryRateLimited (with RetryAfter)
//   - 4xx -> CategoryPermanent
//   - 5xx -> CategoryUnavailable
//   - 0   -> CategoryUnavailable (network-layer error)
//
// @MX:NOTE: [AUTO] HTTP-status rosetta stone. Future contributors adding new
// status-code handling should update this switch first, then add a test row in
// TestCategorizeStatusTable. Mirrors internal/adapters/hn/client.go — shared
// extraction deferred to SPEC-ADP-REFAC-001 (Open Question §11.6).
// @MX:SPEC: SPEC-ADP-005
func categorizeStatus(status int, retryAfter time.Duration, cause error) *types.SourceError {
	se := &types.SourceError{
		Adapter:    "youtube",
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
		// Network-layer error (DNS failure, dial timeout, connection refused, etc.)
		se.Category = types.CategoryUnavailable
	default:
		se.Category = types.CategoryUnknown
	}
	return se
}

// permanentError builds a CategoryPermanent SourceError for non-429 4xx responses.
func permanentError(status int) *types.SourceError {
	return categorizeStatus(status, 0, fmt.Errorf("youtube: permanent failure: %d", status))
}
