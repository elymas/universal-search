package idx5

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// --- Coverage improvement tests ---

func TestServeMISSAndBypassed(t *testing.T) {
	// Cover ServeMISS and ServeBypassed
	rec := httptest.NewRecorder()
	ServeMISS(rec)
	if rec.Header().Get("X-Cache") != "MISS" {
		t.Errorf("ServeMISS: X-Cache = %q, want MISS", rec.Header().Get("X-Cache"))
	}

	rec2 := httptest.NewRecorder()
	ServeBypassed(rec2)
	if rec2.Header().Get("X-Cache") != "BYPASSED" {
		t.Errorf("ServeBypassed: X-Cache = %q, want BYPASSED", rec2.Header().Get("X-Cache"))
	}
}

func TestMarshalResponse(t *testing.T) {
	// Cover MarshalResponse helper
	data := map[string]string{"text": "hello"}
	b, err := MarshalResponse(data)
	if err != nil {
		t.Fatalf("MarshalResponse error: %v", err)
	}
	if string(b) != `{"text":"hello"}` {
		t.Errorf("MarshalResponse = %q, want exact JSON", string(b))
	}
}

func TestServeCachedWithExtraHeaders(t *testing.T) {
	// Cover extra headers path in ServeCached
	ca := &CachedAnswer{
		DocID:        "answer-cache:abc:team-T",
		TeamID:       "team-T",
		ResponseJSON: `{"text":"cached"}`,
		Category:     "web",
		TTLSeconds:   3600,
		CreatedAt:    time.Now().Add(-500 * time.Second),
		Similarity:   0.94,
	}

	rec := httptest.NewRecorder()
	ServeCached(rec, ca, Fresh, map[string]string{
		"X-Cache-Citation-Stale": "1",
	})

	if rec.Header().Get("X-Cache-Citation-Stale") != "1" {
		t.Errorf("extra header not set: %q", rec.Header().Get("X-Cache-Citation-Stale"))
	}
}

func TestLRUSetAndGet(t *testing.T) {
	// Cover LRU Set/Get more thoroughly
	lru := NewRequestLRU(3600)

	lru.Set("req-1", "doc-1", "team-T")
	mapping, ok := lru.Get("req-1")
	if !ok {
		t.Fatal("expected to find req-1")
	}
	if mapping.DocID != "doc-1" {
		t.Errorf("DocID = %q, want doc-1", mapping.DocID)
	}
	if mapping.TeamID != "team-T" {
		t.Errorf("TeamID = %q, want team-T", mapping.TeamID)
	}

	// Update existing entry
	lru.Set("req-1", "doc-2", "team-T")
	mapping2, ok := lru.Get("req-1")
	if !ok {
		t.Fatal("expected to find req-1 after update")
	}
	if mapping2.DocID != "doc-2" {
		t.Errorf("updated DocID = %q, want doc-2", mapping2.DocID)
	}

	// Missing key
	_, ok = lru.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent key")
	}
}

func TestLRUExpiredEntry(t *testing.T) {
	// Cover expired entry path
	lru := NewRequestLRU(0) // 0 second TTL = immediate expiry
	lru.Set("req-expired", "doc-exp", "team-T")

	// Should be expired immediately
	_, ok := lru.Get("req-expired")
	if ok {
		t.Error("entry should be expired with 0 TTL")
	}
}

func TestLookupEmbedderError(t *testing.T) {
	// Cover embedder error path
	emb := &mockEmbedder{
		vec: nil,
		err: http.ErrAbortHandler,
	}
	idx := &mockIndex{}
	cfg := DefaultConfig()
	lk := NewLookup(emb, idx.Search, cfg)

	result, err := lk.Lookup(nil, "test query", "team-T")
	if err == nil {
		t.Error("expected error from embedder failure")
	}
	if result.Outcome != OutcomeMiss {
		t.Errorf("Outcome = %v, want OutcomeMiss on embedder error", result.Outcome)
	}
}

func TestLookupSearchError(t *testing.T) {
	// Cover search error path
	emb := &mockEmbedder{vec: make([]float32, 128)}
	idx := &mockIndex{
		err: http.ErrAbortHandler,
	}
	cfg := DefaultConfig()
	lk := NewLookup(emb, idx.Search, cfg)

	result, err := lk.Lookup(nil, "test query", "team-T")
	if err == nil {
		t.Error("expected error from search failure")
	}
	if result.Outcome != OutcomeMiss {
		t.Errorf("Outcome = %v, want OutcomeMiss on search error", result.Outcome)
	}
}

