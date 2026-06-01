// Package synthesis — tests for SSE stream consumer.
// SPEC-CLI-002 REQ-CLI2-005: Streaming SSE consumer with sentence rendering.
package synthesis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSSEParserIDAndRetryFields exercises the id and retry field cases of the
// parser switch, plus the EOF-with-trailing-content path (no terminating blank
// line), which the existing parser tests do not reach.
func TestSSEParserIDAndRetryFields(t *testing.T) {
	// id and retry are parsed; the event still terminates on the blank line.
	input := "id: 42\nretry: 3000\nevent: sentence\ndata: hi\n\n"
	p := NewSSEParser(strings.NewReader(input))
	ev, err := p.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev.ID != "42" {
		t.Errorf("ID = %q, want 42", ev.ID)
	}
	if ev.Event != "sentence" || ev.Data != "hi" {
		t.Errorf("event/data = %q/%q, want sentence/hi", ev.Event, ev.Data)
	}
}

func TestSSEParserTrailingContentWithoutBlankLine(t *testing.T) {
	// No terminating blank line: the parser must still emit the buffered event
	// at EOF rather than dropping it.
	p := NewSSEParser(strings.NewReader("data: last chunk"))
	ev, err := p.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev.Data != "last chunk" {
		t.Errorf("Data = %q, want 'last chunk'", ev.Data)
	}
}

// TestStreamConsumerUnknownEventTreatedAsSentence verifies the default switch
// branch: an event type the consumer does not recognise is surfaced as a
// sentence rather than being dropped.
func TestStreamConsumerUnknownEventTreatedAsSentence(t *testing.T) {
	input := "event: heartbeat\ndata: still alive\n\n"
	consumer := NewStreamConsumer(strings.NewReader(input))

	ev, err := consumer.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev.Type != StreamSentence {
		t.Errorf("Type = %d, want StreamSentence (default branch)", ev.Type)
	}
	if ev.Text != "still alive" {
		t.Errorf("Text = %q, want %q", ev.Text, "still alive")
	}
}

// TestIsTerminal verifies the char-device detection: a regular file is never a
// terminal, and a stat error (closed file) returns false.
func TestIsTerminal(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "regular"))
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer func() { _ = f.Close() }()

	if IsTerminal(f) {
		t.Error("regular file must not be reported as a terminal")
	}

	// A closed file makes Stat fail, exercising the error branch.
	closed, err := os.Create(filepath.Join(t.TempDir(), "closed"))
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	_ = closed.Close()
	if IsTerminal(closed) {
		t.Error("closed file (stat error) must return false")
	}
}

// --- Phase 4: Streaming SSE tests ---

// TestSSEParserBasicEvent verifies parsing a simple SSE event.
func TestSSEParserBasicEvent(t *testing.T) {
	input := "event: sentence\ndata: Hello world\n\n"
	parser := NewSSEParser(strings.NewReader(input))

	ev, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev.Event != "sentence" {
		t.Errorf("Event = %q, want %q", ev.Event, "sentence")
	}
	if ev.Data != "Hello world" {
		t.Errorf("Data = %q, want %q", ev.Data, "Hello world")
	}

	// Should get EOF on next call.
	_, err = parser.Next()
	if err == nil {
		t.Error("expected EOF after last event")
	}
}

// TestSSEParserMultipleEvents verifies parsing multiple SSE events.
func TestSSEParserMultipleEvents(t *testing.T) {
	input := "event: sentence\ndata: First sentence.\n\n" +
		"event: sentence\ndata: Second sentence.\n\n" +
		"event: done\ndata: complete\n\n"
	parser := NewSSEParser(strings.NewReader(input))

	events := make([]*SSEEvent, 0)
	for {
		ev, err := parser.Next()
		if err != nil {
			break
		}
		events = append(events, ev)
	}

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	if events[0].Data != "First sentence." {
		t.Errorf("event[0].Data = %q", events[0].Data)
	}
	if events[1].Data != "Second sentence." {
		t.Errorf("event[1].Data = %q", events[1].Data)
	}
	if events[2].Event != "done" {
		t.Errorf("event[2].Event = %q", events[2].Event)
	}
}

// TestSSEParserCitationEvent verifies citation event parsing.
func TestSSEParserCitationEvent(t *testing.T) {
	input := "event: citation\ndata: {\"marker\":1,\"doc_id\":\"doc1\",\"url\":\"https://example.com\",\"title\":\"Example\"}\n\n"
	parser := NewSSEParser(strings.NewReader(input))

	ev, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev.Event != "citation" {
		t.Errorf("Event = %q, want %q", ev.Event, "citation")
	}
	if !strings.Contains(ev.Data, "doc1") {
		t.Errorf("Data missing citation info: %q", ev.Data)
	}
}

