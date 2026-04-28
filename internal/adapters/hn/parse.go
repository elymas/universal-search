// Package hn — Algolia HN JSON response → []types.NormalizedDoc transform.
// REQ-ADP2-005: parseHits maps the Algolia HN hit envelope to NormalizedDoc.
package hn

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/elymas/universal-search/pkg/types"
)

// algoliaResponse is the top-level Algolia HN API response envelope.
type algoliaResponse struct {
	Hits        []algoliaHit `json:"hits"`
	NbHits      int          `json:"nbHits"`
	Page        int          `json:"page"`
	NbPages     int          `json:"nbPages"`
	HitsPerPage int          `json:"hitsPerPage"`
}

// algoliaHit holds the fields for a single Algolia HN item.
// _highlightResult is intentionally omitted — IGNORED per SPEC §2.2.
type algoliaHit struct {
	ObjectID    string   `json:"objectID"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Author      string   `json:"author"`
	Points      int      `json:"points"`
	StoryText   string   `json:"story_text"`
	NumComments int      `json:"num_comments"`
	CreatedAtI  int64    `json:"created_at_i"`
	Tags        []string `json:"_tags"`
}

// snippetMaxRunes is the maximum rune length for NormalizedDoc.Snippet.
const snippetMaxRunes = 280

// parseHits parses the Algolia HN response body into NormalizedDoc values.
// retrievedAt is the timestamp to assign to each doc's RetrievedAt field
// (injected by the caller for determinism in tests).
//
// Returns:
//   - (docs, nextCursor, nil) on success; nextCursor is "" when no further pages exist.
//   - (nil, "", nil) when hits is empty.
//   - (nil, "", *SourceError{Permanent}) on malformed JSON.
//
// Hits whose _tags array does NOT contain "story" are silently skipped (defensive
// client-side filter against Algolia API drift per REQ-ADP2-005).
// The next-page cursor is set on the LAST returned doc's Metadata["next_cursor"].
//
// @MX:ANCHOR: [AUTO] NormalizedDoc field-mapping integrity gate. Every HN hit passes
// through this single transform. A bug here corrupts every document returned by the adapter.
// @MX:REASON: fan_in = 1 (Search) but invariant-bearing; field-mapping changes require
// careful coordination with SPEC-IDX-001 and SPEC-SYN-001 consumers.
// @MX:SPEC: SPEC-ADP-002
func parseHits(body []byte, retrievedAt time.Time) ([]types.NormalizedDoc, string, error) {
	var resp algoliaResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", &types.SourceError{
			Adapter:  "hackernews",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("hn: malformed JSON response: %w", err),
		}
	}

	if len(resp.Hits) == 0 {
		return nil, "", nil
	}

	// Determine next-page cursor.
	nextCursor := ""
	if resp.Page+1 < resp.NbPages {
		nextCursor = strconv.Itoa(resp.Page + 1)
	}

	docs := make([]types.NormalizedDoc, 0, len(resp.Hits))
	for _, hit := range resp.Hits {
		// Defensive: skip hits that are not tagged as "story".
		if !hasTag(hit.Tags, "story") {
			continue
		}
		doc := transformHit(hit, retrievedAt)
		docs = append(docs, doc)
	}

	if len(docs) == 0 {
		return nil, "", nil
	}

	// Surface the pagination cursor on the last returned doc only.
	if nextCursor != "" {
		last := &docs[len(docs)-1]
		if last.Metadata == nil {
			last.Metadata = make(map[string]any)
		}
		last.Metadata["next_cursor"] = nextCursor
	}

	return docs, nextCursor, nil
}

// hasTag reports whether the tags slice contains the given tag string (case-sensitive).
func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// transformHit converts a single Algolia HN hit into a NormalizedDoc.
// This is the canonical field mapping per SPEC-ADP-002 §6.3.
func transformHit(h algoliaHit, retrievedAt time.Time) types.NormalizedDoc {
	// Two-branch URL construction: external URL for link posts, HN permalink for self-posts.
	docURL := h.URL
	if docURL == "" {
		docURL = "https://news.ycombinator.com/item?id=" + h.ObjectID
	}

	// HTML-strip story_text for Body and Snippet.
	body := ""
	if h.StoryText != "" {
		body = stripHTML(h.StoryText)
	}
	snippet := buildSnippet(body, h.Title)

	meta := map[string]any{
		"num_comments": h.NumComments,
		"points":       h.Points,
		"tags":         filterInternalTags(h.Tags),
		"external_url": h.URL, // empty string when self-post
	}

	return types.NormalizedDoc{
		ID:          h.ObjectID,
		SourceID:    "hackernews",
		URL:         docURL,
		Title:       h.Title,
		Body:        body,
		Snippet:     snippet,
		PublishedAt: time.Unix(h.CreatedAtI, 0).UTC(),
		RetrievedAt: retrievedAt,
		Author:      h.Author,
		Score:       normalizeScore(h.Points),
		Lang:        "",
		DocType:     types.DocTypePost,
		Citations:   nil,
		Metadata:    meta,
		Hash:        "",
	}
}

// filterInternalTags returns a copy of tags with Algolia-internal tags
// (e.g., "story_<id>", "author_<name>") removed. Story-type tags like
// "story", "ask_hn", "show_hn", "front_page" are preserved.
func filterInternalTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	for _, t := range tags {
		// Skip internal author_ and story_ prefixed tags.
		if len(t) > 7 && t[:7] == "author_" {
			continue
		}
		if len(t) > 6 && t[:6] == "story_" {
			continue
		}
		result = append(result, t)
	}
	return result
}

// buildSnippet creates the Snippet field from body text or title.
// Truncates to snippetMaxRunes runes. Falls back to title if body is empty.
func buildSnippet(body, title string) string {
	text := body
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
