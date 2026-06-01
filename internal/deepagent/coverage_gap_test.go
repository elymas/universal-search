package deepagent

// Behavior tests for previously-uncovered pure-logic functions in the deepagent
// package: tree-mode determination, env-driven tree config, the HTTP researcher
// client decompose path, metric recorders, and crash-recovery reclassification.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestDetermineTreeMode(t *testing.T) {
	tests := []struct {
		name           string
		enabled        bool
		requestBreadth int
		requestDepth   int
		want           DeepTreeMode
	}{
		{"disabled returns none", false, 4, 3, DeepTreeModeNone},
		{"breadth zero fallback", true, 0, 3, DeepTreeModeFallbackBreadthZero},
		{"depth zero fallback", true, 4, 0, DeepTreeModeFallbackDepthZero},
		{"breadth precedence when both zero", true, 0, 0, DeepTreeModeFallbackBreadthZero},
		{"active when enabled and non-zero", true, 4, 3, DeepTreeModeActive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := TreeConfigExtra{Enabled: tt.enabled}
			got := DetermineTreeMode(cfg, tt.requestBreadth, tt.requestDepth)
			if got != tt.want {
				t.Errorf("DetermineTreeMode(enabled=%v, b=%d, d=%d) = %v, want %v",
					tt.enabled, tt.requestBreadth, tt.requestDepth, got, tt.want)
			}
		})
	}
}

