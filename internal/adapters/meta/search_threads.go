// Package meta — Threads live search path.
// REQ-ADP10-002: builds and executes the keyword_search request.
package meta

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

// defaultMaxResults is the default limit when Query.MaxResults == 0.
const defaultMaxResults = 25

// maxResultsLimit is the upper bound for the limit parameter.
const maxResultsLimit = 100

// searchThreads executes a live search against the Threads keyword_search endpoint.
//
// @MX:ANCHOR: [AUTO] Threads live search path entry point.
// @MX:REASON: called by Search for subSource=="threads"; fan_in=1 but public API boundary.
// @MX:SPEC: SPEC-ADP-010
func (a *Adapter) searchThreads(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// REQ-ADP10-009: reject empty or whitespace-only queries.
	if isBlankQuery(q.Text) {
		return nil, &types.SourceError{
			Adapter:  "threads",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	// Build query parameters.
	limit := q.MaxResults
	if limit == 0 {
		limit = defaultMaxResults // default 25
	}
	if limit < 1 {
		limit = 1 // clamp lower bound
	}
	if limit > maxResultsLimit {
		limit = maxResultsLimit // clamp upper bound
	}

	params := url.Values{}
	params.Set("q", q.Text)
	params.Set("limit", strconv.Itoa(limit))
	params.Set("search_type", "TOP")
	params.Set("search_mode", "KEYWORD")

	// REQ-ADP10-006: optional since/until filters.
	for _, f := range q.Filters {
		switch f.Key {
		case "since":
			if _, err := time.Parse(time.RFC3339, f.Value); err == nil {
				params.Set("since", f.Value)
			}
		case "until":
			if _, err := time.Parse(time.RFC3339, f.Value); err == nil {
				params.Set("until", f.Value)
			}
		}
	}

	rawURL := a.baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "threads",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("meta: failed to create request: %w", err),
		}
	}

	resp, err := a.doRequest(req)
	if err != nil {
		if isCrossDomainRedirectErr(err) {
			return nil, &types.SourceError{
				Adapter:  "threads",
				Category: types.CategoryPermanent,
				Cause:    err,
			}
		}
		return nil, categorizeStatus("threads", 0, 0, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		retryAfter := time.Duration(0)
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		}
		return nil, categorizeStatus("threads", resp.StatusCode, retryAfter, nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, categorizeStatus("threads", 0, 0, fmt.Errorf("meta: reading response body: %w", err))
	}

	retrievedAt := time.Now().UTC()
	return parseKeywordSearch(body, retrievedAt)
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
