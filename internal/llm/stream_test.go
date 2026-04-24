// Package llm_test — streaming delta iterator tests.
// REQ-LLM-008: Buffered channel, backpressure, context cancellation, error on final Delta.
package llm_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/llm"
)

// sseResponse builds a minimal SSE body with the given content chunks.
func sseResponse(chunks []string) string {
	var sb strings.Builder
	for _, chunk := range chunks {
		sb.WriteString("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"claude-sonnet-4-6\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"")
		sb.WriteString(chunk)
		sb.WriteString("\"},\"finish_reason\":null}]}\n\n")
	}
	sb.WriteString("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"claude-sonnet-4-6\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
	sb.WriteString("data: [DONE]\n\n")
	return sb.String()
}

// TestStreamChannelClosesOnEOF verifies the channel closes when the stream ends.
// REQ-LLM-008
func TestStreamChannelClosesOnEOF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.WriteString(w, sseResponse([]string{"hello", " ", "world"}))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	ch, err := c.Stream(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var collected []string
	timeout := time.After(5 * time.Second)
	for {
		select {
		case d, ok := <-ch:
			if !ok {
				goto done
			}
			if d.Err != nil {
				t.Errorf("unexpected delta error: %v", d.Err)
			}
			collected = append(collected, d.Content)
		case <-timeout:
			t.Fatal("stream did not close within timeout")
		}
	}
done:
	combined := strings.Join(collected, "")
	if !strings.Contains(combined, "hello") {
		t.Errorf("expected content to contain 'hello', got %q", combined)
	}
}

// TestStreamContextCancellationStopsReading verifies ctx cancel closes the channel.
// REQ-LLM-008
func TestStreamContextCancellationStopsReading(t *testing.T) {
	// Server sends chunks slowly so we can cancel in the middle.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("expected Flusher")
			return
		}
		// Send one chunk then block.
		_, _ = io.WriteString(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"chunk1\"},\"finish_reason\":null}]}\n\n")
		flusher.Flush()
		// Block until context cancels.
		time.Sleep(10 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	c := makeTestClient(t, srv.URL)
	ch, err := c.Stream(ctx, llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	// Read first delta, then cancel.
	timeout := time.After(5 * time.Second)
	select {
	case <-ch:
		cancel()
	case <-timeout:
		cancel()
	}

	// Channel must drain and close after cancellation.
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for range ch {
		}
	}()

	select {
	case <-closed:
		// Channel closed — expected.
	case <-time.After(5 * time.Second):
		t.Error("channel did not close after context cancellation")
	}
}

// TestStreamFinishReasonPropagated verifies finish_reason is set on the final delta.
// REQ-LLM-008
func TestStreamFinishReasonPropagated(t *testing.T) {
	sseBody := "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"done\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sseBody)
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	ch, err := c.Stream(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var lastFinish string
	timeout := time.After(5 * time.Second)
	for {
		select {
		case d, ok := <-ch:
			if !ok {
				goto done
			}
			if d.FinishReason != "" {
				lastFinish = d.FinishReason
			}
		case <-timeout:
			t.Fatal("stream timed out")
		}
	}
done:
	if lastFinish != "stop" {
		t.Errorf("finish_reason: got %q, want %q", lastFinish, "stop")
	}
}

// TestStreamEmptyBodyClosesChannel verifies an empty SSE stream closes the channel cleanly.
// REQ-LLM-008
func TestStreamEmptyBodyClosesChannel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	ch, err := c.Stream(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	timeout := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // expected: channel closed
			}
		case <-timeout:
			t.Error("channel did not close for empty stream")
			return
		}
	}
}

// TestStreamUnknownClassFails verifies ErrModelNotConfigured for unknown class.
// REQ-LLM-008
func TestStreamUnknownClassFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	_, err := c.Stream(context.Background(), llm.Request{
		Class:    llm.ModelClass("nonexistent"),
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Error("expected error for unknown model class, got nil")
	}
}
