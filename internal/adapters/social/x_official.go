// Package social — X (Twitter) official API v2 provider (Option A).
// SPEC-ADP-006-XENABLE: REQ-XEN-008 reference provider against GET /2/tweets/search/recent.
package social

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// xOfficialProvider implements XProvider against the X official API v2
// GET /2/tweets/search/recent endpoint.
//
// @MX:WARN: [AUTO] Outbound network call carrying a Bearer Token.
// @MX:REASON: credential handling + metered cost ($0.005/Post); do not log the token.
// @MX:SPEC: SPEC-ADP-006-XENABLE
type xOfficialProvider struct {
	bearerToken string
	baseURL     string       // defaults to "https://api.x.com"
	httpClient  *http.Client // shared; must be goroutine-safe
}

// XOfficialOptions configures the X official API v2 provider.
type XOfficialOptions struct {
	// BearerToken is the X API v2 Bearer Token from env. Required.
	BearerToken string
	// BaseURL overrides the API base URL. Used in tests.
	BaseURL string
	// HTTPClient overrides the default HTTP client. Used in tests.
	HTTPClient *http.Client
}

// NewXOfficialProvider constructs an X official API v2 provider.
// Returns nil and an error if BearerToken is empty.
func NewXOfficialProvider(opts XOfficialOptions) (*xOfficialProvider, error) {
	if opts.BearerToken == "" {
		return nil, fmt.Errorf("social/x-official: BearerToken is required")
	}
	base := opts.BaseURL
	if base == "" {
		base = "https://api.x.com"
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &xOfficialProvider{
		bearerToken: opts.BearerToken,
		baseURL:     base,
		httpClient:  client,
	}, nil
}

// Name returns "x-official".
func (p *xOfficialProvider) Name() string { return "x-official" }

// xOfficialSearchResponse is the JSON envelope from GET /2/tweets/search/recent.
type xOfficialSearchResponse struct {
	Data     []xOfficialTweet `json:"data"`
	Meta     xOfficialMeta    `json:"meta"`
	Errors   []xOfficialError `json:"errors"`
}

// xOfficialTweet is a single tweet from the official API v2.
type xOfficialTweet struct {
	ID            string                   `json:"id"`
	Text          string                   `json:"text"`
	AuthorID      string                   `json:"author_id"`
	CreatedAt     string                   `json:"created_at"`
	PublicMetrics xOfficialPublicMetrics   `json:"public_metrics"`
	Entities      *xOfficialEntities       `json:"entities,omitempty"`
}

// xOfficialPublicMetrics holds engagement counts.
type xOfficialPublicMetrics struct {
	LikeCount    int `json:"like_count"`
	RetweetCount int `json:"retweet_count"`
	ReplyCount   int `json:"reply_count"`
	QuoteCount   int `json:"quote_count"`
}

// xOfficialEntities holds tweet entities (URLs, etc).
type xOfficialEntities struct {
	URLs []xOfficialURL `json:"urls"`
}

// xOfficialURL holds an expanded URL from tweet entities.
type xOfficialURL struct {
	ExpandedURL string `json:"expanded_url"`
}

// xOfficialMeta holds pagination metadata.
type xOfficialMeta struct {
	NextToken   string `json:"next_token"`
	ResultCount int    `json:"result_count"`
}

// xOfficialError is an error from the X API v2.
type xOfficialError struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Type   string `json:"type"`
}

// SearchTweets executes a search against the X official API v2.
func (p *xOfficialProvider) SearchTweets(ctx context.Context, q types.Query) ([]XTweet, string, error) {
	limit := q.MaxResults
	if limit <= 0 {
		limit = defaultMaxResults
	}
	if limit > 100 {
		limit = 100
	}

	params := url.Values{}
	params.Set("query", q.Text)
	params.Set("max_results", strconv.Itoa(limit))
	params.Set("tweet.fields", "public_metrics,created_at,author_id,entities")

	if q.Cursor != "" {
		params.Set("next_token", q.Cursor)
	}

	endpoint := p.baseURL + "/2/tweets/search/recent?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", &types.SourceError{
			Adapter:  "x",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("social/x-official: creating request: %w", err),
		}
	}
	req.Header.Set("Authorization", "Bearer "+p.bearerToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, "", categorizeStatus("x", 0, 0, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		retryAfter := time.Duration(0)
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
		}
		return nil, "", categorizeStatus("x", resp.StatusCode, retryAfter,
			fmt.Errorf("social/x-official: HTTP %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", categorizeStatus("x", 0, 0, fmt.Errorf("social/x-official: reading body: %w", err))
	}

	var searchResp xOfficialSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, "", &types.SourceError{
			Adapter:  "x",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("social/x-official: malformed JSON: %w", err),
		}
	}

	// Check for API-level errors.
	if len(searchResp.Errors) > 0 {
		e := searchResp.Errors[0]
		return nil, "", &types.SourceError{
			Adapter:  "x",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("social/x-official: %s: %s", e.Title, e.Detail),
		}
	}

	tweets := make([]XTweet, 0, len(searchResp.Data))
	for _, t := range searchResp.Data {
		tweetURL := ""
		// Prefer the first expanded URL from entities.
		if t.Entities != nil && len(t.Entities.URLs) > 0 {
			tweetURL = t.Entities.URLs[0].ExpandedURL
		}
		tweets = append(tweets, XTweet{
			ID:           t.ID,
			Text:         t.Text,
			AuthorHandle: "", // author_id needs user expansion; handle from includes
			URL:          tweetURL,
			LikeCount:    t.PublicMetrics.LikeCount,
			RepostCount:  t.PublicMetrics.RetweetCount,
			ReplyCount:   t.PublicMetrics.ReplyCount,
			QuoteCount:   t.PublicMetrics.QuoteCount,
			CreatedAt:    t.CreatedAt,
		})
	}

	return tweets, searchResp.Meta.NextToken, nil
}

// compile-time assertion.
var _ XProvider = (*xOfficialProvider)(nil)
