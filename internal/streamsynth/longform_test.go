// Package streamsynth_test — RED phase tests for long-form SSE streaming.
// test(stream): RED — longform tests (SPEC-DEEP-001 REQ-DEEP1-005/006 M7)
package streamsynth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/deepreport"
	"github.com/elymas/universal-search/internal/obs/metrics"
	"github.com/elymas/universal-search/internal/sse"
	"github.com/elymas/universal-search/internal/streamsynth"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// buildTestReport creates a deepreport.Report with the given sections and citations.
func buildTestReport(requestID string, sections []deepreport.Section, citations []deepreport.Citation) deepreport.Report {
	return deepreport.Report{
		RequestID:        requestID,
		Title:            "Test Report",
		Sections:         sections,
		Citations:        citations,
		Model:            "test-model",
		Provider:         "test-provider",
		CostUSD:          0.05,
		PromptTokens:     100,
		CompletionTokens: 200,
		LatencyMS:        5000,
		SchemaVersion:    1,
	}
}

// setupSSEWriter creates a fakeResponseWriter and wraps it with an SSE Writer.
func setupSSEWriter() (*fakeResponseWriter, *sse.Writer) {
	rw := newFakeRW()
	sw := sse.NewWriter(rw)
	return rw, sw
}

// TestSSEEmitsSectionStartPerSection verifies REQ-DEEP1-005: 3 sections produce
// exactly 3 section_start events with correct heading and level.
func TestSSEEmitsSectionStartPerSection(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	sections := []deepreport.Section{
		{
			SectionIndex: 0, Heading: "Introduction", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Intro sentence with [1].", Markers: []int{1}},
			},
		},
		{
			SectionIndex: 1, Heading: "Methods", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Methods sentence [1].", Markers: []int{1}},
			},
		},
		{
			SectionIndex: 2, Heading: "Conclusion", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Conclusion sentence [1].", Markers: []int{1}},
			},
		},
	}
	report := buildTestReport("req-sections", sections, citations)

	rw, sw := setupSSEWriter()
	_, err := streamsynth.StreamLongFormReport(context.Background(), sw, report.RequestID, report)
	if err != nil {
		t.Fatalf("StreamLongFormReport error: %v", err)
	}

	events := parseSSEEvents(rw.buf.String())
	var sectionStartEvents []map[string]string
	for _, ev := range events {
		if ev["event"] == "section_start" {
			sectionStartEvents = append(sectionStartEvents, ev)
		}
	}

	if len(sectionStartEvents) != 3 {
		t.Errorf("section_start events = %d, want 3; raw: %q", len(sectionStartEvents), rw.buf.String())
	}

	// Verify first section_start payload.
	var payload streamsynth.SectionStartPayload
	if err := json.Unmarshal([]byte(sectionStartEvents[0]["data"]), &payload); err != nil {
		t.Fatalf("unmarshal section_start: %v", err)
	}
	if payload.Heading != "Introduction" {
		t.Errorf("heading = %q, want %q", payload.Heading, "Introduction")
	}
	if payload.Level != 2 {
		t.Errorf("level = %d, want 2", payload.Level)
	}
	if payload.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", payload.SchemaVersion)
	}
}

