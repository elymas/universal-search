package runner_test

import (
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/eval/runner"
	"github.com/elymas/universal-search/internal/eval/scorer"
)

func ptr(f float64) *float64 { return &f }

func sampleReport() runner.Report {
	return runner.Report{
		MeanScore:     0.86,
		NullCount:     1,
		OverrideCount: 1,
		Queries: []runner.QueryResult{
			{QueryID: "Q1", Category: "factual", Locale: "en", Score: ptr(1.0)},
			{QueryID: "Q2", Category: "synthesis", Locale: "en", Score: ptr(0.40), PerClaim: []scorer.ClaimScore{
				{Text: "bad claim [1]", Supported: false, JudgeRationale: "cited doc unrelated"},
			}},
			{QueryID: "Q3", Category: "korean", Locale: "ko", Score: nil, ErrorClass: "judge_unavailable"},
			{QueryID: "Q4", Category: "edge", Locale: "en", Score: ptr(0.95), Overridden: true},
		},
	}
}

// TestReportHeaderHasMeanAndCounts verifies the markdown summary line.
// REQ-EVAL1-007.
func TestReportHeaderHasMeanAndCounts(t *testing.T) {
	t.Parallel()
	md := runner.RenderMarkdown(sampleReport(), runner.RenderOpts{})
	if !strings.Contains(md, "0.86") {
		t.Errorf("report missing mean score:\n%s", md)
	}
	if !strings.Contains(md, "nulls") && !strings.Contains(md, "Null") {
		t.Errorf("report missing null count")
	}
}

// TestReportTopTenLowestQueriesSection verifies the lowest-scoring section.
// REQ-EVAL1-007.
func TestReportTopTenLowestQueriesSection(t *testing.T) {
	t.Parallel()
	md := runner.RenderMarkdown(sampleReport(), runner.RenderOpts{})
	if !strings.Contains(md, "Lowest-Scoring Queries") {
		t.Errorf("report missing Lowest-Scoring Queries section:\n%s", md)
	}
}

// TestReportContainsJudgeRationalesForLowScores verifies rationales surface.
// REQ-EVAL1-007: un-explained low scores must never appear.
func TestReportContainsJudgeRationalesForLowScores(t *testing.T) {
	t.Parallel()
	md := runner.RenderMarkdown(sampleReport(), runner.RenderOpts{})
	if !strings.Contains(md, "cited doc unrelated") {
		t.Errorf("report missing judge rationale for low-scoring claim:\n%s", md)
	}
}

// TestReportPerCategoryBreakdown verifies per-category aggregation appears.
func TestReportPerCategoryBreakdown(t *testing.T) {
	t.Parallel()
	md := runner.RenderMarkdown(sampleReport(), runner.RenderOpts{})
	// factual (Q1) and synthesis (Q2) have scored queries; korean (Q3) is null
	// and edge (Q4) is overridden, so both are correctly excluded from the mean.
	if !strings.Contains(md, "Per-Category Breakdown") {
		t.Errorf("report missing per-category section:\n%s", md)
	}
	if !strings.Contains(md, "factual") || !strings.Contains(md, "synthesis") {
		t.Errorf("report missing scored categories:\n%s", md)
	}
}

// TestReportCostReported verifies cost is rendered when provided.
// NFR-EVAL1-003.
func TestReportCostReported(t *testing.T) {
	t.Parallel()
	md := runner.RenderMarkdown(sampleReport(), runner.RenderOpts{CostUSD: 0.42})
	if !strings.Contains(md, "0.42") {
		t.Errorf("report missing cost:\n%s", md)
	}
}

// TestReportNullQueriesListed verifies judge-unavailable queries are surfaced.
// REQ-EVAL1-006.
func TestReportNullQueriesListed(t *testing.T) {
	t.Parallel()
	md := runner.RenderMarkdown(sampleReport(), runner.RenderOpts{})
	if !strings.Contains(md, "Q3") {
		t.Errorf("report missing null query Q3:\n%s", md)
	}
}
