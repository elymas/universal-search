// Package ci provides the SPEC-EVAL-001 release gate: a pure decision function
// mapping benchmark scores to a pass/fail verdict and CI exit code.
//
// REQ-EVAL1-008: exit 0 = pass; 1 = below threshold/floor/override cap;
// 2 = judge availability error (null scores); 3 = malformed input.
package ci

import "fmt"

// QueryScore is one query's outcome. Score is nil when the judge could not
// score the query (REQ-EVAL1-006: null, not zero).
type QueryScore struct {
	QueryID string
	Score   *float64
}

// Thresholds are the FROZEN gate thresholds (HISTORY D5).
type Thresholds struct {
	Mean        float64 // aggregate mean floor (0.85)
	Floor       float64 // per-query floor (0.50)
	OverrideCap int     // max active overrides (5)
}

// Result is the gate verdict.
type Result struct {
	Pass          bool
	ExitCode      int
	Mean          float64
	Floor         float64
	OverrideCount int
	NullCount     int
	Reason        string
}

// Exit code constants (REQ-EVAL1-008).
const (
	ExitPass       = 0
	ExitBelow      = 1 // mean/floor/override-cap failure
	ExitJudgeError = 2 // judge availability error (null scores)
	ExitMalformed  = 3 // report parse error
)

// @MX:ANCHOR: [AUTO] Release-gate invariant. Decide is the single chokepoint
// that gates SPEC-REL-001 (V1.0.0 tag). It is consumed by cmd/eval/main.go and
// the eval.yml CI workflow.
// @MX:REASON: The exit-code contract (0 pass / 1 below / 2 judge-error / 3
// malformed), the mean>=0.85 + per-query floor>=0.50 thresholds, and the
// null-excluded-from-mean rule are FROZEN at the SPEC level (HISTORY D5, D7).
// Any change here changes whether a release can ship. Null scores must force
// exit 2 even when the non-null mean clears the threshold, so a degraded judge
// can never be mistaken for a passing benchmark.
// @MX:SPEC: SPEC-EVAL-001 REQ-EVAL1-008

// Decide evaluates scores against the thresholds and returns the gate verdict.
// Pure function — no I/O — so it is fully unit-testable.
func Decide(scores []QueryScore, activeOverrides []string, th Thresholds) Result {
	res := Result{Floor: th.Floor, OverrideCount: len(activeOverrides)}

	var sum float64
	var counted int
	floorViolation := false
	for _, s := range scores {
		if s.Score == nil {
			res.NullCount++
			continue
		}
		sum += *s.Score
		counted++
		if *s.Score < th.Floor {
			floorViolation = true
		}
	}
	if counted > 0 {
		res.Mean = sum / float64(counted)
	}

	// (d) Override cap check first — a config error fails fast.
	if len(activeOverrides) > th.OverrideCap {
		res.ExitCode = ExitBelow
		res.Reason = fmt.Sprintf("override cap exceeded: %d active, cap %d", len(activeOverrides), th.OverrideCap)
		return res
	}

	// (c) Judge availability — null scores force exit 2, regardless of mean.
	if res.NullCount > 0 {
		res.ExitCode = ExitJudgeError
		res.Reason = fmt.Sprintf("judge availability error: %d null scores", res.NullCount)
		return res
	}

	// (b) Per-query floor.
	if floorViolation {
		res.ExitCode = ExitBelow
		res.Reason = fmt.Sprintf("floor violation: a query scored below %.2f", th.Floor)
		return res
	}

	// (a) Aggregate mean.
	if res.Mean < th.Mean {
		res.ExitCode = ExitBelow
		res.Reason = fmt.Sprintf("mean %.3f below threshold %.2f", res.Mean, th.Mean)
		return res
	}

	res.Pass = true
	res.ExitCode = ExitPass
	res.Reason = "all thresholds met"
	return res
}

// SummaryLine renders the grep-friendly one-line CI summary.
// REQ-EVAL1-008: EVAL-001 result=PASS|FAIL mean=<X.XXX> floor=<X.XX> overrides=<N> nulls=<N>
func SummaryLine(res Result) string {
	verdict := "FAIL"
	if res.Pass {
		verdict = "PASS"
	}
	return fmt.Sprintf("EVAL-001 result=%s mean=%.3f floor=%.2f overrides=%d nulls=%d",
		verdict, res.Mean, res.Floor, res.OverrideCount, res.NullCount)
}
