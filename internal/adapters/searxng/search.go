// Package searxng — Search hot path.
// REQ-ADP7-002/003/004/005/006/007/008/010: Query validation, URL construction,
// HTTP execute, response parsing, and error mapping.
package searxng

import (
	"context"
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
// runaway responses. 5 MB is well beyond any real SearXNG response.
const maxResponseBytes = 5 * 1024 * 1024

// Search executes a SearXNG search query and returns normalized results.
//
// Validation: empty or whitespace-only q.Text returns ErrInvalidQuery without
// issuing any HTTP request (REQ-ADP7-008). Non-positive or non-numeric q.Cursor
// returns ErrInvalidCursor without issuing any HTTP request.
//
// URL construction: builds the /search request URL with q, format=json, and
// optional pageno (only when cursor is present) per REQ-ADP7-002.
//
// Error mapping: 429 → CategoryRateLimited (with parsed Retry-After),
// 403+Retry-After → CategoryRateLimited (REQ-ADP7-007),
// 4xx → CategoryPermanent, 5xx → CategoryUnavailable, network errors →
// CategoryUnavailable with HTTPStatus=0.
//
// Concurrency: all state is read-only after construction; the underlying
// *http.Client is goroutine-safe per Go stdlib (REQ-ADP7-010).
//
// @MX:ANCHOR: [AUTO] Sole public entry point for SearXNG search. Callers:
// registry wrappedAdapter (via types.Adapter), FAN-001 fanout, CLI-001, tests.
// @MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001
// + downstream IDX-001 RRF input shape.
// @MX:SPEC: SPEC-ADP-007
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// REQ-ADP7-008: reject empty/whitespace-only queries immediately.
	if isAllWhitespace(q.Text) {
		return nil, &types.SourceError{
			Adapter:  "searxng",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	// REQ-ADP7-008: reject invalid cursors immediately.
	// cursor="" is valid (first page, pageno omitted).
	// cursor="N" where N is integer >= 1 is valid.
	// Anything else is rejected.
	var currentPage int
	if q.Cursor != "" {
		page, err := strconv.Atoi(q.Cursor)
		if err != nil || page < 1 {
			return nil, &types.SourceError{
				Adapter:  "searxng",
				Category: types.CategoryPermanent,
				Cause:    ErrInvalidCursor,
			}
		}
		currentPage = page
	} else {
		// cursor="" means first page; pageno defaults to 1 server-side.
		currentPage = 1
	}

	// Build request URL.
	searchURL := buildSearchURL(a.baseURL, q, currentPage)

	// Create the HTTP request with context for cancellation support.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "searxng",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("searxng: failed to create request: %w", err),
		}
	}

	// Execute the request via doRequest (sets User-Agent + Accept headers).
	resp, err := a.doRequest(req)
	if err != nil {
		// CheckRedirect returning "cross-domain redirect rejected" is a policy
		// violation, not a transient network failure — map to CategoryPermanent.
		if isCrossDomainRedirectErr(err) {
			return nil, &types.SourceError{
				Adapter:  "searxng",
				Category: types.CategoryPermanent,
				Cause:    err,
			}
		}
		// Network-level error (dial failure, TLS, ctx cancel, etc.)
		return nil, categorizeStatus(0, 0, false, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Non-200 response handling.
	if resp.StatusCode != http.StatusOK {
		raHeader := resp.Header.Get("Retry-After")
		hasRA := raHeader != ""
		var retryAfter time.Duration
		if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode == http.StatusForbidden && hasRA) {
			retryAfter = parseRetryAfter(raHeader, time.Now())
		}
		var cause error
		if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode == http.StatusForbidden && hasRA) {
			cause = fmt.Errorf("searxng: rate limited")
		} else {
			cause = fmt.Errorf("searxng: HTTP %d", resp.StatusCode)
		}
		return nil, categorizeStatus(resp.StatusCode, retryAfter, hasRA, cause)
	}

	// Read response body with a cap to prevent OOM.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "searxng",
			Category: types.CategoryUnavailable,
			Cause:    fmt.Errorf("searxng: failed to read response body: %w", err),
		}
	}

	// Parse the SearXNG JSON response.
	docs, _, parseErr := parseSearch(body, time.Now().UTC(), currentPage)
	if parseErr != nil {
		return nil, parseErr
	}

	return docs, nil
}

// buildSearchURL constructs the SearXNG search URL with all required parameters.
// REQ-ADP7-002: q and format=json always present; pageno only when cursor != "".
func buildSearchURL(baseURL string, q types.Query, currentPage int) string {
	params := url.Values{}
	params.Set("q", q.Text)
	params.Set("format", "json")
	// Only include pageno when cursor was non-empty (currentPage was explicitly set).
	if q.Cursor != "" {
		params.Set("pageno", strconv.Itoa(currentPage))
	}
	return baseURL + "/search?" + params.Encode()
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
