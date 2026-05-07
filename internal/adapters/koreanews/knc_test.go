// Package koreanews — KNC sidecar HTTP client tests.
// SPEC-ADP-009 REQ-ADP9-009.
package koreanews_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/adapters/koreanews"
	"github.com/elymas/universal-search/pkg/types"
)

func newKNCServer(t *testing.T, statusCode int, body interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

// TestSearch_KNC_503_returnsErrKNCSidecarDown verifies that HTTP 503 from the
// KNC sidecar maps to ErrKNCSidecarDown wrapped in CategoryUnavailable.
func TestSearch_KNC_503_returnsErrKNCSidecarDown(t *testing.T) {
	t.Parallel()

	srv := newKNCServer(t, http.StatusServiceUnavailable, map[string]string{
		"detail": "knc sidecar not yet implemented",
	})
	defer srv.Close()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled:  false,
		KNCEnabled:  true,
		KNCBaseURL:  srv.URL,
		HTTPClient:  srv.Client(),
		RSSFeeds:    nil,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, searchErr := a.Search(context.Background(), types.Query{Text: "한국 뉴스"})
	if searchErr == nil {
		t.Fatal("expected error from KNC 503, got nil")
	}

	if !errors.Is(searchErr, koreanews.ErrKNCSidecarDown) {
		t.Errorf("expected ErrKNCSidecarDown; got %v", searchErr)
	}

	var se *types.SourceError
	if !errors.As(searchErr, &se) || se.Category != types.CategoryUnavailable {
		t.Errorf("want CategoryUnavailable SourceError; got %T %v", searchErr, searchErr)
	}
}

// TestSearch_KNC_200_returnsDocs verifies successful KNC response maps to NormalizedDocs.
func TestSearch_KNC_200_returnsDocs(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)

	articles := []map[string]interface{}{
		{
			"url":      "https://news.example.com/article-1",
			"title":    "한국 뉴스 1",
			"body":     "첫 번째 기사 본문",
			"date":     fixedTime.Format(time.RFC3339),
			"author":   "김기자",
			"category": "politics",
		},
		{
			"url":      "https://news.example.com/article-2",
			"title":    "한국 뉴스 2",
			"body":     "두 번째 기사 본문",
			"date":     fixedTime.Format(time.RFC3339),
			"author":   "이기자",
			"category": "economy",
		},
	}

	srv := newKNCServer(t, http.StatusOK, map[string]interface{}{
		"articles": articles,
	})
	defer srv.Close()

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	a, err := koreanews.New(koreanews.Options{
		RSSEnabled: false,
		KNCEnabled: true,
		KNCBaseURL: srv.URL,
		HTTPClient: srv.Client(),
		NowFunc:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	docs, err := a.Search(context.Background(), types.Query{Text: "한국 뉴스"})
	if err != nil {
		t.Fatalf("Search: unexpected error: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("got %d docs; want 2", len(docs))
	}

	// Validate required fields.
	for i, doc := range docs {
		if err := doc.Validate(); err != nil {
			t.Errorf("docs[%d].Validate() = %v", i, err)
		}
		if doc.SourceID != "koreanews" {
			t.Errorf("docs[%d].SourceID = %q; want koreanews", i, doc.SourceID)
		}
		if doc.Score != 0.5 {
			t.Errorf("docs[%d].Score = %v; want 0.5", i, doc.Score)
		}
		if doc.Lang != "ko" {
			t.Errorf("docs[%d].Lang = %q; want ko", i, doc.Lang)
		}
		meta, ok := doc.Metadata["subsource"]
		if !ok || meta != "knc" {
			t.Errorf("docs[%d].Metadata[subsource] = %v; want knc", i, meta)
		}
	}
}

// TestSearch_KNC_unreachable verifies connection-refused maps to CategoryUnavailable.
func TestSearch_KNC_unreachable(t *testing.T) {
	t.Parallel()

	a, err := koreanews.New(koreanews.Options{
		RSSEnabled: false,
		KNCEnabled: true,
		KNCBaseURL: "http://127.0.0.1:1", // nothing listens
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, searchErr := a.Search(context.Background(), types.Query{Text: "test"})
	if searchErr == nil {
		t.Fatal("expected error from unreachable KNC, got nil")
	}
	var se *types.SourceError
	if !errors.As(searchErr, &se) || se.Category != types.CategoryUnavailable {
		t.Errorf("want CategoryUnavailable; got %T %v", searchErr, searchErr)
	}
}
