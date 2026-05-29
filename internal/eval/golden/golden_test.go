// Package golden_test validates the SPEC-EVAL-001 golden set integrity:
//   - 50 queries (35 EN + 15 KO) with correct schema and locale partition
//   - ≥200 corpus documents deserializing to pkg/types.NormalizedDoc
//   - Referential integrity: every expected_sources resolves to a corpus doc
//   - Overrides file schema validity
//
// REQ-EVAL1-001, REQ-EVAL1-002, AC-001.
package golden_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// goldenDir is the base directory for all golden-set artifacts.
const goldenDir = "."

// corpusDir is the subdirectory holding NormalizedDoc fixture files.
const corpusDir = "corpus"

// ---------- Query record types ----------

// queryRecord mirrors one line of queries.jsonl.
// REQ-EVAL1-001: id, query, locale, category, expected_sources are required.
type queryRecord struct {
	ID              string   `json:"id"`
	Query           string   `json:"query"`
	Locale          string   `json:"locale"`
	Category        string   `json:"category"`
	ExpectedSources []string `json:"expected_sources"`
}

// validCategories is the closed set of allowed category values.
var validCategories = map[string]bool{
	"factual":    true,
	"comparison": true,
	"synthesis":  true,
	"korean":     true,
	"edge":       true,
}

// validLocales is the closed set of allowed locale values.
var validLocales = map[string]bool{
	"en": true,
	"ko": true,
}

// loadQueries reads all lines from queries.jsonl and returns parsed records.
func loadQueries(t *testing.T) []queryRecord {
	t.Helper()
	path := filepath.Join(goldenDir, "queries.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open queries.jsonl: %v", err)
	}
	defer f.Close()

	var records []queryRecord
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue // skip blank lines
		}
		var rec queryRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("queries.jsonl line %d: unmarshal: %v", lineNum, err)
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanning queries.jsonl: %v", err)
	}
	return records
}

// loadCorpus reads every doc-*.json file from the corpus directory and returns
// a map of doc ID → NormalizedDoc.
func loadCorpus(t *testing.T) map[string]types.NormalizedDoc {
	t.Helper()
	dir := filepath.Join(goldenDir, corpusDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus dir: %v", err)
	}

	corpus := make(map[string]types.NormalizedDoc)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "doc-") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read corpus file %s: %v", entry.Name(), err)
		}
		var doc types.NormalizedDoc
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("unmarshal corpus file %s: %v", entry.Name(), err)
		}
		corpus[doc.ID] = doc
	}
	return corpus
}

// ---------- Tests ----------

// TestGoldenSetCount verifies exactly 50 query records exist.
func TestGoldenSetCount(t *testing.T) {
	records := loadQueries(t)
	if got := len(records); got != 50 {
		t.Errorf("golden set has %d queries, want 50", got)
	}
}

// TestGoldenSetSchema verifies every record has all required fields with valid values.
func TestGoldenSetSchema(t *testing.T) {
	records := loadQueries(t)
	for i, rec := range records {
		// ID must match EVAL-001-Q{NNN}
		if !strings.HasPrefix(rec.ID, "EVAL-001-Q") {
			t.Errorf("record %d: id %q missing EVAL-001-Q prefix", i, rec.ID)
		}
		// Edge category permits intentionally empty/minimal queries (EC pattern).
		if rec.Query == "" && rec.Category != "edge" {
			t.Errorf("record %d (%s): empty query", i, rec.ID)
		}
		if !validLocales[rec.Locale] {
			t.Errorf("record %d (%s): invalid locale %q", i, rec.ID, rec.Locale)
		}
		if !validCategories[rec.Category] {
			t.Errorf("record %d (%s): invalid category %q", i, rec.ID, rec.Category)
		}
		if len(rec.ExpectedSources) == 0 {
			t.Errorf("record %d (%s): expected_sources is empty", i, rec.ID)
		}
	}
}

// TestGoldenSetLocalePartition verifies exactly 35 EN + 15 KO split.
func TestGoldenSetLocalePartition(t *testing.T) {
	records := loadQueries(t)
	enCount, koCount := 0, 0
	for _, rec := range records {
		switch rec.Locale {
		case "en":
			enCount++
		case "ko":
			koCount++
		}
	}
	if enCount != 35 {
		t.Errorf("en locale count = %d, want 35", enCount)
	}
	if koCount != 15 {
		t.Errorf("ko locale count = %d, want 15", koCount)
	}
}

// TestCorpusDeserializes verifies every corpus JSON file deserializes to a
// valid NormalizedDoc (passes Validate).
func TestCorpusDeserializes(t *testing.T) {
	corpus := loadCorpus(t)
	for id, doc := range corpus {
		if err := doc.Validate(); err != nil {
			t.Errorf("corpus doc %s: validate: %v", id, err)
		}
	}
}

