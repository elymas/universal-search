package arxiv

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// clampLimit returns a value in [1, 100]. Zero maps to 25 (default).
func clampLimit(maxResults int) int {
	if maxResults <= 0 {
		return 25
	}
	if maxResults > 100 {
		return 100
	}
	return maxResults
}

// isAllWhitespace reports whether s consists entirely of whitespace characters.
func isAllWhitespace(s string) bool {
	return strings.TrimSpace(s) == ""
}

// buildSearchQuery constructs the search_query parameter value.
// If a "category" filter is present and non-empty, it prepends "cat:<val> AND ".
// Unknown filter keys are silently ignored.
func buildSearchQuery(text string, filters []types.Filter) string {
	category := ""
	for _, f := range filters {
		if f.Key == "category" && f.Value != "" {
			category = f.Value
			break
		}
	}
	if category != "" {
		return "cat:" + category + " AND " + text
	}
	return text
}

// Search executes a paper search against the arXiv Atom API.
// It validates the query, enforces the per-instance rate-limit gate, issues
// one HTTP request, and parses the Atom XML response into NormalizedDocs.
//
// // @MX:ANCHOR: [AUTO] Hot path called by fanout gateway for every search request.
// // @MX:REASON: fan_in >= 3 (fanout gateway, search_test.go, rate_test.go)
// // @MX:SPEC: SPEC-ADP-003 REQ-ADP3-001..REQ-ADP3-012
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// REQ-ADP3-008: reject empty / whitespace-only queries immediately.
	if isAllWhitespace(q.Text) {
		return nil, &types.SourceError{
			Adapter:  "arxiv",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	// REQ-ADP3-008: validate cursor (must be non-negative integer or empty).
	start := 0
	if q.Cursor != "" {
		n, err := strconv.Atoi(strings.TrimSpace(q.Cursor))
		if err != nil || n < 0 {
			return nil, &types.SourceError{
				Adapter:  "arxiv",
				Category: types.CategoryPermanent,
				Cause:    ErrInvalidStart,
			}
		}
		start = n
	}

	// REQ-ADP3-012: honour per-instance minimum request interval.
	if err := a.waitForRateSlot(ctx); err != nil {
		return nil, err
	}

	// Build the request URL.
	rawURL := a.buildURL(q, start)

	// Execute the HTTP request.
	resp, err := doRequest(ctx, a.httpClient, rawURL, a.userAgent)
	if err != nil {
		// Cross-domain redirect: permanent error.
		if strings.Contains(err.Error(), "cross-domain redirect") ||
			strings.Contains(err.Error(), "too many redirects") {
			return nil, &types.SourceError{
				Adapter:  "arxiv",
				Category: types.CategoryPermanent,
				Cause:    fmt.Errorf("arxiv: do request: %w", err),
			}
		}
		// Other network error.
		retryAfter := parseRetryAfter("", time.Now())
		se := categorizeStatus(0, retryAfter, err)
		se.Cause = fmt.Errorf("arxiv: do request: %w", err)
		return nil, se
	}
	defer func() { _ = resp.Body.Close() }()

	// Handle non-200 status codes.
	if resp.StatusCode != 200 {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		se := categorizeStatus(resp.StatusCode, retryAfter, fmt.Errorf("arxiv: HTTP %d", resp.StatusCode))
		return nil, se
	}

	// Read and parse the response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "arxiv",
			Category: types.CategoryTransient,
			Cause:    fmt.Errorf("arxiv: read body: %w", err),
		}
	}

	return parseFeed(body, time.Now().UTC())
}

// buildURL constructs the full arXiv API query URL.
func (a *Adapter) buildURL(q types.Query, start int) string {
	limit := clampLimit(q.MaxResults)
	searchQuery := buildSearchQuery(q.Text, q.Filters)

	params := url.Values{}
	params.Set("search_query", searchQuery)
	params.Set("start", strconv.Itoa(start))
	params.Set("max_results", strconv.Itoa(limit))
	params.Set("sortBy", "relevance")
	params.Set("sortOrder", "descending")

	return a.baseURL + "?" + params.Encode()
}

// waitForRateSlot blocks until the per-instance minimum interval has elapsed
// since the last request, or until ctx is cancelled. The mutex is held only
// briefly to compute/update nextRequest; the actual sleep is outside the lock
// so ctx cancellation is honoured promptly.
func (a *Adapter) waitForRateSlot(ctx context.Context) error {
	if a.minInterval <= 0 {
		return nil
	}

	a.rateMu.Lock()
	now := time.Now()
	wait := a.nextRequest.Sub(now)
	if wait < 0 {
		wait = 0
	}
	a.nextRequest = now.Add(wait).Add(a.minInterval)
	a.rateMu.Unlock()

	if wait <= 0 {
		return nil
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
