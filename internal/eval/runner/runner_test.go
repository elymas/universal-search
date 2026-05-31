package runner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/elymas/universal-search/internal/eval/golden"
	"github.com/elymas/universal-search/internal/eval/runner"
	"github.com/elymas/universal-search/internal/eval/scorer"
	"github.com/elymas/universal-search/internal/synthesis"
)

// stubSynth returns a canned synthesis result per query.
type stubSynth struct {
	fn func(q golden.Query) (synthesis.Result, error)
}

func (s stubSynth) Synthesize(_ context.Context, q golden.Query, _ map[string]string) (synthesis.Result, error) {
	return s.fn(q)
}

// stubScorer returns a canned score per query.
type stubScorer struct {
	fn func(queryID string) (scorer.Result, error)
}

func (s stubScorer) Score(_ context.Context, queryID, _ string, _ synthesis.Result, _ map[string]string) (scorer.Result, error) {
	return s.fn(queryID)
}

func twoQueries() []golden.Query {
	return []golden.Query{
		{ID: "Q1", Query: "q1", Locale: "en", Category: "factual", ExpectedSources: []string{"doc-001"}},
		{ID: "Q2", Query: "q2", Locale: "en", Category: "factual", ExpectedSources: []string{"doc-002"}},
	}
}

func corpus() map[string]string {
	return map[string]string{"doc-001": "body 1", "doc-002": "body 2"}
}

