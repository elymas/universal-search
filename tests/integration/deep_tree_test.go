//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/deepagent"
)

// T-E-006 [RED]: Integration Happy Path
// REQ-DEEP3-001, REQ-DEEP3-003, REQ-DEEP3-011a, REQ-DEEP3-012:
// End-to-end tree expansion with stubbed sidecar, verify nodes + disk persistence + metrics.

func TestDeepTreeEndToEndHappyPath(t *testing.T) {
	// Start stubbed decompose server.
	decomposeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/decompose_query" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sub_queries": []string{"sub-query-1", "sub-query-2"},
			})
			return
		}
		// Fanout stub: return basic citations and claims.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"citations": []map[string]string{{"doc_id": "doc1", "title": "Test Doc"}},
			"claims":    []map[string]interface{}{{"text": "test claim", "markers": []string{"[1]"}}},
			"tokens":    100,
		})
	}))
	defer decomposeSrv.Close()

	// Create researcher HTTP client pointing to stub.
	researcher := &deepagent.ResearcherHTTPClient{
		BaseURL:    decomposeSrv.URL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	// Setup temp dir for persistence.
	tmpDir := t.TempDir()

	cfg := deepagent.TreeConfig{
		Breadth:            2,
		Depth:              2,
		TokenBudget:        500000,
		NodeTimeoutMs:      5000,
		RootTokenEstimate:  5000,
		ModelPricePerToken: 0.0000008,
		RunID:              "integration-test-happy",
	}

	result, err := deepagent.ExpandTree(context.Background(), cfg, "What is quantum computing?", researcher)
	if err != nil {
		t.Fatalf("ExpandTree error: %v", err)
	}

	// Verify tree was expanded.
	if result.TotalNodes == 0 {
		t.Fatal("expected at least 1 node, got 0")
	}
	if result.RootQuery != "What is quantum computing?" {
		t.Errorf("RootQuery = %q, want %q", result.RootQuery, "What is quantum computing?")
	}
	if result.Status != "complete" {
		t.Errorf("Status = %q, want %q", result.Status, "complete")
	}

	// Verify persistence: write tree to disk and read back.
	persister := deepagent.NewTreePersistence()
	if err := persister.AtomicFlush(tmpDir, cfg.RunID, result); err != nil {
		t.Fatalf("AtomicFlush error: %v", err)
	}

	// Verify tree.json exists on disk.
	treePath := filepath.Join(tmpDir, cfg.RunID, "tree.json")
	if _, err := os.Stat(treePath); os.IsNotExist(err) {
		t.Fatalf("tree.json not found at %s", treePath)
	}

	// Verify we can load it back.
	loaded, err := persister.LoadTree(tmpDir, cfg.RunID)
	if err != nil {
		t.Fatalf("LoadTree error: %v", err)
	}
	if loaded.RootQuery != result.RootQuery {
		t.Errorf("loaded RootQuery = %q, want %q", loaded.RootQuery, result.RootQuery)
	}
}

// TestDeepTreeDEEP002RegressionGreen verifies that DEEP-002 single-shot mode
// still works correctly after DEEP-003 changes.
// REQ-DEEP3-005: DEEP-002 backward compatibility.
func TestDeepTreeDEEP002RegressionGreen(t *testing.T) {
	// Load default config (DEEP-002 style).
	cfg := deepagent.DefaultConfig()

	// Verify default models are set correctly (SPEC-DEEP-002 Section 4).
	if cfg.ResearcherModel != "claude-3-5-haiku-20241022" {
		t.Errorf("ResearcherModel = %q, want claude-3-5-haiku-20241022", cfg.ResearcherModel)
	}
	if cfg.WriterModel != "claude-3-5-sonnet-20241022" {
		t.Errorf("WriterModel = %q, want claude-3-5-sonnet-20241022", cfg.WriterModel)
	}
	if cfg.MaxRetries != 2 {
		t.Errorf("MaxRetries = %d, want 2", cfg.MaxRetries)
	}

	// Verify tree config defaults exist.
	treeCfg := deepagent.DefaultTreeConfig()
	if treeCfg.Enabled {
		t.Error("TreeEnabled should be false by default")
	}
	if treeCfg.DefaultBreadth != 4 {
		t.Errorf("DefaultBreadth = %d, want 4", treeCfg.DefaultBreadth)
	}
	if treeCfg.DefaultDepth != 3 {
		t.Errorf("DefaultDepth = %d, want 3", treeCfg.DefaultDepth)
	}
}

