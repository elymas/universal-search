package deepagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// @MX:NOTE: [AUTO] HTTP client implementing TreeResearcher for Python Researcher sidecar
// @MX:SPEC: SPEC-DEEP-003 Phase C

// Compile-time interface compliance check.
var _ TreeResearcher = (*ResearcherHTTPClient)(nil)

// ResearcherHTTPClient implements the TreeResearcher interface by calling
// the Python Researcher sidecar over HTTP.
type ResearcherHTTPClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// decomposeRequestPayload is the JSON body sent to the Python sidecar.
type decomposeRequestPayload struct {
	RootQuery             string `json:"root_query"`
	ParentQuery           string `json:"parent_query"`
	ParentEvidenceSummary string `json:"parent_evidence_summary"`
	Breadth               int    `json:"breadth"`
}

// decomposeResponsePayload is the JSON response from the Python sidecar.
type decomposeResponsePayload struct {
	SubQueries []string `json:"sub_queries"`
}

// defaultDecomposeURL returns the default URL for the decompose endpoint.
// Reads from DEEP_TREE_DECOMPOSE_URL env var, falls back to localhost:8001.
func defaultDecomposeURL() string {
	if url := os.Getenv("DEEP_TREE_DECOMPOSE_URL"); url != "" {
		return url
	}
	return "http://localhost:8001"
}

// NewResearcherHTTPClient creates a new client with default configuration.
func NewResearcherHTTPClient() *ResearcherHTTPClient {
	return &ResearcherHTTPClient{
		BaseURL: defaultDecomposeURL(),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Decompose sends a POST request to the Python sidecar's /decompose_query endpoint.
// It sends root_query, parent_query, parent_evidence_summary, and breadth in the
// request body and parses the returned sub_queries list.
//
// REQ-DEEP3-009a: Prompt context fields are propagated in the request body.
func (c *ResearcherHTTPClient) Decompose(ctx context.Context, req DecomposeRequest) ([]string, error) {
	payload := decomposeRequestPayload(req)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal decompose request: %w", err)
	}

	url := c.BaseURL + "/decompose_query"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create decompose request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("decompose HTTP call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("decompose returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result decomposeResponsePayload
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode decompose response: %w", err)
	}

	return result.SubQueries, nil
}

// Fanout retrieves citations and claims for a given query.
// Phase C stub: returns empty results with 0 tokens.
// Real fanout integration will be implemented in Phase E.
func (c *ResearcherHTTPClient) Fanout(_ context.Context, _ string) ([]NodeCitation, []NodeClaim, int64, error) {
	return nil, nil, 0, nil
}
