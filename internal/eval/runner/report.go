// Package runner — report writing utilities.
//
// REQ-EVAL1-009: JSON report for CI gate consumption.
// REQ-EVAL1-010: Nightly history JSON artifact.
package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// WriteJSONReport writes the run report as JSON to the given path.
func WriteJSONReport(report *RunReport, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// WriteMarkdownReport writes a human-readable markdown report.
func WriteMarkdownReport(report *RunReport, path string) error {
	var md string
	md += "# EVAL-001 Citation Faithfulness Report\n\n"
	md += fmt.Sprintf("- **Total Queries**: %d\n", report.TotalQueries)
	md += fmt.Sprintf("- **Mean Score**: %.3f\n", report.MeanScore)
	md += fmt.Sprintf("- **Floor Score**: %.3f\n", report.FloorScore)
	md += fmt.Sprintf("- **Overrides**: %d\n", report.OverrideCount)
	md += fmt.Sprintf("- **Null Scores**: %d\n", report.NullCount)
	md += fmt.Sprintf("- **Judge Model**: %s\n", report.JudgeModel)
	md += fmt.Sprintf("- **Corpus Revision**: %s\n", report.CorpusRevision)
	md += fmt.Sprintf("- **Runtime**: %.1f seconds\n", report.RuntimeSeconds)
	md += fmt.Sprintf("- **Timestamp**: %s\n\n", report.Timestamp)

	// Lowest-scoring queries.
	scorable := filterScorable(report.Results)
	sort.Slice(scorable, func(i, j int) bool {
		if scorable[i].Score == nil && scorable[j].Score == nil {
			return false
		}
		if scorable[i].Score == nil {
			return true
		}
		if scorable[j].Score == nil {
			return false
		}
		return *scorable[i].Score < *scorable[j].Score
	})

	if len(scorable) > 0 {
		md += "## Lowest-Scoring Queries\n\n"
		limit := 10
		if len(scorable) < limit {
			limit = len(scorable)
		}
		for i := 0; i < limit; i++ {
			s := scorable[i]
			scoreStr := "null"
			if s.Score != nil {
				scoreStr = fmt.Sprintf("%.3f", *s.Score)
			}
			md += fmt.Sprintf("- **%s** (%s/%s): %s", s.QueryID, s.Locale, s.Category, scoreStr)
			if s.Overridden {
				md += " [OVERRIDDEN]"
			}
			if s.Error != "" {
				md += fmt.Sprintf(" [ERROR: %s]", truncate(s.Error, 80))
			}
			md += "\n"
		}
	}

	data := []byte(md)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write markdown report: %w", err)
	}
	return nil
}

func filterScorable(results []QueryResult) []QueryResult {
	out := make([]QueryResult, 0, len(results))
	for _, r := range results {
		if r.Score == nil {
			out = append(out, r)
		} else if !r.Overridden {
			out = append(out, r)
		}
	}
	return out
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
