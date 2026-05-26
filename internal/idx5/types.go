// Package idx5 implements team-shared answer reuse via pre-fanout lookup
// with configurable staleness (SPEC-IDX-005).
//
// Lookup Pipeline:
//  1. Extract team_id from JWT context (IDX-004 enforcement)
//  2. Compute query embedding via IDX-002 embedder
//  3. Search Qdrant with team-scoped filter + DocTypeCachedAnswer
//  4. Evaluate similarity threshold + staleness
//  5. HIT → serve cached SynthesizeResponse with cache headers
//  6. MISS → fall through to fanout, async write-back
//
// @MX:NOTE: [AUTO] This package is the PRIMARY DRIVER of the M6 exit criterion
// (dedup hit rate >= 30%). The middleware intercepts /query before fanout.
// @MX:SPEC: SPEC-IDX-005
package idx5

import (
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// DocTypeCachedAnswer is the canonical DocType for cached answer documents.
// REQ-IDX5-006: stored in Qdrant payload and PG answer_cache table.
var DocTypeCachedAnswer = types.DocType("cached_answer")

// Staleness classifies the freshness of a cached answer.
// REQ-IDX5-003: age-based evaluation with per-category TTL.
type Staleness string

const (
	// Fresh means age < 0.5 * TTL. Serve synchronously.
	Fresh Staleness = "fresh"
	// SoftStale means 0.5 * TTL <= age < TTL. Serve + async refresh.
	SoftStale Staleness = "soft_stale"
	// HardStale means age >= TTL or force_stale=true. MISS path.
	HardStale Staleness = "hard_stale"
)

// LookupOutcome enumerates the possible results of a cache lookup.
type LookupOutcome string

const (
	OutcomeHit       LookupOutcome = "hit"
	OutcomeSoftHit   LookupOutcome = "soft_hit"
	OutcomeMiss      LookupOutcome = "miss"
	OutcomeHardStale LookupOutcome = "hard_stale"
	OutcomeBypassed  LookupOutcome = "bypassed"
)

// CachedAnswer represents a cached synthesis response stored in PG and Qdrant.
// REQ-IDX5-006: durable storage record for team-shared answer reuse.
type CachedAnswer struct {
	// DocID is the unique identifier: "answer-cache:<queryHash>:<team_id>".
	DocID string `json:"doc_id"`
	// TeamID is the tenant isolation key (IDX-004 enforcement).
	TeamID string `json:"team_id"`
	// QueryHash is the SHA-256 hex of the original query text.
	QueryHash string `json:"query_hash"`
	// QueryText is the original query that produced this answer.
	QueryText string `json:"query_text"`
	// Category is the document category (web, social, academic, korean, mixed, unknown).
	Category string `json:"category"`
	// ResponseJSON is the serialized SYN-001 SynthesizeResponse.
	ResponseJSON string `json:"response_json"`
	// Similarity is the cosine similarity score at cache time.
	Similarity float64 `json:"similarity"`
	// TTLSeconds is the time-to-live in seconds for this category.
	TTLSeconds int `json:"ttl_seconds"`
	// CreatedAt is when this cached answer was first written.
	CreatedAt time.Time `json:"created_at"`
	// LastServedAt is the most recent time this answer was served.
	LastServedAt time.Time `json:"last_served_at"`
	// HitCount is the number of times this answer was served.
	HitCount int `json:"hit_count"`
	// ForceStale is set to true by feedback thumbs-down (REQ-IDX5-008).
	ForceStale bool `json:"force_stale"`
}

// LookupResult carries the outcome and metadata of a cache lookup.
// REQ-IDX5-002: returned by the Lookup function to the middleware.
type LookupResult struct {
	// Outcome is the lookup result classification.
	Outcome LookupOutcome `json:"outcome"`
	// Cached is the matched cached answer (nil on MISS/BYPASSED).
	Cached *CachedAnswer `json:"cached,omitempty"`
	// Score is the cosine similarity of the top-1 match.
	Score float64 `json:"score"`
	// Duration is the wall-clock time spent in the lookup pipeline.
	Duration time.Duration `json:"duration"`
}

// Citation represents a synthesis citation with URL for re-validation.
// REQ-IDX5-004: input/output of RevalidateCitations. Mirrors the shape of
// synthesis.Citation and deepreport.Citation; kept local to avoid a
// cross-package import for the cache pipeline.
type Citation struct {
	Marker int    `json:"marker"`
	DocID  string `json:"doc_id"`
	URL    string `json:"url"`
	Title  string `json:"title"`
}
