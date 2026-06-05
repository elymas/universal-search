// Package reddit — Search hot path.
// REQ-ADP-002/007/008/011: Query validation, URL construction, HTTP execute,
// response parsing, and error mapping.
// SPEC-ADP-001a: OAuth auth preamble + 401 refresh+retry.
package reddit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"unicode"

	"github.com/elymas/universal-search/pkg/types"
)

// maxResponseBytes caps the HTTP response body read to prevent OOM on
// runaway responses. 5 MB is well beyond any real Reddit Listing response.
const maxResponseBytes = 5 * 1024 * 1024

// Search executes a Reddit search query and returns normalized results.
//
// Auth preamble (ADP-001a): obtains a valid bearer token from the cache
// (refreshing if needed), sets Authorization header, and issues the request
// against the search endpoint. On 401, invalidates the token, refreshes once,
// and retries exactly once (REQ-ADP-001a-004).
//
// Validation: empty or whitespace-only q.Text returns ErrInvalidQuery without
// issuing any HTTP request (REQ-ADP-008).
//
// URL construction: builds the /search request URL with all required
// parameters per REQ-ADP-002 and the NSFW filter per REQ-ADP-007.
//
// Error mapping: 429 -> CategoryRateLimited (with parsed Retry-After),
// 401 -> refresh+retry once then CategoryUnavailable (ADP-001a),
// 403 -> CategoryPermanent (unchanged),
// 5xx -> CategoryUnavailable, network errors -> CategoryUnavailable with HTTPStatus=0.
//
// Concurrency: the tokenCache is guarded by sync.Mutex; the underlying
// *http.Client is goroutine-safe per Go stdlib (REQ-ADP-011).
//
// @MX:ANCHOR: [AUTO] Sole public entry point for Reddit search. Callers:
// registry wrappedAdapter (via types.Adapter), FAN-001 fanout, CLI-001, tests.
// @MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001
// + SYN-001 and all downstream SPECs that depend on []types.NormalizedDoc.
// ADP-001a adds auth preamble + 401 refresh-retry semantics.
// @MX:SPEC: SPEC-ADP-001 SPEC-ADP-001a
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// REQ-ADP-008: reject empty/whitespace-only queries immediately.
	if isAllWhitespace(q.Text) {
		return nil, &types.SourceError{
			Adapter:  "reddit",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	// Auth preamble (ADP-001a): obtain a valid bearer token.
	token, err := a.tokens.get(ctx, a.acquireToken)
	if err != nil {
		return nil, err
	}

	// Build and execute the search request.
	docs, err := a.doSearch(ctx, q, token)
	if err == nil {
		return docs, nil
	}

	// Handle 401: invalidate token, refresh once, retry exactly once (REQ-ADP-001a-004).
	if isHTTPStatus401(err) {
		a.tokens.invalidate()

		token, refreshErr := a.tokens.get(ctx, a.acquireToken)
		if refreshErr != nil {
			return nil, refreshErr
		}

		docs, retryErr := a.doSearch(ctx, q, token)
		if retryErr == nil {
			return docs, nil
		}
		// If retry also returns 401, wrap as exhausted.
		if isHTTPStatus401(retryErr) {
			return nil, &types.SourceError{
				Adapter:    "reddit",
				Category:   types.CategoryUnavailable,
				HTTPStatus: http.StatusUnauthorized,
				Cause:      ErrTokenRefreshExhausted,
			}
		}
		return nil, retryErr
	}

	return nil, err
}

// doSearch executes a single authenticated search attempt. Returns
// (docs, nil) on success, or (nil, error) for failures.
func (a *Adapter) doSearch(ctx context.Context, q types.Query, token string) ([]types.NormalizedDoc, error) {
	searchURL := buildSearchURL(a.baseURL, q)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "reddit",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("reddit: failed to create request: %w", err),
		}
	}

	// Set Authorization header for bearer token auth (ADP-001a).
	req.Header.Set("Authorization", "bearer "+token)

	// Execute the request via doRequest (sets User-Agent + Accept headers).
	resp, err := a.doRequest(req)
	if err != nil {
		if isCrossDomainRedirectErr(err) {
			return nil, &types.SourceError{
				Adapter:  "reddit",
				Category: types.CategoryPermanent,
				Cause:    err,
			}
		}
		return nil, categorizeStatus(0, 0, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Non-200 response handling.
	if resp.StatusCode != http.StatusOK {
		var retryAfter time.Duration
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		}
		cause := fmt.Errorf("reddit: HTTP %d", resp.StatusCode)
		return nil, categorizeStatus(resp.StatusCode, retryAfter, cause)
	}

	// Read response body with a cap to prevent OOM.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "reddit",
			Category: types.CategoryUnavailable,
			Cause:    fmt.Errorf("reddit: failed to read response body: %w", err),
		}
	}

	// Parse the Reddit JSON Listing.
	docs, _, parseErr := parseListing(body, time.Now().UTC())
	if parseErr != nil {
		return nil, parseErr
	}

	return docs, nil
}

// isHTTPStatus401 checks if the error is a *types.SourceError with HTTPStatus 401.
func isHTTPStatus401(err error) bool {
	var se *types.SourceError
	if errors.As(err, &se) {
		return se.HTTPStatus == http.StatusUnauthorized
	}
	return false
}

// buildSearchURL constructs the Reddit search URL with all required parameters.
func buildSearchURL(baseURL string, q types.Query) string {
	params := url.Values{}
	params.Set("q", q.Text)
	params.Set("sort", "relevance")
	params.Set("t", "all")
	params.Set("type", "link")
	params.Set("limit", strconv.Itoa(clampLimit(q.MaxResults)))
	params.Set("include_over_18", nsfwFilterValue(q.Filters))
	if q.Cursor != "" {
		params.Set("after", q.Cursor)
	}
	return baseURL + "?" + params.Encode()
}

// clampLimit returns a limit value clamped to [1, 100], defaulting to 25 when
// maxResults is 0.
func clampLimit(maxResults int) int {
	if maxResults == 0 {
		return 25
	}
	if maxResults < 1 {
		return 1
	}
	if maxResults > 100 {
		return 100
	}
	return maxResults
}

// nsfwFilterValue returns "true" only when the Filters slice contains an entry
// with Key="nsfw" and Value="true". All other cases return "false".
// REQ-ADP-007: any value other than "true" is treated as false.
func nsfwFilterValue(filters []types.Filter) string {
	for _, f := range filters {
		if f.Key == "nsfw" && f.Value == "true" {
			return "true"
		}
	}
	return "false"
}

// isAllWhitespace reports whether s is empty or contains only Unicode
// whitespace runes as defined by unicode.IsSpace.
func isAllWhitespace(s string) bool {
	if s == "" {
		return true
	}
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
