// Package social — X (Twitter) third-party provider via twitterapi.io (Option B).
// SPEC-ADP-006-XENABLE: alternative to the official API — no X developer-console
// approval, own API key, includes search. ToS-grey; personal/research use only.
package social

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// twitterAPIProvider implements XProvider against the twitterapi.io
// GET /twitter/tweet/advanced_search endpoint.
//
// @MX:WARN: [AUTO] Outbound network call carrying a third-party API key.
// @MX:REASON: credential handling + metered cost (~$0.15/1k posts); do not log the key.
// @MX:SPEC: SPEC-ADP-006-XENABLE
type twitterAPIProvider struct {
	apiKey     string
	baseURL    string       // defaults to "https://api.twitterapi.io"
	httpClient *http.Client // shared; must be goroutine-safe
}

// TwitterAPIOptions configures the twitterapi.io provider.
type TwitterAPIOptions struct {
	// APIKey is the twitterapi.io API key (sent as X-API-Key header). Required.
	APIKey string
	// BaseURL overrides the API base URL. Used in tests.
	BaseURL string
	// HTTPClient overrides the default HTTP client. Used in tests.
	HTTPClient *http.Client
}

// NewTwitterAPIProvider constructs a twitterapi.io provider.
// Returns an error if APIKey is empty.
func NewTwitterAPIProvider(opts TwitterAPIOptions) (*twitterAPIProvider, error) {
	if opts.APIKey == "" {
		return nil, fmt.Errorf("social/twitterapi-io: APIKey is required")
	}
	base := opts.BaseURL
	if base == "" {
		base = "https://api.twitterapi.io"
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &twitterAPIProvider{
		apiKey:     opts.APIKey,
		baseURL:    base,
		httpClient: client,
	}, nil
}

// Name returns "twitterapi-io".
func (p *twitterAPIProvider) Name() string { return "twitterapi-io" }

// twitterAPISearchResponse is the JSON envelope from advanced_search.
//
// ponytail: field names track the twitterapi.io advanced_search docs. The API
// surface drifts; if a key is configured and results come back empty, diff the
// live JSON against these struct tags and adjust here — that's the only knob.
type twitterAPISearchResponse struct {
	Tweets      []twitterAPITweet `json:"tweets"`
	HasNextPage bool              `json:"has_next_page"`
	NextCursor  string            `json:"next_cursor"`
	// Some error responses set a top-level message instead of HTTP status.
	Message string `json:"message"`
	Status  string `json:"status"`
}

// twitterAPITweet is a single tweet from twitterapi.io.
type twitterAPITweet struct {
	ID           string           `json:"id"`
	URL          string           `json:"url"`
	Text         string           `json:"text"`
	CreatedAt    string           `json:"createdAt"`
	LikeCount    int              `json:"likeCount"`
	RetweetCount int              `json:"retweetCount"`
	ReplyCount   int              `json:"replyCount"`
	QuoteCount   int              `json:"quoteCount"`
	Author       twitterAPIAuthor `json:"author"`
}

// twitterAPIAuthor holds the tweet author.
type twitterAPIAuthor struct {
	UserName string `json:"userName"`
}

// SearchTweets executes an advanced search against twitterapi.io.
// The endpoint paginates by cursor (no per-request size param); queryType
// "Latest" prioritizes recency to surface real-time discussion.
func (p *twitterAPIProvider) SearchTweets(ctx context.Context, q types.Query) ([]XTweet, string, error) {
	params := url.Values{}
	params.Set("query", q.Text)
	params.Set("queryType", "Latest")
	if q.Cursor != "" {
		params.Set("cursor", q.Cursor)
	}

	endpoint := p.baseURL + "/twitter/tweet/advanced_search?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", &types.SourceError{
			Adapter:  "x",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("social/twitterapi-io: creating request: %w", err),
		}
	}
	req.Header.Set("X-API-Key", p.apiKey)
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
			fmt.Errorf("social/twitterapi-io: HTTP %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", categorizeStatus("x", 0, 0, fmt.Errorf("social/twitterapi-io: reading body: %w", err))
	}

	var searchResp twitterAPISearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, "", &types.SourceError{
			Adapter:  "x",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("social/twitterapi-io: malformed JSON: %w", err),
		}
	}

	// Some failures return HTTP 200 with an error message in the body.
	if searchResp.Status == "error" && searchResp.Message != "" {
		return nil, "", &types.SourceError{
			Adapter:  "x",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("social/twitterapi-io: %s", searchResp.Message),
		}
	}

	tweets := make([]XTweet, 0, len(searchResp.Tweets))
	for _, t := range searchResp.Tweets {
		tweets = append(tweets, XTweet{
			ID:           t.ID,
			Text:         t.Text,
			AuthorHandle: t.Author.UserName,
			URL:          t.URL,
			LikeCount:    t.LikeCount,
			RepostCount:  t.RetweetCount,
			ReplyCount:   t.ReplyCount,
			QuoteCount:   t.QuoteCount,
			CreatedAt:    t.CreatedAt,
		})
	}

	nextCursor := ""
	if searchResp.HasNextPage {
		nextCursor = searchResp.NextCursor
	}
	return tweets, nextCursor, nil
}

// compile-time assertion.
var _ XProvider = (*twitterAPIProvider)(nil)