func TestLookupHardStaleResult(t *testing.T) {
	// Cover hard-stale lookup path
	emb := &mockEmbedder{vec: make([]float32, 128)}
	now := time.Now()
	idx := &mockIndex{
		results: []IndexResult{
			{
				Doc: types.NormalizedDoc{
					ID:          "answer-cache:abc:team-T",
					SourceID:    "idx5",
					URL:         "cache://team-T/abc",
					RetrievedAt: now,
					DocType:     DocTypeCachedAnswer,
					Metadata: map[string]any{
						"team_id":      "team-T",
						"query_hash":   "abc",
						"category":     "web",
						"ttl_seconds":  int64(3600),
						"created_at":   now.Add(-7200 * time.Second).Format(time.RFC3339), // 2h ago
						"force_stale":  false,
					},
				},
				Score: 0.95,
			},
		},
	}
	cfg := DefaultConfig()
	lk := NewLookup(emb, idx.Search, cfg)

	result, err := lk.Lookup(nil, "test query", "team-T")
	if err != nil {
		t.Fatalf("Lookup error: %v", err)
	}
	if result.Outcome != OutcomeHardStale {
		t.Errorf("Outcome = %v, want OutcomeHardStale", result.Outcome)
	}
	if result.Cached == nil {
		t.Error("HardStale result should still carry the cached answer for observability")
	}
}

func TestFeedbackIgnoredOnPositiveScore(t *testing.T) {
	// Cover positive score path (ignored)
	store := NewFeedbackStore()
	lru := NewRequestLRU(3600)
	handler := NewFeedbackHandler(store, lru)

	result := handler.HandleFeedback("req-123", "team-T", 1)
	if result.Status != "ignored" {
		t.Errorf("positive score status = %q, want ignored", result.Status)
	}
}

func TestFeedbackStoreError(t *testing.T) {
	// Cover store error path
	store := &errorFeedbackStore{}
	lru := NewRequestLRU(3600)
	lru.Set("req-err", "doc-err", "team-T")
	handler := NewFeedbackHandler(store, lru)

	result := handler.HandleFeedback("req-err", "team-T", -1)
	if result.Status != "error" {
		t.Errorf("error store status = %q, want error", result.Status)
	}
}

type errorFeedbackStore struct{}

func (e *errorFeedbackStore) MarkStale(docID, teamID string) error {
	return http.ErrAbortHandler
}

func TestDocToCachedAnswerWithFloatTTL(t *testing.T) {
	// Cover float64 ttl_seconds path in docToCachedAnswer
	doc := types.NormalizedDoc{
		ID:          "answer-cache:abc:team-T",
		SourceID:    "idx5",
		URL:         "cache://team-T/abc",
		RetrievedAt: time.Now(),
		DocType:     DocTypeCachedAnswer,
		Metadata: map[string]any{
			"team_id":       "team-T",
			"query_hash":    "abc",
			"category":      "web",
			"response_json": `{"text":"test"}`,
			"ttl_seconds":   float64(3600),
			"created_at":    time.Now().Format(time.RFC3339),
			"force_stale":   true,
			"query_text":    "test query",
		},
	}
	ca := docToCachedAnswer(&doc, 0.95)
	if ca.TTLSeconds != 3600 {
		t.Errorf("TTLSeconds = %d, want 3600", ca.TTLSeconds)
	}
	if !ca.ForceStale {
		t.Error("ForceStale should be true")
	}
	if ca.QueryText != "test query" {
		t.Errorf("QueryText = %q, want 'test query'", ca.QueryText)
	}
}

func TestEffectiveTTLFallback(t *testing.T) {
	// Cover fallback path in effectiveTTL when category not in map
	ca := &CachedAnswer{
		Category:   "nonexistent_category",
		TTLSeconds: 5400, // stored TTL
	}
	ttl := effectiveTTL(ca, map[string]int{"web": 3600})
	if ttl != 5400 {
		t.Errorf("effectiveTTL = %d, want 5400 (stored value)", ttl)
	}

	// Cover zero stored TTL fallback
	ca2 := &CachedAnswer{
		Category:   "nonexistent_category",
		TTLSeconds: 0,
	}
	ttl2 := effectiveTTL(ca2, map[string]int{"web": 3600})
	if ttl2 != 7200 {
		t.Errorf("effectiveTTL = %d, want 7200 (default)", ttl2)
	}
}

func TestLookupWithBypassNormal(t *testing.T) {
	// Cover non-bypass path of LookupWithBypass
	emb := &mockEmbedder{vec: make([]float32, 128)}
	idx := &mockIndex{results: nil}
	cfg := DefaultConfig()
	lk := NewLookup(emb, idx.Search, cfg)

	result, err := lk.LookupWithBypass(nil, "test query", "team-T", false)
	if err != nil {
		t.Fatalf("LookupWithBypass error: %v", err)
	}
	if result.Outcome != OutcomeMiss {
		t.Errorf("Outcome = %v, want OutcomeMiss", result.Outcome)
	}
}