func TestFallbackHeaderValue(t *testing.T) {
	tests := []struct {
		mode DeepTreeMode
		want string
	}{
		{DeepTreeModeFallbackBreadthZero, "breadth_zero"},
		{DeepTreeModeFallbackDepthZero, "depth_zero"},
		{DeepTreeModeActive, ""},
		{DeepTreeModeNone, ""},
	}
	for _, tt := range tests {
		if got := FallbackHeaderValue(tt.mode); got != tt.want {
			t.Errorf("FallbackHeaderValue(%v) = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestFallbackHeader(t *testing.T) {
	tests := []struct {
		breadth, depth int
		want           string
	}{
		{0, 3, "breadth_zero"},
		{4, 0, "depth_zero"},
		{0, 0, "breadth_zero"}, // breadth takes precedence
		{4, 3, ""},
	}
	for _, tt := range tests {
		if got := FallbackHeader(tt.breadth, tt.depth); got != tt.want {
			t.Errorf("FallbackHeader(%d, %d) = %q, want %q", tt.breadth, tt.depth, got, tt.want)
		}
	}
}

func TestDefaultTreeConfig(t *testing.T) {
	cfg := DefaultTreeConfig()
	if cfg.Enabled {
		t.Error("default config should be disabled")
	}
	if cfg.DefaultBreadth != 4 {
		t.Errorf("DefaultBreadth = %d, want 4", cfg.DefaultBreadth)
	}
	if cfg.DefaultDepth != 3 {
		t.Errorf("DefaultDepth = %d, want 3", cfg.DefaultDepth)
	}
	if cfg.TokenBudget != 60000 {
		t.Errorf("TokenBudget = %d, want 60000", cfg.TokenBudget)
	}
	if cfg.PersistenceDir != ".moai/runs" {
		t.Errorf("PersistenceDir = %q, want .moai/runs", cfg.PersistenceDir)
	}
}

func TestNewTreeConfigFromEnv_Defaults(t *testing.T) {
	// With no env vars set, values must equal DefaultTreeConfig.
	cfg, err := NewTreeConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := DefaultTreeConfig()
	if cfg != want {
		t.Errorf("config = %+v, want %+v", cfg, want)
	}
}

func TestNewTreeConfigFromEnv_Overrides(t *testing.T) {
	t.Setenv("DEEP_TREE_ENABLED", "true")
	t.Setenv("DEEP_TREE_DEFAULT_BREADTH", "6")
	t.Setenv("DEEP_TREE_DEFAULT_DEPTH", "5")
	t.Setenv("DEEP_TREE_TOKEN_BUDGET", "120000")
	t.Setenv("DEEP_TREE_ROOT_TOKEN_ESTIMATE", "8000")
	t.Setenv("DEEP_TREE_NODE_TIMEOUT_MS", "45000")
	t.Setenv("DEEP_TREE_DECOMPOSE_MODEL", "custom-model")
	t.Setenv("DEEP_TREE_PERSISTENCE_DIR", "/tmp/runs")

	cfg, err := NewTreeConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if cfg.DefaultBreadth != 6 {
		t.Errorf("DefaultBreadth = %d, want 6", cfg.DefaultBreadth)
	}
	if cfg.DefaultDepth != 5 {
		t.Errorf("DefaultDepth = %d, want 5", cfg.DefaultDepth)
	}
	if cfg.TokenBudget != 120000 {
		t.Errorf("TokenBudget = %d, want 120000", cfg.TokenBudget)
	}
	if cfg.RootTokenEstimate != 8000 {
		t.Errorf("RootTokenEstimate = %d, want 8000", cfg.RootTokenEstimate)
	}
	if cfg.NodeTimeoutMs != 45000 {
		t.Errorf("NodeTimeoutMs = %d, want 45000", cfg.NodeTimeoutMs)
	}
	if cfg.DecomposeModel != "custom-model" {
		t.Errorf("DecomposeModel = %q, want custom-model", cfg.DecomposeModel)
	}
	if cfg.PersistenceDir != "/tmp/runs" {
		t.Errorf("PersistenceDir = %q, want /tmp/runs", cfg.PersistenceDir)
	}
}

func TestNewTreeConfigFromEnv_InvalidValuesError(t *testing.T) {
	cases := []string{
		"DEEP_TREE_DEFAULT_BREADTH",
		"DEEP_TREE_DEFAULT_DEPTH",
		"DEEP_TREE_TOKEN_BUDGET",
		"DEEP_TREE_ROOT_TOKEN_ESTIMATE",
		"DEEP_TREE_NODE_TIMEOUT_MS",
	}
	for _, envKey := range cases {
		t.Run(envKey, func(t *testing.T) {
			t.Setenv(envKey, "not-a-number")
			if _, err := NewTreeConfigFromEnv(); err == nil {
				t.Errorf("expected error for invalid %s, got nil", envKey)
			}
		})
	}
}

func TestDefaultDecomposeURL(t *testing.T) {
	t.Run("default localhost", func(t *testing.T) {
		t.Setenv("DEEP_TREE_DECOMPOSE_URL", "")
		if got := defaultDecomposeURL(); got != "http://localhost:8001" {
			t.Errorf("defaultDecomposeURL() = %q, want default", got)
		}
	})
	t.Run("env override", func(t *testing.T) {
		t.Setenv("DEEP_TREE_DECOMPOSE_URL", "http://sidecar:9999")
		if got := defaultDecomposeURL(); got != "http://sidecar:9999" {
			t.Errorf("defaultDecomposeURL() = %q, want override", got)
		}
	})
}

func TestNewResearcherHTTPClient(t *testing.T) {
	t.Setenv("DEEP_TREE_DECOMPOSE_URL", "http://sidecar:9999")
	c := NewResearcherHTTPClient()
	if c.BaseURL != "http://sidecar:9999" {
		t.Errorf("BaseURL = %q, want override", c.BaseURL)
	}
	if c.HTTPClient == nil || c.HTTPClient.Timeout != 30*time.Second {
		t.Errorf("HTTPClient should have 30s timeout, got %+v", c.HTTPClient)
	}
}

func TestResearcherHTTPClient_Decompose(t *testing.T) {
	t.Run("success returns sub_queries", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/decompose_query" {
				t.Errorf("unexpected path %q", r.URL.Path)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("missing JSON content-type")
			}
			_, _ = w.Write([]byte(`{"sub_queries":["q1","q2"]}`))
		}))
		defer srv.Close()

		c := &ResearcherHTTPClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
		got, err := c.Decompose(context.Background(), DecomposeRequest{
			RootQuery:   "root",
			ParentQuery: "parent",
			Breadth:     2,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || got[0] != "q1" || got[1] != "q2" {
			t.Errorf("sub_queries = %v, want [q1 q2]", got)
		}
	})

	t.Run("non-200 status returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		}))
		defer srv.Close()

		c := &ResearcherHTTPClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
		_, err := c.Decompose(context.Background(), DecomposeRequest{RootQuery: "r", Breadth: 1})
		if err == nil {
			t.Fatal("expected error for 500 status, got nil")
		}
	})

	t.Run("invalid JSON body returns decode error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("not json"))
		}))
		defer srv.Close()

		c := &ResearcherHTTPClient{BaseURL: srv.URL, HTTPClient: srv.Client()}
		_, err := c.Decompose(context.Background(), DecomposeRequest{RootQuery: "r", Breadth: 1})
		if err == nil {
			t.Fatal("expected decode error, got nil")
		}
	})
}

func TestResearcherHTTPClient_Fanout_Stub(t *testing.T) {
	c := &ResearcherHTTPClient{}
	cites, claims, tokens, err := c.Fanout(context.Background(), "anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cites != nil || claims != nil || tokens != 0 {
		t.Errorf("Fanout stub = (%v, %v, %d), want all-empty", cites, claims, tokens)
	}
}

func TestMetricsRecorder_NilSafe(t *testing.T) {
	// A recorder built from nil collectors must not panic.
	rec := NewMetricsRecorder(nil, nil, nil)
	rec.RecordAgentDuration(AgentWriter, "success", time.Second)
	rec.RecordWriterRetry()
	rec.RecordVerifierGateResult("pass")
}

