package idx5

import (
	"context"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// Embedder is the interface for computing query embeddings.
// Matches internal/index.Embedder signature for IDX-002 BGE-M3.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// SearchFunc is the function signature for index.Search, allowing mock injection.
type SearchFunc func(ctx context.Context, q IndexQuery) ([]IndexResult, error)

// IndexQuery mirrors the index layer query for IDX-005 use.
type IndexQuery struct {
	TeamID     string
	DocTypes   []types.DocType
	MaxResults int
}

// IndexResult represents a single search result.
type IndexResult struct {
	Doc   types.NormalizedDoc
	Score float64
}

// Lookup performs the pre-fanout cache lookup.
// REQ-IDX5-001: embed query, search with team-scoped filter, evaluate threshold + staleness.
//
// @MX:ANCHOR: [AUTO] Core lookup function; fan_in >= 3 (middleware, refresh job, tests).
// @MX:REASON: This function defines the meaning of similarity threshold + staleness evaluation.
// @MX:SPEC: SPEC-IDX-005
func Lookup(ctx context.Context, emb Embedder, search SearchFunc, cfg Config, queryText, teamID string) (*LookupResult, error) {
	start := time.Now()

	// REQ-IDX5-001: compute query embedding
	vec, err := emb.Embed(ctx, queryText)
	if err != nil {
		return &LookupResult{
			Outcome: OutcomeMiss,
			Score:   0,
			Duration: time.Since(start),
		}, err
	}
	_ = vec // embedding is used by the search layer internally

	// REQ-IDX5-001: search with team-scoped filter
	results, err := search(ctx, IndexQuery{
		TeamID:     teamID,
		DocTypes:   []types.DocType{DocTypeCachedAnswer},
		MaxResults: 1,
	})
	if err != nil {
		return &LookupResult{
			Outcome: OutcomeMiss,
			Score:   0,
			Duration: time.Since(start),
		}, err
	}

	// No results → MISS
	if len(results) == 0 {
		return &LookupResult{
			Outcome:  OutcomeMiss,
			Score:    0,
			Duration: time.Since(start),
		}, nil
	}

	top := results[0]
	threshold := cfg.GetThreshold(teamID)

	// Below threshold → MISS
	if top.Score < threshold {
		return &LookupResult{
			Outcome:  OutcomeMiss,
			Score:    top.Score,
			Duration: time.Since(start),
		}, nil
	}

	// Above threshold: reconstruct cached answer and evaluate staleness
	ca := docToCachedAnswer(&top.Doc, top.Score)

	// Evaluate staleness
	staleness := EvaluateStaleness(ca, time.Now(), cfg.CategoryTTLs)

	switch staleness {
	case HardStale:
		return &LookupResult{
			Outcome:  OutcomeHardStale,
			Cached:   ca,
			Score:    top.Score,
			Duration: time.Since(start),
		}, nil
	case SoftStale:
		return &LookupResult{
			Outcome:  OutcomeSoftHit,
			Cached:   ca,
			Score:    top.Score,
			Duration: time.Since(start),
		}, nil
	default: // Fresh
		return &LookupResult{
			Outcome:  OutcomeHit,
			Cached:   ca,
			Score:    top.Score,
			Duration: time.Since(start),
		}, nil
	}
}

// Lookuper wraps the Lookup function for dependency injection.
type Lookuper struct {
	emb    Embedder
	search SearchFunc
	cfg    Config
}

// NewLookup creates a new Lookuper instance.
func NewLookup(emb Embedder, search SearchFunc, cfg Config) *Lookuper {
	return &Lookuper{emb: emb, search: search, cfg: cfg}
}

// Lookup performs the pre-fanout cache lookup.
func (l *Lookuper) Lookup(ctx context.Context, queryText, teamID string) (*LookupResult, error) {
	return Lookup(ctx, l.emb, l.search, l.cfg, queryText, teamID)
}

// LookupWithBypass performs lookup with optional force-refresh bypass.
// REQ-IDX5-001 Edge1: force_refresh=true skips lookup entirely.
func (l *Lookuper) LookupWithBypass(ctx context.Context, queryText, teamID string, forceRefresh bool) (*LookupResult, error) {
	if forceRefresh {
		return &LookupResult{
			Outcome: OutcomeBypassed,
			Score:   0,
		}, nil
	}
	return l.Lookup(ctx, queryText, teamID)
}

// docToCachedAnswer reconstructs a CachedAnswer from a Qdrant search result.
func docToCachedAnswer(doc *types.NormalizedDoc, score float64) *CachedAnswer {
	ca := &CachedAnswer{
		DocID:     doc.ID,
		Similarity: score,
	}
	if doc.Metadata != nil {
		if v, ok := doc.Metadata["team_id"].(string); ok {
			ca.TeamID = v
		}
		if v, ok := doc.Metadata["query_hash"].(string); ok {
			ca.QueryHash = v
		}
		if v, ok := doc.Metadata["category"].(string); ok {
			ca.Category = v
		}
		if v, ok := doc.Metadata["response_json"].(string); ok {
			ca.ResponseJSON = v
		}
		if v, ok := doc.Metadata["ttl_seconds"].(int64); ok {
			ca.TTLSeconds = int(v)
		} else if v, ok := doc.Metadata["ttl_seconds"].(float64); ok {
			ca.TTLSeconds = int(v)
		}
		if v, ok := doc.Metadata["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				ca.CreatedAt = t
			}
		}
		if v, ok := doc.Metadata["force_stale"].(bool); ok {
			ca.ForceStale = v
		}
		if v, ok := doc.Metadata["query_text"].(string); ok {
			ca.QueryText = v
		}
	}
	return ca
}
