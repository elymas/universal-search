// Package deepreport provides the Go-side HTTP client for the STORM sidecar.
//
// SPEC-DEEP-001 M6: Long-form report generation via sidecar HTTP endpoint.
// REQ-DEEP1-001: POST /generate_report returns structured report JSON.
// REQ-DEEP1-004: Deadline and budget error mapping.
package deepreport

import "errors"

// NormalizedDocPayload mirrors the Python-side doc payload for the STORM sidecar.
type NormalizedDocPayload struct {
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

// Request is the payload sent to the STORM sidecar POST /generate_report.
type Request struct {
	RequestID       string                   `json:"request_id"`
	Query           string                   `json:"query"`
	Lang            string                   `json:"lang,omitempty"`
	Docs            []NormalizedDocPayload   `json:"docs"`
	MaxTokens       *float64                 `json:"max_tokens,omitempty"`
	MaxCostUSD      *float64                 `json:"max_cost_usd,omitempty"`
	MaxPerspectives *float64                 `json:"max_perspectives,omitempty"`
	MaxConvTurns    *float64                 `json:"max_conv_turns,omitempty"`
	MaxLatencyMS    *float64                 `json:"max_latency_ms,omitempty"`
}

// Report is the parsed response from the STORM sidecar.
type Report struct {
	RequestID       string    `json:"request_id"`
	Title           string    `json:"title"`
	Sections        []Section `json:"sections"`
	Citations       []Citation `json:"citations"`
	Model           string    `json:"model"`
	Provider        string    `json:"provider"`
	CostUSD         float64   `json:"cost_usd"`
	PromptTokens    int       `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	LatencyMS       int64     `json:"latency_ms"`
	Degraded        bool      `json:"degraded"`
	Notice          string    `json:"notice"`
	SchemaVersion   int       `json:"schema_version"`
}

// Section is a single section of the long-form report.
type Section struct {
	SectionIndex int       `json:"section_index"`
	Heading      string    `json:"heading"`
	Level        int       `json:"level"`
	Text         string    `json:"text"`
	Sentences    []Sentence `json:"sentences"`
}

// Sentence is a single sentence within a section, with citation markers.
type Sentence struct {
	SentenceIndex int   `json:"sentence_index"`
	Text          string `json:"text"`
	Markers       []int  `json:"markers"`
}

// Citation maps a numeric marker to a source document.
type Citation struct {
	Marker int    `json:"marker"`
	DocID  string `json:"doc_id"`
	URL    string `json:"url"`
	Title  string `json:"title"`
}

// Error sentinels used by the Go-side deepreport client.
var (
	// ErrInvalidRequest is returned when the sidecar rejects the request (HTTP 422).
	ErrInvalidRequest = errors.New("deepreport: invalid request")
	// ErrSidecarUnreachable is returned when the sidecar is not reachable (conn error / 5xx).
	ErrSidecarUnreachable = errors.New("deepreport: sidecar unreachable")
	// ErrTimeout is returned when the context deadline is exceeded.
	ErrTimeout = errors.New("deepreport: timeout")
	// ErrBudgetExceeded is returned when the sidecar reports cost exceeded (HTTP 402).
	ErrBudgetExceeded = errors.New("deepreport: budget exceeded")
	// ErrDeadlineExceeded is returned when the sidecar reports latency exceeded (HTTP 504).
	ErrDeadlineExceeded = errors.New("deepreport: deadline exceeded")
)
