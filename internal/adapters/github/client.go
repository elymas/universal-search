// Package github — HTTP client construction and error categorization.
// REQ-ADP4-006: User-Agent, redirect allowlist, request-ID transport.
// REQ-ADP4-009: SSRF guard via CheckRedirect allowlist.
package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	gogithub "github.com/google/go-github/v73/github"

	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/pkg/types"
)

// allowedRedirectHosts is the SSRF guard allowlist for GitHub CDN domains.
// Adding a host requires a security review.
//
// @MX:NOTE: [AUTO] 4-entry redirect allowlist. Security boundary for SSRF
// via GitHub CDN redirects. Any host addition needs security review.
// @MX:SPEC: SPEC-ADP-004
var allowedRedirectHosts = map[string]struct{}{
	"api.github.com":            {},
	"github.com":                {},
	"raw.githubusercontent.com": {},
	"codeload.github.com":       {},
}

// newDefaultHTTPClient constructs the default *http.Client for the GitHub adapter.
// 10s timeout; reqid transport for request-ID propagation; redirect allowlist.
//
// @MX:WARN: [AUTO] Outbound HTTP client with redirect allowlist.
// @MX:REASON: Removing CheckRedirect re-opens SSRF via GitHub CDN redirects.
// @MX:SPEC: SPEC-ADP-004
func newDefaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		Transport:     reqid.NewTransport(http.DefaultTransport),
		CheckRedirect: redirectAllowlist,
	}
}

// redirectAllowlist is the CheckRedirect function that enforces the host
// allowlist and caps redirect chains at 3 hops.
func redirectAllowlist(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return errors.New("github: too many redirects (max 3)")
	}
	host := req.URL.Hostname()
	if _, ok := allowedRedirectHosts[host]; !ok {
		return fmt.Errorf("github: cross-domain redirect rejected: %s", host)
	}
	return nil
}

// categorizeError maps a go-github error to a *types.SourceError with the
// appropriate Category. Returns nil when err is nil.
//
// The rosetta order matters: RateLimitError and AbuseRateLimitError are checked
// first because they are subtypes of the conditions that would also match
// ErrorResponse (both return 403). errors.As is used throughout.
//
// @MX:NOTE: [AUTO] go-github typed-error rosetta. Future contributors adding
// support for a new go-github error type should extend this function and add
// a matching test case to TestCategorizeErrorTable.
// @MX:SPEC: SPEC-ADP-004
func categorizeError(err error) *types.SourceError {
	if err == nil {
		return nil
	}

	// Primary rate limit: go-github detects 403 + X-RateLimit-Remaining=0.
	var rateLimitErr *gogithub.RateLimitError
	if errors.As(err, &rateLimitErr) {
		retryAfter := time.Until(rateLimitErr.Rate.Reset.Time)
		if retryAfter <= 0 {
			retryAfter = defaultRetryAfter
		}
		if retryAfter > maxRetryAfter {
			retryAfter = maxRetryAfter
		}
		return &types.SourceError{
			Adapter:    "github",
			Category:   types.CategoryRateLimited,
			HTTPStatus: rateLimitErr.Response.StatusCode,
			RetryAfter: retryAfter,
			Cause:      rateLimitErr,
		}
	}

	// Abuse / secondary rate limit: 403 with documentation URL pointing to
	// secondary-rate-limits docs.
	var abuseErr *gogithub.AbuseRateLimitError
	if errors.As(err, &abuseErr) {
		retryAfter := defaultRetryAfter
		if abuseErr.RetryAfter != nil && *abuseErr.RetryAfter > 0 {
			retryAfter = *abuseErr.RetryAfter
		}
		if retryAfter > maxRetryAfter {
			retryAfter = maxRetryAfter
		}
		status := 0
		if abuseErr.Response != nil {
			status = abuseErr.Response.StatusCode
		}
		return &types.SourceError{
			Adapter:    "github",
			Category:   types.CategoryRateLimited,
			HTTPStatus: status,
			RetryAfter: retryAfter,
			Cause:      abuseErr,
		}
	}

	// All other API errors: ErrorResponse carries the HTTP status.
	var errResp *gogithub.ErrorResponse
	if errors.As(err, &errResp) {
		status := errResp.Response.StatusCode
		if status >= 400 && status < 500 {
			return &types.SourceError{
				Adapter:    "github",
				Category:   types.CategoryPermanent,
				HTTPStatus: status,
				Cause:      errResp,
			}
		}
		if status >= 500 && status < 600 {
			return &types.SourceError{
				Adapter:    "github",
				Category:   types.CategoryUnavailable,
				HTTPStatus: status,
				Cause:      errResp,
			}
		}
	}

	// JSON decode errors: malformed response body — permanent (retry won't fix it).
	// go-github may return the json error directly (*json.SyntaxError) or wrapped
	// as an io.ErrUnexpectedEOF for truncated bodies.
	var syntaxErr *json.SyntaxError
	var unmarshalErr *json.UnmarshalTypeError
	if errors.As(err, &syntaxErr) || errors.As(err, &unmarshalErr) ||
		errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return &types.SourceError{
			Adapter:    "github",
			Category:   types.CategoryPermanent,
			HTTPStatus: 0,
			Cause:      err,
		}
	}

	// Network-layer or unknown error — unavailable.
	return &types.SourceError{
		Adapter:    "github",
		Category:   types.CategoryUnavailable,
		HTTPStatus: 0,
		Cause:      err,
	}
}
