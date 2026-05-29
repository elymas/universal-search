// Package eval is the SPEC-EVAL-001 citation faithfulness benchmark harness.
//
// The benchmark is out-of-band: it exercises the already-shipped synthesis +
// structural-faithfulness path (SPEC-SYN-002 / SPEC-DEEP-002) read-only against
// a frozen golden set, scores each response with the DeepEval judge, and gates
// PR merges at an aggregate mean >= 0.85 (the M8 release gate).
//
// Sub-packages:
//   - golden:  loads the 50-query golden set + NormalizedDoc corpus + overrides.
//   - scorer:  Go->Python DeepEval judge bridge (locale-aware claim segmentation).
//   - runner:  orchestrates query -> synthesis -> judge -> score aggregation.
//   - ci:      pure release-gate decision (exit codes 0/1/2/3).
//
// The standalone entrypoint is cmd/eval/main.go.
package eval

import (
	"github.com/elymas/universal-search/internal/eval/ci"
	"github.com/elymas/universal-search/internal/eval/runner"
)

// ToQueryScores projects a runner.Report into the ci.QueryScore slice the gate
// consumes. Overridden queries are excluded from the gate's scoring set.
// REQ-EVAL1-003, REQ-EVAL1-008.
func ToQueryScores(rep runner.Report) []ci.QueryScore {
	out := make([]ci.QueryScore, 0, len(rep.Queries))
	for _, q := range rep.Queries {
		if q.Overridden {
			continue
		}
		out = append(out, ci.QueryScore{QueryID: q.QueryID, Score: q.Score})
	}
	return out
}
