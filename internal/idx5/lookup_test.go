package idx5

import (
	"context"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// mockEmbedder records calls and returns a preset embedding.
type mockEmbedder struct {
	called int
	vec    []float32
	err    error
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	m.called++
	return m.vec, m.err
}

// mockIndex records search calls and returns preset results.
type mockIndex struct {
	searchCalled int
	lastQuery    *IndexQuery
	results      []IndexResult
	err          error
}

func (m *mockIndex) Search(ctx context.Context, q IndexQuery) ([]IndexResult, error) {
	m.searchCalled++
	qcopy := q
	m.lastQuery = &qcopy
	return m.results, m.err
}

// IndexQuery and IndexResult are defined in lookup.go

func TestLookupCallsEmbedderOnce(t *testing.T) {
	// REQ-IDX5-001: embedder must be called exactly once
	emb := &mockEmbedder{vec: make([]float32, 128)}
	idx := &mockIndex{}
	cfg := DefaultConfig()

	lk := NewLookup(emb, idx.Search, cfg)
	_, _ = lk.Lookup(context.Background(), "test query", "team-T")

	if emb.called != 1 {
		t.Errorf("embedder called %d times, want 1", emb.called)
	}
}

func TestLookupCallsIndexSearchWithTeamScopedFilter(t *testing.T) {
	// REQ-IDX5-001: IndexQuery.TeamID and DocTypes=[DocTypeCachedAnswer], MaxResults=1
	emb := &mockEmbedder{vec: make([]float32, 128)}
	idx := &mockIndex{}
	cfg := DefaultConfig()

	lk := NewLookup(emb, idx.Search, cfg)
	_, _ = lk.Lookup(context.Background(), "test query", "team-T")

	if idx.searchCalled != 1 {
		t.Errorf("index.Search called %d times, want 1", idx.searchCalled)
	}
	q := idx.lastQuery
	if q.TeamID != "team-T" {
		t.Errorf("TeamID = %q, want %q", q.TeamID, "team-T")
	}
	if len(q.DocTypes) != 1 || q.DocTypes[0] != DocTypeCachedAnswer {
		t.Errorf("DocTypes = %v, want [cached_answer]", q.DocTypes)
	}
	if q.MaxResults != 1 {
		t.Errorf("MaxResults = %d, want 1", q.MaxResults)
	}
}

func TestLookupHitAboveThresholdServesCached(t *testing.T) {
	// REQ-IDX5-002: score 0.95 + fresh → hit
	emb := &mockEmbedder{vec: make([]float32, 128)}
	now := time.Now()
	idx := &mockIndex{
		results: []IndexResult{
			{
				Doc: types.NormalizedDoc{
					ID:          "answer-cache:abc123:team-T",
					SourceID:    "idx5",
					URL:         "cache://team-T/abc123",
					RetrievedAt: now,
					DocType:     DocTypeCachedAnswer,
					Metadata: map[string]any{
						"team_id":      "team-T",
						"query_hash":   "abc123",
						"category":     "web",
						"response_json": `{"text":"hello"}`,
						"ttl_seconds":  int64(3600),
						"created_at":   now.Format(time.RFC3339),
						"force_stale":  false,
					},
				},
				Score: 0.95,
			},
		},
	}
	cfg := DefaultConfig()

	lk := NewLookup(emb, idx.Search, cfg)
	result, err := lk.Lookup(context.Background(), "test query", "team-T")

	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}
	if result.Outcome != OutcomeHit {
		t.Errorf("Outcome = %v, want OutcomeHit", result.Outcome)
	}
	if result.Score != 0.95 {
		t.Errorf("Score = %f, want 0.95", result.Score)
	}
	if result.Cached == nil {
		t.Fatal("Cached should not be nil on hit")
	}
}

func TestLookupMissBelowThresholdFallsThrough(t *testing.T) {
	// REQ-IDX5-002: score 0.89 → MISS (below default threshold 0.92)
	emb := &mockEmbedder{vec: make([]float32, 128)}
	idx := &mockIndex{
		results: []IndexResult{
			{
				Doc: types.NormalizedDoc{
					ID:          "answer-cache:abc123:team-T",
					SourceID:    "idx5",
					URL:         "cache://team-T/abc123",
					RetrievedAt: time.Now(),
					DocType:     DocTypeCachedAnswer,
				},
				Score: 0.89,
			},
		},
	}
	cfg := DefaultConfig()

	lk := NewLookup(emb, idx.Search, cfg)
	result, err := lk.Lookup(context.Background(), "test query", "team-T")

	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}
	if result.Outcome != OutcomeMiss {
		t.Errorf("Outcome = %v, want OutcomeMiss", result.Outcome)
	}
}

func TestLookupNoResultsIsMiss(t *testing.T) {
	// REQ-IDX5-002: empty results → MISS
	emb := &mockEmbedder{vec: make([]float32, 128)}
	idx := &mockIndex{results: nil}
	cfg := DefaultConfig()

	lk := NewLookup(emb, idx.Search, cfg)
	result, err := lk.Lookup(context.Background(), "test query", "team-T")

	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}
	if result.Outcome != OutcomeMiss {
		t.Errorf("Outcome = %v, want OutcomeMiss", result.Outcome)
	}
}

func TestLookupPerTeamThresholdOverride(t *testing.T) {
	// REQ-IDX5-002: team-specific threshold override
	emb := &mockEmbedder{vec: make([]float32, 128)}
	now := time.Now()
	idx := &mockIndex{
		results: []IndexResult{
			{
				Doc: types.NormalizedDoc{
					ID:          "answer-cache:abc123:team-low",
					SourceID:    "idx5",
					URL:         "cache://team-low/abc123",
					RetrievedAt: now,
					DocType:     DocTypeCachedAnswer,
					Metadata: map[string]any{
						"team_id":       "team-low",
						"query_hash":    "abc123",
						"category":      "web",
						"response_json": `{"text":"hello"}`,
						"ttl_seconds":   int64(3600),
						"created_at":    now.Format(time.RFC3339),
						"force_stale":   false,
					},
				},
				Score: 0.88, // below default 0.92, but team-low has override 0.85
			},
		},
	}
	cfg := DefaultConfig()
	cfg.TeamThresholdOverrides = map[string]float64{"team-low": 0.85}

	lk := NewLookup(emb, idx.Search, cfg)
	result, err := lk.Lookup(context.Background(), "test query", "team-low")

	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}
	if result.Outcome != OutcomeHit {
		t.Errorf("Outcome = %v, want OutcomeHit (team override 0.85, score 0.88)", result.Outcome)
	}
}
