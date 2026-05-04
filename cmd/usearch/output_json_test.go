package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestFormatJSONShape(t *testing.T) {
	resp := &queryResponse{
		Query:    "hello world",
		Category: "social",
		Lang:     "en",
		Adapters: []string{"reddit", "hackernews"},
		Summary:  "Summary text.",
		Citations: []queryCitation{
			{Index: 1, Title: "Reddit Post", URL: "https://reddit.com/r/test", Source: "reddit", DocID: "doc1"},
		},
		Stats: queryStats{
			RequestID:    "01HRZBTEST1234567890123456",
			AdapterCount: 2,
			DocCount:     3,
			SynthOK:      true,
		},
	}

	var buf bytes.Buffer
	if err := formatJSON(&buf, resp); err != nil {
		t.Fatalf("formatJSON error: %v", err)
	}

	// Must parse as valid JSON.
	var out map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	// Required keys present.
	requiredKeys := []string{"schema_version", "query", "category", "lang", "adapters", "summary", "citations", "stats"}
	for _, k := range requiredKeys {
		if _, ok := out[k]; !ok {
			t.Errorf("missing required JSON key: %q", k)
		}
	}

	// schema_version must be "1".
	var sv string
	if err := json.Unmarshal(out["schema_version"], &sv); err != nil {
		t.Fatalf("schema_version unmarshal: %v", err)
	}
	if sv != "1" {
		t.Errorf("schema_version = %q, want %q", sv, "1")
	}

	// citations must be an array.
	var cites []map[string]any
	if err := json.Unmarshal(out["citations"], &cites); err != nil {
		t.Fatalf("citations not an array: %v", err)
	}
	if len(cites) != 1 {
		t.Errorf("citations length = %d, want 1", len(cites))
	}
}

func TestFormatJSONEmptyCollections(t *testing.T) {
	resp := &queryResponse{
		Query:     "test",
		Adapters:  nil,
		Citations: nil,
	}

	var buf bytes.Buffer
	if err := formatJSON(&buf, resp); err != nil {
		t.Fatalf("formatJSON error: %v", err)
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	// adapters and citations must be [] not null.
	var adapters []string
	if err := json.Unmarshal(out["adapters"], &adapters); err != nil {
		t.Fatalf("adapters not an array: %v", err)
	}
	var cites []queryCitation
	if err := json.Unmarshal(out["citations"], &cites); err != nil {
		t.Fatalf("citations not an array: %v", err)
	}
}

func TestSchemaVersionConstant(t *testing.T) {
	if schemaVersion != "1" {
		t.Errorf("schemaVersion = %q, want %q", schemaVersion, "1")
	}
}
