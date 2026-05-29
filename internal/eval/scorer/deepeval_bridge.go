// Package scorer provides the Go→Python DeepEval bridge for citation
// faithfulness scoring.
//
// REQ-EVAL1-004: Marshals claims + corpus, calls Python judge over HTTP.
// REQ-EVAL1-005: Returns per-claim faithfulness scores.
// REQ-EVAL1-006: 30s timeout per judge call (NFR-EVAL1-002).
// REQ-EVAL1-007: Per-claim rationale extraction.
package scorer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultTimeout is the per-query judge call timeout.
// NFR-EVAL1-002: 30s wall-clock per judge call.
const DefaultTimeout = 30 * time.Second

// Claim represents a single extracted claim with its cited doc IDs.
type Claim struct {
	Text        string   `json:"text"`
	CitedDocIDs []string `json:"cited_doc_ids"`
}

// CorpusEntry represents a single corpus document excerpt.
type CorpusEntry struct {
	DocID string `json:"doc_id"`
	Body  string `json:"body"`
}

// ClaimScore is the per-claim faithfulness verdict from the judge.
type ClaimScore struct {
	Text           string `json:"text"`
	Supported      bool   `json:"supported"`
	JudgeRationale string `json:"judge_rationale"`
}

// ScoreResponse is the full response from the judge endpoint.
type ScoreResponse struct {
	QueryID           string       `json:"query_id"`
	JudgeModel        string       `json:"judge_model"`
	FaithfulnessScore float64      `json:"faithfulness_score"`
	ClaimScores       []ClaimScore `json:"claim_scores"`
}

// judgeRequest is the JSON body sent to the Python judge.
type judgeRequest struct {
	QueryID string        `json:"query_id"`
	Claims  []Claim       `json:"claims"`
	Corpus  []CorpusEntry `json:"corpus"`
}

// Bridge calls the Python eval_judge service over HTTP.
//
// @MX:ANCHOR: [AUTO] DeepEval HTTP bridge; callers: runner, CI gate, tests
// @MX:REASON: fan_in >= 3; central bridge for all faithfulness scoring
type Bridge struct {
	baseURL    string
	httpClient *http.Client
	judgeModel string
}

// NewBridge creates a Bridge pointing at the judge service at baseURL.
// The default timeout is 30s (DefaultTimeout).
func NewBridge(baseURL, judgeModel string) *Bridge {
	return &Bridge{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		judgeModel: judgeModel,
	}
}

// Timeout returns the configured HTTP client timeout.
func (b *Bridge) Timeout() time.Duration {
	return b.httpClient.Timeout
}

// Score sends claims + corpus to the Python judge and returns per-claim scores.
// The ctx deadline is respected for cancellation; the bridge also enforces
// DefaultTimeout on the underlying HTTP client.
func (b *Bridge) Score(ctx context.Context, queryID string, claims []Claim, corpus []CorpusEntry) (*ScoreResponse, error) {
	reqBody := judgeRequest{
		QueryID: queryID,
		Claims:  claims,
		Corpus:  corpus,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal judge request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+"/judge/faithfulness", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create judge request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("judge HTTP call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("judge returned %d: %s", resp.StatusCode, string(respBody))
	}

	var scoreResp ScoreResponse
	if err := json.NewDecoder(resp.Body).Decode(&scoreResp); err != nil {
		return nil, fmt.Errorf("decode judge response: %w", err)
	}

	return &scoreResp, nil
}
