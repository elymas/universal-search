// Package sse_test — RED phase tests for the SSE writer.
// test(stream): RED — SSE writer wire-format and header tests (SPEC-SYN-004 REQ-SYN4-001a/001b)
package sse_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/sse"
)

// TestSSEContentTypeSet verifies REQ-SYN4-001a: every SSE response includes the
// required three headers.
func TestSSEContentTypeSet(t *testing.T) {
	t.Parallel()
	rr := httptest.NewRecorder()
	w := sse.NewWriter(rr)
	w.SetHeaders()

	hdr := rr.Result().Header
	if got := hdr.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", got, "text/event-stream")
	}
	if got := hdr.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
	}
	if got := hdr.Get("Connection"); got != "keep-alive" {
		t.Errorf("Connection = %q, want %q", got, "keep-alive")
	}
}

// TestSSEWireFormatBlankLineTerminator verifies REQ-SYN4-001b: each event ends with \n\n.
func TestSSEWireFormatBlankLineTerminator(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	rr := &fakeResponseWriter{buf: &buf, header: make(http.Header)}
	w := sse.NewWriter(rr)

	if err := w.WriteEvent("sentence", []byte(`{"text":"Hello [1]."}`)); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.HasSuffix(got, "\n\n") {
		t.Errorf("event does not end with \\n\\n: %q", got)
	}
	if !strings.HasPrefix(got, "event: sentence\ndata: ") {
		t.Errorf("event does not start with expected prefix: %q", got)
	}
}

// TestSSECommentWireFormat verifies REQ-SYN4-001b: heartbeat comment is `: ping\n\n`.
func TestSSECommentWireFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	rr := &fakeResponseWriter{buf: &buf, header: make(http.Header)}
	w := sse.NewWriter(rr)

	if err := w.WriteComment("ping"); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	want := ": ping\n\n"
	got := buf.String()
	if got != want {
		t.Errorf("comment = %q, want %q", got, want)
	}
}

// TestSSEDataMultilineRepeatsPrefix verifies REQ-SYN4-001b: JSON with embedded \n
// is emitted as multiple data: lines.
func TestSSEDataMultilineRepeatsPrefix(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	rr := &fakeResponseWriter{buf: &buf, header: make(http.Header)}
	w := sse.NewWriter(rr)

	payload := "line1\nline2"
	if err := w.WriteEvent("sentence", []byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	if !strings.Contains(got, "data: line1\ndata: line2") {
		t.Errorf("multiline data not split correctly: %q", got)
	}
}

// fakeResponseWriter is a minimal http.ResponseWriter backed by a bytes.Buffer.
// It implements http.Flusher so Writer.Flush() can work.
type fakeResponseWriter struct {
	buf    *bytes.Buffer
	header http.Header
	status int
}

func (f *fakeResponseWriter) Header() http.Header        { return f.header }
func (f *fakeResponseWriter) WriteHeader(code int)       { f.status = code }
func (f *fakeResponseWriter) Write(b []byte) (int, error) { return f.buf.Write(b) }
func (f *fakeResponseWriter) Flush()                      {} // implements http.Flusher