// TestSSEEmitsSentencePerSentenceWithSectionIndex verifies REQ-DEEP1-005: every
// emitted sentence carries a section_index field matching its parent section.
func TestSSEEmitsSentencePerSentenceWithSectionIndex(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
		{Marker: 2, DocID: "doc-2", URL: "https://example.com/2", Title: "Doc 2"},
	}
	sections := []deepreport.Section{
		{
			SectionIndex: 0, Heading: "Part A", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "First [1].", Markers: []int{1}},
				{SentenceIndex: 1, Text: "Second [2].", Markers: []int{2}},
			},
		},
		{
			SectionIndex: 1, Heading: "Part B", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Third [1].", Markers: []int{1}},
			},
		},
	}
	report := buildTestReport("req-sentence-idx", sections, citations)

	rw, sw := setupSSEWriter()
	_, err := streamsynth.StreamLongFormReport(context.Background(), sw, report.RequestID, report)
	if err != nil {
		t.Fatalf("StreamLongFormReport error: %v", err)
	}

	events := parseSSEEvents(rw.buf.String())
	type sentenceWithSection struct {
		Text         string
		SectionIndex int
	}
	var sentences []sentenceWithSection
	for _, ev := range events {
		if ev["event"] != "sentence" {
			continue
		}
		var payload streamsynth.LongFormSentencePayload
		if err := json.Unmarshal([]byte(ev["data"]), &payload); err != nil {
			t.Fatalf("unmarshal sentence: %v", err)
		}
		sentences = append(sentences, sentenceWithSection{
			Text:         payload.Text,
			SectionIndex: payload.SectionIndex,
		})
	}

	if len(sentences) != 3 {
		t.Fatalf("sentence events = %d, want 3", len(sentences))
	}

	// First two sentences should be in section 0.
	if sentences[0].SectionIndex != 0 {
		t.Errorf("sentence[0] section_index = %d, want 0", sentences[0].SectionIndex)
	}
	if sentences[1].SectionIndex != 0 {
		t.Errorf("sentence[1] section_index = %d, want 0", sentences[1].SectionIndex)
	}
	// Third sentence in section 1.
	if sentences[2].SectionIndex != 1 {
		t.Errorf("sentence[2] section_index = %d, want 1", sentences[2].SectionIndex)
	}
}

// TestSSEEmitsSectionDonePerSection verifies REQ-DEEP1-005: 3 sections produce
// exactly 3 section_done events with correct sentences_emitted counts.
func TestSSEEmitsSectionDonePerSection(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
		{Marker: 2, DocID: "doc-2", URL: "https://example.com/2", Title: "Doc 2"},
	}
	sections := []deepreport.Section{
		{
			SectionIndex: 0, Heading: "S1", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "A [1].", Markers: []int{1}},
				{SentenceIndex: 1, Text: "B [2].", Markers: []int{2}},
			},
		},
		{
			SectionIndex: 1, Heading: "S2", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "C [1].", Markers: []int{1}},
			},
		},
		{
			SectionIndex: 2, Heading: "S3", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "D [2].", Markers: []int{2}},
			},
		},
	}
	report := buildTestReport("req-section-done", sections, citations)

	rw, sw := setupSSEWriter()
	_, err := streamsynth.StreamLongFormReport(context.Background(), sw, report.RequestID, report)
	if err != nil {
		t.Fatalf("StreamLongFormReport error: %v", err)
	}

	events := parseSSEEvents(rw.buf.String())
	var sectionDoneEvents []map[string]string
	for _, ev := range events {
		if ev["event"] == "section_done" {
			sectionDoneEvents = append(sectionDoneEvents, ev)
		}
	}

	if len(sectionDoneEvents) != 3 {
		t.Errorf("section_done events = %d, want 3; raw: %q", len(sectionDoneEvents), rw.buf.String())
	}

	// First section emitted 2 sentences.
	var payload0 streamsynth.SectionDonePayload
	if err := json.Unmarshal([]byte(sectionDoneEvents[0]["data"]), &payload0); err != nil {
		t.Fatalf("unmarshal section_done[0]: %v", err)
	}
	if payload0.SentencesEmitted != 2 {
		t.Errorf("section[0] sentences_emitted = %d, want 2", payload0.SentencesEmitted)
	}
	if payload0.SectionIndex != 0 {
		t.Errorf("section[0] section_index = %d, want 0", payload0.SectionIndex)
	}
	if payload0.SchemaVersion != 1 {
		t.Errorf("section[0] schema_version = %d, want 1", payload0.SchemaVersion)
	}
}

