// Package youtube — sidecar JSON envelope parsing and NormalizedDoc transform.
// REQ-ADP5-005: field mapping from YTItem → NormalizedDoc per §6.3.
package youtube

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/elymas/universal-search/pkg/types"
)

// maxTranscriptRunes is the maximum number of runes stored in
// Metadata["transcript_snippet"]. Defensive cap against sidecar contract drift.
const maxTranscriptRunes = 500

// maxSnippetRunes is the maximum rune count for NormalizedDoc.Snippet.
const maxSnippetRunes = 280

// ytSearchResponse is the sidecar's JSON envelope for POST /search.
type ytSearchResponse struct {
	Items   []ytItem       `json:"items"`
	HasMore bool           `json:"has_more"`
	Error   *ytErrEnvelope `json:"error,omitempty"`
}

// ytErrEnvelope is the sidecar's error envelope when HTTP status ≠ 200.
type ytErrEnvelope struct {
	Category string `json:"category"`
	Message  string `json:"message"`
	// Some 503 responses use "reason" instead of "message".
	Reason string `json:"reason,omitempty"`
}

// ytItem represents one video entry in the sidecar response.
type ytItem struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Channel     string `json:"channel"`
	ChannelID   string `json:"channel_id"`
	ChannelURL  string `json:"channel_url"`
	Uploader    string `json:"uploader"`
	UploaderID  string `json:"uploader_id"`
	// DurationSeconds is 0 for livestream-archived videos.
	DurationSeconds int64 `json:"duration_seconds"`
	// ViewCount may be null in JSON for livestream-archived videos; use pointer.
	ViewCount    *int64   `json:"view_count"`
	LikeCount    *int64   `json:"like_count"`
	UploadDate   string   `json:"upload_date"`
	ThumbnailURL string   `json:"thumbnail_url"`
	Tags         []string `json:"tags"`
	// AvailableTranscriptLangs is always populated (may be empty).
	AvailableTranscriptLangs []string `json:"available_transcript_langs"`
	TranscriptSnippet        string   `json:"transcript_snippet"`
	TranscriptLang           string   `json:"transcript_lang"`
	TranscriptIsAuto         bool     `json:"transcript_is_auto"`
	// Error signals a per-item extraction failure; such items are skipped silently.
	Error *string `json:"error,omitempty"`
}

// parseSearchResponse parses the sidecar's JSON body into []types.NormalizedDoc.
// Parameters:
//   - body: raw JSON bytes from the sidecar response
//   - retrievedAt: set as NormalizedDoc.RetrievedAt for all returned docs
//   - currentOffset: the cursor offset that was sent in the request (int)
//   - selectedLang: the transcript_lang used in the request (e.g. "ko" or "en")
//
// Returns (docs, nextCursor, error):
//   - docs: successfully-parsed NormalizedDocs; items with a per-item error field
//     are silently skipped.
//   - nextCursor: decimal-string offset for the next page when has_more=true AND
//     currentOffset+len(items) < 100; empty string otherwise.
//   - error: *types.SourceError{CategoryPermanent} on JSON parse failure or
//     top-level sidecar error envelope.
//
// @MX:ANCHOR: [AUTO] Every YouTube item passes through this single transform.
// @MX:REASON: NormalizedDoc field-mapping integrity gate; bug here corrupts every
// returned doc. All field mappings from §6.3 are implemented here.
// @MX:SPEC: SPEC-ADP-005
func parseSearchResponse(body []byte, retrievedAt time.Time, currentOffset int, selectedLang string) ([]types.NormalizedDoc, string, error) {
	var resp ytSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", &types.SourceError{
			Adapter:  "youtube",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("youtube: malformed JSON from sidecar: %w", err),
		}
	}

	// Top-level error envelope (used when sidecar returns an error body at HTTP 200,
	// but primarily for bodies read after non-200 status codes).
	if resp.Error != nil {
		msg := resp.Error.Message
		if msg == "" {
			msg = resp.Error.Reason
		}
		cat := sidecarCategoryToType(resp.Error.Category)
		return nil, "", &types.SourceError{
			Adapter:  "youtube",
			Category: cat,
			Cause:    fmt.Errorf("youtube: sidecar error: %s", msg),
		}
	}

	if len(resp.Items) == 0 {
		return nil, "", nil
	}

	docs := make([]types.NormalizedDoc, 0, len(resp.Items))
	for _, item := range resp.Items {
		// Skip items with per-item extraction errors silently (sole-emitter discipline).
		if item.Error != nil {
			continue
		}

		doc := transformItem(item, retrievedAt, selectedLang)
		docs = append(docs, doc)
	}

	// Compute next-cursor: only when has_more=true AND cap not hit.
	var nextCursor string
	nextOffset := currentOffset + len(resp.Items)
	if resp.HasMore && nextOffset < 100 {
		nextCursor = strconv.Itoa(nextOffset)
	}

	// Surface next_cursor on the LAST returned doc (same pattern as HN adapter).
	if nextCursor != "" && len(docs) > 0 {
		last := &docs[len(docs)-1]
		if last.Metadata == nil {
			last.Metadata = make(map[string]any)
		}
		last.Metadata["next_cursor"] = nextCursor
	}

	return docs, nextCursor, nil
}

