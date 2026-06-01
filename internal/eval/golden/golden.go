// Package golden loads and validates the SPEC-EVAL-001 frozen golden set:
// the 50-query benchmark queries, the NormalizedDoc fixture corpus, and the
// manual false-positive override list.
//
// REQ-EVAL1-001: 50-query golden set (35 EN + 15 KO).
// REQ-EVAL1-002: frozen NormalizedDoc corpus (V1 floor 50 docs).
// REQ-EVAL1-003: manual override list with a simple cap check.
package golden

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/elymas/universal-search/pkg/types"
)

// Query is one golden-set record (one JSON line in queries.jsonl).
type Query struct {
	ID              string   `json:"id"`
	Query           string   `json:"query"`
	Locale          string   `json:"locale"`
	ExpectedSources []string `json:"expected_sources,omitempty"`
	Category        string   `json:"category"`
	Notes           string   `json:"notes,omitempty"`
}

// Override is one entry in the manual false-positive override list.
type Override struct {
	QueryID        string `json:"query_id"`
	ManualOverride string `json:"manual_override"`
	OverrideReason string `json:"override_reason"`
	CreatedAt      string `json:"created_at,omitempty"`
	CreatedBy      string `json:"created_by,omitempty"`
	ExpiresAt      string `json:"expires_at,omitempty"`
}

// goldenDir returns the absolute path to this package's directory so loaders
// resolve fixtures regardless of the caller's working directory (tests run
// with a per-package CWD).
func goldenDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

// QueriesPath returns the path to queries.jsonl.
func QueriesPath() string { return filepath.Join(goldenDir(), "queries.jsonl") }

// CorpusDir returns the path to the corpus fixture directory.
func CorpusDir() string { return filepath.Join(goldenDir(), "corpus") }

// OverridesPath returns the path to overrides.json.
func OverridesPath() string { return filepath.Join(goldenDir(), "overrides.json") }

// LoadQueries parses queries.jsonl into a slice of Query records.
func LoadQueries(path string) ([]Query, error) {
	f, err := os.Open(path) //nolint:gosec // path is a fixed package-relative fixture path.
	if err != nil {
		return nil, fmt.Errorf("golden: open queries: %w", err)
	}
	defer func() { _ = f.Close() }()

	var out []Query
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var q Query
		if err := json.Unmarshal([]byte(raw), &q); err != nil {
			return nil, fmt.Errorf("golden: queries line %d: %w", line, err)
		}
		out = append(out, q)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("golden: scan queries: %w", err)
	}
	return out, nil
}

// LoadCorpus reads every *.json fixture in dir and returns a map keyed by doc ID.
func LoadCorpus(dir string) (map[string]types.NormalizedDoc, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("golden: read corpus dir: %w", err)
	}
	out := make(map[string]types.NormalizedDoc)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p) //nolint:gosec // fixed package-relative fixture path.
		if err != nil {
			return nil, fmt.Errorf("golden: read %s: %w", e.Name(), err)
		}
		var doc types.NormalizedDoc
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("golden: parse %s: %w", e.Name(), err)
		}
		out[doc.ID] = doc
	}
	return out, nil
}

// LoadOverrides parses overrides.json into a slice of Override entries. A
// missing file is treated as an empty list (no overrides active).
func LoadOverrides(path string) ([]Override, error) {
	data, err := os.ReadFile(path) //nolint:gosec // fixed package-relative fixture path.
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("golden: read overrides: %w", err)
	}
	var out []Override
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("golden: parse overrides: %w", err)
	}
	return out, nil
}

// CheckOverrideCap returns an error when len(overrides) exceeds cap.
// REQ-EVAL1-003: simple cap check (default cap 5).
func CheckOverrideCap(overrides []Override, cap int) error {
	if len(overrides) > cap {
		return fmt.Errorf("golden: override cap exceeded: %d entries, cap %d", len(overrides), cap)
	}
	return nil
}
