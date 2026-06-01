package synthesis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// T-M4-001 [RED]: CheckFaithfulness Go wrapper unit tests
// REQ-DEEP2-006: Go wrapper calls Python sidecar POST /faithfulness_check.

func TestCheckFaithfulnessSuccessPass(t *testing.T) {
	// Simulate sidecar returning uncited_count == 0 → PASS.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/faithfulness_check" {
			t.Errorf("expected /faithfulness_check, got %s", r.URL.Path)
		}

		// Decode request to verify fields.
		var req FaithfulnessRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}

		resp := FaithfulnessResponse{
			UncitedSentencesCount: 0,
			UncitedSentences:      []string{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	result, err := CheckFaithfulness(context.Background(), srv.Client(), srv.URL+"/faithfulness_check", "some text", []string{"[1]"}, []string{"doc body"})
	if err != nil {
		t.Fatalf("CheckFaithfulness() error: %v", err)
	}

	if !result.Pass {
		t.Error("expected Pass=true when uncited_count == 0")
	}
	if result.UncitedCount != 0 {
		t.Errorf("UncitedCount = %d, want 0", result.UncitedCount)
	}
}

func TestCheckFaithfulnessFailWithUncitedSentences(t *testing.T) {
	// Simulate sidecar returning uncited_count > 0 → FAIL.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := FaithfulnessResponse{
			UncitedSentencesCount: 3,
			UncitedSentences:      []string{"s1", "s2", "s3"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	result, err := CheckFaithfulness(context.Background(), srv.Client(), srv.URL+"/faithfulness_check", "text", []string{"[1]"}, []string{"doc"})
	if err != nil {
		t.Fatalf("CheckFaithfulness() error: %v", err)
	}

	if result.Pass {
		t.Error("expected Pass=false when uncited_count > 0")
	}
	if result.UncitedCount != 3 {
		t.Errorf("UncitedCount = %d, want 3", result.UncitedCount)
	}
	if len(result.UncitedSentences) != 3 {
		t.Errorf("len(UncitedSentences) = %d, want 3", len(result.UncitedSentences))
	}
}

func TestCheckFaithfulnessWrapperHandlesSidecar5xx(t *testing.T) {
	// Simulate sidecar returning 500 → error with fail_error classification.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := CheckFaithfulness(context.Background(), srv.Client(), srv.URL+"/faithfulness_check", "text", []string{}, []string{})
	if err == nil {
		t.Fatal("expected error for 5xx response, got nil")
	}
}

func TestCheckFaithfulnessWrapperHandlesTransportFailure(t *testing.T) {
	// Simulate unreachable server by using a closed server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	_, err := CheckFaithfulness(context.Background(), client, "http://"+srv.Listener.Addr().String()+"/faithfulness_check", "text", nil, nil)
	if err == nil {
		t.Fatal("expected error for transport failure, got nil")
	}
}

func TestCheckFaithfulnessWrapperHandlesSidecar4xx(t *testing.T) {
	// A 4xx response must be surfaced as a client-error (distinct from the 5xx
	// branch already covered).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad payload"))
	}))
	defer srv.Close()

	_, err := CheckFaithfulness(context.Background(), srv.Client(), srv.URL+"/faithfulness_check", "text", nil, nil)
	if err == nil {
		t.Fatal("expected error for 4xx response, got nil")
	}
	if !strings.Contains(err.Error(), "client error") {
		t.Errorf("error = %v, want a client error", err)
	}
}

func TestCheckFaithfulnessWrapperHandlesDecodeError(t *testing.T) {
	// A 200 response with a non-JSON body must surface a decode error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := CheckFaithfulness(context.Background(), srv.Client(), srv.URL+"/faithfulness_check", "text", nil, nil)
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("error = %v, want a decode error", err)
	}
}

func TestCheckFaithfulnessWrapperHandlesTimeout(t *testing.T) {
	// Simulate slow sidecar that exceeds context deadline.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	client := &http.Client{}
	_, err := CheckFaithfulness(ctx, client, srv.URL+"/faithfulness_check", "text", nil, nil)
	if err == nil {
		t.Fatal("expected error for timeout, got nil")
	}
}

func TestCheckFaithfulnessSendsCorrectPayload(t *testing.T) {
	// Verify the wrapper sends the correct JSON payload to the sidecar.
	var receivedReq FaithfulnessRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&receivedReq); err != nil {
			t.Errorf("decode request: %v", err)
		}
		resp := FaithfulnessResponse{
			UncitedSentencesCount: 0,
			UncitedSentences:      []string{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	text := "This is a test passage [1]."
	citations := []string{"[1] citation text"}
	docs := []string{"Document body text"}

	_, err := CheckFaithfulness(context.Background(), srv.Client(), srv.URL+"/faithfulness_check", text, citations, docs)
	if err != nil {
		t.Fatalf("CheckFaithfulness() error: %v", err)
	}

	if receivedReq.Text != text {
		t.Errorf("request Text = %q, want %q", receivedReq.Text, text)
	}
	if len(receivedReq.Citations) != 1 || receivedReq.Citations[0] != "[1] citation text" {
		t.Errorf("request Citations = %v, want %v", receivedReq.Citations, citations)
	}
	if len(receivedReq.Docs) != 1 || receivedReq.Docs[0] != "Document body text" {
		t.Errorf("request Docs = %v, want %v", receivedReq.Docs, docs)
	}
}
