// Package naver — DataLab trend API support.
// REQ-ADP8-013: DataLab POST endpoint, one NormalizedDoc per keywordGroups row.
package naver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// datalabResult is a single keyword group result from the DataLab API.
type datalabResult struct {
	Title    string         `json:"title"`
	Keywords []string       `json:"keywords"`
	Data     []datalabPoint `json:"data"`
}

// datalabPoint is a single data point (period + ratio) in a DataLab result.
type datalabPoint struct {
	Period string  `json:"period"`
	Ratio  float64 `json:"ratio"`
}

// datalabResponse is the top-level DataLab API response.
type datalabResponse struct {
	StartDate string          `json:"startDate"`
	EndDate   string          `json:"endDate"`
	TimeUnit  string          `json:"timeUnit"`
	Results   []datalabResult `json:"results"`
}

// searchDataLab executes a POST request to the Naver DataLab search trends API.
// Query.Text must be the JSON-encoded request body per REQ-ADP8-013.
// Returns one NormalizedDoc per keyword group in the response.
//
// @MX:ANCHOR: [AUTO] DataLab POST path. Query.Text must be JSON request body.
// @MX:REASON: fan_in = Search (via vertical dispatch); sole DataLab execution path.
// @MX:SPEC: SPEC-ADP-008
func (a *Adapter) searchDataLab(ctx context.Context, q types.Query, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURLDataLab, bytes.NewBufferString(q.Text))
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "naver",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("naver: failed to create datalab request: %w", err),
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.doRequest(req)
	if err != nil {
		return nil, categorizeStatus(0, 0, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var retryAfter time.Duration
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		}
		cause := fmt.Errorf("naver: datalab HTTP %d", resp.StatusCode)
		return nil, categorizeStatus(resp.StatusCode, retryAfter, cause)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  "naver",
			Category: types.CategoryUnavailable,
			Cause:    fmt.Errorf("naver: failed to read datalab response body: %w", err),
		}
	}

	return parseDatalabResponse(body, retrievedAt)
}

// parseDatalabResponse parses a Naver DataLab JSON response into NormalizedDoc values.
// Returns one NormalizedDoc per keyword group in results.
func parseDatalabResponse(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	var resp datalabResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &types.SourceError{
			Adapter:  "naver",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("naver: malformed datalab JSON: %w", err),
		}
	}

	if len(resp.Results) == 0 {
		return nil, nil
	}

	docs := make([]types.NormalizedDoc, 0, len(resp.Results))
	for _, result := range resp.Results {
		// Encode the data points array as JSON for Body.
		dataJSON, err := json.Marshal(result.Data)
		if err != nil {
			dataJSON = []byte("[]")
		}

		title := fmt.Sprintf("DataLab: %s (%s ~ %s)", result.Title, resp.StartDate, resp.EndDate)
		body := string(dataJSON)
		snippet := truncateRunes(fmt.Sprintf("%s keywords: %v", result.Title, result.Keywords), snippetMaxRunes)

		// Stable synthetic ID based on title + date range.
		idSrc := fmt.Sprintf("naver-datalab-%s-%s-%s", result.Title, resp.StartDate, resp.EndDate)
		meta := map[string]any{
			"keywords":   result.Keywords,
			"start_date": resp.StartDate,
			"end_date":   resp.EndDate,
			"time_unit":  resp.TimeUnit,
			"data_count": len(result.Data),
		}

		doc := types.NormalizedDoc{
			ID:          syntheticID(idSrc),
			SourceID:    "naver",
			URL:         fmt.Sprintf("https://datalab.naver.com/keyword/trendSearch.naver"),
			Title:       title,
			Body:        body,
			Snippet:     snippet,
			RetrievedAt: retrievedAt,
			Score:       defaultScore,
			Lang:        "ko",
			DocType:     types.DocTypeOther,
			Metadata:    meta,
		}
		docs = append(docs, doc)
	}

	return docs, nil
}