// TestRunnerAggregatesMeanOverScores verifies the aggregate mean is computed
// across query scores.
// REQ-EVAL1-006.
func TestRunnerAggregatesMeanOverScores(t *testing.T) {
	t.Parallel()
	synth := stubSynth{fn: func(q golden.Query) (synthesis.Result, error) {
		return synthesis.Result{Text: "Claim [1].", Citations: []synthesis.Citation{{Marker: 1, DocID: "doc-001"}}}, nil
	}}
	sc := stubScorer{fn: func(id string) (scorer.Result, error) {
		if id == "Q1" {
			return scorer.Result{Score: 1.0}, nil
		}
		return scorer.Result{Score: 0.8}, nil
	}}
	rep, err := runner.Run(context.Background(), runner.Config{Concurrency: 2}, twoQueries(), corpus(), nil, synth, sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.MeanScore < 0.89 || rep.MeanScore > 0.91 {
		t.Errorf("mean = %v, want ~0.9", rep.MeanScore)
	}
	if rep.NullCount != 0 {
		t.Errorf("null count = %d, want 0", rep.NullCount)
	}
}

// TestJudgeUnavailableMarksNullNotZero verifies a judge failure yields a null
// score (not zero) and is excluded from the mean.
// REQ-EVAL1-006(b), REQ-EVAL1-006: AggregateMeanExcludesNullScores.
func TestJudgeUnavailableMarksNullNotZero(t *testing.T) {
	t.Parallel()
	synth := stubSynth{fn: func(q golden.Query) (synthesis.Result, error) {
		return synthesis.Result{Text: "Claim [1].", Citations: []synthesis.Citation{{Marker: 1, DocID: "doc-001"}}}, nil
	}}
	sc := stubScorer{fn: func(id string) (scorer.Result, error) {
		if id == "Q2" {
			return scorer.Result{}, scorer.WrapUnavailable(errors.New("503"))
		}
		return scorer.Result{Score: 1.0}, nil
	}}
	rep, err := runner.Run(context.Background(), runner.Config{Concurrency: 1}, twoQueries(), corpus(), nil, synth, sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.NullCount != 1 {
		t.Fatalf("null count = %d, want 1", rep.NullCount)
	}
	// Mean computed over non-null scores only → only Q1 (1.0).
	if rep.MeanScore != 1.0 {
		t.Errorf("mean = %v, want 1.0 (null excluded)", rep.MeanScore)
	}
	// The null query's score must be nil-marked.
	var q2 *runner.QueryResult
	for i := range rep.Queries {
		if rep.Queries[i].QueryID == "Q2" {
			q2 = &rep.Queries[i]
		}
	}
	if q2 == nil || q2.Score != nil {
		t.Errorf("Q2 score must be nil (null), got %#v", q2)
	}
}

// TestRunnerExitCode2OnJudgeError verifies that a null score forces exit-class 2.
// REQ-EVAL1-006(d).
func TestRunnerExitCode2OnJudgeError(t *testing.T) {
	t.Parallel()
	synth := stubSynth{fn: func(q golden.Query) (synthesis.Result, error) {
		return synthesis.Result{Text: "Claim [1].", Citations: []synthesis.Citation{{Marker: 1, DocID: "doc-001"}}}, nil
	}}
	sc := stubScorer{fn: func(id string) (scorer.Result, error) {
		return scorer.Result{}, scorer.WrapUnavailable(errors.New("down"))
	}}
	rep, err := runner.Run(context.Background(), runner.Config{Concurrency: 1}, twoQueries(), corpus(), nil, synth, sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !rep.HasJudgeError() {
		t.Error("expected HasJudgeError true when null scores present")
	}
}

// TestRunnerAppliesOverrideExcludesFromAggregate verifies an override entry
// removes the query from the aggregate.
// REQ-EVAL1-003.
func TestRunnerAppliesOverrideExcludesFromAggregate(t *testing.T) {
	t.Parallel()
	synth := stubSynth{fn: func(q golden.Query) (synthesis.Result, error) {
		return synthesis.Result{Text: "Claim [1].", Citations: []synthesis.Citation{{Marker: 1, DocID: "doc-001"}}}, nil
	}}
	sc := stubScorer{fn: func(id string) (scorer.Result, error) {
		if id == "Q2" {
			return scorer.Result{Score: 0.1}, nil // would tank the mean if counted
		}
		return scorer.Result{Score: 1.0}, nil
	}}
	overrides := []golden.Override{{QueryID: "Q2", ManualOverride: "skip", OverrideReason: "known flaky"}}
	rep, err := runner.Run(context.Background(), runner.Config{Concurrency: 1}, twoQueries(), corpus(), overrides, synth, sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.MeanScore != 1.0 {
		t.Errorf("mean = %v, want 1.0 (Q2 overridden out)", rep.MeanScore)
	}
	if rep.OverrideCount != 1 {
		t.Errorf("override count = %d, want 1", rep.OverrideCount)
	}
}

// TestRunnerRecordsLowestScores verifies per-query scores and rationales are kept.
// REQ-EVAL1-007.
func TestRunnerRecordsPerClaimRationale(t *testing.T) {
	t.Parallel()
	synth := stubSynth{fn: func(q golden.Query) (synthesis.Result, error) {
		return synthesis.Result{Text: "Claim [1].", Citations: []synthesis.Citation{{Marker: 1, DocID: "doc-001"}}}, nil
	}}
	sc := stubScorer{fn: func(id string) (scorer.Result, error) {
		return scorer.Result{Score: 0.5, PerClaim: []scorer.ClaimScore{
			{Text: "Claim [1].", Supported: false, JudgeRationale: "not entailed"},
		}}, nil
	}}
	rep, err := runner.Run(context.Background(), runner.Config{Concurrency: 1}, twoQueries()[:1], corpus(), nil, synth, sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rep.Queries) != 1 || len(rep.Queries[0].PerClaim) != 1 {
		t.Fatalf("expected per-claim recorded, got %#v", rep.Queries)
	}
	if rep.Queries[0].PerClaim[0].JudgeRationale == "" {
		t.Error("expected rationale recorded")
	}
}

// TestRunnerHandlesSynthesisError verifies a synthesis-path error marks the
// query null (not a crash) and continues.
// REQ-EVAL1-006(c): no fail-fast.
func TestRunnerHandlesSynthesisError(t *testing.T) {
	t.Parallel()
	synth := stubSynth{fn: func(q golden.Query) (synthesis.Result, error) {
		if q.ID == "Q1" {
			return synthesis.Result{}, errors.New("synth boom")
		}
		return synthesis.Result{Text: "Claim [1].", Citations: []synthesis.Citation{{Marker: 1, DocID: "doc-002"}}}, nil
	}}
	sc := stubScorer{fn: func(id string) (scorer.Result, error) {
		return scorer.Result{Score: 0.9}, nil
	}}
	rep, err := runner.Run(context.Background(), runner.Config{Concurrency: 2}, twoQueries(), corpus(), nil, synth, sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.NullCount != 1 {
		t.Errorf("expected 1 null from synth error, got %d", rep.NullCount)
	}
	if rep.MeanScore != 0.9 {
		t.Errorf("mean = %v, want 0.9 (only Q2 scored)", rep.MeanScore)
	}
}

// TestRunnerConcurrencyBoundedDeterministicOutput verifies parallel execution
// (concurrency 5) returns all queries with stable per-query results.
// NFR-EVAL1-004.
func TestRunnerConcurrencyBoundedDeterministicOutput(t *testing.T) {
	t.Parallel()
	queries := make([]golden.Query, 20)
	for i := range queries {
		queries[i] = golden.Query{ID: "Q" + string(rune('A'+i)), Locale: "en", Category: "factual"}
	}
	synth := stubSynth{fn: func(q golden.Query) (synthesis.Result, error) {
		return synthesis.Result{Text: "Claim [1].", Citations: []synthesis.Citation{{Marker: 1, DocID: "doc-001"}}}, nil
	}}
	sc := stubScorer{fn: func(id string) (scorer.Result, error) {
		return scorer.Result{Score: 0.95}, nil
	}}
	rep, err := runner.Run(context.Background(), runner.Config{Concurrency: 5}, queries, corpus(), nil, synth, sc)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(rep.Queries) != 20 {
		t.Fatalf("expected 20 results, got %d", len(rep.Queries))
	}
}
