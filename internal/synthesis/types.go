// Package synthesis provides the Go-side HTTP client for the researcher sidecar.
//
// REQ-SYN-005: Context timeout + exponential backoff retry.
// REQ-SYN-006: Per-call observability (slog + prometheus + otel).
package synthesis

import "errors"

// Request is the payload sent to the researcher sidecar POST /synthesize.
type Request struct {
	RequestID string `json:"request_id"`
	Query     string `json:"query"`
	Lang      string `json:"lang,omitempty"`
	Docs      []Doc  `json:"docs"`
}

// Doc is the Go-side representation of a NormalizedDocPayload sent to Python.
// Uses snake_case JSON to match the Python Pydantic model.
type Doc struct {
	ID          string         `json:"id"`
	SourceID    string         `json:"source_id"`
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Snippet     string         `json:"snippet"`
	PublishedAt string         `json:"published_at"`
	RetrievedAt string         `json:"retrieved_at"`
	Author      string         `json:"author"`
	Score       float64        `json:"score"`
	Lang        string         `json:"lang"`
	DocType     string         `json:"doc_type"`
	Citations   []string       `json:"citations,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Hash        string         `json:"hash"`
}

// Result is the parsed response from the researcher sidecar.
type Result struct {
	RequestID        string     `json:"request_id"`
	Text             string     `json:"text"`
	Citations        []Citation `json:"citations"`
	Model            string     `json:"model"`
	Provider         string     `json:"provider"`
	CostUSD          float64    `json:"cost_usd"`
	PromptTokens     int        `json:"prompt_tokens"`
	CompletionTokens int        `json:"completion_tokens"`
	LatencyMs        float64    `json:"latency_ms"`
	Degraded         bool       `json:"degraded"`
	Notice           string     `json:"notice"`
}

// Citation is a single inline citation produced by the synthesis sidecar.
type Citation struct {
	Marker int    `json:"marker"`
	DocID  string `json:"doc_id"`
	URL    string `json:"url"`
	Title  string `json:"title"`
}

// Error sentinels used by the Go-side client.
var (
	// ErrInvalidRequest is returned when the sidecar rejects the request (HTTP 4xx).
	ErrInvalidRequest = errors.New("synthesis: invalid request")
	// ErrSidecarUnreachable is returned when the sidecar is not reachable (conn error).
	ErrSidecarUnreachable = errors.New("synthesis: sidecar unreachable")
	// ErrTimeout is returned when the context deadline is exceeded.
	ErrTimeout = errors.New("synthesis: timeout")
)
