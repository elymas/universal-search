package tools

import (
	"context"
	"testing"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/pkg/types"
)

// stubAdapter is a minimal Adapter for testing.
type stubAdapter struct {
	name      string
	caps      types.Capabilities
	docs      []types.NormalizedDoc
	searchErr error
}

func (s *stubAdapter) Name() string { return s.name }
func (s *stubAdapter) Search(_ context.Context, _ types.Query) ([]types.NormalizedDoc, error) {
	return s.docs, s.searchErr
}
func (s *stubAdapter) Healthcheck(_ context.Context) error { return nil }
func (s *stubAdapter) Capabilities() types.Capabilities    { return s.caps }

// TestListSourcesReturnsRegisteredAdapters verifies that list_sources returns
// all registered adapters with correct fields (REQ-MCP-011).
func TestListSourcesReturnsRegisteredAdapters(t *testing.T) {
	reg := adapters.NewRegistry(nil)
	_ = reg.Register(&stubAdapter{
		name: "reddit",
		caps: types.Capabilities{
			SourceID:       "reddit",
			DisplayName:    "Reddit",
			DocTypes:       []types.DocType{types.DocTypePost},
			SupportedLangs: []string{"en"},
			RequiresAuth:   false,
		},
	})
	_ = reg.Register(&stubAdapter{
		name: "hackernews",
		caps: types.Capabilities{
			SourceID:       "hackernews",
			DisplayName:    "Hacker News",
			DocTypes:       []types.DocType{types.DocTypeArticle},
			SupportedLangs: []string{"en"},
			RequiresAuth:   false,
		},
	})

	handler := ListSourcesHandler(reg)
	_, output, err := handler(context.Background(), nil, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(output.Sources))
	}

	// Verify first source.
	if output.Sources[0].Name != "hackernews" {
		t.Errorf("first source: got %q, want hackernews", output.Sources[0].Name)
	}
	if output.Sources[0].Description != "Hacker News" {
		t.Errorf("first source description: got %q, want 'Hacker News'", output.Sources[0].Description)
	}
	if output.Sources[0].Category != "article" {
		t.Errorf("first source category: got %q, want 'article'", output.Sources[0].Category)
	}
	if output.Sources[0].AuthRequired {
		t.Error("first source: auth_required should be false")
	}

	// Verify second source.
	if output.Sources[1].Name != "reddit" {
		t.Errorf("second source: got %q, want reddit", output.Sources[1].Name)
	}
}

// TestListSourcesSortDeterministic verifies the response is sorted by name.
func TestListSourcesSortDeterministic(t *testing.T) {
	reg := adapters.NewRegistry(nil)
	// Register in reverse alphabetical order.
	_ = reg.Register(&stubAdapter{name: "z-source", caps: types.Capabilities{SourceID: "z-source", DisplayName: "Z Source"}})
	_ = reg.Register(&stubAdapter{name: "a-source", caps: types.Capabilities{SourceID: "a-source", DisplayName: "A Source"}})
	_ = reg.Register(&stubAdapter{name: "m-source", caps: types.Capabilities{SourceID: "m-source", DisplayName: "M Source"}})

	handler := ListSourcesHandler(reg)
	_, output, err := handler(context.Background(), nil, struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"a-source", "m-source", "z-source"}
	for i, src := range output.Sources {
		if src.Name != expected[i] {
			t.Errorf("source[%d]: got %q, want %q", i, src.Name, expected[i])
		}
	}
}
