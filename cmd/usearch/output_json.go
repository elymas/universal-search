// Package main — JSON formatter for the query subcommand output.
//
// REQ-CLI-004: --format json emits a single JSON object to stdout with
// fields {schema_version, query, category, lang, adapters, summary, citations, stats}.
//
// @MX:NOTE: [AUTO] schemaVersion constant is "1"; bump on breaking JSON shape
// changes and add CHANGELOG entry + SPEC update. Downstream CI scripts parse
// this version field.
// @MX:SPEC: SPEC-CLI-001
package main

import (
	"encoding/json"
	"io"
)

// schemaVersion is the JSON output schema version.
// Bump this string (and create a SPEC + CHANGELOG entry) on any breaking change.
const schemaVersion = "1"

// jsonOutput is the top-level JSON object written to stdout.
type jsonOutput struct {
	SchemaVersion string          `json:"schema_version"`
	Query         string          `json:"query"`
	Category      string          `json:"category"`
	Lang          string          `json:"lang"`
	Adapters      []string        `json:"adapters"`
	Summary       string          `json:"summary"`
	Citations     []queryCitation `json:"citations"`
	Stats         queryStats      `json:"stats"`
}

// formatJSON encodes resp as a single JSON object to w (stdout).
// The JSON object always has schema_version="1".
func formatJSON(w io.Writer, resp *queryResponse) error {
	out := jsonOutput{
		SchemaVersion: schemaVersion,
		Query:         resp.Query,
		Category:      resp.Category,
		Lang:          resp.Lang,
		Adapters:      resp.Adapters,
		Summary:       resp.Summary,
		Citations:     resp.Citations,
		Stats:         resp.Stats,
	}
	if out.Adapters == nil {
		out.Adapters = []string{}
	}
	if out.Citations == nil {
		out.Citations = []queryCitation{}
	}
	enc := json.NewEncoder(w)
	return enc.Encode(out)
}
