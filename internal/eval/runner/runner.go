// Package runner orchestrates the SPEC-EVAL-001 benchmark: for each golden
// query it drives the synthesis path, ships the result to the DeepEval judge
// bridge, collects per-claim scores, applies manual overrides, and aggregates
// a mean across non-null scores.
//
// REQ-EVAL1-006: null-score handling (judge unavailable → null, not zero).
// REQ-EVAL1-003: override apply + exclusion from aggregate.
// NFR-EVAL1-004: bounded concurrency (default 5).
package runner

import (
	"context"
	"sort"
	"sync"

	"github.com/elymas/universal-search/internal/eval/golden"
	"github.com/elymas/universal-search/internal/eval/scorer"
	"github.com/elymas/universal-search/internal/synthesis"
)

// Synthesizer drives the synthesis path for one golden query against the
// frozen corpus. The runner stubs this in tests; CI wires the real client.
type Synthesizer interface {
	Synthesize(ctx context.Context, q golden.Query, corpus map[string]string) (synthesis.Result, error)
}

// Scorer scores a synthesized result against the judge (the DeepEval bridge).
type Scorer interface {
	Score(ctx context.Context, queryID, locale string, result synthesis.Result, corpus map[string]string) (scorer.Result, error)
}

// Config tunes the run.
type Config struct {
	// Concurrency bounds parallel query execution (NFR-EVAL1-004: max 5).
	Concurrency int
}

// QueryResult is the per-query outcome.
type QueryResult struct {
	QueryID  string
	Category string
	Locale   string
	// Score is nil when the judge could not score the query (REQ-EVAL1-006).
	Score      *float64
	PerClaim   []scorer.ClaimScore
	Overridden bool
	ErrorClass string // "", "judge_unavailable", "synth_error"
}

// Report is the aggregate benchmark result handed to the gate + report writer.
type Report struct {
	Queries       []QueryResult
	MeanScore     float64
	NullCount     int
	OverrideCount int
}

// HasJudgeError reports whether any query was marked null due to a judge
// availability error (drives exit code 2).
// REQ-EVAL1-006(d).
func (r *Report) HasJudgeError() bool {
	for _, q := range r.Queries {
		if q.ErrorClass == "judge_unavailable" {
			return true
		}
	}
	return false
}

// @MX:ANCHOR: [AUTO] Benchmark orchestration entrypoint; consumed by the CI
// gate and report writer. The null-vs-zero distinction and mean-over-non-null
// contract are release-gate invariants.
// @MX:REASON: gate.Decide + report.Write both depend on Report.MeanScore,
// NullCount, and per-query Score==nil semantics; changing them shifts the
// M8 release gate behaviour.
// @MX:SPEC: SPEC-EVAL-001 REQ-EVAL1-006

// Run executes the benchmark over the given queries with bounded concurrency.
// Overrides marked "skip" or "pass" exclude their query from the aggregate.
func Run(ctx context.Context, cfg Config, queries []golden.Query, corpus map[string]string, overrides []golden.Override, synth Synthesizer, sc Scorer) (Report, error) {
	conc := cfg.Concurrency
	if conc <= 0 {
		conc = 5
	}

	overridden := make(map[string]bool, len(overrides))
	for _, o := range overrides {
		overridden[o.QueryID] = true
	}

	results := make([]QueryResult, len(queries))
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup

	for i, q := range queries {
		// @MX:WARN: [AUTO] Bounded worker fan-out over the judge bridge.
		// @MX:REASON: each goroutine performs a network call; the semaphore
		// caps concurrency at cfg.Concurrency to respect judge rate limits.
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, q golden.Query) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = scoreOne(ctx, q, corpus, overridden[q.ID], synth, sc)
		}(i, q)
	}
	wg.Wait()

	return aggregate(results), nil
}

func scoreOne(ctx context.Context, q golden.Query, corpus map[string]string, overridden bool, synth Synthesizer, sc Scorer) QueryResult {
	qr := QueryResult{QueryID: q.ID, Category: q.Category, Locale: q.Locale, Overridden: overridden}

	res, err := synth.Synthesize(ctx, q, corpus)
	if err != nil {
		qr.ErrorClass = "synth_error"
		return qr // Score stays nil → null.
	}

	score, err := sc.Score(ctx, q.ID, q.Locale, res, corpus)
	if err != nil {
		if scorer.IsUnavailable(err) {
			qr.ErrorClass = "judge_unavailable"
		} else {
			qr.ErrorClass = "score_error"
		}
		return qr // Score stays nil → null.
	}

	s := score.Score
	qr.Score = &s
	qr.PerClaim = score.PerClaim
	return qr
}

func aggregate(results []QueryResult) Report {
	rep := Report{Queries: results}
	var sum float64
	var counted int
	for _, qr := range results {
		if qr.Overridden {
			rep.OverrideCount++
			continue
		}
		if qr.Score == nil {
			rep.NullCount++
			continue
		}
		sum += *qr.Score
		counted++
	}
	if counted > 0 {
		rep.MeanScore = sum / float64(counted)
	}
	// Stable ordering for deterministic reports.
	sort.SliceStable(rep.Queries, func(i, j int) bool {
		return rep.Queries[i].QueryID < rep.Queries[j].QueryID
	})
	return rep
}