// TestCorpusSize verifies at least 200 corpus documents exist.
func TestCorpusSize(t *testing.T) {
	// Also count files on disk to ensure no duplicates lost.
	dir := filepath.Join(goldenDir, corpusDir)
	fileCount := 0
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasPrefix(d.Name(), "doc-") && strings.HasSuffix(d.Name(), ".json") {
			fileCount++
		}
		return nil
	})
	if fileCount < 200 {
		t.Errorf("corpus has %d doc files, want >= 200", fileCount)
	}

	// Also verify the map has the same size.
	corpus := loadCorpus(t)
	if len(corpus) < 200 {
		t.Errorf("corpus has %d unique doc IDs, want >= 200", len(corpus))
	}
}

// TestExpectedSourcesResolveToCorpus verifies every expected_sources entry in
// the query set maps to a real corpus document.
func TestExpectedSourcesResolveToCorpus(t *testing.T) {
	records := loadQueries(t)
	corpus := loadCorpus(t)

	for _, rec := range records {
		for _, src := range rec.ExpectedSources {
			if _, ok := corpus[src]; !ok {
				t.Errorf("query %s: expected_source %q not found in corpus", rec.ID, src)
			}
		}
	}
}

// TestOverridesSchemaValid verifies overrides.json parses correctly.
func TestOverridesSchemaValid(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(goldenDir, "overrides.json"))
	if err != nil {
		t.Fatalf("read overrides.json: %v", err)
	}

	var overrides struct {
		Overrides []struct {
			QueryID         string `json:"query_id"`
			ManualOverride  string `json:"manual_override"`
			OverrideReason  string `json:"override_reason"`
			ExpiresAt       string `json:"expires_at"`
			CreatedAt       string `json:"created_at"`
			CreatedBy       string `json:"created_by"`
		} `json:"overrides"`
	}
	if err := json.Unmarshal(raw, &overrides); err != nil {
		t.Fatalf("unmarshal overrides.json: %v", err)
	}

	// V1 initial file should have zero overrides.
	if len(overrides.Overrides) != 0 {
		// Not a hard failure — but validate schema for any present overrides.
		for i, o := range overrides.Overrides {
			if o.QueryID == "" {
				t.Errorf("override %d: empty query_id", i)
			}
			if o.ManualOverride != "pass" {
				t.Errorf("override %d: manual_override must be 'pass', got %q", i, o.ManualOverride)
			}
			if _, err := time.Parse(time.RFC3339, o.ExpiresAt); err != nil {
				t.Errorf("override %d: expires_at %q not RFC3339: %v", i, o.ExpiresAt, err)
			}
		}
	}
}

// TestManifestExists verifies manifest.json is present with required fields.
func TestManifestExists(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(goldenDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}

	var manifest struct {
		CorpusRevision string `json:"corpus_revision"`
		TotalDocs      int    `json:"total_docs"`
		CreatedAt      string `json:"created_at"`
		LicenseSummary string `json:"license_summary"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("unmarshal manifest.json: %v", err)
	}
	if manifest.CorpusRevision == "" {
		t.Error("manifest.json: corpus_revision is empty")
	}
	if manifest.TotalDocs < 200 {
		t.Errorf("manifest.json: total_docs = %d, want >= 200", manifest.TotalDocs)
	}
	if manifest.CreatedAt == "" {
		t.Error("manifest.json: created_at is empty")
	}
	if manifest.LicenseSummary == "" {
		t.Error("manifest.json: license_summary is empty")
	}

	// Verify total_docs matches actual corpus size.
	corpus := loadCorpus(t)
	if manifest.TotalDocs != len(corpus) {
		t.Errorf("manifest.json total_docs = %d but corpus has %d docs", manifest.TotalDocs, len(corpus))
	}
}

// TestQueryIDsAreUnique verifies all query IDs are distinct.
func TestQueryIDsAreUnique(t *testing.T) {
	records := loadQueries(t)
	seen := make(map[string]int)
	for _, rec := range records {
		if prev, exists := seen[rec.ID]; exists {
			t.Errorf("duplicate query ID %q at positions %d and %d", rec.ID, prev, seen[rec.ID])
		}
		seen[rec.ID] = len(seen)
	}
}

// TestQueryIDsAreSequential verifies IDs follow EVAL-001-Q001..Q050 pattern.
func TestQueryIDsAreSequential(t *testing.T) {
	records := loadQueries(t)
	seen := make(map[int]bool, len(records))
	for _, rec := range records {
		var num int
		n, err := fmt.Sscanf(rec.ID, "EVAL-001-Q%d", &num)
		if err != nil || n != 1 || num < 1 || num > 50 {
			t.Errorf("query ID %q does not match EVAL-001-Q001..Q050 pattern", rec.ID)
			continue
		}
		if seen[num] {
			t.Errorf("duplicate query number %d", num)
		}
		seen[num] = true
	}
	if len(seen) != 50 {
		t.Errorf("found %d unique query numbers, want 50", len(seen))
	}
}