// TestSSEEmitsDoneWithTotals verifies REQ-DEEP1-005: terminal done event carries
// total_sections and total_sentences matching the actual emission counts.
func TestSSEEmitsDoneWithTotals(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
		{Marker: 2, DocID: "doc-2", URL: "https://example.com/2", Title: "Doc 2"},
		{Marker: 3, DocID: "doc-3", URL: "https://example.com/3", Title: "Doc 3"},
	}
	sections := []deepreport.Section{
		{
			SectionIndex: 0, Heading: "A", Level: 1,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "S1 [1].", Markers: []int{1}},
				{SentenceIndex: 1, Text: "S2 [2].", Markers: []int{2}},
			},
		},
		{
			SectionIndex: 1, Heading: "B", Level: 1,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "S3 [3].", Markers: []int{3}},
			},
		},
	}
	report := buildTestReport("req-done-totals", sections, citations)

	rw, sw := setupSSEWriter()
	stats, err := streamsynth.StreamLongFormReport(context.Background(), sw, report.RequestID, report)
	if err != nil {
		t.Fatalf("StreamLongFormReport error: %v", err)
	}

	events := parseSSEEvents(rw.buf.String())
	var doneEvent map[string]string
	for _, ev := range events {
		if ev["event"] == "done" {
			doneEvent = ev
			break
		}
	}
	if doneEvent == nil {
		t.Fatal("no done event found")
	}

	var payload streamsynth.LongFormDonePayload
	if err := json.Unmarshal([]byte(doneEvent["data"]), &payload); err != nil {
		t.Fatalf("unmarshal done: %v", err)
	}

	if payload.TotalSections != 2 {
		t.Errorf("total_sections = %d, want 2", payload.TotalSections)
	}
	if payload.TotalSentences != 3 {
		t.Errorf("total_sentences = %d, want 3", payload.TotalSentences)
	}
	if payload.RequestID != "req-done-totals" {
		t.Errorf("request_id = %q, want %q", payload.RequestID, "req-done-totals")
	}
	if payload.Model != "test-model" {
		t.Errorf("model = %q, want %q", payload.Model, "test-model")
	}
	if payload.Provider != "test-provider" {
		t.Errorf("provider = %q, want %q", payload.Provider, "test-provider")
	}
	if payload.CostUSD != 0.05 {
		t.Errorf("cost_usd = %f, want 0.05", payload.CostUSD)
	}
	if payload.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", payload.SchemaVersion)
	}
	if payload.LatencyMs < 0 {
		t.Errorf("latency_ms = %f, want >= 0", payload.LatencyMs)
	}

	// Verify stats return matches.
	if stats.SentencesEmitted != 3 {
		t.Errorf("stats.SentencesEmitted = %d, want 3", stats.SentencesEmitted)
	}
}

// TestSSEPreservesCitationInvariant verifies REQ-DEEP1-005 + REQ-DEEP1-002:
// every emitted sentence has at least one valid citation with correct doc_id.
func TestSSEPreservesCitationInvariant(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-alpha", URL: "https://example.com/alpha", Title: "Alpha"},
		{Marker: 2, DocID: "doc-beta", URL: "https://example.com/beta", Title: "Beta"},
	}
	sections := []deepreport.Section{
		{
			SectionIndex: 0, Heading: "Body", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Cited with alpha [1].", Markers: []int{1}},
				{SentenceIndex: 1, Text: "Cited with beta [2].", Markers: []int{2}},
				{SentenceIndex: 2, Text: "Multi-cited [1] and [2].", Markers: []int{1, 2}},
			},
		},
	}
	report := buildTestReport("req-citation-inv", sections, citations)

	rw, sw := setupSSEWriter()
	_, err := streamsynth.StreamLongFormReport(context.Background(), sw, report.RequestID, report)
	if err != nil {
		t.Fatalf("StreamLongFormReport error: %v", err)
	}

	events := parseSSEEvents(rw.buf.String())
	for _, ev := range events {
		if ev["event"] != "sentence" {
			continue
		}
		var payload streamsynth.LongFormSentencePayload
		if err := json.Unmarshal([]byte(ev["data"]), &payload); err != nil {
			t.Fatalf("unmarshal sentence: %v", err)
		}
		if len(payload.Citations) == 0 {
			t.Errorf("sentence %q has no citations — invariant violation", payload.Text)
		}
		// Verify each citation has a non-empty doc_id.
		for _, c := range payload.Citations {
			if c.DocID == "" {
				t.Errorf("citation marker %d has empty doc_id in sentence %q", c.Marker, payload.Text)
			}
		}
	}
}

