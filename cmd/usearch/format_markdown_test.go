// Package main — tests for the markdown output formatter.
// SPEC-CLI-002 REQ-CLI2-006: --format markdown output.
package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// TestFormatMarkdownBasic verifies basic markdown output.
func TestFormatMarkdownBasic(t *testing.T) {
	resp := &queryResponse{
		Summary: "AI is transforming search.",
		Citations: []queryCitation{
			{Index: 1, Title: "AI Research", URL: "https://example.com/ai"},
			{Index: 2, Title: "Search Trends", URL: "https://example.com/search"},
		},
	}

	var buf bytes.Buffer
	if err := formatMarkdown(&buf, resp); err != nil {
		t.Fatalf("formatMarkdown failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "## Answer") {
		t.Error("missing ## Answer header")
	}
	if !strings.Contains(output, "## Sources") {
		t.Error("missing ## Sources header")
	}
	if !strings.Contains(output, "[AI Research](https://example.com/ai)") {
		t.Error("missing markdown citation link")
	}
	if !strings.Contains(output, "AI is transforming search.") {
		t.Error("missing summary text")
	}
}

// TestFormatMarkdownDegraded verifies degraded mode (no summary).
func TestFormatMarkdownDegraded(t *testing.T) {
	resp := &queryResponse{
		Summary: "",
		Docs: []types.NormalizedDoc{
			{Title: "Result 1", Snippet: "Snippet 1"},
			{Title: "Result 2", Snippet: "Snippet 2"},
		},
	}

	var buf bytes.Buffer
	if err := formatMarkdown(&buf, resp); err != nil {
		t.Fatalf("formatMarkdown failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Snippet 1") {
		t.Error("missing degraded mode snippet")
	}
}

// TestFormatMarkdownNoCitations verifies output without citations.
func TestFormatMarkdownNoCitations(t *testing.T) {
	resp := &queryResponse{
		Summary:   "Simple answer.",
		Citations: nil,
	}

	var buf bytes.Buffer
	if err := formatMarkdown(&buf, resp); err != nil {
		t.Fatalf("formatMarkdown failed: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "## Sources") {
		t.Error("should not show Sources header when no citations")
	}
	if !strings.Contains(output, "Simple answer.") {
		t.Error("missing summary text")
	}
}
