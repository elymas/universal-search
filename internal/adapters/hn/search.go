// Package hn — Search hot path.
// REQ-ADP2-002/007/008/010: Query validation, URL construction, HTTP execute,
// response parsing, and error mapping.
package hn

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/elymas/universal-search/pkg/types"
)

// maxResponseBytes caps the HTTP response body read to prevent OOM on
// runaway responses. 5 MB is well beyond any real Algolia HN response.
const maxResponseBytes = 5 * 1024 * 1024

// Search executes a Hacker News Algolia search query and returns normalized results.
//
// Validation: empty or whitespace-only q.Text returns ErrInvalidQuery without
// issuing any HTTP request (REQ-ADP2-008). Non-integer or negative q.Cursor
// returns ErrInvalidCursor without issuing any HTTP request.
//
// URL construction: builds the Algolia HN search request URL with all required
// parameters per REQ-ADP2-002 and optional numeric filters per REQ-ADP2-007.
//
// Error mapping: 429 -> CategoryRateLimited (with parsed Retry-After),
// 4xx -> CategoryPermanent, 5xx -> CategoryUnavailable, network errors ->
// CategoryUnavailable with HTTPStatus=0.
//
// Concurrency: all state is read-only after construction; the underlying
// *http.Client is goroutine-safe per Go stdlib (REQ-ADP2-010).
//
// @MX:ANCHOR: [AUTO] Sole public entry point for HN search. Callers:
// registry wrappedAdapter (via types.Adapter), FAN-001 fanout, CLI-001, tests.
// @MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001
// + SYN-001 and all downstream SPECs that depend on []types.NormalizedDoc.
// @MX:SPEC: SPEC-ADP-002
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// REQ-ADP2-008: reject empty/whitespace-only queries immediately.
	if isAllWhitespace(q.Text) {
		return nil, &types.SourceError{
			Adapter:  "hackernews",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	// REQ-ADP2-008: reject invalid cursors immediately.
	if q.Cursor != "" {
		page, err := strconv.Atoi(q.Cursor)
		if err != nil || page < 0 {
			return nil, &types.SourceError{
				Adapter:  "hackernews",
				Category: types.CategoryPermanent,
				Cause:    ErrInvalidCursor,
			}
		}
	}

	// Build request URL.
	searchURL := buildSearchURL(a.baseURL, q)

	// Create the HTTP request with context for cancellation support.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "hackernews",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("hn: failed to create request: %w", err),
		}
	}

	// Execute the request via doRequest (sets User-Agent + Accept headers).
	resp, err := a.doRequest(req)
	if err != nil {
		// CheckRedirect returning "cross-domain redirect rejected" is a policy
		// violation, not a transient network failure — map to CategoryPermanent.
		if isCrossDomainRedirectErr(err) {
			return nil, &types.SourceError{
				Adapter:  "hackernews",
				Category: types.CategoryPermanent,
				Cause:    err,
			}
		}
		// Network-level error (dial failure, TLS, ctx cancel, etc.)
		return nil, categorizeStatus(0, 0, err)
	}
	defer resp.Body.Close()

	// Non-200 response handling.
	if resp.StatusCode != http.StatusOK {
		var retryAfter time.Duration
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		}
		cause := fmt.Errorf("hn: HTTP %d", resp.StatusCode)
		return nil, categorizeStatus(resp.StatusCode, retryAfter, cause)
	}

	// Read response body with a cap to prevent OOM.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "hackernews",
			Category: types.CategoryUnavailable,
			Cause:    fmt.Errorf("hn: failed to read response body: %w", err),
		}
	}

	// Parse the Algolia HN hits envelope.
	docs, _, parseErr := parseHits(body, time.Now().UTC())
	if parseErr != nil {
		return nil, parseErr
	}

	return docs, nil
}

// buildSearchURL constructs the Algolia HN search URL with all required parameters.
// REQ-ADP2-002: query, tags=story, hitsPerPage mandatory.
// REQ-ADP2-007: numericFilters optional.
func buildSearchURL(baseURL string, q types.Query) string {
	params := url.Values{}
	params.Set("query", q.Text)
	params.Set("tags", "story")
	params.Set("hitsPerPage", strconv.Itoa(clampHitsPerPage(q.MaxResults)))

	// Pagination cursor: parse as non-negative integer page number.
	if q.Cursor != "" {
		if page, err := strconv.Atoi(q.Cursor); err == nil && page >= 0 {
			params.Set("page", q.Cursor)
		}
	}

	// REQ-ADP2-007: numeric filters for since and min_points.
	if numericFilter := buildNumericFilters(q.Filters); numericFilter != "" {
		params.Set("numericFilters", numericFilter)
	}

	return baseURL + "?" + params.Encode()
}

// clampHitsPerPage returns a hitsPerPage value clamped to [1, 100], defaulting to 25
// when maxResults is 0.
func clampHitsPerPage(maxResults int) int {
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

// buildNumericFilters constructs the numericFilters parameter value from Query.Filters.
// Recognized filter keys: "since" (maps to created_at_i>=<value>),
// "min_points" (maps to points>=<value>). All other keys are silently ignored.
// Malformed or negative values are silently dropped.
func buildNumericFilters(filters []types.Filter) string {
	var exprs []string
	for _, f := range filters {
		switch f.Key {
		case "since":
			v, err := strconv.ParseInt(f.Value, 10, 64)
			if err != nil || v <= 0 {
				continue
			}
			exprs = append(exprs, fmt.Sprintf("created_at_i>=%d", v))
		case "min_points":
			v, err := strconv.Atoi(f.Value)
			if err != nil || v <= 0 {
				continue
			}
			exprs = append(exprs, fmt.Sprintf("points>=%d", v))
		// Unknown keys: silently ignored.
		}
	}
	return strings.Join(exprs, ",")
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
