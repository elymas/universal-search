package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/pkg/types"
)

// stubAdapter is a minimal adapter for orchestrator tests.
type stubAdapter struct {
	name    string
	caps    types.Capabilities
	docs    []types.NormalizedDoc
	searchErr error
}

func (s *stubAdapter) Name() string                                                { return s.name }
func (s *stubAdapter) Search(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) { return s.docs, s.searchErr }
func (s *stubAdapter) Healthcheck(_ context.Context) error                          { return nil }
func (s *stubAdapter) Capabilities() types.Capabilities                             { return s.caps }

// stubSynth is a synthesis function that returns a fixed result.
func stubSynth(_ context.Context, query, _ string, docs []types.NormalizedDoc) (string, []Citation, error) {
	citations := make([]Citation, len(docs))
	for i, d := range docs {
		citations[i] = Citation{DocID: d.ID, Title: d.Title, URL: d.URL, Source: d.SourceID}
	}
	return "summary of: " + query, citations, nil
}

// TestSharedSearchOrchestratorHappyPath verifies the full pipeline with
// stub adapters and synthesis succeeds end-to-end.
func TestSharedSearchOrchestratorHappyPath(t *testing.T) {
	reg := adapters.NewRegistry(nil)
	_ = reg.Register(&stubAdapter{
		name: "test-adapter",
		caps: types.Capabilities{
			SourceID:  "test-adapter",
			DocTypes:  []types.DocType{types.DocTypeArticle},
		},
		docs: []types.NormalizedDoc{
			{ID: "d1", Title: "Doc 1", URL: "http://example.com", SourceID: "test-adapter"},
		},
	})

	result, err := Search(context.Background(), reg, SearchParams{Query: "test query"}, stubSynth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary == "" {
		t.Error("expected non-empty summary")
	}
	if len(result.Docs) != 1 {
		t.Errorf("expected 1 doc, got %d", len(result.Docs))
	}
	if len(result.Citations) != 1 {
		t.Errorf("expected 1 citation, got %d", len(result.Citations))
	}
	if result.Citations[0].DocID != "d1" {
		t.Errorf("citation doc_id: got %q, want d1", result.Citations[0].DocID)
	}
	if len(result.AdapterSet) == 0 {
		t.Error("expected non-empty adapter set")
	}
}

// TestSharedSearchOrchestratorPartialFailure verifies that partial adapter
// failure still produces results from successful adapters.
func TestSharedSearchOrchestratorPartialFailure(t *testing.T) {
	reg := adapters.NewRegistry(nil)
	_ = reg.Register(&stubAdapter{
		name: "good-adapter",
		caps: types.Capabilities{
			SourceID: "good-adapter",
			DocTypes: []types.DocType{types.DocTypeArticle},
		},
		docs: []types.NormalizedDoc{
			{ID: "d1", Title: "Good Doc", URL: "http://good.com", SourceID: "good-adapter"},
		},
	})
	_ = reg.Register(&stubAdapter{
		name: "bad-adapter",
		caps: types.Capabilities{
			SourceID: "bad-adapter",
			DocTypes: []types.DocType{types.DocTypeArticle},
		},
		searchErr: errors.New("connection refused"),
	})

	result, err := Search(context.Background(), reg, SearchParams{Query: "test"}, stubSynth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Docs) == 0 {
		t.Error("expected at least 1 doc from good adapter")
	}
	if len(result.AdapterErrors) == 0 {
		t.Error("expected at least 1 adapter error")
	}
	if _, ok := result.AdapterErrors["bad-adapter"]; !ok {
		t.Error("expected bad-adapter error in AdapterErrors")
	}
}

// TestSharedSearchOrchestratorNoAdapters verifies that an empty registry
// returns an error (either from router construction or classification).
func TestSharedSearchOrchestratorNoAdapters(t *testing.T) {
	reg := adapters.NewRegistry(nil) // empty registry

	_, err := Search(context.Background(), reg, SearchParams{Query: "test"}, stubSynth)
	if err == nil {
		t.Fatal("expected error for no adapters, got nil")
	}
	// Empty registry may fail at router construction or classification.
	// Both are valid failure modes.
}