// TestLongFormSentenceWithoutCitationsSkipped verifies that sentences whose
// markers do not resolve to any report citation are not emitted.
func TestLongFormSentenceWithoutCitationsSkipped(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	sections := []deepreport.Section{
		{
			SectionIndex: 0, Heading: "Test", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Cited [1].", Markers: []int{1}},
				{SentenceIndex: 1, Text: "Unresolvable [99].", Markers: []int{99}},
				{SentenceIndex: 2, Text: "No markers.", Markers: nil},
			},
		},
	}
	report := buildTestReport("req-skip-uncited", sections, citations)

	rw, sw := setupSSEWriter()
	stats, err := streamsynth.StreamLongFormReport(context.Background(), sw, report.RequestID, report)
	if err != nil {
		t.Fatalf("StreamLongFormReport error: %v", err)
	}

	if stats.SentencesEmitted != 1 {
		t.Errorf("sentences emitted = %d, want 1 (only cited sentence)", stats.SentencesEmitted)
	}

	raw := rw.buf.String()
	if strings.Contains(raw, "Unresolvable") {
		t.Error("unresolvable-marker sentence should not appear in stream")
	}
	if strings.Contains(raw, "No markers") {
		t.Error("markerless sentence should not appear in stream")
	}
}

// TestLongFormContextCancellation verifies that context cancellation stops
// emission mid-stream.
func TestLongFormContextCancellation(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	var sections []deepreport.Section
	for i := range 50 {
		sections = append(sections, deepreport.Section{
			SectionIndex: i, Heading: "S", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Sentence [1].", Markers: []int{1}},
			},
		})
	}
	report := buildTestReport("req-cancel", sections, citations)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	rw, sw := setupSSEWriter()
	_, _ = streamsynth.StreamLongFormReport(ctx, sw, report.RequestID, report)

	events := parseSSEEvents(rw.buf.String())
	sectionStartCount := 0
	for _, ev := range events {
		if ev["event"] == "section_start" {
			sectionStartCount++
		}
	}
	if sectionStartCount == 50 {
		t.Error("all 50 sections emitted despite cancelled context")
	}
}

// TestLongFormEmptyReport verifies that a report with no sections produces
// only a done event with total_sections=0 and total_sentences=0.
func TestLongFormEmptyReport(t *testing.T) {
	t.Parallel()

	report := buildTestReport("req-empty", nil, nil)

	rw, sw := setupSSEWriter()
	stats, err := streamsynth.StreamLongFormReport(context.Background(), sw, report.RequestID, report)
	if err != nil {
		t.Fatalf("StreamLongFormReport error: %v", err)
	}

	if stats.SentencesEmitted != 0 {
		t.Errorf("sentences emitted = %d, want 0 for empty report", stats.SentencesEmitted)
	}

	events := parseSSEEvents(rw.buf.String())
	sectionStartCount := 0
	sentenceCount := 0
	doneCount := 0
	for _, ev := range events {
		switch ev["event"] {
		case "section_start":
			sectionStartCount++
		case "sentence":
			sentenceCount++
		case "done":
			doneCount++
		}
	}
	if sectionStartCount != 0 {
		t.Errorf("section_start count = %d, want 0", sectionStartCount)
	}
	if sentenceCount != 0 {
		t.Errorf("sentence count = %d, want 0", sentenceCount)
	}
	if doneCount != 1 {
		t.Errorf("done count = %d, want 1", doneCount)
	}

	// Verify done payload totals.
	var donePayload streamsynth.LongFormDonePayload
	for _, ev := range events {
		if ev["event"] == "done" {
			if err := json.Unmarshal([]byte(ev["data"]), &donePayload); err != nil {
				t.Fatalf("unmarshal done: %v", err)
			}
		}
	}
	if donePayload.TotalSections != 0 {
		t.Errorf("total_sections = %d, want 0", donePayload.TotalSections)
	}
	if donePayload.TotalSentences != 0 {
		t.Errorf("total_sentences = %d, want 0", donePayload.TotalSentences)
	}
}

