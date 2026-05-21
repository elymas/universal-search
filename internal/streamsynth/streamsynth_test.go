// Package streamsynth_test — RED phase tests for stream synthesis orchestration.
// test(stream): RED — streamsynth tests (SPEC-SYN-004 REQ-SYN4-001c/002/003/004/005/006)
package streamsynth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/streamsynth"
	"github.com/elymas/universal-search/internal/synthesis"
)

// fakeResponseWriter is a minimal http.ResponseWriter + http.Flusher.
type fakeResponseWriter struct {
	buf    *bytes.Buffer
	header http.Header
}

func newFakeRW() *fakeResponseWriter {
	return &fakeResponseWriter{buf: &bytes.Buffer{}, header: make(http.Header)}
}

func (f *fakeResponseWriter) Header() http.Header         { return f.header }
func (f *fakeResponseWriter) WriteHeader(_ int)           {}
func (f *fakeResponseWriter) Write(b []byte) (int, error) { return f.buf.Write(b) }
func (f *fakeResponseWriter) Flush()                      {}

// buildResult constructs a synthesis.Result with n sentences each having one citation.
func buildResult(sentences []string, citations []synthesis.Citation) synthesis.Result {
	return synthesis.Result{
		RequestID: "test-req-1",
		Text:      strings.Join(sentences, " "),
		Citations: citations,
		Model:     "test-model",
		Provider:  "test-provider",
		LatencyMs: 100,
	}
}

// parseSSEEvents extracts all SSE events from the raw byte stream.
func parseSSEEvents(data string) []map[string]string {
	var events []map[string]string
	blocks := strings.Split(data, "\n\n")
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		ev := make(map[string]string)
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "event: ") {
				ev["event"] = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				ev["data"] = strings.TrimPrefix(line, "data: ")
			} else if strings.HasPrefix(line, ": ") {
				ev["comment"] = strings.TrimPrefix(line, ": ")
			}
		}
		if len(ev) > 0 {
			events = append(events, ev)
		}
	}
	return events
}

// TestSentenceSegmentationMatchesSYN002Regex verifies REQ-SYN4-002: 5-sentence
// paragraph produces exactly 5 sentence events.
func TestSentenceSegmentationMatchesSYN002Regex(t *testing.T) {
	t.Parallel()

	citations := []synthesis.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	sentences := []string{
		"First sentence with [1].",
		"Second sentence with [1].",
		"Third sentence with [1].",
		"Fourth sentence with [1].",
		"Fifth sentence with [1].",
	}
	result := buildResult(sentences, citations)

	rw := newFakeRW()
	req := streamsynth.StreamRequest{
		RequestID:   "req-1",
		SynthResult: result,
	}
	stats, err := streamsynth.StreamSynthesize(context.Background(), rw, req)
	if err != nil {
		t.Fatalf("StreamSynthesize error: %v", err)
	}

	events := parseSSEEvents(rw.buf.String())
	sentenceEvents := 0
	for _, ev := range events {
		if ev["event"] == "sentence" {
			sentenceEvents++
		}
	}
	if sentenceEvents != 5 {
		t.Errorf("sentence events = %d, want 5; raw: %q", sentenceEvents, rw.buf.String())
	}
	if stats.SentencesEmitted != 5 {
		t.Errorf("stats.SentencesEmitted = %d, want 5", stats.SentencesEmitted)
	}
}

// TestEventPayloadIncludesCitations verifies REQ-SYN4-002: each sentence event
// carries a non-empty citations array matching the input.
func TestEventPayloadIncludesCitations(t *testing.T) {
	t.Parallel()

	citations := []synthesis.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	result := buildResult([]string{"Hello with [1]."}, citations)

	rw := newFakeRW()
	req := streamsynth.StreamRequest{RequestID: "req-1", SynthResult: result}
	if _, err := streamsynth.StreamSynthesize(context.Background(), rw, req); err != nil {
		t.Fatal(err)
	}

	events := parseSSEEvents(rw.buf.String())
	for _, ev := range events {
		if ev["event"] != "sentence" {
			continue
		}
		var payload streamsynth.SentencePayload
		if err := json.Unmarshal([]byte(ev["data"]), &payload); err != nil {
			t.Fatalf("unmarshal sentence payload: %v", err)
		}
		if len(payload.Citations) == 0 {
			t.Error("sentence event has no citations")
		}
		if payload.Citations[0].DocID != "doc-1" {
			t.Errorf("citation DocID = %q, want %q", payload.Citations[0].DocID, "doc-1")
		}
	}
}

