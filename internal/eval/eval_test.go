package eval_test

import (
	"testing"

	"github.com/elymas/universal-search/internal/eval"
	"github.com/elymas/universal-search/internal/eval/runner"
)

func ptr(f float64) *float64 { return &f }

// TestToQueryScoresExcludesOverridden verifies overridden queries are dropped
// from the gate's scoring set, while null scores pass through as nil.
// REQ-EVAL1-003, REQ-EVAL1-008.
func TestToQueryScoresExcludesOverridden(t *testing.T) {
	t.Parallel()
	rep := runner.Report{
		Queries: []runner.QueryResult{
			{QueryID: "Q1", Score: ptr(0.9)},
			{QueryID: "Q2", Score: nil},                        // null preserved
			{QueryID: "Q3", Score: ptr(0.1), Overridden: true}, // dropped
		},
	}
	scores := eval.ToQueryScores(rep)
	if len(scores) != 2 {
		t.Fatalf("expected 2 scores (Q3 overridden out), got %d", len(scores))
	}
	for _, s := range scores {
		if s.QueryID == "Q3" {
			t.Error("Q3 should be excluded (overridden)")
		}
		if s.QueryID == "Q2" && s.Score != nil {
			t.Error("Q2 null score must pass through as nil")
		}
	}
}
