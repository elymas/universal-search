// Package runner orchestrates the 50-query faithfulness benchmark.
//
// REQ-EVAL1-008: Runner with max 5 concurrent queries.
// REQ-EVAL1-003: Override handling with ≤5 cap.
// REQ-EVAL1-006: Judge errors produce null scores (not zero).
package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/elymas/universal-search/internal/eval/scorer"
)

// maxOverrides is the hard cap on simultaneous active overrides.
// REQ-EVAL1-003: Maximum 5 overrides allowed simultaneously.
const maxOverrides = 5

// QueryRecord represents one query from the golden set.
type QueryRecord struct {
	ID              string   `json:"id"`
	Query           string   `json:"query"`
	Locale          string   `json:"locale"`
	Category        string   `json:"category"`
	ExpectedSources []string `json:"expected_sources"`
}

// Override represents a manual override entry.
type Override struct {
	QueryID        string `json:"query_id"`
	ManualOverride string `json:"manual_override"`
	OverrideReason string `json:"override_reason"`
	ExpiresAt      string `json:"expires_at"`
	CreatedAt      string `json:"created_at"`
	CreatedBy      string `json:"created_by"`
}

// QueryResult is the per-query scoring result.
type QueryResult struct {
	QueryID       string   `json:"query_id"`
	Locale        string   `json:"locale"`
	Category      string   `json:"category"`
	Score         *float64 `json:"score"`          // nil when judge error
	Error         string   `json:"error,omitempty"` // non-empty when judge error
	Overridden    bool     `json:"overridden"`
	OverrideReason string  `json:"override_reason,omitempty"`
}

// RunReport is the aggregate benchmark report.
type RunReport struct {
	TotalQueries   int           `json:"total_queries"`
	MeanScore      float64       `json:"mean_score"`
	FloorScore     float64       `json:"floor_score"`
	OverrideCount  int           `json:"override_count"`
	NullCount      int           `json:"null_count"`
	JudgeModel     string        `json:"judge_model"`
	CorpusRevision string        `json:"corpus_revision"`
	Results        []QueryResult `json:"results"`
	RuntimeSeconds float64       `json:"runtime_seconds"`
	Timestamp      string        `json:"timestamp"`
}

// BridgeIface abstracts the scorer.Bridge for testing.
type BridgeIface interface {
	Score(ctx context.Context, queryID string, claims []scorer.Claim, corpus []scorer.CorpusEntry) (*scorer.ScoreResponse, error)
}

// Runner orchestrates the benchmark.
type Runner struct {
	bridge   BridgeIface
	maxConc  int
}

// NewRunner creates a benchmark runner with the given concurrency limit.
// maxConc must be > 0; typical value is 5 (NFR-EVAL1-004).
func NewRunner(bridge BridgeIface, maxConc int) *Runner {
	if maxConc <= 0 {
		maxConc = 5
	}
	return &Runner{bridge: bridge, maxConc: maxConc}
}

// Run executes the benchmark against all queries, applying overrides.
// Returns the aggregate RunReport. Returns error only for pre-check failures
// (e.g., override cap exceeded).
func (r *Runner) Run(ctx context.Context, queries []QueryRecord, overrides []Override) (*RunReport, error) {
	start := time.Now()

	// Pre-check: validate override cap.
	if len(overrides) > maxOverrides {
		return nil, fmt.Errorf("override cap exceeded: %d active overrides, max %d allowed", len(overrides), maxOverrides)
	}

	// Build override lookup.
	overrideMap := make(map[string]Override, len(overrides))
	for _, o := range overrides {
		overrideMap[o.QueryID] = o
	}

	// Score queries with bounded concurrency.
	results := make([]QueryResult, len(queries))
	sem := make(chan struct{}, r.maxConc)
	var wg sync.WaitGroup

	for i, q := range queries {
		wg.Add(1)
		go func(idx int, query QueryRecord) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = r.scoreQuery(ctx, query, overrideMap)
		}(i, q)
	}
	wg.Wait()

	// Compute aggregate.
	return buildReport(results, start), nil
}

// scoreQuery scores a single query, applying overrides.
func (r *Runner) scoreQuery(ctx context.Context, query QueryRecord, overrides map[string]Override) QueryResult {
	result := QueryResult{
		QueryID:  query.ID,
		Locale:   query.Locale,
		Category: query.Category,
	}

	// Check override.
	if o, ok := overrides[query.ID]; ok {
		result.Overridden = true
		result.OverrideReason = o.OverrideReason
		score := 1.0
		result.Score = &score
		return result
	}

	// Build claims from the query (simplified: one claim per query).
	claims := []scorer.Claim{
		{Text: query.Query, CitedDocIDs: query.ExpectedSources},
	}

	resp, err := r.bridge.Score(ctx, query.ID, claims, nil)
	if err != nil {
		// REQ-EVAL1-006: Judge errors → null score, NOT zero.
		result.Error = err.Error()
		return result
	}

	result.Score = &resp.FaithfulnessScore
	return result
}

// buildReport computes aggregate metrics from individual results.
func buildReport(results []QueryResult, start time.Time) *RunReport {
	report := &RunReport{
		TotalQueries: len(results),
		Results:      results,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		RuntimeSeconds: time.Since(start).Seconds(),
	}

	var sum float64
	var count int
	floor := 1.0

	for _, r := range results {
		if r.Score == nil {
			report.NullCount++
			continue
		}
		if r.Overridden {
			report.OverrideCount++
		}
		sum += *r.Score
		if *r.Score < floor {
			floor = *r.Score
		}
		count++
	}

	if count > 0 {
		report.MeanScore = sum / float64(count)
		report.FloorScore = floor
	}

	return report
}