func TestMetricsRecorder_RecordsValues(t *testing.T) {
	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "test_deep_dur", Help: "h"},
		[]string{"agent", "outcome"},
	)
	retries := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_deep_retries", Help: "h"},
		[]string{"agent"},
	)
	gate := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_deep_gate", Help: "h"},
		[]string{"result"},
	)
	rec := NewMetricsRecorder(duration, retries, gate)

	rec.RecordWriterRetry()
	rec.RecordWriterRetry()
	if got := testutil.ToFloat64(retries.WithLabelValues("writer")); got != 2 {
		t.Errorf("writer retries = %v, want 2", got)
	}

	rec.RecordVerifierGateResult("fail_uncited")
	if got := testutil.ToFloat64(gate.WithLabelValues("fail_uncited")); got != 1 {
		t.Errorf("verifier gate fail_uncited = %v, want 1", got)
	}

	// Duration histogram: observing must not panic and must register the sample.
	rec.RecordAgentDuration(AgentResearcher, "success", 250*time.Millisecond)
	if c := testutil.CollectAndCount(duration); c == 0 {
		t.Error("expected at least one duration series after recording")
	}
}

func TestTreePersistence_ReloadReclassify_Identity(t *testing.T) {
	p := NewTreePersistence()
	tree := &TreeResult{RootQuery: "q", Status: "complete"}
	if got := p.ReloadReclassify(tree); got != tree {
		t.Error("ReloadReclassify should return the same tree pointer")
	}
}

func TestTreePersistence_ReloadReclassifyNodes(t *testing.T) {
	p := NewTreePersistence()
	in := []*Node{
		{ID: "a", Status: NodeStatusPending},
		{ID: "b", Status: NodeStatusExpanding},
		{ID: "c", Status: NodeStatusComplete},
	}
	out := p.ReloadReclassifyNodes(in)

	if out[0].Status != NodeStatusFailed {
		t.Errorf("pending node should become failed, got %s", out[0].Status)
	}
	if out[1].Status != NodeStatusFailed {
		t.Errorf("expanding node should become failed, got %s", out[1].Status)
	}
	if out[2].Status != NodeStatusComplete {
		t.Errorf("complete node should stay complete, got %s", out[2].Status)
	}
	// Input must not be mutated (copy semantics).
	if in[0].Status != NodeStatusPending {
		t.Errorf("input node 0 was mutated: %s", in[0].Status)
	}
}

func TestTreePersistence_GzipCompressedSize(t *testing.T) {
	p := NewTreePersistence()
	tree := &TreeResult{
		RootQuery:  "what is the capital of France",
		Status:     "complete",
		TotalNodes: 5,
		Citations: []NodeCitation{
			{DocID: "d1", Title: "Paris", URL: "http://x", Snippet: "Paris is the capital"},
		},
	}
	size, err := p.GzipCompressedSize(tree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size <= 0 {
		t.Errorf("compressed size = %d, want > 0", size)
	}
}

func TestTreePersistence_ExpandReloadedTree_Rejected(t *testing.T) {
	p := NewTreePersistence()
	tree := &TreeResult{Status: "reloaded"}
	if err := p.ExpandReloadedTree(context.Background(), tree); err == nil {
		t.Fatal("expected error expanding a reloaded tree, got nil")
	}
}

func TestBuildEvidenceSummary(t *testing.T) {
	t.Run("empty claims yields empty summary", func(t *testing.T) {
		if got := buildEvidenceSummary(&Node{}); got != "" {
			t.Errorf("buildEvidenceSummary(no claims) = %q, want empty", got)
		}
	})
	t.Run("single claim", func(t *testing.T) {
		n := &Node{Claims: []NodeClaim{{Text: "claim one"}}}
		if got := buildEvidenceSummary(n); got != "claim one" {
			t.Errorf("buildEvidenceSummary = %q, want 'claim one'", got)
		}
	})
	t.Run("multiple claims joined with semicolons", func(t *testing.T) {
		n := &Node{Claims: []NodeClaim{{Text: "a"}, {Text: "b"}, {Text: "c"}}}
		if got := buildEvidenceSummary(n); got != "a; b; c" {
			t.Errorf("buildEvidenceSummary = %q, want 'a; b; c'", got)
		}
	})
}

func TestTruncateFrontier(t *testing.T) {
	state := &treeState{
		mu: sync.Mutex{},
		nodes: []*Node{
			{ID: "a", Status: NodeStatusPending},
			{ID: "b", Status: NodeStatusExpanding},
			{ID: "c", Status: NodeStatusComplete},
			{ID: "d", Status: NodeStatusFailed},
		},
	}
	truncateFrontier(state)

	if state.nodes[0].Status != NodeStatusBudgetExceeded {
		t.Errorf("pending -> %s, want budget_exceeded", state.nodes[0].Status)
	}
	if state.nodes[1].Status != NodeStatusBudgetExceeded {
		t.Errorf("expanding -> %s, want budget_exceeded", state.nodes[1].Status)
	}
	if state.nodes[2].Status != NodeStatusComplete {
		t.Errorf("complete -> %s, want complete (unchanged)", state.nodes[2].Status)
	}
	if state.nodes[3].Status != NodeStatusFailed {
		t.Errorf("failed -> %s, want failed (unchanged)", state.nodes[3].Status)
	}
}
