// Package main — queryResponse is the internal pipeline output that formatters consume.
//
// REQ-CLI-004: Both formatText and formatJSON consume a queryResponse struct.
// REQ-CLI-011: stats.request_id carries the ULID for the invocation.
package main

import "github.com/elymas/universal-search/pkg/types"

// queryCitation represents a single numbered citation in the CLI output.
type queryCitation struct {
	Index  int    `json:"index"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Source string `json:"source"`
	DocID  string `json:"doc_id"`
}

// queryStats holds per-invocation metadata surfaced in JSON output.
// REQ-CLI-011: request_id is a 26-char Crockford Base32 ULID.
type queryStats struct {
	RequestID    string `json:"request_id"`
	AdapterCount int    `json:"adapter_count"`
	DocCount     int    `json:"doc_count"`
	SynthOK      bool   `json:"synth_ok"`
}

// queryResponse is the internal representation of a completed pipeline result.
// It is the input to both formatText and formatJSON.
type queryResponse struct {
	Query     string                `json:"query"`
	Category  string                `json:"category"`
	Lang      string                `json:"lang"`
	Adapters  []string              `json:"adapters"`
	Summary   string                `json:"summary"`
	Citations []queryCitation       `json:"citations"`
	Stats     queryStats            `json:"stats"`
	Docs      []types.NormalizedDoc `json:"-"` // used for degraded text output
}
