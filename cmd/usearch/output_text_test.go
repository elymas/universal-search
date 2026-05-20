package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

func TestFormatTextShape(t *testing.T) {
	resp := &queryResponse{
		Summary: "This is the synthesized summary.",
		Citations: []queryCitation{
			{Index: 1, Title: "Reddit Post", URL: "https://reddit.com/r/test"},
			{Index: 2, Title: "HN Thread", URL: "https://news.ycombinator.com/item?id=123"},
		},
	}

	var buf bytes.Buffer
	if err := formatText(&buf, resp); err != nil {
		t.Fatalf("formatText error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "This is the synthesized summary.") {
		t.Errorf("output missing summary: %q", out)
	}
	if !strings.Contains(out, "Citations:") {
		t.Errorf("output missing Citations: block: %q", out)
	}
	if !strings.Contains(out, "[1] Reddit Post") {
		t.Errorf("output missing [1]: %q", out)
	}
	if !strings.Contains(out, "[2] HN Thread") {
		t.Errorf("output missing [2]: %q", out)
	}
	if !strings.Contains(out, "https://reddit.com/r/test") {
		t.Errorf("output missing reddit URL: %q", out)
	}
}

func TestFormatTextDegradedMode(t *testing.T) {
	now := time.Now()
	resp := &queryResponse{
		Summary: "", // empty = degraded
		Docs: []types.NormalizedDoc{
			{ID: "1", SourceID: "reddit", URL: "https://example.com", Title: "Doc A",
				Snippet: "snippet A", RetrievedAt: now},
			{ID: "2", SourceID: "hn", URL: "https://example.com/2", Title: "Doc B",
				Snippet: "snippet B", RetrievedAt: now},
		},
		Citations: []queryCitation{},
	}

	var buf bytes.Buffer
	if err := formatText(&buf, resp); err != nil {
		t.Fatalf("formatText degraded error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[1] snippet A") {
		t.Errorf("degraded output missing [1] snippet A: %q", out)
	}
	if !strings.Contains(out, "[2] snippet B") {
		t.Errorf("degraded output missing [2] snippet B: %q", out)
	}
}

func TestFormatTextNoCitations(t *testing.T) {
	resp := &queryResponse{
		Summary:   "Just a summary, no citations.",
		Citations: []queryCitation{},
	}

	var buf bytes.Buffer
	if err := formatText(&buf, resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "Citations:") {
		t.Errorf("output should not have Citations: block when empty: %q", out)
	}
}