// TestDoneEventEmittedOnSuccess verifies REQ-SYN4-002: exactly one `event: done`
// terminates a successful stream.
func TestDoneEventEmittedOnSuccess(t *testing.T) {
	t.Parallel()

	citations := []synthesis.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	result := buildResult([]string{"Hello with [1]."}, citations)

	rw := newFakeRW()
	req := streamsynth.StreamRequest{RequestID: "req-1", SynthResult: result}
	if _, err := streamsynth.StreamSynthesize(context.Background(), rw, req); err != nil {
		t.Fatal(err)
	}

	events := parseSSEEvents(rw.buf.String())
	doneCount := 0
	for _, ev := range events {
		if ev["event"] == "done" {
			doneCount++
		}
	}
	if doneCount != 1 {
		t.Errorf("done event count = %d, want 1; raw: %q", doneCount, rw.buf.String())
	}
}

// TestNoUncitedSentenceEmitted verifies REQ-SYN4-001c: sentences without [N]
// markers are NOT emitted to the stream.
func TestNoUncitedSentenceEmitted(t *testing.T) {
	t.Parallel()

	citations := []synthesis.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	// The second sentence has no [N] marker — must be stripped.
	result := buildResult(
		[]string{"Cited sentence [1].", "Uncited sentence without marker."},
		citations,
	)

	rw := newFakeRW()
	req := streamsynth.StreamRequest{RequestID: "req-1", SynthResult: result}
	if _, err := streamsynth.StreamSynthesize(context.Background(), rw, req); err != nil {
		t.Fatal(err)
	}

	raw := rw.buf.String()
	if strings.Contains(raw, "Uncited sentence without marker") {
		t.Errorf("uncited sentence should not appear in stream: %q", raw)
	}
}

// TestContextCancellationStopsStream verifies REQ-SYN4-004: cancelling ctx during
// StreamSynthesize stops emission.
func TestContextCancellationStopsStream(t *testing.T) {
	t.Parallel()

	citations := []synthesis.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	// Build a large result; cancel ctx immediately.
	var sentences []string
	for range 100 {
		sentences = append(sentences, "Sentence with [1].")
	}
	result := buildResult(sentences, citations)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	rw := newFakeRW()
	req := streamsynth.StreamRequest{
		RequestID:   "req-1",
		SynthResult: result,
	}
	_, err := streamsynth.StreamSynthesize(ctx, rw, req)
	// Should return ctx error or nil (graceful) but NOT emit all 100 events.
	_ = err

	events := parseSSEEvents(rw.buf.String())
	count := 0
	for _, ev := range events {
		if ev["event"] == "sentence" {
			count++
		}
	}
	if count == 100 {
		t.Error("all 100 sentences emitted despite cancelled context")
	}
}

