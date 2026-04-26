// Package reddit — Search hot path.
// REQ-ADP-002/007/008/011: Query validation, URL construction, HTTP execute,
// response parsing, and error mapping.
package reddit

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
// runaway responses. 5 MB is well beyond any real Reddit Listing response.
const maxResponseBytes = 5 * 1024 * 1024

// Search executes a Reddit search query and returns normalized results.
//
// Validation: empty or whitespace-only q.Text returns ErrInvalidQuery without
// issuing any HTTP request (REQ-ADP-008).
//
// URL construction: builds the /search.json request URL with all required
// parameters per REQ-ADP-002 and the NSFW filter per REQ-ADP-007.
//
// Error mapping: 429 -> CategoryRateLimited (with parsed Retry-After),
// 4xx -> CategoryPermanent, 5xx -> CategoryUnavailable, network errors ->
// CategoryUnavailable with HTTPStatus=0.
//
// Concurrency: all state is read-only after construction; the underlying
// *http.Client is goroutine-safe per Go stdlib (REQ-ADP-011).
//
// @MX:ANCHOR: [AUTO] Sole public entry point for Reddit search. Callers:
// registry wrappedAdapter (via types.Adapter), FAN-001 fanout, CLI-001, tests.
// @MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001
// + SYN-001 and all downstream SPECs that depend on []types.NormalizedDoc.
// @MX:SPEC: SPEC-ADP-001
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// REQ-ADP-008: reject empty/whitespace-only queries immediately.
	if isAllWhitespace(q.Text) {
		return nil, &types.SourceError{
			Adapter:  "reddit",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	// Build request URL.
	searchURL := buildSearchURL(a.baseURL, q)

	// Create the HTTP request with context for cancellation support.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "reddit",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("reddit: failed to create request: %w", err),
		}
	}

	// Execute the request via doRequest (sets User-Agent + Accept headers).
	resp, err := a.doRequest(req)
	if err != nil {
		// CheckRedirect returning "cross-domain redirect rejected" is a policy
		// violation, not a transient network failure — map to CategoryPermanent.
		if isCrossDomainRedirectErr(err) {
			return nil, &types.SourceError{
				Adapter:  "reddit",
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
