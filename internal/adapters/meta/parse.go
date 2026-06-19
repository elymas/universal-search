// Package meta — parse keyword_search JSON response.
// REQ-ADP10-004/005: envelope parsing, error-before-data, field mapping.
package meta

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/elymas/universal-search/pkg/types"
)

// keywordSearchResponse is the top-level Threads keyword_search JSON envelope.
type keywordSearchResponse struct {
	Data  []keywordSearchPost `json:"data"`
	Error *graphError         `json:"error,omitempty"`
}

// graphError represents the Meta Graph API error envelope.
type graphError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    int    `json:"code"`
}

// keywordSearchPost represents a single post in the keyword_search response.
type keywordSearchPost struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Text        string `json:"text"`
	Permalink   string `json:"permalink"`
	Timestamp   string `json:"timestamp"`
	MediaType   string `json:"media_type"`
	HasReplies  *bool  `json:"has_replies,omitempty"`
	IsReply     *bool  `json:"is_reply,omitempty"`
	IsQuotePost *bool  `json:"is_quote_post,omitempty"`
}

// @MX:ANCHOR: [AUTO] Every Threads doc passes through this transform.
// @MX:REASON: NormalizedDoc field-mapping integrity gate.
// @MX:SPEC: SPEC-ADP-010
func parseKeywordSearch(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	var resp keywordSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &types.SourceError{
			Adapter:  "threads",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("meta/threads: malformed JSON: %w", err),
		}
	}

	// Graph error envelope is checked BEFORE reading data.
	if resp.Error != nil {
		return nil, &types.SourceError{
			Adapter:  "threads",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("threads: %s (code %d)", resp.Error.Message, resp.Error.Code),
		}
	}

	// Empty data is a valid empty result, not an error.
	if len(resp.Data) == 0 {
		return nil, nil
	}

	docs := make([]types.NormalizedDoc, 0, len(resp.Data))
	for _, post := range resp.Data {
		docs = append(docs, mapPostToNormalizedDoc(post, retrievedAt))
	}
	return docs, nil
}

// mapPostToNormalizedDoc maps a keyword_search post to a NormalizedDoc per §6.5.
func mapPostToNormalizedDoc(post keywordSearchPost, retrievedAt time.Time) types.NormalizedDoc {
	publishedAt := parseTimestamp(post.Timestamp)
	snippet := truncateRunes(post.Text, 280)

	metadata := map[string]any{
		"username":   post.Username,
		"permalink":  post.Permalink,
		"media_type": post.MediaType,
		"posted_at":  post.Timestamp,
		"sub_source": "threads",
	}
	if post.HasReplies != nil {
		metadata["has_replies"] = *post.HasReplies
	}
	if post.IsReply != nil {
		metadata["is_reply"] = *post.IsReply
	}
	if post.IsQuotePost != nil {
		metadata["is_quote_post"] = *post.IsQuotePost
	}

	return types.NormalizedDoc{
		ID:          "threads:" + post.ID,
		SourceID:    "threads",
		URL:         post.Permalink,
		Author:      post.Username,
		Body:        post.Text,
		Title:       snippet,
		Snippet:     snippet,
		PublishedAt: publishedAt,
		RetrievedAt: retrievedAt,
		Score:       neutralScore(),
		Lang:        "",
		DocType:     types.DocTypePost,
		Citations:   nil,
		Metadata:    metadata,
		Hash:        "",
	}
}

// parseTimestamp parses a timestamp string, trying RFC 3339 first, then Unix.
// Returns zero time on failure.
func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// Try RFC 3339.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

// truncateRunes returns the first n runes of s, or s if shorter.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := make([]rune, 0, n)
	for _, r := range s {
		if len(runes) >= n {
			break
		}
		runes = append(runes, r)
	}
	return string(runes)
}
