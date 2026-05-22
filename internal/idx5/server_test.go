package idx5

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestServeReconstructsSynthesizeResponse(t *testing.T) {
	// REQ-IDX5-002: cached_answer record -> SynthesizeResponse JSON
	ca := &CachedAnswer{
		DocID:        "answer-cache:abc:team-T",
		TeamID:       "team-T",
		ResponseJSON: `{"text":"hello world","citations":[],"model":"gpt-4"}`,
		Category:     "web",
		TTLSeconds:   3600,
		CreatedAt:    time.Now().Add(-500 * time.Second),
		Similarity:   0.94,
	}

	rec := httptest.NewRecorder()
	ServeCached(rec, ca, Fresh, nil)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "hello world") {
		t.Errorf("response body should contain cached text, got: %s", body)
	}
}

func TestLookupResponseHeadersOnHit(t *testing.T) {
	// REQ-IDX5-002: X-Cache: HIT, X-Cache-Age-Seconds, X-Cache-Score
	ca := &CachedAnswer{
		DocID:        "answer-cache:abc:team-T",
		TeamID:       "team-T",
		ResponseJSON: `{"text":"cached"}`,
		Category:     "web",
		TTLSeconds:   3600,
		CreatedAt:    time.Now().Add(-1800 * time.Second),
		Similarity:   0.94,
	}

	rec := httptest.NewRecorder()
	ServeCached(rec, ca, Fresh, nil)

	headers := rec.Header()
	if headers.Get("X-Cache") != "HIT" {
		t.Errorf("X-Cache = %q, want %q", headers.Get("X-Cache"), "HIT")
	}
	if headers.Get("X-Cache-Score") != "0.94" {
		t.Errorf("X-Cache-Score = %q, want %q", headers.Get("X-Cache-Score"), "0.94")
	}
	ageStr := headers.Get("X-Cache-Age-Seconds")
	if ageStr == "" {
		t.Error("X-Cache-Age-Seconds should not be empty")
	}
}

func TestServeSoftStaleSetsSoftHitHeader(t *testing.T) {
	// REQ-IDX5-002: soft-stale -> X-Cache: SOFT-HIT
	ca := &CachedAnswer{
		DocID:        "answer-cache:abc:team-T",
		TeamID:       "team-T",
		ResponseJSON: `{"text":"cached"}`,
		Category:     "web",
		TTLSeconds:   3600,
		CreatedAt:    time.Now().Add(-3000 * time.Second),
		Similarity:   0.93,
	}

	rec := httptest.NewRecorder()
	ServeCached(rec, ca, SoftStale, nil)

	if rec.Header().Get("X-Cache") != "SOFT-HIT" {
		t.Errorf("X-Cache = %q, want %q", rec.Header().Get("X-Cache"), "SOFT-HIT")
	}
}

func TestServeHardStaleSetsMissHeader(t *testing.T) {
	// REQ-IDX5-002: hard-stale -> X-Cache: MISS
	ca := &CachedAnswer{
		DocID:        "answer-cache:abc:team-T",
		TeamID:       "team-T",
		ResponseJSON: `{"text":"cached"}`,
		Category:     "web",
		TTLSeconds:   3600,
		CreatedAt:    time.Now().Add(-5400 * time.Second),
		Similarity:   0.94,
	}

	rec := httptest.NewRecorder()
	ServeCached(rec, ca, HardStale, nil)

	// HardStale should NOT be served — it triggers MISS path
	// ServeCached should set MISS header for hard-stale
	if rec.Header().Get("X-Cache") != "MISS" {
		t.Errorf("X-Cache = %q, want %q", rec.Header().Get("X-Cache"), "MISS")
	}
}

func TestCacheWriteOnMissFireAndForget(t *testing.T) {
	// REQ-IDX5-006: fanout MISS -> async write to Qdrant + PG
	written := make(chan string, 1)
	wb := NewWriteback(func(docID string) error {
		written <- docID
		return nil
	})

	wb.FireAndForget("answer-cache:abc:team-T", "team-T", "test query", "web", `{"text":"result"}`, 0.95)

	// Should complete without blocking
	select {
	case docID := <-written:
		if docID != "answer-cache:abc:team-T" {
			t.Errorf("write docID = %q, want %q", docID, "answer-cache:abc:team-T")
		}
	case <-time.After(2 * time.Second):
		t.Error("FireAndForget did not complete within timeout")
	}
}

func TestCacheWriteFailureDoesNotBlock(t *testing.T) {
	// REQ-IDX5-006: write failure should not impact caller
	wb := NewWriteback(func(docID string) error {
		return http.ErrAbortHandler // simulate failure
	})

	// Should return immediately, no panic
	done := make(chan struct{})
	go func() {
		wb.FireAndForget("answer-cache:abc:team-T", "team-T", "test query", "web", `{}`, 0.5)
		close(done)
	}()

	select {
	case <-done:
		// Success - did not block
	case <-time.After(2 * time.Second):
		t.Error("FireAndForget blocked on write failure")
	}
}

func TestCacheWriteUpsertOverwritesIdempotent(t *testing.T) {
	// REQ-IDX5-006: same doc_id re-write is overwrite (idempotent)
	var mu sync.Mutex
	callCount := 0
	wb := NewWriteback(func(docID string) error {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	})

	wb.FireAndForget("answer-cache:abc:team-T", "team-T", "test query", "web", `{}`, 0.5)
	wb.FireAndForget("answer-cache:abc:team-T", "team-T", "test query", "web", `{"text":"updated"}`, 0.95)

	// Wait for all goroutines to complete
	wb.Wait()

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 2 {
		t.Errorf("write called %d times, want 2", count)
	}
}
