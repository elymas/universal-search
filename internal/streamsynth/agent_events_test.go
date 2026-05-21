package streamsynth

import (
	"encoding/json"
	"testing"
)

// T-M5-001 [RED]: Agent events payload JSON round-trip tests
// REQ-DEEP2-007: All payloads carry schema_version:1 and request_id.

func TestAgentStartedPayloadJSONRoundTrip(t *testing.T) {
	original := AgentStartedPayload{
		RequestID:     "test-123",
		Agent:         "researcher",
		SchemaVersion: 1,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded AgentStartedPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.RequestID != "test-123" {
		t.Errorf("RequestID = %q, want %q", decoded.RequestID, "test-123")
	}
	if decoded.Agent != "researcher" {
		t.Errorf("Agent = %q, want %q", decoded.Agent, "researcher")
	}
	if decoded.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", decoded.SchemaVersion)
	}
}

func TestAgentCompletedPayloadJSONRoundTrip(t *testing.T) {
	original := AgentCompletedPayload{
		RequestID:     "test-456",
		Agent:         "writer",
		Outcome:       "success",
		DurationMs:    1500,
		CostUSD:       0.05,
		SchemaVersion: 1,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded AgentCompletedPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Outcome != "success" {
		t.Errorf("Outcome = %q, want %q", decoded.Outcome, "success")
	}
	if decoded.DurationMs != 1500 {
		t.Errorf("DurationMs = %d, want 1500", decoded.DurationMs)
	}
	if decoded.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want 0.05", decoded.CostUSD)
	}
}

func TestRetryStartedPayloadJSONRoundTrip(t *testing.T) {
	original := RetryStartedPayload{
		RequestID:     "test-789",
		Agent:         "writer",
		Attempt:       2,
		MaxAttempts:   3,
		SchemaVersion: 1,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded RetryStartedPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Attempt != 2 {
		t.Errorf("Attempt = %d, want 2", decoded.Attempt)
	}
	if decoded.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", decoded.MaxAttempts)
	}
}

func TestVerifierResultPayloadJSONRoundTrip(t *testing.T) {
	original := VerifierResultPayload{
		RequestID:     "test-verifier",
		Pass:          false,
		UncitedCount:  3,
		SchemaVersion: 1,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded VerifierResultPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Pass {
		t.Error("expected Pass=false")
	}
	if decoded.UncitedCount != 3 {
		t.Errorf("UncitedCount = %d, want 3", decoded.UncitedCount)
	}
}

func TestPipelineFailedPayloadJSONRoundTrip(t *testing.T) {
	original := PipelineFailedPayload{
		RequestID:     "test-fail",
		FailedAgent:   "writer",
		Reason:        "verifier_rejection_exhausted",
		Attempts:      3,
		RetryCount:    2,
		SchemaVersion: 1,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded PipelineFailedPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.FailedAgent != "writer" {
		t.Errorf("FailedAgent = %q, want %q", decoded.FailedAgent, "writer")
	}
	if decoded.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", decoded.Attempts)
	}
}

func TestPipelineCancelledPayloadJSONRoundTrip(t *testing.T) {
	original := PipelineCancelledPayload{
		RequestID:     "test-cancel",
		AtAgent:       "writer",
		SchemaVersion: 1,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded PipelineCancelledPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.AtAgent != "writer" {
		t.Errorf("AtAgent = %q, want %q", decoded.AtAgent, "writer")
	}
}

func TestAllPayloadsHaveSchemaVersion1(t *testing.T) {
	// Verify all payload types include schema_version:1 in JSON.
	payloads := map[string]any{
		"agent_started":    AgentStartedPayload{RequestID: "t", SchemaVersion: 1},
		"agent_completed":  AgentCompletedPayload{RequestID: "t", SchemaVersion: 1},
		"retry_started":    RetryStartedPayload{RequestID: "t", SchemaVersion: 1},
		"verifier_result":  VerifierResultPayload{RequestID: "t", SchemaVersion: 1},
		"pipeline_failed":  PipelineFailedPayload{RequestID: "t", SchemaVersion: 1},
		"pipeline_cancelled": PipelineCancelledPayload{RequestID: "t", SchemaVersion: 1},
	}

	for name, payload := range payloads {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Errorf("marshal %s: %v", name, err)
			continue
		}

		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Errorf("unmarshal %s: %v", name, err)
			continue
		}

		sv, ok := raw["schema_version"]
		if !ok {
			t.Errorf("%s: missing schema_version field", name)
			continue
		}
		if sv != float64(1) {
			t.Errorf("%s: schema_version = %v, want 1", name, sv)
		}

		rid, ok := raw["request_id"]
		if !ok {
			t.Errorf("%s: missing request_id field", name)
			continue
		}
		if rid != "t" {
			t.Errorf("%s: request_id = %v, want 't'", name, rid)
		}
	}
}
