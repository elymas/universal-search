// Package redditrss — HTTP client construction and request helpers.
// SPEC-ADP-001b REQ-ADP1B-003,004,017.
//
// The redirect allowlist is derived from the configured BaseURL host so that:
//   - Production: allows only www.reddit.com (SSRF guard, REQ-ADP1B-017)
//   - Tests: allows the loopback httptest host (BaseURL override, REQ-ADP1B-002)
package redditrss

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// newClientForBase constructs a *http.Client whose redirect policy allows only
// the host extracted from baseURL (max 3 hops). This allows loopback httptest
// hosts in tests while restricting production to www.reddit.com.
//
// @MX:WARN: [AUTO] Redirect allowlist is the SSRF guard for outbound RSS fetches.
// @MX:REASON: Without the allowlist an attacker-controlled redirect could fan out
// requests to internal hosts via the Reddit CDN (SSRF). The allowlist is derived
// from the configured base host so tests remain functional without weakening
// production.
// @MX:SPEC: SPEC-ADP-001b REQ-ADP1B-017
func newClientForBase(baseURL string, timeout time.Duration) *http.Client {
	allowedHost := canonicalHost(baseURL)
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("reddit-rss: too many redirects (max 3)")
			}
			host := req.URL.Hostname()
			if host != allowedHost {
				return fmt.Errorf("reddit-rss: cross-domain redirect rejected: %s", host)
			}
			return nil
		},
	}
}

// canonicalHost extracts the hostname (without port) from rawURL.
// Falls back to "www.reddit.com" if the URL cannot be parsed.
func canonicalHost(rawURL string) string {
	if rawURL == "" {
		return "www.reddit.com"
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return "www.reddit.com"
	}
	return u.Hostname()
}

// categorizeStatus maps an HTTP status code to a *types.SourceError with the
// correct Category for the Reddit RSS adapter.
//
// NOTE: This mapping DIFFERS from reddit/client.go on HTTP 403:
//   - reddit OAuth: 403 → CategoryPermanent (genuinely forbidden)
//   - reddit-rss:   403 → CategoryUnavailable (anonymous block, retryable, REQ-ADP1B-012)
//
// @MX:NOTE: [AUTO] HTTP-status rosetta stone for reddit-rss. 403 is CategoryUnavailable
// (retryable anon-block) unlike the OAuth adapter where 403 is CategoryPermanent.
// @MX:SPEC: SPEC-ADP-001b REQ-ADP1B-011..015
func categorizeStatus(status int, retryAfterHeader string, cause error) *types.SourceError {
	se := &types.SourceError{
		Adapter:    "reddit-rss",
		HTTPStatus: status,
		Cause:      cause,
	}
	switch {
	case status == 429:
		se.Category = types.CategoryRateLimited
		se.RetryAfter = parseRetryAfter(retryAfterHeader)
	case status == 403:
		// REQ-ADP1B-012: 403 from Reddit means anonymous-block (retryable).
		se.Category = types.CategoryUnavailable
	case status >= 400 && status < 500:
		// REQ-ADP1B-012a: other 4xx → permanent.
		se.Category = types.CategoryPermanent
	case status >= 500 && status < 600:
		// REQ-ADP1B-013: 5xx → unavailable.
		se.Category = types.CategoryUnavailable
	default:
		// Includes 0 (network-layer) and unexpected 1xx/2xx/3xx.
		se.Category = types.CategoryUnavailable
	}
	return se
}

// parseRetryAfter parses the Retry-After header value (seconds integer) into a
// time.Duration. Returns 0 when the header is absent or not a valid integer.
func parseRetryAfter(val string) time.Duration {
	if val == "" {
		return 0
	}
	secs, err := strconv.ParseInt(val, 10, 64)
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}
