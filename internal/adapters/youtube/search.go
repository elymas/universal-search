// Package youtube — Search hot path.
// REQ-ADP5-002: HTTP POST to sidecar /search, JSON body construction.
// REQ-ADP5-007: Korean-locale auto-detection + lang/since filter handling.
// REQ-ADP5-008: empty query, invalid cursor, cursor-over-cap validation.
// REQ-ADP5-009: ctx cancellation discipline.
package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
	"unicode"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	// searchPath is the sidecar's POST /search endpoint path.
	searchPath = "/search"

	// defaultMaxResults is used when Query.MaxResults is 0.
	defaultMaxResults = 25

	// maxAllowedResults clamps MaxResults to [1, 100].
	maxAllowedResults = 100

	// paginationCap is the max value of max_results + cursor_offset (D7).
	paginationCap = 100
)

// searchRequestBody is the JSON body sent to the sidecar POST /search.
type searchRequestBody struct {
	Query              string `json:"query"`
	MaxResults         int    `json:"max_results"`
	CursorOffset       int    `json:"cursor_offset"`
	TranscriptLang     string `json:"transcript_lang"`
	IncludeTranscripts bool   `json:"include_transcripts"`
	Since              *int64 `json:"since,omitempty"`
}

// Search executes a YouTube search via the sidecar HTTP endpoint.
// Validates the query, builds the request body, executes the request,
// parses the response, and returns NormalizedDocs or a categorised SourceError.
//
// @MX:ANCHOR: [AUTO] Sole entry point for all YouTube fanout calls.
// @MX:REASON: contract boundary; signature change ripples to FAN-001, CLI-001,
// SYN-001 and the synthesizer's video citation handler.
// @MX:SPEC: SPEC-ADP-005
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// REQ-ADP5-009: ctx cancellation takes precedence over validation.
	if ctx.Err() != nil {
		return nil, &types.SourceError{
			Adapter:  "youtube",
			Category: types.CategoryUnavailable,
			Cause:    ctx.Err(),
		}
	}

	// REQ-ADP5-008: empty/whitespace query rejection.
	if isAllWhitespace(q.Text) {
		return nil, &types.SourceError{
			Adapter:  "youtube",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	// REQ-ADP5-008: cursor validation.
	cursorOffset := 0
	if q.Cursor != "" {
		n, err := strconv.Atoi(q.Cursor)
		if err != nil || n < 0 {
			return nil, &types.SourceError{
				Adapter:  "youtube",
				Category: types.CategoryPermanent,
				Cause:    ErrInvalidCursor,
			}
		}
		cursorOffset = n
	}

	// Clamp max_results to [1, 100].
	maxResults := q.MaxResults
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}
	if maxResults > maxAllowedResults {
		maxResults = maxAllowedResults
	}

	// REQ-ADP5-008: cursor-over-cap check.
	if maxResults+cursorOffset > paginationCap {
		return nil, &types.SourceError{
			Adapter:  "youtube",
			Category: types.CategoryPermanent,
			Cause:    ErrCursorOverCap,
		}
	}

	// REQ-ADP5-007: determine transcript language.
	transcriptLang := selectTranscriptLang(q.Text, q.Filters)

	// REQ-ADP5-007: since filter.
	var since *int64
	for _, f := range q.Filters {
		if f.Key == "since" {
			n, err := strconv.ParseInt(f.Value, 10, 64)
			if err == nil && n > 0 {
				since = &n
			}
			break
		}
	}

	// Build request body.
	reqBody := searchRequestBody{
		Query:              q.Text,
		MaxResults:         maxResults,
		CursorOffset:       cursorOffset,
		TranscriptLang:     transcriptLang,
		IncludeTranscripts: true,
		Since:              since,
	}
	encoded, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "youtube",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("youtube: marshal request: %w", err),
		}
	}

	// Build HTTP request.
	url := a.baseURL + searchPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "youtube",
			Category: types.CategoryUnavailable,
			Cause:    fmt.Errorf("youtube: build request: %w", err),
		}
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request.
	retrievedAt := time.Now().UTC()
	resp, err := a.doRequest(req)
	if err != nil {
		// Network-layer failure (sidecar unreachable, ctx cancelled, timeout).
		return nil, wrapNetworkError(ctx, err)
	}
	defer resp.Body.Close() //nolint:errcheck // body close error is non-actionable after successful read

	// Read response body (limit to 10 MB to prevent runaway allocations).
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "youtube",
			Category: types.CategoryUnavailable,
			Cause:    fmt.Errorf("youtube: read response: %w", err),
		}
	}

	// Handle non-200 responses.
	if resp.StatusCode != http.StatusOK {
		return nil, handleNonOK(resp, body)
	}

	// Parse successful response.
	docs, _, parseErr := parseSearchResponse(body, retrievedAt, cursorOffset, transcriptLang)
	if parseErr != nil {
		return nil, parseErr
	}
	return docs, nil
}

// wrapNetworkError categorises a network-layer error (connection refused,
// ctx cancelled, dial timeout, etc.) into a *types.SourceError.
func wrapNetworkError(ctx context.Context, err error) *types.SourceError {
	if ctx.Err() != nil {
		return &types.SourceError{
			Adapter:  "youtube",
			Category: types.CategoryUnavailable,
			Cause:    ctx.Err(),
		}
	}
	return &types.SourceError{
		Adapter:    "youtube",
		Category:   types.CategoryUnavailable,
		HTTPStatus: 0,
		Cause:      err,
	}
}

// handleNonOK converts a non-200 HTTP response into a *types.SourceError.
func handleNonOK(resp *http.Response, body []byte) *types.SourceError {
	status := resp.StatusCode

	// For 429, parse Retry-After.
	if status == 429 {
		ra := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		return categorizeStatus(429, ra, fmt.Errorf("youtube: rate limited"))
	}

	// For 5xx with sidecar error envelope, preserve inner reason.
	if status >= 500 {
		if env := parseErrorEnvelope(body); env != nil {
			msg := env.Message
			if msg == "" {
				msg = env.Reason
			}
			return &types.SourceError{
				Adapter:    "youtube",
				Category:   types.CategoryUnavailable,
				HTTPStatus: status,
				Cause:      fmt.Errorf("youtube: sidecar error: %s", msg),
			}
		}
		return categorizeStatus(status, 0, fmt.Errorf("youtube: sidecar HTTP %d", status))
	}

	// 4xx (not 429).
	return permanentError(status)
}

// isAllWhitespace returns true when s is empty or contains only Unicode
// whitespace runes per unicode.IsSpace.
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
