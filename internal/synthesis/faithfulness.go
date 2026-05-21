package synthesis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// FaithfulnessRequest is the JSON payload sent to the Python sidecar
// POST /faithfulness_check endpoint.
type FaithfulnessRequest struct {
	Text      string   `json:"text"`
	Citations []string `json:"citations"`
	Docs      []string `json:"docs"`
}

// FaithfulnessResponse is the JSON response from the sidecar.
type FaithfulnessResponse struct {
	UncitedSentencesCount int      `json:"uncited_sentences_count"`
	UncitedSentences      []string `json:"uncited_sentences"`
}

// FaithfulnessResult is the Go-level result returned to callers.
// REQ-DEEP2-006: PASS iff uncited_count == 0.
type FaithfulnessResult struct {
	Pass             bool
	UncitedCount     int
	UncitedSentences []string
}

// @MX:ANCHOR: [AUTO] Single chokepoint for SYN-002 faithfulness gate invocation from Go side
// @MX:REASON: Verifier agent (DEEP-002) is the only caller in v0; future SPECs may add additional callers but contract (UncitedSentencesCount → PASS/FAIL binary) is FROZEN
// @MX:SPEC: SPEC-SYN-002, SPEC-DEEP-002 REQ-DEEP2-006

// CheckFaithfulness calls the researcher sidecar POST /faithfulness_check endpoint.
// REQ-DEEP2-006: Returns a FaithfulnessResult with Pass=true iff UncitedSentencesCount == 0.
func CheckFaithfulness(ctx context.Context, client *http.Client, url string, text string, citations []string, docs []string) (FaithfulnessResult, error) {
	reqPayload := FaithfulnessRequest{
		Text:      text,
		Citations: citations,
		Docs:      docs,
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return FaithfulnessResult{}, fmt.Errorf("faithfulness: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return FaithfulnessResult{}, fmt.Errorf("faithfulness: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return FaithfulnessResult{}, fmt.Errorf("faithfulness: http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 500 {
		respBody, _ := io.ReadAll(resp.Body)
		return FaithfulnessResult{}, fmt.Errorf("faithfulness: server error %d: %s", resp.StatusCode, string(respBody))
	}
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return FaithfulnessResult{}, fmt.Errorf("faithfulness: client error %d: %s", resp.StatusCode, string(respBody))
	}

	var fResp FaithfulnessResponse
	if err := json.NewDecoder(resp.Body).Decode(&fResp); err != nil {
		return FaithfulnessResult{}, fmt.Errorf("faithfulness: decode response: %w", err)
	}

	result := FaithfulnessResult{
		Pass:             fResp.UncitedSentencesCount == 0,
		UncitedCount:     fResp.UncitedSentencesCount,
		UncitedSentences: fResp.UncitedSentences,
	}

	return result, nil
}
