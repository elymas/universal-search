package deepagent

import (
	"testing"

	"github.com/elymas/universal-search/internal/streamsynth"
)

// T-M3-003 [RED]: LongFormSource interface compliance test
// REQ-DEEP2-008: WriterDraft must satisfy streamsynth.LongFormSource for SSE streaming.

func TestWriterDraftSatisfiesLongFormSource(t *testing.T) {
	// Compile-time interface compliance check.
	// If WriterDraft does not implement streamsynth.LongFormSource, this fails to compile.
	var _ streamsynth.LongFormSource = WriterDraft{}
}

func TestLongFormSourceMethodReturnsCorrectData(t *testing.T) {
	draft := WriterDraft{
		Sections: []DraftSection{
			{
				SectionIndex:    0,
				Heading:         "Introduction",
				Text:            "First sentence [1]. Second sentence [2].",
				CitationMarkers: []int{1, 2},
			},
		},
		Citations: []DraftCitation{
			{Marker: 1, DocID: "d1", URL: "https://a.com", Title: "A"},
			{Marker: 2, DocID: "d2", URL: "https://b.com", Title: "B"},
		},
		CostUSD:  0.005,
		Model:    "gpt-4",
		Provider: "openai",
	}

	var src streamsynth.LongFormSource = draft

	sections := src.SourceSections()
	if len(sections) != 1 {
		t.Fatalf("len(sections) = %d, want 1", len(sections))
	}
	if sections[0].Heading != "Introduction" {
		t.Errorf("heading = %q, want %q", sections[0].Heading, "Introduction")
	}

	citations := src.SourceCitations()
	if len(citations) != 2 {
		t.Fatalf("len(citations) = %d, want 2", len(citations))
	}
	if citations[0].Marker != 1 {
		t.Errorf("citation[0].Marker = %d, want 1", citations[0].Marker)
	}

	meta := src.SourceMetadata()
	if meta.Model != "gpt-4" {
		t.Errorf("metadata.Model = %q, want %q", meta.Model, "gpt-4")
	}
	if meta.Provider != "openai" {
		t.Errorf("metadata.Provider = %q, want %q", meta.Provider, "openai")
	}
	if meta.CostUSD != 0.005 {
		t.Errorf("metadata.CostUSD = %f, want 0.005", meta.CostUSD)
	}
}
