package streamsynth

// Agent event payload types for SSE streaming.
// REQ-DEEP2-007: All payloads carry schema_version:1 and request_id.

// AgentStartedPayload is emitted when an agent begins execution.
type AgentStartedPayload struct {
	RequestID     string `json:"request_id"`
	Agent         string `json:"agent"`
	SchemaVersion int    `json:"schema_version"`
}

// AgentCompletedPayload is emitted when an agent finishes successfully.
type AgentCompletedPayload struct {
	RequestID     string  `json:"request_id"`
	Agent         string  `json:"agent"`
	Outcome       string  `json:"outcome"`
	DurationMs    int64   `json:"duration_ms"`
	CostUSD       float64 `json:"cost_usd"`
	SchemaVersion int     `json:"schema_version"`
}

// RetryStartedPayload is emitted before each Writer retry.
// REQ-DEEP2-003: Only Verifier rejection triggers retry.
type RetryStartedPayload struct {
	RequestID     string `json:"request_id"`
	Agent         string `json:"agent"`
	Attempt       int    `json:"attempt"`
	MaxAttempts   int    `json:"max_attempts"`
	SchemaVersion int    `json:"schema_version"`
}

// VerifierResultPayload is emitted after each Verifier check.
// REQ-DEEP2-006: PASS iff uncited_count == 0.
type VerifierResultPayload struct {
	RequestID     string   `json:"request_id"`
	Pass          bool     `json:"pass"`
	UncitedCount  int      `json:"uncited_count"`
	Sentences     []string `json:"sentences,omitempty"`
	SchemaVersion int      `json:"schema_version"`
}

// PipelineFailedPayload is the terminal event when the pipeline exhausts retries
// or encounters an unrecoverable error.
// REQ-DEEP2-009a/009b: SSE active → event on 200; buffered → 503 JSON.
type PipelineFailedPayload struct {
	RequestID     string `json:"request_id"`
	FailedAgent   string `json:"failed_agent"`
	Reason        string `json:"reason"`
	Attempts      int    `json:"attempts"`
	RetryCount    int    `json:"retry_count"`
	SchemaVersion int    `json:"schema_version"`
}

// PipelineCancelledPayload is the terminal event when context is cancelled.
type PipelineCancelledPayload struct {
	RequestID     string `json:"request_id"`
	AtAgent       string `json:"at_agent"`
	SchemaVersion int    `json:"schema_version"`
}
