// Package tokenizer provides the Go client for the mecab-ko tokenizer sidecar.
//
// SPEC-IDX-003 REQ-IDX-003-006: HTTP client with retry, timeout, and graceful
// fallback when the sidecar is unreachable.
package tokenizer

import "errors"

// Request is the payload sent to POST /tokenize.
type Request struct {
	RequestID string `json:"request_id"`
	Text      string `json:"text"`
}

// Result is the parsed response from POST /tokenize.
type Result struct {
	// RequestID echoes the caller's request_id.
	RequestID string `json:"request_id"`
	// Tokens is the ordered list of surface-form morphemes.
	Tokens []string `json:"tokens"`
	// Joined is ' '.join(tokens); equals strings.Join(Tokens, " ").
	Joined string `json:"joined"`
	// MorphemeCount equals len(Tokens).
	MorphemeCount int `json:"morpheme_count"`
	// LatencyMs is the sidecar-measured parse latency in milliseconds.
	LatencyMs float64 `json:"latency_ms"`
	// DictVersion identifies the mecab-ko dictionary version.
	DictVersion string `json:"dict_version"`
}

// ErrInvalidInput is returned when the sidecar responds 400 (empty or oversize text).
var ErrInvalidInput = errors.New("tokenizer: invalid input")

// ErrSidecarUnreachable is returned when all retry attempts fail to connect.
var ErrSidecarUnreachable = errors.New("tokenizer: sidecar unreachable")

// ErrTimeout is returned when the request exceeds the configured deadline.
var ErrTimeout = errors.New("tokenizer: request timed out")
