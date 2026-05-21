package deepreport_test

import (
	"testing"

	"github.com/elymas/universal-search/internal/deepreport"
	"github.com/elymas/universal-search/internal/streamsynth"
)

// T-M5-003 [RED]: Report implements LongFormSource interface.

func TestReportImplementsLongFormSource(t *testing.T) {
	// Compile-time interface check.
	var _ streamsynth.LongFormSource = deepreport.Report{}
}

func TestReportSourceSections(t *testing.T) {
	report := deepreport.Report{
		Sections: []deepreport.Section{
			{SectionIndex: 0, Heading: "Intro", Level: 1, Text: "Hello world", Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Hello world.", Markers: []int{1}},
			}},
			{SectionIndex: 1, Heading: "Body", Level: 2, Text: "Details here", Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Details.", Markers: []int{2}},
				{SentenceIndex: 1, Text: "More details.", Markers: []int{}},
			}},
		},
	}

	sections := report.SourceSections()
	if len(sections) != 2 {
		t.Fatalf("len(sections) = %d, want 2", len(sections))
	}

	if sections[0].Heading != "Intro" {
		t.Errorf("sections[0].Heading = %q, want %q", sections[0].Heading, "Intro")
	}
	if sections[0].Level != 1 {
		t.Errorf("sections[0].Level = %d, want 1", sections[0].Level)
	}
	if sections[1].Heading != "Body" {
		t.Errorf("sections[1].Heading = %q, want %q", sections[1].Heading, "Body")
	}

	// Verify markers are collected from sentences.
	if len(sections[0].Markers) != 1 || sections[0].Markers[0] != 1 {
		t.Errorf("sections[0].Markers = %v, want [1]", sections[0].Markers)
	}
	if len(sections[1].Markers) != 1 || sections[1].Markers[0] != 2 {
		t.Errorf("sections[1].Markers = %v, want [2]", sections[1].Markers)
	}
}

func TestReportSourceCitations(t *testing.T) {
	report := deepreport.Report{
		Citations: []deepreport.Citation{
			{Marker: 1, DocID: "d1", URL: "https://a.com", Title: "A"},
			{Marker: 2, DocID: "d2", URL: "https://b.com", Title: "B"},
		},
	}

	citations := report.SourceCitations()
	if len(citations) != 2 {
		t.Fatalf("len(citations) = %d, want 2", len(citations))
	}
	if citations[0].Marker != 1 {
		t.Errorf("citations[0].Marker = %d, want 1", citations[0].Marker)
	}
	if citations[1].DocID != "d2" {
		t.Errorf("citations[1].DocID = %q, want %q", citations[1].DocID, "d2")
	}
}

func TestReportSourceMetadata(t *testing.T) {
	report := deepreport.Report{
		Model:    "claude-3",
		Provider: "anthropic",
		CostUSD:  0.05,
	}

	meta := report.SourceMetadata()
	if meta.Model != "claude-3" {
		t.Errorf("Model = %q, want %q", meta.Model, "claude-3")
	}
	if meta.Provider != "anthropic" {
		t.Errorf("Provider = %q, want %q", meta.Provider, "anthropic")
	}
	if meta.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want 0.05", meta.CostUSD)
	}
}

func TestReportEmptySectionsAndCitations(t *testing.T) {
	report := deepreport.Report{}

	sections := report.SourceSections()
	if sections != nil {
		t.Errorf("expected nil sections for empty report, got %v", sections)
	}

	citations := report.SourceCitations()
	if citations != nil {
		t.Errorf("expected nil citations for empty report, got %v", citations)
	}

	meta := report.SourceMetadata()
	if meta.Model != "" {
		t.Errorf("expected empty Model, got %q", meta.Model)
	}
}