// TestSSEParserErrorEvent verifies error event parsing.
func TestSSEParserErrorEvent(t *testing.T) {
	input := "event: error\ndata: synthesis timeout\n\n"
	parser := NewSSEParser(strings.NewReader(input))

	ev, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev.Event != "error" {
		t.Errorf("Event = %q, want %q", ev.Event, "error")
	}
	if ev.Data != "synthesis timeout" {
		t.Errorf("Data = %q, want %q", ev.Data, "synthesis timeout")
	}
}

// TestSSEParserCommentSkipped verifies comments are ignored.
func TestSSEParserCommentSkipped(t *testing.T) {
	input := ": this is a comment\nevent: sentence\ndata: Hello\n\n"
	parser := NewSSEParser(strings.NewReader(input))

	ev, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev.Data != "Hello" {
		t.Errorf("Data = %q, want %q", ev.Data, "Hello")
	}
}

// TestSSEParserMultilineData verifies multiline data fields.
func TestSSEParserMultilineData(t *testing.T) {
	input := "data: line one\ndata: line two\ndata: line three\n\n"
	parser := NewSSEParser(strings.NewReader(input))

	ev, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	expected := "line one\nline two\nline three"
	if ev.Data != expected {
		t.Errorf("Data = %q, want %q", ev.Data, expected)
	}
}

// TestSSEParserEmptyStream verifies EOF on empty input.
func TestSSEParserEmptyStream(t *testing.T) {
	parser := NewSSEParser(strings.NewReader(""))

	_, err := parser.Next()
	if err == nil {
		t.Error("expected error on empty stream")
	}
}

// TestStreamConsumerSentenceEvents verifies the consumer converts SSE to StreamEvents.
func TestStreamConsumerSentenceEvents(t *testing.T) {
	input := "event: sentence\ndata: AI research is advancing.\n\n" +
		"event: sentence\ndata: New models show promise.\n\n" +
		"event: done\ndata: \n\n"

	consumer := NewStreamConsumer(strings.NewReader(input))

	// First sentence.
	ev1, err := consumer.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev1.Type != StreamSentence {
		t.Errorf("Type = %d, want StreamSentence", ev1.Type)
	}
	if ev1.Text != "AI research is advancing." {
		t.Errorf("Text = %q", ev1.Text)
	}

	// Second sentence.
	ev2, err := consumer.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev2.Text != "New models show promise." {
		t.Errorf("Text = %q", ev2.Text)
	}

	// Done event.
	ev3, err := consumer.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev3.Type != StreamDone {
		t.Errorf("Type = %d, want StreamDone", ev3.Type)
	}
}

// TestStreamConsumerCitationEvent verifies citation stream events.
func TestStreamConsumerCitationEvent(t *testing.T) {
	input := "event: citation\ndata: {\"marker\":1}\n\n"
	consumer := NewStreamConsumer(strings.NewReader(input))

	ev, err := consumer.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev.Type != StreamCitation {
		t.Errorf("Type = %d, want StreamCitation", ev.Type)
	}
}

// TestStreamConsumerErrorEvent verifies error stream events.
func TestStreamConsumerErrorEvent(t *testing.T) {
	input := "event: error\ndata: timeout exceeded\n\n"
	consumer := NewStreamConsumer(strings.NewReader(input))

	ev, err := consumer.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if ev.Type != StreamError {
		t.Errorf("Type = %d, want StreamError", ev.Type)
	}
	if ev.Text != "timeout exceeded" {
		t.Errorf("Text = %q", ev.Text)
	}
}

// TestSSEParserDefaultEvent verifies default event type (empty event field).
func TestSSEParserDefaultEvent(t *testing.T) {
	input := "data: default event\n\n"
	parser := NewSSEParser(strings.NewReader(input))

	ev, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	// Consumer should treat default event as sentence.
	consumer := &StreamConsumer{parser: parser}
	_ = consumer

	// Verify the raw SSE event has empty event field.
	if ev.Event != "" {
		t.Errorf("Event = %q, want empty string (default)", ev.Event)
	}
	if ev.Data != "default event" {
		t.Errorf("Data = %q", ev.Data)
	}
}

// TestParseSSEField verifies field parsing.
func TestParseSSEField(t *testing.T) {
	tests := []struct {
		line      string
		wantField string
		wantValue string
	}{
		{"data: hello", "data", "hello"},
		{"data:hello", "data", "hello"},
		{"event: sentence", "event", "sentence"},
		{"id: 123", "id", "123"},
		{"retry: 5000", "retry", "5000"},
		{"data", "data", ""},
	}

	for _, tt := range tests {
		field, value := parseSSEField(tt.line)
		if field != tt.wantField || value != tt.wantValue {
			t.Errorf("parseSSEField(%q) = (%q, %q), want (%q, %q)",
				tt.line, field, value, tt.wantField, tt.wantValue)
		}
	}
}
