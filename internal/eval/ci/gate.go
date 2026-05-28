// Package ci provides the CI gate logic for SPEC-EVAL-001.
//
// REQ-EVAL1-009: Exit code mapping:
//
//	0 = pass (mean ≥ 0.85, floor ≥ 0.50, no judge errors)
//	1 = score fail (mean or floor below threshold)
//	2 = judge error (one or more null scores)
//	3 = parse error (malformed report)
package ci

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/elymas/universal-search/internal/eval/runner"
)

const (
	// ExitPass indicates the benchmark passed all thresholds.
	ExitPass = 0
	// ExitScoreFail indicates mean or floor below threshold.
	ExitScoreFail = 1
	// ExitJudgeError indicates one or more queries had judge errors.
	ExitJudgeError = 2
	// ExitParseError indicates the report file could not be parsed.
	ExitParseError = 3

	// ThresholdMean is the minimum aggregate mean score (FROZEN).
	ThresholdMean = 0.85
	// ThresholdFloor is the minimum per-query score (FROZEN).
	ThresholdFloor = 0.50
)

// Evaluate checks a RunReport against CI thresholds and returns an exit code
// plus a human-readable summary line.
func Evaluate(report *runner.RunReport) (int, string) {
	// Check for judge errors first (exit 2 takes priority).
	if report.NullCount > 0 {
		summary := fmt.Sprintf(
			"EVAL-001 result=JUDGE_ERROR mean=%.3f floor=%.3f overrides=%d nulls=%d",
			report.MeanScore, report.FloorScore, report.OverrideCount, report.NullCount,
		)
		return ExitJudgeError, summary
	}

	// Check floor violation.
	if report.FloorScore < ThresholdFloor {
		summary := fmt.Sprintf(
			"EVAL-001 result=FAIL mean=%.3f floor=%.3f overrides=%d nulls=%d floor violation: %.3f < %.3f",
			report.MeanScore, report.FloorScore, report.OverrideCount, report.NullCount,
			report.FloorScore, ThresholdFloor,
		)
		return ExitScoreFail, summary
	}

	// Check mean threshold.
	if report.MeanScore < ThresholdMean {
		summary := fmt.Sprintf(
			"EVAL-001 result=FAIL mean=%.3f floor=%.3f overrides=%d nulls=%d",
			report.MeanScore, report.FloorScore, report.OverrideCount, report.NullCount,
		)
		return ExitScoreFail, summary
	}

	// Pass.
	summary := fmt.Sprintf(
		"EVAL-001 result=PASS mean=%.3f floor=%.3f overrides=%d nulls=%d",
		report.MeanScore, report.FloorScore, report.OverrideCount, report.NullCount,
	)
	return ExitPass, summary
}

// EvaluateFile reads a JSON report file and evaluates it.
// Returns exit code 3 if the file cannot be parsed.
func EvaluateFile(path string) (int, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExitParseError, fmt.Sprintf("EVAL-001 result=PARSE_ERROR read: %v", err)
	}

	var report runner.RunReport
	if err := json.Unmarshal(data, &report); err != nil {
		return ExitParseError, fmt.Sprintf("EVAL-001 result=PARSE_ERROR json: %v", err)
	}

	return Evaluate(&report)
}
