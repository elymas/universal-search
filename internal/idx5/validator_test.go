package idx5

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestCitationRevalidationLazyDefault(t *testing.T) {
	// REQ-IDX5-004: default lazy mode keeps all citations
	citations := []Citation{
		{Marker: 1, URL: "https://example.com/1", Title: "First"},
		{Marker: 2, URL: "https://example.com/2", Title: "Second"},
	}
	cfg := Config{CitationRevalidationMode: "lazy"}

	result, stripped := RevalidateCitations(nil, citations, cfg)
	if len(result) != 2 {
		t.Errorf("lazy mode: got %d citations, want 2", len(result))
	}
	if stripped != 0 {
		t.Errorf("lazy mode: stripped %d, want 0", stripped)
	}
}

func TestCitationRevalidationEagerTopNStrips404(t *testing.T) {
	// REQ-IDX5-004: 404 citation stripped + X-Cache-Citation-Stale header
	// Set up a test server that returns 404 for /1 and 200 for /2, /3
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	citations := []Citation{
		{Marker: 1, URL: server.URL + "/1", Title: "First"},
		{Marker: 2, URL: server.URL + "/2", Title: "Second"},
		{Marker: 3, URL: server.URL + "/3", Title: "Third"},
	}
	cfg := Config{CitationRevalidationMode: "eager_top_n", EagerTopN: 3}

	result, stripped := RevalidateCitations(nil, citations, cfg)
	if len(result) != 2 {
		t.Errorf("eager_top_n: got %d citations, want 2 (1 stripped)", len(result))
	}
	if stripped != 1 {
		t.Errorf("eager_top_n: stripped %d, want 1", stripped)
	}
	// Verify the right citation was kept
	if len(result) > 0 && result[0].Marker != 2 {
		t.Errorf("first remaining citation marker = %d, want 2", result[0].Marker)
	}
}

func TestCitationRevalidationEagerTopNKeepsTimeout(t *testing.T) {
	// REQ-IDX5-004: timeout/5xx citation is kept (not stripped)
	// Set up a server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	citations := []Citation{
		{Marker: 1, URL: server.URL + "/1", Title: "First"},
	}
	cfg := Config{CitationRevalidationMode: "eager_top_n", EagerTopN: 3}

	result, stripped := RevalidateCitations(nil, citations, cfg)
	if len(result) != 1 {
		t.Errorf("5xx: got %d citations, want 1 (5xx kept)", len(result))
	}
	if stripped != 0 {
		t.Errorf("5xx: stripped %d, want 0", stripped)
	}
}

func TestCitationRevalidationEagerTopNLimit(t *testing.T) {
	// REQ-IDX5-004: only top-N citations are probed
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	citations := []Citation{
		{Marker: 1, URL: server.URL + "/1"},
		{Marker: 2, URL: server.URL + "/2"},
		{Marker: 3, URL: server.URL + "/3"},
		{Marker: 4, URL: server.URL + "/4"},
		{Marker: 5, URL: server.URL + "/5"},
	}
	cfg := Config{CitationRevalidationMode: "eager_top_n", EagerTopN: 3}

	// All should be kept since they return 200
	result, stripped := RevalidateCitations(nil, citations, cfg)
	if stripped != 0 {
		t.Errorf("all 200: stripped %d, want 0", stripped)
	}
	if len(result) != 5 {
		t.Errorf("got %d citations, want 5", len(result))
	}
}

// FeedbackStore records force_stale operations for testing.
type FeedbackStore struct {
	mu      sync.Mutex
	updated map[string]bool // doc_id -> force_stale set
}

func NewFeedbackStore() *FeedbackStore {
	return &FeedbackStore{updated: make(map[string]bool)}
}

func (fs *FeedbackStore) MarkStale(docID, teamID string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.updated[docID] = true
	return nil
}

func (fs *FeedbackStore) WasMarked(docID string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.updated[docID]
}

func TestFeedbackMarksForceStale(t *testing.T) {
	// REQ-IDX5-008: POST /feedback {score: -1} -> force_stale=TRUE
	store := NewFeedbackStore()
	lru := NewRequestLRU(24 * 3600)

	// Simulate a prior request mapping
	lru.Set("req-abc123", "answer-cache:abc:team-T", "team-T")

	handler := NewFeedbackHandler(store, lru)
	result := handler.HandleFeedback("req-abc123", "team-T", -1)

	if result.Status != "marked_stale" {
		t.Errorf("status = %q, want %q", result.Status, "marked_stale")
	}
	if !store.WasMarked("answer-cache:abc:team-T") {
		t.Error("doc_id was not marked stale")
	}
}

func TestFeedbackIdempotentOnDuplicate(t *testing.T) {
	// REQ-IDX5-008: duplicate feedback within 24h is idempotent
	store := NewFeedbackStore()
	lru := NewRequestLRU(24 * 3600)
	lru.Set("req-abc123", "answer-cache:abc:team-T", "team-T")

	handler := NewFeedbackHandler(store, lru)

	// First call
	r1 := handler.HandleFeedback("req-abc123", "team-T", -1)
	if r1.Status != "marked_stale" {
		t.Errorf("first: status = %q, want %q", r1.Status, "marked_stale")
	}

	// Second call (idempotent)
	r2 := handler.HandleFeedback("req-abc123", "team-T", -1)
	if r2.Status != "marked_stale" {
		t.Errorf("second: status = %q, want %q", r2.Status, "marked_stale")
	}
}

func TestFeedbackUnmappedIncrementsCounter(t *testing.T) {
	// REQ-IDX5-008: unmapped request_id -> counter increment
	store := NewFeedbackStore()
	lru := NewRequestLRU(24 * 3600) // empty LRU

	handler := NewFeedbackHandler(store, lru)
	result := handler.HandleFeedback("req-nonexistent", "team-T", -1)

	if result.Status != "unmapped" {
		t.Errorf("status = %q, want %q", result.Status, "unmapped")
	}
	if result.UnmappedCount != 1 {
		t.Errorf("unmapped_count = %d, want 1", result.UnmappedCount)
	}
}

func TestFeedbackRespectsTenantBoundary(t *testing.T) {
	// REQ-IDX5-008 NFR-004: team U cannot mark team T's doc stale
	store := NewFeedbackStore()
	lru := NewRequestLRU(24 * 3600)
	lru.Set("req-abc123", "answer-cache:abc:team-T", "team-T")

	handler := NewFeedbackHandler(store, lru)

	// team-U tries to mark team-T's doc
	result := handler.HandleFeedback("req-abc123", "team-U", -1)

	if result.Status != "tenant_mismatch" {
		t.Errorf("status = %q, want %q", result.Status, "tenant_mismatch")
	}
	if store.WasMarked("answer-cache:abc:team-T") {
		t.Error("team-U should not be able to mark team-T's doc stale")
	}
}
