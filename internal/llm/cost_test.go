// Package llm_test — cost extraction and budget cap unit tests.
// REQ-LLM-006: x-litellm-response-cost header parsing.
// NFR-LLM-003: Budget cap enforcement.
package llm_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elymas/universal-search/internal/llm"
	"github.com/elymas/universal-search/internal/llm/config"
)

// makeClientWithCap creates a test client with a specific per-request budget cap.
func makeClientWithCap(t *testing.T, serverURL string, capUSD float64) llm.Client {
	t.Helper()
	cfg := config.Config{
		BaseURL:          serverURL,
		MasterKey:        "test-master-key",
		PerRequestCapUSD: capUSD,
		TimeoutSeconds:   5,
	}
	o := makeTestObs(t)
	c, err := llm.New(cfg, o)
	if err != nil {
		t.Fatalf("llm.New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestCostRoundTripperCapturesHeader verifies the cost middleware captures the header value.
// REQ-LLM-006
func TestCostRoundTripperCapturesHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-litellm-response-cost", "0.0042")
		_, _ = w.Write([]byte(validChatResponse("cost check", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	resp, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil && !errors.Is(err, llm.ErrBudgetExceeded) {
		t.Fatalf("Complete: %v", err)
	}
	if resp.CostUSD < 0.004 || resp.CostUSD > 0.005 {
		t.Errorf("CostUSD: got %f, want ~0.0042", resp.CostUSD)
	}
}

// TestCostHeaderZeroValue verifies zero cost is handled correctly.
// REQ-LLM-006
func TestCostHeaderZeroValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-litellm-response-cost", "0.0")
		_, _ = w.Write([]byte(validChatResponse("zero cost", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	c := makeTestClient(t, srv.URL)
	resp, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil && !errors.Is(err, llm.ErrBudgetExceeded) {
		t.Fatalf("Complete: %v", err)
	}
	if resp.CostUSD != 0 {
		t.Errorf("CostUSD with 0.0 header: got %f, want 0", resp.CostUSD)
	}
}

// TestCheckBudgetUnlimited verifies cap==0 never triggers ErrBudgetExceeded.
// NFR-LLM-003
func TestCheckBudgetUnlimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-litellm-response-cost", "999.99")
		_, _ = w.Write([]byte(validChatResponse("no cap", "claude-sonnet-4-6")))
	}))
	defer srv.Close()

	// PerRequestCapUSD=0 means unlimited.
	c := makeClientWithCap(t, srv.URL, 0)
	_, err := c.Complete(context.Background(), llm.Request{
		Class:    llm.Summary,
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Errorf("expected no error with unlimited cap, got %v", err)
	}
}
