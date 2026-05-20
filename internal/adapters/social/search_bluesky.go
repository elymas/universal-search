// Package social — Bluesky live search path.
// REQ-ADP6-002: builds and executes the AppView searchPosts request.
package social

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

// defaultBlueskyBaseURL is the Bluesky AppView XRPC search endpoint.
const defaultBlueskyBaseURL = "https://public.api.bsky.app/xrpc/app.bsky.feed.searchPosts"

// defaultMaxResults is the default limit when Query.MaxResults == 0.
const defaultMaxResults = 25

// maxResultsLimit is the upper bound for the limit parameter.
const maxResultsLimit = 100

// ErrInvalidQuery is returned by Search when the query text is empty or
// contains only Unicode whitespace runes. Wrapped in *types.SourceError.
var ErrInvalidQuery = fmt.Errorf("social: query text empty or whitespace-only")

// searchBluesky executes a live search against the Bluesky AppView.
//
// @MX:ANCHOR: [AUTO] Bluesky live search path entry point.
// @MX:REASON: called by Search for subSource=="bluesky"; fan_in=1 but public API boundary.
// @MX:SPEC: SPEC-ADP-006
func (a *Adapter) searchBluesky(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// Validate query text — reject empty or whitespace-only.
	if isBlankQuery(q.Text) {
		return nil, &types.SourceError{
			Adapter:  "bluesky",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	// Build query parameters.
	limit := q.MaxResults
	if limit <= 0 {
		limit = defaultMaxResults
	}
	if limit > maxResultsLimit {
		limit = maxResultsLimit
	}

	params := url.Values{}
	params.Set("q", q.Text)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("sort", "top")

	if q.Cursor != "" {
		params.Set("cursor", q.Cursor)
	}
	if q.Lang != "" {
		params.Set("lang", q.Lang)
	}

	// Check for "since" filter (RFC 3339 datetime only).
	for _, f := range q.Filters {
		if f.Key == "since" {
			if _, err := time.Parse(time.RFC3339, f.Value); err == nil {
				params.Set("since", f.Value)
			}
			// Malformed since values are silently dropped.
		}
		// Unknown filter keys are silently ignored.
	}

	rawURL := a.baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "bluesky",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("social: failed to create request: %w", err),
		}
	}

	resp, err := a.doRequest(req)
	if err != nil {
		if isCrossDomainRedirectErr(err) {
			return nil, &types.SourceError{
				Adapter:  "bluesky",
				Category: types.CategoryPermanent,
				Cause:    err,
			}
		}
		return nil, categorizeStatus("bluesky", 0, 0, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		retryAfter := time.Duration(0)
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		}
		return nil, categorizeStatus("bluesky", resp.StatusCode, retryAfter, nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, categorizeStatus("bluesky", 0, 0, fmt.Errorf("social: reading response body: %w", err))
	}

	retrievedAt := time.Now().UTC()
	docs, _, parseErr := parseSearchPosts(body, retrievedAt)
	if parseErr != nil {
		return nil, parseErr
	}

	return docs, nil
}

// isBlankQuery returns true when s is empty or contains only Unicode whitespace.
func isBlankQuery(s string) bool {
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
