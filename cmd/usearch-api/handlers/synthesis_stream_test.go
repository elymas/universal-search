// Package handlers_test — integration tests for synthesis SSE endpoint.
// test(stream): RED/GREEN — HTTP handler integration tests (SPEC-SYN-004 REQ-SYN4-001a/005)
package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elymas/universal-search/cmd/usearch-api/handlers"
	"github.com/elymas/universal-search/internal/synthesis"
)

// fakeClient implements handlers.SynthesisClient for testing.
type fakeClient struct {
	result synthesis.Result
	err    error
}

func (f *fakeClient) Synthesize(_ interface{}, _ synthesis.Request) (synthesis.Result, error) {
	return f.result, f.err
}

func newTestResult() synthesis.Result {
	return synthesis.Result{
		RequestID: "test-req-1",
		Text:      "Hello [1]. World [1].",
		Citations: []synthesis.Citation{
			{Marker: 1, DocID: "doc-1", URL: "https://example.com/1", Title: "Doc 1"},
		},
		Model:     "test-model",
		Provider:  "test-provider",
		LatencyMs: 50,
	}
}

// TestSSEHappyPath verifies REQ-SYN4-001a: with Accept: text/event-stream,
// the response has Content-Type: text/event-stream and sentence events.
func TestSSEHappyPath(t *testing.T) {
	t.Parallel()

	client := &fakeClient{result: newTestResult()}
	h := handlers.NewSynthesisHandler(client, handlers.DefaultConfig())

	req := httptest.NewRequest(http.MethodPost, "/query/stream", strings.NewReader(`{
		"query": "test query",
		"docs": []
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if ct := rr.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "event: sentence") {
		t.Errorf("response body missing sentence event: %q", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Errorf("response body missing done event: %q", body)
	}
}

// TestAcceptMissingFallsBackToJSON verifies REQ-SYN4-005: missing Accept header
// returns JSON, NOT SSE.
func TestAcceptMissingFallsBackToJSON(t *testing.T) {
	t.Parallel()

	client := &fakeClient{result: newTestResult()}
	h := handlers.NewSynthesisHandler(client, handlers.DefaultConfig())

	req := httptest.NewRequest(http.MethodPost, "/query/stream", strings.NewReader(`{
		"query": "test query",
		"docs": []
	}`))
	req.Header.Set("Content-Type", "application/json")
	// No Accept header.

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var result synthesis.Result
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("response body is not valid JSON: %v\nbody: %q", err, rr.Body.String())
	}
}

// TestAcceptApplicationJSONFallsBackToJSON verifies REQ-SYN4-005:
// explicit Accept: application/json returns JSON.
func TestAcceptApplicationJSONFallsBackToJSON(t *testing.T) {
	t.Parallel()

	client := &fakeClient{result: newTestResult()}
	h := handlers.NewSynthesisHandler(client, handlers.DefaultConfig())

	req := httptest.NewRequest(http.MethodPost, "/query/stream", strings.NewReader(`{
		"query": "test query",
		"docs": []
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestSSEHeadersAbsentOnJSONFallback verifies REQ-SYN4-001a (negative): JSON
// fallback path must NOT emit SSE headers.
func TestSSEHeadersAbsentOnJSONFallback(t *testing.T) {
	t.Parallel()

	client := &fakeClient{result: newTestResult()}
	h := handlers.NewSynthesisHandler(client, handlers.DefaultConfig())

	req := httptest.NewRequest(http.MethodPost, "/query/stream", strings.NewReader(`{
		"query": "test query",
		"docs": []
	}`))
	req.Header.Set("Content-Type", "application/json")
	// No Accept header — JSON fallback path.

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if cc := rr.Header().Get("Cache-Control"); cc == "no-cache" {
		t.Errorf("Cache-Control should not be no-cache on JSON fallback, got: %q", cc)
	}
}

// TestSynthesisErrorReturns500 verifies that synthesis errors are propagated
// on the JSON fallback path as HTTP 500.
func TestSynthesisErrorReturns500(t *testing.T) {
	t.Parallel()

	client := &fakeClient{err: errors.New("upstream unavailable")}
	h := handlers.NewSynthesisHandler(client, handlers.DefaultConfig())

	req := httptest.NewRequest(http.MethodPost, "/query/stream", strings.NewReader(`{
		"query": "test query",
		"docs": []
	}`))
	req.Header.Set("Content-Type", "application/json")
	// No Accept — JSON fallback.

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

// TestBadRequestBodyReturns400 verifies that malformed JSON body returns 400.
func TestBadRequestBodyReturns400(t *testing.T) {
	t.Parallel()

	client := &fakeClient{result: newTestResult()}
	h := handlers.NewSynthesisHandler(client, handlers.DefaultConfig())

	req := httptest.NewRequest(http.MethodPost, "/query/stream", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// TestSSEErrorEventOnSynthesisFailure verifies that synthesis errors are
// emitted as `event: error` on the SSE path.
func TestSSEErrorEventOnSynthesisFailure(t *testing.T) {
	t.Parallel()

	client := &fakeClient{err: errors.New("upstream error")}
	h := handlers.NewSynthesisHandler(client, handlers.DefaultConfig())

	req := httptest.NewRequest(http.MethodPost, "/query/stream", strings.NewReader(`{
		"query": "test query",
		"docs": []
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Errorf("expected error event in SSE stream on synthesis failure: %q", body)
	}
}

// TestAcceptTextHTMLFallsBackToJSON verifies REQ-SYN4-005: any non-event-stream
// Accept value triggers JSON fallback.
func TestAcceptTextHTMLFallsBackToJSON(t *testing.T) {
	t.Parallel()

	client := &fakeClient{result: newTestResult()}
	h := handlers.NewSynthesisHandler(client, handlers.DefaultConfig())

	req := httptest.NewRequest(http.MethodPost, "/query/stream", strings.NewReader(`{
		"query": "test query",
		"docs": []
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/html")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json for text/html accept", ct)
	}
}
