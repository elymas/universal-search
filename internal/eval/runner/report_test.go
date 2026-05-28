// Package runner_test — report writer tests.
package runner_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/elymas/universal-search/internal/eval/runner"
)

func TestReportWritesJSON(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.json")

	scores := []float64{1.0, 0.8, 0.6}
	results := make([]runner.QueryResult, 3)
	for i := range results {
		results[i] = runner.QueryResult{
			QueryID:   "EVAL-001-Q001",
			Locale:    "en",
			Category:  "factual",
			Score:     &scores[i],
			Overridden: false,
		}
	}

	report := &runner.RunReport{
		TotalQueries:    3,
		MeanScore:       0.8,
		FloorScore:      0.6,
		OverrideCount:   0,
		NullCount:       0,
		JudgeModel:      "test-model",
		CorpusRevision:  "1.0.0",
		Results:         results,
	}

	err := runner.WriteJSONReport(report, outPath)
	if err != nil {
		t.Fatalf("WriteJSONReport() error: %v", err)
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	var parsed runner.RunReport
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if parsed.MeanScore != 0.8 {
		t.Errorf("parsed mean = %f, want 0.8", parsed.MeanScore)
	}
	if parsed.TotalQueries != 3 {
		t.Errorf("parsed total = %d, want 3", parsed.TotalQueries)
	}
}

func TestReportWritesMarkdown(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.md")

	score := 0.75
	results := []runner.QueryResult{
		{
			QueryID:  "EVAL-001-Q001",
			Locale:   "en",
			Category: "factual",
			Score:    &score,
		},
	}

	report := &runner.RunReport{
		TotalQueries:   1,
		MeanScore:      0.75,
		FloorScore:     0.75,
		OverrideCount:  0,
		NullCount:      0,
		JudgeModel:     "test-model",
		CorpusRevision: "1.0.0",
		Results:        results,
	}

	err := runner.WriteMarkdownReport(report, outPath)
	if err != nil {
		t.Fatalf("WriteMarkdownReport() error: %v", err)
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	content := string(raw)
	if len(content) == 0 {
		t.Error("markdown report is empty")
	}
	// Check key sections exist
	checks := []string{"EVAL-001", "Mean Score", "Floor Score", "Judge Model"}
	for _, check := range checks {
		if !contains(content, check) {
			t.Errorf("markdown missing %q", check)
		}
	}
}

func TestMarkdownWithNullScoresAndOverrides(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.md")

	score := 0.3
	results := []runner.QueryResult{
		{QueryID: "EVAL-001-Q001", Locale: "en", Category: "factual", Score: &score},
		{QueryID: "EVAL-001-Q002", Locale: "en", Category: "factual", Score: nil, Error: "judge unavailable"},
		{QueryID: "EVAL-001-Q003", Locale: "ko", Category: "korean", Score: nil, Overridden: true, OverrideReason: "known flaky"},
	}

	report := &runner.RunReport{
		TotalQueries:   3,
		MeanScore:      0.3,
		FloorScore:     0.3,
		OverrideCount:  1,
		NullCount:      1,
		JudgeModel:     "test-model",
		CorpusRevision: "1.0.0",
		Results:        results,
	}

	err := runner.WriteMarkdownReport(report, outPath)
	if err != nil {
		t.Fatalf("WriteMarkdownReport() error: %v", err)
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	content := string(raw)
	if !contains(content, "OVERRIDDEN") {
		t.Error("markdown missing OVERRIDDEN marker")
	}
	if !contains(content, "ERROR") {
		t.Error("markdown missing ERROR marker")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && findSubstr(s, sub)))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
