// Package reddit — Reddit JSON Listing → []types.NormalizedDoc transform.
// REQ-ADP-006: parseListing maps Reddit's JSON envelope to NormalizedDoc.
package reddit

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/elymas/universal-search/pkg/types"
)

// redditListing is the top-level Reddit API response envelope.
type redditListing struct {
	Data struct {
		After    *string       `json:"after"`
		Children []redditChild `json:"children"`
	} `json:"data"`
}

// redditChild is a single item in the children array.
type redditChild struct {
	Kind string         `json:"kind"`
	Data redditPostData `json:"data"`
}

// redditPostData holds the fields for a t3 (link/self) post.
type redditPostData struct {
	Name                  string  `json:"name"`
	Permalink             string  `json:"permalink"`
	Title                 string  `json:"title"`
	Selftext              string  `json:"selftext"`
	CreatedUTC            float64 `json:"created_utc"`
	Author                string  `json:"author"`
	Score                 int     `json:"score"`
	Subreddit             string  `json:"subreddit"`
	Over18                bool    `json:"over_18"`
	NumComments           int     `json:"num_comments"`
	UpvoteRatio           float64 `json:"upvote_ratio"`
	URL                   string  `json:"url"`
	SubredditNamePrefixed string  `json:"subreddit_name_prefixed"`
	Ups                   int     `json:"ups"`
	Spoiler               bool    `json:"spoiler"`
	Locked                bool    `json:"locked"`
	Stickied              bool    `json:"stickied"`
	LinkFlairText         string  `json:"link_flair_text"`
	PostHint              string  `json:"post_hint"`
}

// snippetMaxRunes is the maximum rune length for NormalizedDoc.Snippet.
const snippetMaxRunes = 280

// parseListing parses a Reddit JSON Listing response body into NormalizedDoc
// values. retrievedAt is the timestamp to assign to each doc's RetrievedAt field
// (injected by the caller for determinism in tests).
//
// Returns:
//   - (docs, cursor, nil) on success; cursor is data.after (empty if null).
//   - (nil, "", nil) when the children array is empty.
//   - (nil, "", *SourceError{Permanent}) on malformed JSON.
//
// Children whose kind != "t3" are silently skipped per REQ-ADP-006.
// The last doc in the slice gets Metadata["next_cursor"] set when cursor != "".
//
// @MX:ANCHOR: [AUTO] NormalizedDoc field-mapping integrity gate. Every Reddit
// doc passes through this single transform function. A bug here corrupts every
// document returned by the adapter.
// @MX:REASON: fan_in = 1 (Search) but invariant-bearing; field-mapping changes
// require careful coordination with SPEC-IDX-001 and SPEC-SYN-001 consumers.
// @MX:SPEC: SPEC-ADP-001
func parseListing(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, string, error) {
	var listing redditListing
	if err := json.Unmarshal(body, &listing); err != nil {
		return nil, "", &types.SourceError{
			Adapter:  "reddit",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("reddit: malformed JSON response: %w", err),
		}
	}

	children := listing.Data.Children
	if len(children) == 0 {
		return nil, "", nil
	}

	cursor := ""
	if listing.Data.After != nil {
		cursor = *listing.Data.After
	}

	var docs []types.NormalizedDoc
	for _, child := range children {
		if child.Kind != "t3" {
			continue
		}
		doc := transformPost(child.Data, retrievedAt)
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

// transformPost converts a single Reddit post data struct into a NormalizedDoc.
// This is the canonical field mapping per SPEC-ADP-001 §6.3.
func transformPost(d redditPostData, retrievedAt time.Time) types.NormalizedDoc {
	url := "https://www.reddit.com" + d.Permalink
	snippet := buildSnippet(d.Selftext, d.Title)

	meta := map[string]any{
		"subreddit":    d.Subreddit,
		"over_18":      d.Over18,
		"num_comments": d.NumComments,
		"upvote_ratio": d.UpvoteRatio,
		"external_url": d.URL,
		"kind":         "t3",
	}

	// Optional metadata keys — included only when non-zero/non-empty.
	if d.SubredditNamePrefixed != "" {
		meta["subreddit_name_prefixed"] = d.SubredditNamePrefixed
	}
	if d.Ups != 0 {
		meta["ups"] = d.Ups
	}
	if d.Spoiler {
		meta["spoiler"] = d.Spoiler
	}
	if d.Locked {
		meta["locked"] = d.Locked
	}
	if d.Stickied {
		meta["stickied"] = d.Stickied
	}
	if d.LinkFlairText != "" {
		meta["link_flair_text"] = d.LinkFlairText
	}
	if d.PostHint != "" {
		meta["post_hint"] = d.PostHint
	}

	return types.NormalizedDoc{
		ID:          d.Name,
		SourceID:    "reddit",
		URL:         url,
		Title:       d.Title,
		Body:        d.Selftext,
		Snippet:     snippet,
		PublishedAt: time.Unix(int64(d.CreatedUTC), 0).UTC(),
		RetrievedAt: retrievedAt,
		Author:      d.Author,
		Score:       normalizeScore(d.Score),
		Lang:        "",
		DocType:     types.DocTypePost,
		Citations:   nil,
		Metadata:    meta,
		Hash:        "",
	}
}

// buildSnippet creates the Snippet field from selftext or title.
// Truncates to snippetMaxRunes runes, appending "..." when truncated.
// Falls back to title if selftext is empty.
func buildSnippet(selftext, title string) string {
	text := selftext
	if text == "" {
		text = title
	}
	return truncateRunes(text, snippetMaxRunes)
}

// truncateRunes truncates s to at most maxRunes runes.
// If truncation occurs, "..." is appended (replacing the last 3 runes).
func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	// Build a slice of up to (maxRunes - 3) runes, then append "..."
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
