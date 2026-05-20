// Package social — Bluesky AppView JSON → []types.NormalizedDoc transform.
// REQ-ADP6-005/006: parseSearchPosts maps Bluesky's JSON envelope to NormalizedDoc.
package social

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/elymas/universal-search/pkg/types"
)

// blueskySearchResponse is the top-level Bluesky AppView search response envelope.
type blueskySearchResponse struct {
	// xrpcError fields — present when Bluesky returns an error as HTTP 200.
	Error   string `json:"error"`
	Message string `json:"message"`

	Cursor    string        `json:"cursor"`
	HitsTotal int           `json:"hitsTotal"`
	Posts     []blueskyPost `json:"posts"`
}

// blueskyPost is a single post in the AppView search response.
type blueskyPost struct {
	URI    string        `json:"uri"`
	CID    string        `json:"cid"`
	Author blueskyAuthor `json:"author"`
	Record blueskyRecord `json:"record"`

	ReplyCount  int `json:"replyCount"`
	RepostCount int `json:"repostCount"`
	LikeCount   int `json:"likeCount"`
	QuoteCount  int `json:"quoteCount"`

	IndexedAt string `json:"indexedAt"`
}

// blueskyAuthor holds post author fields.
type blueskyAuthor struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

// blueskyRecord holds the post content record.
type blueskyRecord struct {
	Type      string   `json:"$type"`
	Text      string   `json:"text"`
	CreatedAt string   `json:"createdAt"`
	Langs     []string `json:"langs"`
}

// snippetMaxRunes is the maximum rune length for NormalizedDoc.Snippet.
const snippetMaxRunes = 280

// parseSearchPosts parses a Bluesky AppView search response body into
// NormalizedDoc values. retrievedAt is the timestamp assigned to each doc's
// RetrievedAt field (injected by the caller for determinism in tests).
//
// Returns:
//   - (docs, cursor, nil) on success; cursor is the opaque pagination token.
//   - (nil, "", nil) when the posts array is empty.
//   - (nil, "", *SourceError{Permanent}) on malformed JSON.
//   - (nil, "", *SourceError{Permanent}) when the response is an XRPC error envelope.
//
// The last doc in the slice gets Metadata["next_cursor"] set when cursor != "".
//
// @MX:ANCHOR: [AUTO] NormalizedDoc field-mapping integrity gate for Bluesky.
// Every Bluesky post passes through this single transform function.
// @MX:REASON: fan_in = 1 (searchBluesky) but invariant-bearing; field-mapping
// changes require coordination with SPEC-IDX-001 and SPEC-SYN-001 consumers.
// @MX:SPEC: SPEC-ADP-006
func parseSearchPosts(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, string, error) {
	var resp blueskySearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", &types.SourceError{
			Adapter:  "bluesky",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("social: malformed Bluesky JSON response: %w", err),
		}
	}

	// Detect XRPC error envelope: HTTP 200 with {error, message} keys.
	if resp.Error != "" {
		return nil, "", &types.SourceError{
			Adapter:  "bluesky",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("social: Bluesky XRPC error %s: %s", resp.Error, resp.Message),
		}
	}

	posts := resp.Posts
	if len(posts) == 0 {
		return nil, "", nil
	}

	cursor := resp.Cursor

	docs := make([]types.NormalizedDoc, 0, len(posts))
	for i := range posts {
		doc, err := transformBlueskyPost(&posts[i], retrievedAt)
		if err != nil {
			// Skip posts that cannot be transformed (e.g., malformed AT-URI).
			continue
		}
		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		return nil, "", nil
	}

	// Surface the pagination cursor on the last returned doc.
	if cursor != "" {
		last := &docs[len(docs)-1]
		if last.Metadata == nil {
			last.Metadata = make(map[string]any)
		}
		last.Metadata["next_cursor"] = cursor
	}

	return docs, cursor, nil
}

// transformBlueskyPost converts a single Bluesky post into a NormalizedDoc.
// This is the canonical field mapping per SPEC-ADP-006 §6.5.
func transformBlueskyPost(p *blueskyPost, retrievedAt time.Time) (types.NormalizedDoc, error) {
	// Parse AT-URI to extract rkey for URL construction.
	did, rkey, err := parseATURI(p.URI)
	if err != nil {
		return types.NormalizedDoc{}, fmt.Errorf("social: invalid AT-URI %q: %w", p.URI, err)
	}

	// Prefer handle; fall back to DID when handle is empty.
	handleOrDID := p.Author.Handle
	if handleOrDID == "" {
		handleOrDID = did
	}

	postURL := constructBlueskyURL(handleOrDID, rkey)

	// Parse PublishedAt from record.createdAt (RFC 3339).
	var publishedAt time.Time
	if p.Record.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, p.Record.CreatedAt); err == nil {
			publishedAt = t.UTC()
		}
	}

	// Author: prefer DisplayName; fall back to handle or DID.
	author := p.Author.DisplayName
	if author == "" {
		author = p.Author.Handle
	}
	if author == "" {
		author = p.Author.DID
	}

	// Lang: first element of record.langs, or "" if empty.
	lang := ""
	if len(p.Record.Langs) > 0 {
		lang = p.Record.Langs[0]
	}

	snippet := truncateRunes(p.Record.Text, snippetMaxRunes)

	// Build required metadata keys per REQ-ADP6-005.
	meta := map[string]any{
		"handle":       p.Author.Handle,
		"post_uri":     p.URI,
		"repost_count": p.RepostCount,
		"like_count":   p.LikeCount,
		"posted_at":    p.Record.CreatedAt,
		"sub_source":   "bluesky",
	}

	// Optional metadata keys.
	if p.CID != "" {
		meta["cid"] = p.CID
	}
	if p.Author.DID != "" {
		meta["did"] = p.Author.DID
	}
	if p.Author.DisplayName != "" {
		meta["display_name"] = p.Author.DisplayName
	}
	if p.ReplyCount != 0 {
		meta["reply_count"] = p.ReplyCount
	}
	if p.QuoteCount != 0 {
		meta["quote_count"] = p.QuoteCount
	}
	if p.IndexedAt != "" {
		meta["indexed_at"] = p.IndexedAt
	}
	if len(p.Record.Langs) > 0 {
		meta["langs"] = p.Record.Langs
	}

	return types.NormalizedDoc{
		ID:          rkey,
		SourceID:    "bluesky",
		URL:         postURL,
		Title:       "", // AT spec has no headline field (H2 fix: Title = "")
		Body:        p.Record.Text,
		Snippet:     snippet,
		PublishedAt: publishedAt,
		RetrievedAt: retrievedAt,
		Author:      author,
		Score:       normalizeScore(p.LikeCount, p.RepostCount),
		Lang:        lang,
		DocType:     types.DocTypePost,
		Citations:   nil,
		Metadata:    meta,
		Hash:        "",
	}, nil
}

// truncateRunes truncates s to at most maxRunes runes.
// If truncation occurs, "..." is appended (replacing the last 3 runes).
func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	count := 0
	target := maxRunes - 3
	for i, r := range s {
		if count >= target {
			return s[:i] + "..."
		}
		_ = r
		count++
	}
	return s + "..."
}
