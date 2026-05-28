// Package ci_test validates the CI gate logic.
//
// REQ-EVAL1-009: Exit code mapping (0=pass, 1=score fail, 2=judge error, 3=parse error).
// REQ-EVAL1-008: Mean ≥ 0.85 and floor ≥ 0.50.
// AC-005, AC-006, AC-007, EC-002.
package ci_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/elymas/universal-search/internal/eval/ci"
	"github.com/elymas/universal-search/internal/eval/runner"
)

func scorePtr(v float64) *float64 { return &v }

// ---------- Test: Gate passes with mean ≥ 0.85 and floor ≥ 0.50 ----------

func TestGatePassesWithGoodScores(t *testing.T) {
	report := &runner.RunReport{
		TotalQueries:   50,
		MeanScore:      0.89,
		FloorScore:     0.62,
		OverrideCount:  0,
		NullCount:      0,
		JudgeModel:     "claude-haiku-4-5",
		CorpusRevision: "1.0.0",
		Results:        make([]runner.QueryResult, 50),
	}
	exitCode, summary := ci.Evaluate(report)
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0; summary: %s", exitCode, summary)
	}
}

// ---------- Test: Gate fails when mean < 0.85 ----------

func TestGateFailsOnLowMean(t *testing.T) {
	report := &runner.RunReport{
		TotalQueries:   50,
		MeanScore:      0.82,
		FloorScore:     0.60,
		OverrideCount:  0,
		NullCount:      0,
		JudgeModel:     "claude-haiku-4-5",
		CorpusRevision: "1.0.0",
		Results:        make([]runner.QueryResult, 50),
	}
	exitCode, summary := ci.Evaluate(report)
	if exitCode != 1 {
		t.Errorf("exit code = %d, want 1; summary: %s", exitCode, summary)
	}
}

// ---------- Test: Gate fails when floor < 0.50 ----------

func TestGateFailsOnLowFloor(t *testing.T) {
	report := &runner.RunReport{
		TotalQueries:   50,
		MeanScore:      0.87,
		FloorScore:     0.40,
		OverrideCount:  0,
		NullCount:      0,
		JudgeModel:     "claude-haiku-4-5",
		CorpusRevision: "1.0.0",
		Results:        make([]runner.QueryResult, 50),
	}
	exitCode, summary := ci.Evaluate(report)
	if exitCode != 1 {
		t.Errorf("exit code = %d, want 1; summary: %s", exitCode, summary)
	}
	if !contains(summary, "floor violation") {
		t.Errorf("summary missing 'floor violation': %s", summary)
	}
}

// ---------- Test: Gate returns 2 on judge errors ----------

func TestGateReturnsTwoOnJudgeErrors(t *testing.T) {
	report := &runner.RunReport{
		TotalQueries:   50,
		MeanScore:      0.90,
		FloorScore:     0.70,
		OverrideCount:  0,
		NullCount:      3,
		JudgeModel:     "claude-haiku-4-5",
		CorpusRevision: "1.0.0",
		Results:        make([]runner.QueryResult, 50),
	}
	exitCode, _ := ci.Evaluate(report)
	if exitCode != 2 {
		t.Errorf("exit code = %d, want 2 (judge errors)", exitCode)
	}
}

// ---------- Test: Gate returns 3 on malformed report ----------

func TestGateReturnsThreeOnMalformedReport(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.json")
	os.WriteFile(badPath, []byte(`{invalid json`), 0o644)

	exitCode, _ := ci.EvaluateFile(badPath)
	if exitCode != 3 {
		t.Errorf("exit code = %d, want 3 (parse error)", exitCode)
	}
}

// ---------- Test: Gate reads valid file ----------

func TestGateReadsValidFile(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.json")

	report := &runner.RunReport{
		TotalQueries:   2,
		MeanScore:      0.90,
		FloorScore:     0.80,
		OverrideCount:  0,
		NullCount:      0,
		JudgeModel:     "claude-haiku-4-5",
		CorpusRevision: "1.0.0",
		Results:        make([]runner.QueryResult, 2),
	}

	data, _ := json.Marshal(report)
	os.WriteFile(reportPath, data, 0o644)

	exitCode, _ := ci.EvaluateFile(reportPath)
	if exitCode != 0 {
		t.Error("expected pass for valid report file")
	}
}

// ---------- Test: Summary format ----------

func TestSummaryFormat(t *testing.T) {
	report := &runner.RunReport{
		TotalQueries:   50,
		MeanScore:      0.890,
		FloorScore:     0.62,
		OverrideCount:  2,
		NullCount:      0,
		JudgeModel:     "claude-haiku-4-5",
		CorpusRevision: "1.0.0",
		Results:        make([]runner.QueryResult, 50),
	}
	_, summary := ci.Evaluate(report)
	if !contains(summary, "EVAL-001") {
		t.Errorf("summary missing EVAL-001: %s", summary)
	}
	if !contains(summary, "PASS") {
		t.Errorf("summary missing PASS: %s", summary)
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
