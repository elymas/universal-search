// Package synthesis — SSE stream consumer for real-time synthesis output.
//
// SPEC-CLI-002 REQ-CLI2-005: Streaming SSE consumer with sentence-by-sentence
// rendering. Shared between CLI and MCP transport.
package synthesis

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// SSEEvent represents a single Server-Sent Event.
type SSEEvent struct {
	Event string // event type (e.g., "sentence", "citation", "done", "error")
	Data  string // data payload
	ID    string // optional event ID
	Retry int    // optional retry interval
}

// SSEParser reads SSE events from an io.Reader.
// Implements the SSE specification: https://html.spec.whatwg.org/multipage/server-sent-events.html
type SSEParser struct {
	scanner *bufio.Scanner
}

// NewSSEParser creates a new SSE parser reading from r.
func NewSSEParser(r io.Reader) *SSEParser {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line
	return &SSEParser{scanner: s}
}

// Next reads the next SSE event. Returns io.EOF when the stream ends.
func (p *SSEParser) Next() (*SSEEvent, error) {
	var event SSEEvent
	hasContent := false

	for p.scanner.Scan() {
		line := p.scanner.Text()

		// Empty line dispatches the event.
		if line == "" {
			if hasContent {
				return &event, nil
			}
			continue
		}

		// Skip comments (lines starting with ':').
		if strings.HasPrefix(line, ":") {
			continue
		}

		hasContent = true

		// Parse field: value.
		field, value := parseSSEField(line)
		switch field {
		case "event":
			event.Event = value
		case "data":
			if event.Data != "" {
				event.Data += "\n"
			}
			event.Data += value
		case "id":
			event.ID = value
		case "retry":
			// retry is parsed but not used by this consumer.
		}
	}

	if err := p.scanner.Err(); err != nil {
		return nil, fmt.Errorf("sse: scan: %w", err)
	}

	if hasContent {
		return &event, nil
	}
	return nil, io.EOF
}

// parseSSEField splits "field: value" or "field" into field and value.
func parseSSEField(line string) (string, string) {
	idx := strings.IndexByte(line, ':')
	if idx < 0 {
		return line, ""
	}
	// If the character after ':' is a space, skip it (per SSE spec).
	valueStart := idx + 1
	if valueStart < len(line) && line[valueStart] == ' ' {
		valueStart++
	}
	return line[:idx], line[valueStart:]
}

// StreamEvent represents a parsed stream event for display.
type StreamEvent struct {
	Type      StreamEventType
	Text      string   // sentence text or error message
	Citations []Citation // citations for the current sentence
}

// StreamEventType identifies the type of stream event.
type StreamEventType int

const (
	StreamSentence StreamEventType = iota
	StreamCitation
	StreamDone
	StreamError
)

// StreamConsumer processes SSE events into display-ready StreamEvents.
type StreamConsumer struct {
	parser *SSEParser
}

// NewStreamConsumer creates a consumer that reads SSE events from r.
func NewStreamConsumer(r io.Reader) *StreamConsumer {
	return &StreamConsumer{
		parser: NewSSEParser(r),
	}
}

// Next returns the next display-ready stream event.
// Returns io.EOF when the stream is complete.
func (c *StreamConsumer) Next() (*StreamEvent, error) {
	ev, err := c.parser.Next()
	if err != nil {
		return nil, err
	}

	switch ev.Event {
	case "sentence", "": // default event type is "message" (empty event field)
		return &StreamEvent{
			Type: StreamSentence,
			Text: ev.Data,
		}, nil
	case "citation":
		return &StreamEvent{
			Type: StreamCitation,
			Text: ev.Data,
		}, nil
	case "done":
		return &StreamEvent{
			Type: StreamDone,
			Text: ev.Data,
		}, nil
	case "error":
		return &StreamEvent{
			Type: StreamError,
			Text: ev.Data,
		}, nil
	default:
		// Unknown event type: treat as sentence.
		return &StreamEvent{
			Type: StreamSentence,
			Text: ev.Data,
		}, nil
	}
}

// IsTerminal checks if the file descriptor is connected to a terminal.
// This is used to auto-detect --stream mode.
func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
