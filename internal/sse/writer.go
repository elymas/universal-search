// Package sse provides a W3C Server-Sent Events writer for net/http.
//
// REQ-SYN4-001a: SSE response headers (Content-Type, Cache-Control, Connection).
// REQ-SYN4-001b: W3C SSE wire format (event/data/comment fields, \n\n terminator).
// REQ-SYN4-003:  Thread-safe writes shared between heartbeat and main writer goroutines.
//
// @MX:ANCHOR: [AUTO] SSE wire-format writer; callers: streamsynth.StreamSynthesize, heartbeat, handlers
// @MX:REASON: fan_in >= 3; all SSE emission flows through Writer.WriteEvent and WriteComment
package sse

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// Writer wraps an http.ResponseWriter and provides SSE-compliant event emission.
// All methods are safe for concurrent use.
type Writer struct {
	mu sync.Mutex
	w  http.ResponseWriter
	f  http.Flusher
}

// NewWriter constructs a Writer backed by w. w must implement http.Flusher for
// SSE flushing to work; if it does not, Flush() is a no-op.
func NewWriter(w http.ResponseWriter) *Writer {
	sw := &Writer{w: w}
	if f, ok := w.(http.Flusher); ok {
		sw.f = f
	}
	return sw
}

// SetHeaders writes the three required SSE response headers (REQ-SYN4-001a).
// Must be called before the first Write to the underlying ResponseWriter.
func (w *Writer) SetHeaders() {
	h := w.w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
}

// WriteEvent emits an SSE event with the given event type and data payload
// (REQ-SYN4-001b). Multi-line data is split across multiple `data:` lines.
func (w *Writer) WriteEvent(eventType string, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writeEventLocked(eventType, data)
}

func (w *Writer) writeEventLocked(eventType string, data []byte) error {
	// Write event: field.
	if _, err := fmt.Fprintf(w.w, "event: %s\n", eventType); err != nil {
		return err
	}

	// Write data: field — split on embedded newlines per W3C spec.
	payload := string(data)
	lines := strings.Split(payload, "\n")
	for _, line := range lines {
		if _, err := fmt.Fprintf(w.w, "data: %s\n", line); err != nil {
			return err
		}
	}

	// Blank line terminates the event.
	if _, err := fmt.Fprintf(w.w, "\n"); err != nil {
		return err
	}
	return nil
}

// WriteComment emits an SSE comment in the form `: <text>\n\n` (REQ-SYN4-001b).
// Used by the heartbeat goroutine.
func (w *Writer) WriteComment(text string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := fmt.Fprintf(w.w, ": %s\n\n", text)
	return err
}

// Flush flushes the underlying ResponseWriter if it implements http.Flusher.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f != nil {
		w.f.Flush()
	}
	return nil
}

// Close is a no-op for HTTP/1.1 SSE (connection close is handled by the HTTP
// server lifecycle). Provided for interface symmetry.
func (w *Writer) Close() error {
	return nil
}
