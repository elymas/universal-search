// Package types — NormalizedDoc canonical search-result type.
// REQ-CORE-001: 15-field struct with Validate and CanonicalHash methods.
// REQ-CORE-007: Validate returns *ValidationError on missing required fields.
package types

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

// NormalizedDoc is the canonical shape every search adapter returns.
// All adapters (Reddit, HN, arXiv, GitHub, YouTube, Bluesky, X, SearXNG, Naver,
// Daum, KoreaNewsCrawler, RSS, Polymarket, ...) converge to this type so
// downstream layers (Intent Router, Fanout, Index ingestion, Synthesis) can
// process a single shape regardless of source.
//
// Field semantics:
//   - ID: unique within (SourceID, URL); adapter-assigned.
//   - SourceID: matches Adapter.Name(); used as Prometheus label value.
//   - URL: canonical URL — adapters MUST normalize away tracking params.
//   - Title / Body / Snippet: text content. Body is the ranking input;
//     Snippet is short UI excerpt.
//   - PublishedAt: zero (time.Time{}) when the source provides no date.
//   - RetrievedAt: when this adapter saw the doc; required.
//   - Score: normalized [0.0, 1.0]; 0 means unscored, NOT zero engagement.
//   - Lang: BCP-47; empty means unknown.
//   - Citations: doc IDs referenced by this doc; SPEC-SYN-002 consumes for
//     per-claim provenance.
//   - Metadata: adapter-specific extension bag; NOT part of CanonicalHash.
//   - Hash: cached CanonicalHash() output for JSON round-trip; recomputed by
//     callers who need a fresh value.
//
// @MX:NOTE: [AUTO] URL must be canonical (no tracking params). Hash is
// content-only and excludes Metadata so adapter-specific fields cannot
// produce false dedup misses. Score == 0.0 means unscored, not zero
// engagement. Lang is BCP-47; empty means unknown.
// @MX:SPEC: SPEC-CORE-001
type NormalizedDoc struct {
	ID          string         `json:"id"`
	SourceID    string         `json:"source_id"`
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Snippet     string         `json:"snippet"`
	PublishedAt time.Time      `json:"published_at"`
	RetrievedAt time.Time      `json:"retrieved_at"`
	Author      string         `json:"author"`
	Score       float64        `json:"score"`
	Lang        string         `json:"lang"`
	DocType     DocType        `json:"doc_type"`
	Citations   []string       `json:"citations,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Hash        string         `json:"hash"`
}

// Validate returns nil for a complete doc, or a *ValidationError naming the
// first missing required field. Required fields: ID, SourceID, URL, RetrievedAt.
//
// Validate does not coerce, default, or auto-fill any field — the caller is
// responsible for constructing valid docs. NFR-CORE-001 mandates < 1 µs/op.
func (d *NormalizedDoc) Validate() error {
	if d.ID == "" {
		return &ValidationError{Field: "ID", Cause: errors.New("empty")}
	}
	if d.SourceID == "" {
		return &ValidationError{Field: "SourceID", Cause: errors.New("empty")}
	}
	if d.URL == "" {
		return &ValidationError{Field: "URL", Cause: errors.New("empty")}
	}
	if d.RetrievedAt.IsZero() {
		return &ValidationError{Field: "RetrievedAt", Cause: errors.New("zero time")}
	}
	return nil
}

// CanonicalHash returns a 16-character lowercase hex hash derived from the
// content quartet {SourceID, URL, Title, Body}. Metadata is intentionally
// excluded so dedup decisions are stable across adapters that enrich Metadata
// differently.
//
// The hash is the first 16 hex chars of SHA-256 over a stable separator-
// delimited byte stream. Two calls on the same doc return byte-equal output.
//
// @MX:NOTE: [AUTO] Hash is content-only (SourceID|URL|Title|Body) and
// truncated to 16 hex chars. Collisions are theoretically possible but
// negligible at our 14-adapter scale; revisit if dedup precision degrades.
// @MX:SPEC: SPEC-CORE-001
func (d *NormalizedDoc) CanonicalHash() string {
	h := sha256.New()
	// NUL separator prevents field-boundary smuggling (e.g., a Title ending
	// in the value of the next field). NUL cannot appear in canonical URLs
	// or our text fields in practice, but the separator is defensive.
	const sep = "\x00"
	_, _ = h.Write([]byte(d.SourceID))
	_, _ = h.Write([]byte(sep))
	_, _ = h.Write([]byte(d.URL))
	_, _ = h.Write([]byte(sep))
	_, _ = h.Write([]byte(d.Title))
	_, _ = h.Write([]byte(sep))
	_, _ = h.Write([]byte(d.Body))
	full := h.Sum(nil)
	return hex.EncodeToString(full[:8]) // 8 bytes = 16 hex chars
}
