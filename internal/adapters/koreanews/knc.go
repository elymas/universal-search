// Package koreanews — KoreaNewsCrawler (KNC) sidecar sub-source.
// SPEC-ADP-009 REQ-ADP9-009: HTTP client scaffold for the KNC Python sidecar.
// Full sidecar implementation deferred to SPEC-ADP-009-KNC.
// The sidecar at services/koreanews/ is a stub returning HTTP 503 in v0.1.
package koreanews

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// kncSearchRequest is the JSON body sent to the KNC sidecar POST /search.
type kncSearchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

// kncArticle is one item returned by the KNC sidecar.
type kncArticle struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Date      string `json:"date"` // RFC 3339 or ""; empty → zero time
	Author    string `json:"author"`
	Category  string `json:"category"`
}

// kncSearchResponse is the JSON body returned by the KNC sidecar on success.
type kncSearchResponse struct {
	Articles []kncArticle `json:"articles"`
}

// searchKNC POSTs to the KNC sidecar at opts.KNCBaseURL/search.
// Returns ErrKNCSidecarDown (wrapped in *types.SourceError{CategoryUnavailable})
// when the sidecar returns HTTP 503 or is unreachable.
//
// @MX:NOTE: [AUTO] KNC sidecar scaffold — stub returns 503 in v0.1.
// Full implementation deferred to SPEC-ADP-009-KNC.
// @MX:SPEC: SPEC-ADP-009
func searchKNC(
	ctx context.Context,
	adapterName string,
	opts Options,
	client *http.Client,
	userAgent string,
	q types.Query,
) ([]types.NormalizedDoc, error) {
	body, err := json.Marshal(kncSearchRequest{
		Query:      q.Text,
		MaxResults: q.MaxResults,
	})
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  adapterName,
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("knc: marshal request: %w", err),
		}
	}

	url := strings.TrimRight(opts.KNCBaseURL, "/") + "/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  adapterName,
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("knc: build request: %w", err),
		}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  adapterName,
			Category: types.CategoryUnavailable,
			Cause:    fmt.Errorf("%w: %s", ErrKNCSidecarDown, err.Error()),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, &types.SourceError{
			Adapter:    adapterName,
			Category:   types.CategoryUnavailable,
			HTTPStatus: resp.StatusCode,
			Cause:      ErrKNCSidecarDown,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &types.SourceError{
			Adapter:    adapterName,
			Category:   types.CategoryTransient,
			HTTPStatus: resp.StatusCode,
			Cause:      fmt.Errorf("knc: unexpected status %d", resp.StatusCode),
		}
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  adapterName,
			Category: types.CategoryTransient,
			Cause:    fmt.Errorf("knc: read body: %w", err),
		}
	}

	var kncResp kncSearchResponse
	if err := json.Unmarshal(raw, &kncResp); err != nil {
		return nil, &types.SourceError{
			Adapter:  adapterName,
			Category: types.CategoryTransient,
			Cause:    fmt.Errorf("knc: unmarshal response: %w", err),
		}
	}

	return kncArticlesToDocs(adapterName, opts.NowFunc, kncResp.Articles), nil
}

// kncArticlesToDocs converts KNC sidecar articles to NormalizedDoc slice.
func kncArticlesToDocs(
	adapterName string,
	nowFunc func() time.Time,
	articles []kncArticle,
) []types.NormalizedDoc {
	now := nowFunc()
	docs := make([]types.NormalizedDoc, 0, len(articles))

	for _, a := range articles {
		if a.URL == "" {
			continue
		}

		var published time.Time
		if a.Date != "" {
			if t, err := time.Parse(time.RFC3339, a.Date); err == nil {
				published = t
			}
		}

		doc := types.NormalizedDoc{
			ID:          fmt.Sprintf("knc-%s", a.URL),
			SourceID:    adapterName,
			URL:         a.URL,
			Title:       a.Title,
			Body:        a.Body,
			Snippet:     truncate(a.Body, 200),
			PublishedAt: published,
			RetrievedAt: now,
			Author:      a.Author,
			Score:       0.5,
			Lang:        "ko",
			DocType:     types.DocTypeArticle,
			Metadata: map[string]any{
				"subsource": "knc",
				"category":  a.Category,
			},
		}
		doc.Hash = doc.CanonicalHash()
		docs = append(docs, doc)
	}
	return docs
}

// truncate returns the first n runes of s, or s if len(runes) <= n.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