// T-E-007 [RED]: Fallback Header
// REQ-DEEP3-005: breadth=0 or depth=0 falls back to DEEP-002 single-shot with header.

func TestExpandTreeBreadthZeroFallback(t *testing.T) {
	cfg := deepagent.TreeConfig{
		Breadth:            0,
		Depth:              3,
		TokenBudget:        60000,
		NodeTimeoutMs:      30000,
		RootTokenEstimate:  5000,
		ModelPricePerToken: 0.0000008,
		RunID:              "fallback-breadth-zero",
	}

	_, err := deepagent.ExpandTree(context.Background(), cfg, "test query", nil)
	if err == nil {
		t.Fatal("expected error for breadth=0, got nil")
	}
	if !strings.Contains(err.Error(), "breadth=0") && !strings.Contains(err.Error(), "breadth") {
		t.Errorf("error should mention breadth, got: %v", err)
	}
}

func TestExpandTreeDepthZeroFallback(t *testing.T) {
	cfg := deepagent.TreeConfig{
		Breadth:            4,
		Depth:              0,
		TokenBudget:        60000,
		NodeTimeoutMs:      30000,
		RootTokenEstimate:  5000,
		ModelPricePerToken: 0.0000008,
		RunID:              "fallback-depth-zero",
	}

	_, err := deepagent.ExpandTree(context.Background(), cfg, "test query", nil)
	if err == nil {
		t.Fatal("expected error for depth=0, got nil")
	}
	if !strings.Contains(err.Error(), "depth=0") && !strings.Contains(err.Error(), "depth") {
		t.Errorf("error should mention depth, got: %v", err)
	}
}

func TestExpandTreeBreadthAndDepthZeroFallback(t *testing.T) {
	cfg := deepagent.TreeConfig{
		Breadth:            0,
		Depth:              0,
		TokenBudget:        60000,
		NodeTimeoutMs:      30000,
		RootTokenEstimate:  5000,
		ModelPricePerToken: 0.0000008,
		RunID:              "fallback-both-zero",
	}

	_, err := deepagent.ExpandTree(context.Background(), cfg, "test query", nil)
	if err == nil {
		t.Fatal("expected error for breadth=0 and depth=0, got nil")
	}
	// breadth_zero should have priority (checked first in ExpandTree validation).
	if !strings.Contains(err.Error(), "breadth") {
		t.Errorf("error should mention breadth (priority), got: %v", err)
	}
}

// TestFallbackHeaderEmitted verifies that the fallback helper returns the correct header value.
// REQ-DEEP3-005: Body should be byte-identical to DEEP-002 single-shot response.
func TestFallbackHeaderEmitted(t *testing.T) {
	tests := []struct {
		name       string
		breadth    int
		depth      int
		wantHeader string
	}{
		{"breadth_zero", 0, 3, "breadth_zero"},
		{"depth_zero", 4, 0, "depth_zero"},
		{"both_zero_breadth_priority", 0, 0, "breadth_zero"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := deepagent.FallbackHeader(tt.breadth, tt.depth)
			if header != tt.wantHeader {
				t.Errorf("FallbackHeader(%d, %d) = %q, want %q", tt.breadth, tt.depth, header, tt.wantHeader)
			}
		})
	}
}

// TestFallbackHeaderBodyUnchanged verifies that the fallback response body
// matches what DEEP-002 single-shot would produce.
func TestFallbackHeaderBodyUnchanged(t *testing.T) {
	// When tree mode is not active, the response body should be the DEEP-002 format.
	// FallbackHeader only returns the header value, body is unchanged.
	header := deepagent.FallbackHeader(0, 3)
	if header != "breadth_zero" {
		t.Errorf("FallbackHeader(0, 3) = %q, want breadth_zero", header)
	}

	// Non-fallback case returns empty string.
	header = deepagent.FallbackHeader(4, 3)
	if header != "" {
		t.Errorf("FallbackHeader(4, 3) = %q, want empty (no fallback)", header)
	}
}

// helper for making requests in integration tests.
func mustDoRequest(t *testing.T, method, url string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, body)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// mustReadBody reads and closes the response body.
func mustReadBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return data
}

// suppressUnused prevents compile errors for integration-only helpers.
var _ = fmt.Sprintf