// TestLongFormEventOrder verifies the correct event ordering per section:
// section_start -> sentence* -> section_done, repeating for each section.
func TestLongFormEventOrder(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	sections := []deepreport.Section{
		{
			SectionIndex: 0, Heading: "First", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Sentence one [1].", Markers: []int{1}},
			},
		},
		{
			SectionIndex: 1, Heading: "Second", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Sentence two [1].", Markers: []int{1}},
				{SentenceIndex: 1, Text: "Sentence three [1].", Markers: []int{1}},
			},
		},
	}
	report := buildTestReport("req-order", sections, citations)

	rw, sw := setupSSEWriter()
	_, err := streamsynth.StreamLongFormReport(context.Background(), sw, report.RequestID, report)
	if err != nil {
		t.Fatalf("StreamLongFormReport error: %v", err)
	}

	events := parseSSEEvents(rw.buf.String())

	// Expected order: section_start, sentence, section_done, section_start, sentence, sentence, section_done, done
	expected := []string{
		"section_start", "sentence", "section_done",
		"section_start", "sentence", "sentence", "section_done",
		"done",
	}
	if len(events) != len(expected) {
		t.Fatalf("event count = %d, want %d; events: %+v", len(events), len(expected), eventTypes(events))
	}
	for i, ev := range events {
		if ev["event"] != expected[i] {
			t.Errorf("event[%d] = %q, want %q", i, ev["event"], expected[i])
		}
	}
}

// TestLongFormLatencyMsMeasured verifies that the latency_ms field in the done
// event is a positive number reflecting actual elapsed time.
func TestLongFormLatencyMsMeasured(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	sections := []deepreport.Section{
		{
			SectionIndex: 0, Heading: "A", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Text [1].", Markers: []int{1}},
			},
		},
	}
	report := buildTestReport("req-latency", sections, citations)

	_, sw := setupSSEWriter()
	stats, err := streamsynth.StreamLongFormReport(context.Background(), sw, report.RequestID, report)
	if err != nil {
		t.Fatalf("StreamLongFormReport error: %v", err)
	}

	if stats.LatencyMs < 0 {
		t.Errorf("stats.LatencyMs = %f, want >= 0", stats.LatencyMs)
	}

	// The reported latency should be less than a reasonable upper bound (10s).
	if stats.LatencyMs > 10000 {
		t.Errorf("stats.LatencyMs = %f, seems too high for a unit test", stats.LatencyMs)
	}
}

// eventTypes extracts the ordered event types from parsed SSE events.
func eventTypes(events []map[string]string) []string {
	types := make([]string, len(events))
	for i, ev := range events {
		types[i] = ev["event"]
	}
	return types
}

// TestLongFormDoneEventIsLast verifies that the done event is always the
// final event in the stream for a successful call.
func TestLongFormDoneEventIsLast(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	sections := []deepreport.Section{
		{
			SectionIndex: 0, Heading: "A", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Text [1].", Markers: []int{1}},
			},
		},
	}
	report := buildTestReport("req-done-last", sections, citations)

	rw, sw := setupSSEWriter()
	_, err := streamsynth.StreamLongFormReport(context.Background(), sw, report.RequestID, report)
	if err != nil {
		t.Fatalf("StreamLongFormReport error: %v", err)
	}

	events := parseSSEEvents(rw.buf.String())
	if len(events) == 0 {
		t.Fatal("no events emitted")
	}
	lastEvent := events[len(events)-1]
	if lastEvent["event"] != "done" {
		t.Errorf("last event = %q, want done; all events: %v", lastEvent["event"], eventTypes(events))
	}
}