// transformItem converts one ytItem into a types.NormalizedDoc per §6.3.
func transformItem(item ytItem, retrievedAt time.Time, selectedLang string) types.NormalizedDoc {
	viewCount := int64(0)
	if item.ViewCount != nil {
		viewCount = *item.ViewCount
	}

	// PublishedAt: parse "2006-01-02" date string, UTC midnight.
	var publishedAt time.Time
	if item.UploadDate != "" {
		t, err := time.Parse("2006-01-02", item.UploadDate)
		if err == nil {
			publishedAt = t.UTC()
		}
	}

	// Author: channel display name, falls back to uploader.
	author := item.Channel
	if author == "" {
		author = item.Uploader
	}

	// Lang priority: selectedLang (from filter/auto-detect) > sidecar transcript_lang > "".
	lang := selectedLang
	if lang == "" {
		lang = item.TranscriptLang
	}

	// Snippet: first 280 runes of description, or transcript_snippet, or title.
	snippet := truncateRunes(item.Description, maxSnippetRunes)
	if snippet == "" {
		snippet = truncateRunes(item.TranscriptSnippet, maxSnippetRunes)
	}
	if snippet == "" {
		snippet = truncateRunes(item.Title, maxSnippetRunes)
	}

	// Build required Metadata keys.
	langs := item.AvailableTranscriptLangs
	if langs == nil {
		langs = []string{}
	}
	meta := map[string]any{
		"channel_id":                 item.ChannelID,
		"channel_url":                item.ChannelURL,
		"duration_seconds":           item.DurationSeconds,
		"view_count":                 viewCount,
		"thumbnail_url":              item.ThumbnailURL,
		"available_transcript_langs": langs,
	}

	// Optional Metadata keys.
	if item.LikeCount != nil {
		meta["like_count"] = *item.LikeCount
	}
	if len(item.Tags) > 0 {
		meta["tags"] = item.Tags
	}
	if item.UploaderID != "" {
		meta["uploader_id"] = item.UploaderID
	}
	if item.TranscriptSnippet != "" {
		snippet500 := truncateRunes(item.TranscriptSnippet, maxTranscriptRunes)
		meta["transcript_snippet"] = snippet500
		meta["transcript_lang"] = item.TranscriptLang
		meta["transcript_is_auto"] = item.TranscriptIsAuto
	}

	return types.NormalizedDoc{
		ID:          item.ID,
		SourceID:    "youtube",
		URL:         item.URL,
		Title:       item.Title,
		Body:        item.Description,
		Snippet:     snippet,
		PublishedAt: publishedAt,
		RetrievedAt: retrievedAt,
		Author:      author,
		Score:       normalizeViewScore(viewCount),
		Lang:        lang,
		DocType:     types.DocTypeVideo,
		Citations:   nil,
		Metadata:    meta,
		Hash:        "",
	}
}

// truncateRunes returns s truncated to at most maxRunes runes, appending "…"
// if truncation occurred. Returns empty string when s is empty.
func truncateRunes(s string, maxRunes int) string {
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "…"
}

// sidecarCategoryToType maps the sidecar's string category to types.Category.
func sidecarCategoryToType(cat string) types.Category {
	switch cat {
	case "rate_limited":
		return types.CategoryRateLimited
	case "permanent":
		return types.CategoryPermanent
	case "transient":
		return types.CategoryTransient
	case "unavailable":
		return types.CategoryUnavailable
	default:
		return types.CategoryUnavailable
	}
}

// parseErrorEnvelope attempts to decode a sidecar error envelope from body.
// Returns nil when the body is not a valid error envelope.
func parseErrorEnvelope(body []byte) *ytErrEnvelope {
	var env struct {
		Error *ytErrEnvelope `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil
	}
	return env.Error
}
