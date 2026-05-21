// Package naver — Search hot path.
// REQ-ADP8-002/007/008/011: Query validation, vertical dispatch, URL construction,
// HTTP execute, response parsing, and error mapping.
package naver

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

// maxResponseBytes caps the HTTP response body read to prevent OOM.
// 5 MB is well beyond any real Naver search response.
const maxResponseBytes = 5 * 1024 * 1024

// Search executes a Naver search query and returns normalized results.
// The vertical is selected by Query.Filters[{naver_vertical, <name>}]; default=blog.
//
// Validation: empty or whitespace-only q.Text returns ErrInvalidQuery without
// issuing any HTTP request (REQ-ADP8-008). Exception: datalab vertical passes
// q.Text as raw JSON body and skips whitespace validation.
//
// Error mapping: 401/403 -> CategoryPermanent, 429 -> CategoryRateLimited,
// 5xx -> CategoryUnavailable, network errors -> CategoryUnavailable.
//
// Concurrency: all state is read-only after construction; the underlying
// *http.Client is goroutine-safe per Go stdlib (REQ-ADP8-011).
//
// @MX:ANCHOR: [AUTO] Sole public entry point for Naver search. Callers:
// registry wrappedAdapter (via types.Adapter), FAN-001 fanout, CLI-001, tests.
// @MX:REASON: contract boundary; signature change ripples to FAN-001 + CLI-001
// + SYN-001 and all downstream SPECs that depend on []types.NormalizedDoc.
// @MX:SPEC: SPEC-ADP-008
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	vertical := filterVertical(q.Filters)

	// DataLab skips the whitespace check — q.Text is a JSON body.
	if vertical == verticalDataLab {
		return a.searchDataLab(ctx, q, time.Now().UTC())
	}

	// REQ-ADP8-008: reject empty/whitespace-only queries.
	if isAllWhitespace(q.Text) {
		return nil, &types.SourceError{
			Adapter:  "naver",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	endpointURL := a.endpointForVertical(vertical)
	reqURL := buildSearchURL(endpointURL, q)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "naver",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("naver: failed to create request: %w", err),
		}
	}

	resp, err := a.doRequest(req)
	if err != nil {
		return nil, categorizeStatus(0, 0, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var retryAfter time.Duration
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		}
		cause := fmt.Errorf("naver: HTTP %d", resp.StatusCode)
		return nil, categorizeStatus(resp.StatusCode, retryAfter, cause)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "naver",
			Category: types.CategoryUnavailable,
			Cause:    fmt.Errorf("naver: failed to read response body: %w", err),
		}
	}

	retrievedAt := time.Now().UTC()
	return a.parseForVertical(vertical, body, retrievedAt)
}

// endpointForVertical returns the base URL for the given vertical.
func (a *Adapter) endpointForVertical(vertical string) string {
	switch vertical {
	case verticalNews:
		return a.baseURLNews
	case verticalWeb:
		return a.baseURLWeb
	case verticalShop:
		return a.baseURLShop
	default:
		return a.baseURLBlog
	}
}

// parseForVertical dispatches parsing to the appropriate parse function.
func (a *Adapter) parseForVertical(vertical string, body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	switch vertical {
	case verticalNews:
		return parseNewsResponse(body, retrievedAt)
	case verticalWeb:
		return parseWebResponse(body, retrievedAt)
	case verticalShop:
		return parseShopResponse(body, retrievedAt)
	default:
		return parseBlogResponse(body, retrievedAt)
	}
}

// buildSearchURL constructs the Naver search URL with required parameters.
// start is derived from q.Cursor (1-based integer string); defaults to 1.
func buildSearchURL(endpointURL string, q types.Query) string {
	params := url.Values{}
	params.Set("query", q.Text)
	params.Set("display", strconv.Itoa(clampDisplay(q.MaxResults)))
	start := parseStartCursor(q.Cursor)
	params.Set("start", strconv.Itoa(start))
	return endpointURL + "?" + params.Encode()
}

// clampDisplay returns a display value clamped to [1, 100], defaulting to 25.
func clampDisplay(maxResults int) int {
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

// parseStartCursor parses a cursor string as a 1-based start integer.
// Returns 1 if the cursor is empty or cannot be parsed as a positive integer.
func parseStartCursor(cursor string) int {
	if cursor == "" {
		return 1
	}
	n, err := strconv.Atoi(cursor)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// filterVertical extracts the naver_vertical filter value from q.Filters.
// Returns "blog" (default) when the filter is absent or unrecognized.
func filterVertical(filters []types.Filter) string {
	for _, f := range filters {
		if f.Key == filterKeyVertical {
			switch f.Value {
			case verticalBlog, verticalNews, verticalWeb, verticalShop, verticalDataLab:
				return f.Value
			}
		}
	}
	return verticalBlog
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