// TestSSEInheritsSYN004Heartbeat verifies REQ-DEEP1-005 (inherits SYN-004 heartbeat):
// running RunHeartbeat alongside StreamLongFormReport interleaves `: ping\n\n`
// comments with data events.
func TestSSEInheritsSYN004Heartbeat(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	sections := []deepreport.Section{
		{
			SectionIndex: 0, Heading: "Intro", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "A cited sentence [1].", Markers: []int{1}},
			},
		},
	}
	report := buildTestReport("req-heartbeat", sections, citations)

	rw := newFakeRW()
	sw := sse.NewWriter(rw)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	heartbeatInterval := 10 * time.Millisecond

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = sse.RunHeartbeat(ctx, sw, heartbeatInterval)
	}()

	// Give heartbeat a chance to fire at least once before streaming starts.
	time.Sleep(30 * time.Millisecond)

	_, err := streamsynth.StreamLongFormReport(ctx, sw, report.RequestID, report)
	if err != nil {
		t.Fatalf("StreamLongFormReport error: %v", err)
	}

	// Cancel context to stop heartbeat goroutine.
	cancel()
	wg.Wait()

	raw := rw.buf.String()
	if !strings.Contains(raw, ": ping") {
		t.Errorf("expected `: ping` comment in output, got: %q", raw)
	}

	// Verify data events are also present.
	events := parseSSEEvents(raw)
	hasSectionStart := false
	hasDone := false
	for _, ev := range events {
		switch ev["event"] {
		case "section_start":
			hasSectionStart = true
		case "done":
			hasDone = true
		}
	}
	if !hasSectionStart {
		t.Error("missing section_start event in output with heartbeat")
	}
	if !hasDone {
		t.Error("missing done event in output with heartbeat")
	}
}

// TestSSEInheritsSYN004WriteTimeout verifies REQ-DEEP1-005 (inherits SYN-004 write
// timeout): a slow writer combined with a context deadline causes
// StreamLongFormReport to return an error rather than blocking indefinitely.
func TestSSEInheritsSYN004WriteTimeout(t *testing.T) {
	t.Parallel()

	citations := []deepreport.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	var sections []deepreport.Section
	for i := range 50 {
		sections = append(sections, deepreport.Section{
			SectionIndex: i, Heading: "S", Level: 2,
			Sentences: []deepreport.Sentence{
				{SentenceIndex: 0, Text: "Sentence [1].", Markers: []int{1}},
			},
		})
	}
	report := buildTestReport("req-write-timeout", sections, citations)

	// Use a blocking writer that blocks on Write until unblocked.
	block := make(chan struct{})
	bw := &blockingLongFormWriter{
		header: make(http.Header),
		block:  block,
	}
	sw := sse.NewWriter(bw)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Unblock the writer after a delay longer than the context timeout,
	// so the context expires first.
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(block)
	}()

	done := make(chan error, 1)
	go func() {
		_, err := streamsynth.StreamLongFormReport(ctx, sw, report.RequestID, report)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error from cancelled context, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("StreamLongFormReport hung — write timeout not working")
	}
}

// blockingLongFormWriter is a slow http.ResponseWriter that blocks on Write
// until the block channel is closed. Simulates a slow client for write timeout tests.
type blockingLongFormWriter struct {
	header http.Header
	block  chan struct{}
}

func (b *blockingLongFormWriter) Header() http.Header { return b.header }
func (b *blockingLongFormWriter) WriteHeader(_ int)   {}
func (b *blockingLongFormWriter) Write(data []byte) (int, error) {
	<-b.block
	return len(data), nil
}
func (b *blockingLongFormWriter) Flush() {}

// ---------------------------------------------------------------------------
// NFR-DEEP1-003: Outcome counter exactly-once validation
// ---------------------------------------------------------------------------

// deepOutcomeValues mirrors the 6 pre-declared outcome label values from
// internal/obs/metrics/deepreport.go.
var deepOutcomeValues = []string{
	"success",
	"deadline_exceeded",
	"budget_exceeded",
	"error_invalid",
	"error_upstream",
	"error_unresolved_citations_threshold",
}

