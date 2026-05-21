package deepagent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// T-C-005: Go HTTP client tests for ResearcherHTTPClient
// REQ-DEEP3-009a: Prompt context fields propagated correctly
// ---------------------------------------------------------------------------

func TestResearcherHTTPDecomposeRoundTrip(t *testing.T) {
	// Stub server that verifies the request and returns sub-queries.
	wantSubQueries := []string{
		"How does climate change affect GDP?",
		"What industries are most vulnerable?",
		"Economic benefits of adaptation?",
		"Insurance market responses?",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/decompose_query" {
			t.Errorf("expected path /decompose_query, got %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Verify request body shape.
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("failed to parse request JSON: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Verify required fields exist.
		if _, ok := req["root_query"]; !ok {
			t.Error("missing root_query field")
		}
		if _, ok := req["parent_query"]; !ok {
			t.Error("missing parent_query field")
		}
		if _, ok := req["parent_evidence_summary"]; !ok {
			t.Error("missing parent_evidence_summary field")
		}
		if _, ok := req["breadth"]; !ok {
			t.Error("missing breadth field")
		}

		// Return sub-queries.
		resp := map[string]interface{}{
			"sub_queries": wantSubQueries,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &ResearcherHTTPClient{
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}

	subQueries, err := client.Decompose(context.Background(), DecomposeRequest{
		RootQuery:             "What are the effects of climate change?",
		ParentQuery:           "What are the economic effects?",
		ParentEvidenceSummary: "Rising temps reduce crop yields.",
		Breadth:               4,
	})
	if err != nil {
		t.Fatalf("Decompose returned error: %v", err)
	}

	if len(subQueries) != 4 {
		t.Errorf("expected 4 sub-queries, got %d", len(subQueries))
	}

	for i, sq := range subQueries {
		if sq != wantSubQueries[i] {
			t.Errorf("sub_queries[%d]: got %q, want %q", i, sq, wantSubQueries[i])
		}
	}
}

func TestResearcherHTTPPropagatesPromptContext(t *testing.T) {
	// Verify that prompt context fields are sent correctly in the request body.
	type decomposeRequestPayload struct {
		RootQuery             string `json:"root_query"`
		ParentQuery           string `json:"parent_query"`
		ParentEvidenceSummary string `json:"parent_evidence_summary"`
		Breadth               int    `json:"breadth"`
	}

	var capturedPayload decomposeRequestPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if err := json.Unmarshal(body, &capturedPayload); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		resp := map[string]interface{}{
			"sub_queries": []string{"query1", "query2"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := &ResearcherHTTPClient{
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}

	_, err := client.Decompose(context.Background(), DecomposeRequest{
		RootQuery:             "root query value",
		ParentQuery:           "parent query value",
		ParentEvidenceSummary: "evidence summary value",
		Breadth:               3,
	})
	if err != nil {
		t.Fatalf("Decompose returned error: %v", err)
	}

	// Verify all prompt context fields were sent correctly.
	if capturedPayload.RootQuery != "root query value" {
		t.Errorf("root_query: got %q, want %q", capturedPayload.RootQuery, "root query value")
	}
	if capturedPayload.ParentQuery != "parent query value" {
		t.Errorf("parent_query: got %q, want %q", capturedPayload.ParentQuery, "parent query value")
	}
	if capturedPayload.ParentEvidenceSummary != "evidence summary value" {
		t.Errorf("parent_evidence_summary: got %q, want %q", capturedPayload.ParentEvidenceSummary, "evidence summary value")
	}
	if capturedPayload.Breadth != 3 {
		t.Errorf("breadth: got %d, want %d", capturedPayload.Breadth, 3)
	}
}

func TestResearcherHTTPDecomposeServer500(t *testing.T) {
	// Verify that server errors are propagated correctly.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &ResearcherHTTPClient{
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}

	_, err := client.Decompose(context.Background(), DecomposeRequest{
		RootQuery:             "test",
		ParentQuery:           "test",
		ParentEvidenceSummary: "test",
		Breadth:               2,
	})
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestResearcherHTTPFanoutReturnsEmpty(t *testing.T) {
	// Phase C: Fanout returns empty citations/claims with 0 tokens.
	// Real fanout integration comes in Phase E.
	client := &ResearcherHTTPClient{
		BaseURL:    "http://unused",
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}

	citations, claims, tokens, err := client.Fanout(context.Background(), "test query")
	if err != nil {
		t.Fatalf("Fanout returned error: %v", err)
	}
	if len(citations) != 0 {
		t.Errorf("expected 0 citations, got %d", len(citations))
	}
	if len(claims) != 0 {
		t.Errorf("expected 0 claims, got %d", len(claims))
	}
	if tokens != 0 {
		t.Errorf("expected 0 tokens, got %d", tokens)
	}
}
