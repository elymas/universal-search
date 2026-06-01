package audit

// Behavior tests for the HTTP LiteLLM client's FetchSpendLogs paths not already
// exercised through Reconcile: direct success decoding, the URL/query shape,
// the JSON-decode-error path, and the request-construction error path.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchSpendLogs_SuccessDecodesAndBuildsURL(t *testing.T) {
	var gotPath, gotStart, gotEnd, gotSummarize string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotStart = r.URL.Query().Get("start_date")
		gotEnd = r.URL.Query().Get("end_date")
		gotSummarize = r.URL.Query().Get("summarize")
		_, _ = w.Write([]byte(`[{"request_id":"r1","model":"m","spend":0.5,"prompt_tokens":10,"completion_tokens":5}]`))
	}))
	defer srv.Close()

	c := NewHTTPLiteLLMClient(srv.URL)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	logs, err := c.FetchSpendLogs(context.Background(), start, end)
	if err != nil {
		t.Fatalf("FetchSpendLogs: %v", err)
	}
	if len(logs) != 1 || logs[0].RequestID != "r1" || logs[0].Spend != 0.5 {
		t.Fatalf("unexpected logs: %+v", logs)
	}
	if gotPath != "/spend/logs" {
		t.Errorf("path = %q, want /spend/logs", gotPath)
	}
	if gotSummarize != "false" {
		t.Errorf("summarize = %q, want false", gotSummarize)
	}
	if !strings.HasPrefix(gotStart, "2026-01-01") {
		t.Errorf("start_date = %q, want RFC3339 2026-01-01", gotStart)
	}
	if !strings.HasPrefix(gotEnd, "2026-01-02") {
		t.Errorf("end_date = %q, want RFC3339 2026-01-02", gotEnd)
	}
}

func TestFetchSpendLogs_DecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`this is not json`))
	}))
	defer srv.Close()

	c := NewHTTPLiteLLMClient(srv.URL)
	_, err := c.FetchSpendLogs(context.Background(), time.Now(), time.Now())
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("error = %v, want decode error", err)
	}
}

func TestFetchSpendLogs_RequestConstructionError(t *testing.T) {
	// A control character in the endpoint makes http.NewRequestWithContext fail
	// before any network call, exercising the request-construction error branch.
	c := NewHTTPLiteLLMClient("http://\x7f-invalid")
	_, err := c.FetchSpendLogs(context.Background(), time.Now(), time.Now())
	if err == nil {
		t.Fatal("expected request construction error, got nil")
	}
	if !strings.Contains(err.Error(), "request") {
		t.Errorf("error = %v, want request error", err)
	}
}

func TestFetchSpendLogs_TransportError(t *testing.T) {
	// Point at a server that is immediately closed so the Do() call fails.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	c := NewHTTPLiteLLMClient(url)
	_, err := c.FetchSpendLogs(context.Background(), time.Now(), time.Now())
	if err == nil {
		t.Fatal("expected transport error against closed server, got nil")
	}
	if !strings.Contains(err.Error(), "fetch") {
		t.Errorf("error = %v, want fetch error", err)
	}
}