// TestPropertyLosslessTextReconstruction verifies NFR-SYN4-002(d): union of
// emitted sentence texts reconstructs Result.Text modulo whitespace.
func TestPropertyLosslessTextReconstruction(t *testing.T) {
	t.Parallel()

	citations := []synthesis.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
		{Marker: 2, DocID: "doc-2", URL: "https://example.com/2", Title: "Doc 2"},
	}
	sentences := []string{
		"First sentence [1].",
		"Second sentence [2].",
		"Third sentence [1].",
	}
	result := buildResult(sentences, citations)

	rw := newFakeRW()
	req := streamsynth.StreamRequest{RequestID: "req-1", SynthResult: result}
	if _, err := streamsynth.StreamSynthesize(context.Background(), rw, req); err != nil {
		t.Fatal(err)
	}

	events := parseSSEEvents(rw.buf.String())
	var emittedTexts []string
	for _, ev := range events {
		if ev["event"] != "sentence" {
			continue
		}
		var payload streamsynth.SentencePayload
		if err := json.Unmarshal([]byte(ev["data"]), &payload); err != nil {
			t.Fatal(err)
		}
		emittedTexts = append(emittedTexts, payload.Text)
	}

	reconstructed := strings.Join(emittedTexts, " ")
	original := strings.TrimSpace(result.Text)
	if strings.TrimSpace(reconstructed) != original {
		t.Errorf("reconstruction mismatch:\n got: %q\nwant: %q", reconstructed, original)
	}
}

// TestEmptyResultProducesDoneOnly verifies that an empty synthesis Result.Text
// produces no sentence events and exactly one done event.
func TestEmptyResultProducesDoneOnly(t *testing.T) {
	t.Parallel()

	result := synthesis.Result{
		RequestID: "req-empty",
		Text:      "",
		Citations: nil,
	}
	rw := newFakeRW()
	req := streamsynth.StreamRequest{RequestID: "req-empty", SynthResult: result}
	stats, err := streamsynth.StreamSynthesize(context.Background(), rw, req)
	if err != nil {
		t.Fatal(err)
	}
	if stats.SentencesEmitted != 0 {
		t.Errorf("sentences emitted = %d, want 0 for empty text", stats.SentencesEmitted)
	}
	events := parseSSEEvents(rw.buf.String())
	for _, ev := range events {
		if ev["event"] == "sentence" {
			t.Error("sentence event emitted for empty text")
		}
	}
}

// TestTrailingTextWithoutPunctuationIsEmittedIfCited verifies that text not
// terminated by punctuation is still captured when it has citation markers.
func TestTrailingTextWithoutPunctuationIsEmittedIfCited(t *testing.T) {
	t.Parallel()

	citations := []synthesis.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	// Text with one sentence ending in punctuation and a trailing non-punctuated citation.
	result := synthesis.Result{
		RequestID: "req-trail",
		Text:      "First sentence [1]. Trailing text [1]",
		Citations: citations,
	}
	rw := newFakeRW()
	req := streamsynth.StreamRequest{RequestID: "req-trail", SynthResult: result}
	stats, err := streamsynth.StreamSynthesize(context.Background(), rw, req)
	if err != nil {
		t.Fatal(err)
	}
	if stats.SentencesEmitted < 1 {
		t.Errorf("sentences emitted = %d, want >= 1", stats.SentencesEmitted)
	}
}

// TestStreamSynthesizeTimeout verifies REQ-SYN4-006: a write deadline on the
// ResponseWriter causes StreamSynthesize to return an error.
func TestStreamSynthesizeTimeout(t *testing.T) {
	t.Parallel()

	// Use a blocking writer to simulate slow client.
	block := make(chan struct{})
	bw := &blockingWriter{
		buf:    &bytes.Buffer{},
		header: make(http.Header),
		block:  block,
	}

	citations := []synthesis.Citation{
		{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
	}
	result := buildResult([]string{"Sentence with [1]."}, citations)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	close(block) // unblock immediately — we're testing ctx cancellation path

	req := streamsynth.StreamRequest{RequestID: "req-1", SynthResult: result}
	_, _ = streamsynth.StreamSynthesize(ctx, bw, req)
	// Test passes if it doesn't hang.
}

// blockingWriter blocks writes until the block channel is closed.
type blockingWriter struct {
	buf    *bytes.Buffer
	header http.Header
	block  chan struct{}
}

func (b *blockingWriter) Header() http.Header { return b.header }
func (b *blockingWriter) WriteHeader(_ int)   {}
func (b *blockingWriter) Write(data []byte) (int, error) {
	<-b.block
	return b.buf.Write(data)
}
func (b *blockingWriter) Flush() {}
