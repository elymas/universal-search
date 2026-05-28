// Package scorer_test validates the DeepEval HTTP bridge.
//
// REQ-EVAL1-004: Go bridge marshals claims + corpus and calls Python judge.
// REQ-EVAL1-005: Extracts per-claim faithfulness scores.
// REQ-EVAL1-006: 30s timeout enforced per judge call.
// REQ-EVAL1-007: Per-claim rationale extraction.
package scorer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/eval/scorer"
)

// ---------- Test: Bridge marshals claims correctly ----------

func TestBridgeMarshalsClaims(t *testing.T) {
	// Verify the bridge sends the correct JSON structure to the judge.
	var receivedBody map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/judge/faithfulness" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Errorf("decode request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"query_id":           "EVAL-001-Q001",
			"judge_model":        "test-model",
			"faithfulness_score": 1.0,
			"claim_scores": []map[string]any{
				{"text": "Test claim.", "supported": true, "judge_rationale": "Supported."},
			},
		})
	}))
	defer srv.Close()

	bridge := scorer.NewBridge(srv.URL, "test-model")
	claims := []scorer.Claim{
		{Text: "Test claim.", CitedDocIDs: []string{"doc-001"}},
	}
	corpus := []scorer.CorpusEntry{
		{DocID: "doc-001", Body: "Context text."},
	}

	resp, err := bridge.Score(context.Background(), "EVAL-001-Q001", claims, corpus)
	if err != nil {
		t.Fatalf("Score() error: %v", err)
	}
	if resp.FaithfulnessScore != 1.0 {
		t.Errorf("score = %f, want 1.0", resp.FaithfulnessScore)
	}

	// Verify query_id was sent
	var qid string
	json.Unmarshal(receivedBody["query_id"], &qid)
	if qid != "EVAL-001-Q001" {
		t.Errorf("query_id = %q, want EVAL-001-Q001", qid)
	}
}

// ---------- Test: Bridge extracts citations from response ----------

func TestBridgeExtractsCitations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"query_id":           "EVAL-001-Q002",
			"judge_model":        "test-model",
			"faithfulness_score": 0.667,
			"claim_scores": []map[string]any{
				{"text": "Claim A.", "supported": true, "judge_rationale": "Directly supported by doc."},
				{"text": "Claim B.", "supported": true, "judge_rationale": "Partial support."},
				{"text": "Claim C.", "supported": false, "judge_rationale": "No support found."},
			},
		})
	}))
	defer srv.Close()

	bridge := scorer.NewBridge(srv.URL, "test-model")
	claims := []scorer.Claim{
		{Text: "Claim A.", CitedDocIDs: []string{"doc-001"}},
		{Text: "Claim B.", CitedDocIDs: []string{"doc-002"}},
		{Text: "Claim C.", CitedDocIDs: []string{"doc-003"}},
	}

	resp, err := bridge.Score(context.Background(), "EVAL-001-Q002", claims, nil)
	if err != nil {
		t.Fatalf("Score() error: %v", err)
	}

	if len(resp.ClaimScores) != 3 {
		t.Fatalf("got %d claim scores, want 3", len(resp.ClaimScores))
	}
	if resp.ClaimScores[0].Supported != true {
		t.Error("claim 0: want supported=true")
	}
	if resp.ClaimScores[2].Supported != false {
		t.Error("claim 2: want supported=false")
	}
	if resp.ClaimScores[2].JudgeRationale != "No support found." {
		t.Errorf("claim 2 rationale = %q", resp.ClaimScores[2].JudgeRationale)
	}
}

// ---------- Test: Bridge timeout enforced ----------

func TestBridgeTimeoutEnforced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow judge (> 30s default timeout, but we use a short timeout for testing)
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	bridge := scorer.NewBridge(srv.URL, "test-model")
	// Use a short context timeout for test speed
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	claims := []scorer.Claim{{Text: "Test.", CitedDocIDs: []string{"doc-001"}}}
	_, err := bridge.Score(ctx, "EVAL-001-Q001", claims, nil)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// ---------- Test: Bridge returns per-claim rationale ----------

func TestBridgeReturnsPerClaimRationale(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"query_id":           "EVAL-001-Q003",
			"judge_model":        "test-model",
			"faithfulness_score": 0.5,
			"claim_scores": []map[string]any{
				{"text": "Supported claim.", "supported": true, "judge_rationale": "The cited document directly states this claim."},
				{"text": "Unsupported claim.", "supported": false, "judge_rationale": "The cited document discusses a different topic."},
			},
		})
	}))
	defer srv.Close()

	bridge := scorer.NewBridge(srv.URL, "test-model")
	claims := []scorer.Claim{
		{Text: "Supported claim.", CitedDocIDs: []string{"doc-001"}},
		{Text: "Unsupported claim.", CitedDocIDs: []string{"doc-002"}},
	}

	resp, err := bridge.Score(context.Background(), "EVAL-001-Q003", claims, nil)
	if err != nil {
		t.Fatalf("Score() error: %v", err)
	}

	for i, cs := range resp.ClaimScores {
		if cs.JudgeRationale == "" {
			t.Errorf("claim_scores[%d]: empty rationale", i)
		}
		if cs.Text == "" {
			t.Errorf("claim_scores[%d]: empty text", i)
		}
	}
}

// ---------- Test: Bridge handles server error ----------

func TestBridgeHandlesServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": "service unavailable"}`))
	}))
	defer srv.Close()

	bridge := scorer.NewBridge(srv.URL, "test-model")
	claims := []scorer.Claim{{Text: "Test.", CitedDocIDs: []string{"doc-001"}}}

	_, err := bridge.Score(context.Background(), "EVAL-001-Q001", claims, nil)
	if err == nil {
		t.Fatal("expected error for 503 response, got nil")
	}
}

// ---------- Test: Bridge default timeout is 30s ----------

func TestBridgeDefaultTimeoutIs30s(t *testing.T) {
	bridge := scorer.NewBridge("http://localhost:0", "test-model")
	if bridge.Timeout() != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", bridge.Timeout())
	}
}

// ---------- Test: Bridge returns JudgeModel from response ----------

func TestBridgeReturnsJudgeModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"query_id":           "EVAL-001-Q001",
			"judge_model":        "claude-haiku-4-5",
			"faithfulness_score": 1.0,
			"claim_scores":       []map[string]any{},
		})
	}))
	defer srv.Close()

	bridge := scorer.NewBridge(srv.URL, "test-model")
	resp, err := bridge.Score(context.Background(), "EVAL-001-Q001", nil, nil)
	if err != nil {
		t.Fatalf("Score() error: %v", err)
	}
	if resp.JudgeModel != "claude-haiku-4-5" {
		t.Errorf("judge_model = %q, want claude-haiku-4-5", resp.JudgeModel)
	}
}