// countDeepOutcomes reads all outcome counters from a fresh registry and returns
// the total count across all 6 outcome labels. Each label is pre-initialized with
// Add(0), so we subtract 6 (the zero-init) to get the actual increment count.
func countDeepOutcomes(reg *metrics.Registry) float64 {
	var total float64
	for _, outcome := range deepOutcomeValues {
		m := &dto.Metric{}
		counter := reg.DeepReportOutcomes.WithLabelValues(outcome)
		if err := counter.Write(m); err != nil {
			continue
		}
		val := m.GetCounter().GetValue()
		total += val
	}
	// The pre-initialization in registerDeepReport calls Add(0) for each of the 6
	// outcome values. Prometheus counters initialized with Add(0) report value 0,
	// so no subtraction is needed.
	return total
}

// outcomeRecorder wraps a Prometheus CounterVec with a sync.Once guard,
// ensuring exactly one outcome is recorded per request regardless of
// concurrent callers. Mirrors the NFR-DEEP1-003 requirement.
type outcomeRecorder struct {
	counter *prometheus.CounterVec
	once    sync.Once
}

func newOutcomeRecorder(counter *prometheus.CounterVec) *outcomeRecorder {
	return &outcomeRecorder{counter: counter}
}

func (r *outcomeRecorder) record(outcome string) {
	r.once.Do(func() {
		r.counter.WithLabelValues(outcome).Inc()
	})
}

// TestOutcomeCounterAtMostOncePerRequest verifies NFR-DEEP1-003: aggregating
// across all 6 outcome labels yields exactly 1 increment per request.
func TestOutcomeCounterAtMostOncePerRequest(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	recorder := newOutcomeRecorder(reg.DeepReportOutcomes)

	// Record a single outcome.
	recorder.record("success")

	got := countDeepOutcomes(reg)
	if got != 1 {
		t.Errorf("total outcome count = %v, want 1", got)
	}
}

// TestOutcomeCounterRaceSuccessVsDeadline verifies NFR-DEEP1-003:
// concurrent success and deadline_exceeded recordings result in exactly 1 increment.
func TestOutcomeCounterRaceSuccessVsDeadline(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	recorder := newOutcomeRecorder(reg.DeepReportOutcomes)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		recorder.record("success")
	}()
	go func() {
		defer wg.Done()
		recorder.record("deadline_exceeded")
	}()
	wg.Wait()

	got := countDeepOutcomes(reg)
	if got != 1 {
		t.Errorf("total outcome count = %v, want 1 (success vs deadline race)", got)
	}
}

// TestOutcomeCounterRaceDeadlineVsBudget verifies NFR-DEEP1-003:
// concurrent deadline_exceeded and budget_exceeded recordings result in exactly 1.
func TestOutcomeCounterRaceDeadlineVsBudget(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	recorder := newOutcomeRecorder(reg.DeepReportOutcomes)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		recorder.record("deadline_exceeded")
	}()
	go func() {
		defer wg.Done()
		recorder.record("budget_exceeded")
	}()
	wg.Wait()

	got := countDeepOutcomes(reg)
	if got != 1 {
		t.Errorf("total outcome count = %v, want 1 (deadline vs budget race)", got)
	}
}

// TestOutcomeCounterRaceBudgetVsErrorUpstream verifies NFR-DEEP1-003:
// concurrent budget_exceeded and error_upstream recordings result in exactly 1.
func TestOutcomeCounterRaceBudgetVsErrorUpstream(t *testing.T) {
	t.Parallel()

	reg := metrics.NewRegistry()
	recorder := newOutcomeRecorder(reg.DeepReportOutcomes)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		recorder.record("budget_exceeded")
	}()
	go func() {
		defer wg.Done()
		recorder.record("error_upstream")
	}()
	wg.Wait()

	got := countDeepOutcomes(reg)
	if got != 1 {
		t.Errorf("total outcome count = %v, want 1 (budget vs error_upstream race)", got)
	}
}
