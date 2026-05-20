// Package searxng — SearXNG JSON response → []types.NormalizedDoc transform.
// REQ-ADP7-005: parseSearch maps SearXNG's JSON envelope to NormalizedDoc.
package searxng

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/elymas/universal-search/pkg/types"
)

// searxngResponse is the top-level SearXNG JSON response envelope.
type searxngResponse struct {
	Query           string       `json:"query"`
	NumberOfResults int          `json:"number_of_results"`
	Results         []searxngHit `json:"results"`
	// Suggestions/Corrections/Answers/Infoboxes/UnresponsiveEngines
	// intentionally omitted — IGNORED in v0.1 per SPEC-ADP-007 §2.2.
}

// searxngHit holds the fields for a single SearXNG result entry.
type searxngHit struct {
	URL           string   `json:"url"`
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	Engine        string   `json:"engine"`
	Engines       []string `json:"engines"`
	Category      string   `json:"category"`
	Score         float64  `json:"score"`
	Template      string   `json:"template"`
	Positions     []int    `json:"positions"`
	PublishedDate string   `json:"publishedDate"`
}

// snippetMaxRunes is the maximum rune length for NormalizedDoc.Snippet.
const snippetMaxRunes = 280

// parseSearch parses a SearXNG JSON response body into NormalizedDoc values.
// retrievedAt is the timestamp to assign to each doc's RetrievedAt field
// (injected by the caller for determinism in tests).
// currentPage is the 1-based page number of the current request.
//
// Returns:
//   - (docs, nextCursor, nil) on success; nextCursor is strconv.Itoa(currentPage+1)
//     when len(results) > 0.
//   - (nil, "", nil) when the results array is empty.
//   - (nil, "", *SourceError{Permanent}) on malformed JSON.
//
// The last doc in the slice gets Metadata["next_cursor"] set to nextCursor.
//
// @MX:ANCHOR: [AUTO] NormalizedDoc field-mapping integrity gate. Every SearXNG
// hit passes through this single transform function. A bug here corrupts every
// document returned by the adapter.
// @MX:REASON: NormalizedDoc field-mapping integrity gate; engine-of-origin
// Metadata contract is consumer-visible (SPEC-IDX-001 RRF, SPEC-FAN-001 dedup).
// @MX:SPEC: SPEC-ADP-007
func parseSearch(body []byte, retrievedAt time.Time, currentPage int) ([]types.NormalizedDoc, string, error) {
	var resp searxngResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", &types.SourceError{
			Adapter:  "searxng",
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("searxng: malformed JSON response: %w", err),
		}
	}

	if len(resp.Results) == 0 {
		return nil, "", nil
	}

	docs := make([]types.NormalizedDoc, 0, len(resp.Results))
	for _, hit := range resp.Results {
		doc := transformHit(hit, retrievedAt)
		docs = append(docs, doc)
	}

	// Surface the pagination cursor on the last returned doc.
	// @MX:NOTE: [AUTO] Metadata["engine"] / Metadata["engines"] / Metadata["category"]
	// are the engine-of-origin contract for downstream RRF (SPEC-IDX-001) and
	// dedup (SPEC-FAN-001 §2.4) consumers.
	// @MX:SPEC: SPEC-ADP-007
	nextCursor := strconv.Itoa(currentPage + 1)
	last := &docs[len(docs)-1]
	if last.Metadata == nil {
		last.Metadata = make(map[string]any)
	}
	last.Metadata["next_cursor"] = nextCursor

	return docs, nextCursor, nil
}

// transformHit converts a single SearXNG hit into a NormalizedDoc.
// This is the canonical field mapping per SPEC-ADP-007 §6.3.
func transformHit(h searxngHit, retrievedAt time.Time) types.NormalizedDoc {
	// Derive ID from URL via sha256 hash (SearXNG does not assign per-result IDs).
	sum := sha256.Sum256([]byte(h.URL))
	id := "searxng:" + hex.EncodeToString(sum[:8])

	// Parse publishedDate if non-empty; leave zero on failure (graceful degradation).
	var publishedAt time.Time
	if h.PublishedDate != "" {
		if t, err := time.Parse(time.RFC3339, h.PublishedDate); err == nil {
			publishedAt = t
		}
		// On parse failure, publishedAt remains zero (no error per REQ-ADP7-005).
	}

	// Clamp score to [0.0, 1.0]. §2.3: direct clamp, not Tanh.
	score := math.Max(0.0, math.Min(1.0, h.Score))

	// Build snippet: rune-truncate content to 280 runes.
	snippet := truncateRunes(h.Content, snippetMaxRunes)

	// Engine-of-origin metadata (REQ-ADP7-005 contractual minimum).
	engines := h.Engines
	if len(engines) == 0 {
		// M4 fallback: when engines field is null, missing, or empty, use single-engine list.
		if h.Engine != "" {
			engines = []string{h.Engine}
		} else {
			engines = []string{}
		}
	}
	primaryEngine := h.Engine
	if primaryEngine == "" && len(engines) > 0 {
		primaryEngine = engines[0]
	}

	meta := map[string]any{
		"engine":    primaryEngine,
		"engines":   engines,
		"category":  h.Category,
		"score_raw": h.Score,
	}

	// Optional metadata keys.
	if h.Template != "" {
		meta["template"] = h.Template
	}
	if len(h.Positions) > 0 {
		meta["positions"] = h.Positions
	}

	return types.NormalizedDoc{
		ID:          id,
		SourceID:    "searxng",
		URL:         h.URL,
		Title:       h.Title,
		Body:        h.Content,
		Snippet:     snippet,
		PublishedAt: publishedAt,
		RetrievedAt: retrievedAt,
		Author:      "", // SearXNG aggregates from many engines; per-result authorship not consistently surfaced.
		Score:       score,
		Lang:        "", // SearXNG does not expose per-result language.
		DocType:     types.DocTypeArticle,
		Citations:   nil,
		Metadata:    meta,
		Hash:        "", // Consumers compute via CanonicalHash().
	}
}

// truncateRunes truncates s to at most maxRunes runes.
// If truncation occurs, "..." is appended (replacing the last 3 rune positions).
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
