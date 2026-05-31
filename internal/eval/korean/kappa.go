package korean

import (
	"errors"
	"fmt"
	"sort"
)

// KappaGateThreshold is the Light's mean-κ pass line for a scoring round to be
// marked valid (REQ-EVAL-004, Landis-Koch 1977 "substantial agreement").
const KappaGateThreshold = 0.6

// MinRaters is the minimum independent rater count required to compute Light's
// mean-κ (REQ-EVAL-004, EC-001). Below this the round is rejected outright.
const MinRaters = 3

// ErrTooFewRaters is returned when fewer than MinRaters rater score vectors are
// supplied (EC-001 single-rater bypass guard).
var ErrTooFewRaters = errors.New("korean eval: fewer than 3 raters; Light's mean-κ requires at least 3 independent rater sheets")

// ErrRaterLengthMismatch is returned when rater score vectors differ in length
// (they must cover the same golden-set queries in the same order).
var ErrRaterLengthMismatch = errors.New("korean eval: rater score vectors have mismatched lengths")

// ErrEmptyScores is returned when rater vectors contain no items.
var ErrEmptyScores = errors.New("korean eval: rater score vectors are empty")

// RaterScores is one rater's ordinal relevance scores over a fixed,
// position-aligned set of golden-set queries. Each score is the 1–5
// ranking_score for the corresponding query.
type RaterScores struct {
	RaterID string
	Scores  []int
}

// KappaResult is the output of MeanKappa: the per-pair Cohen κ values plus the
// Light's mean-κ aggregate and the round validity verdict.
type KappaResult struct {
	// Pairwise holds one Cohen κ per unordered rater pair, in stable
	// (i<j) order.
	Pairwise []PairKappa
	// MeanKappa is Light's mean-κ: the arithmetic mean of all pairwise κ.
	MeanKappa float64
	// Valid reports whether MeanKappa >= KappaGateThreshold (REQ-EVAL-004).
	Valid bool
}

// PairKappa is a single rater-pair Cohen κ value.
type PairKappa struct {
	RaterA string
	RaterB string
	Kappa  float64
}

// CohenKappa computes Cohen's κ for two raters' ordinal scores over the same
// items (position-aligned). Returns ErrRaterLengthMismatch / ErrEmptyScores on
// malformed input.
//
// κ = (po - pe) / (1 - pe), where po is observed agreement and pe is the
// chance-expected agreement from each rater's marginal label distribution.
// When raters agree perfectly AND pe == 1 (both gave a single constant label),
// κ is defined as 1.0 (perfect agreement) rather than 0/0.
func CohenKappa(a, b []int) (float64, error) {
	if len(a) != len(b) {
		return 0, ErrRaterLengthMismatch
	}
	if len(a) == 0 {
		return 0, ErrEmptyScores
	}
	n := float64(len(a))

	var agree int
	marginA := make(map[int]int)
	marginB := make(map[int]int)
	for i := range a {
		if a[i] == b[i] {
			agree++
		}
		marginA[a[i]]++
		marginB[b[i]]++
	}
	po := float64(agree) / n

	var pe float64
	for label, ca := range marginA {
		cb := marginB[label]
		pe += (float64(ca) / n) * (float64(cb) / n)
	}

	if pe == 1.0 {
		// Both raters used a single constant label. If they agree on it,
		// κ = 1.0; otherwise po would be < 1 and pe < 1, so this branch only
		// hits on identical constant vectors.
		if po == 1.0 {
			return 1.0, nil
		}
	}
	return (po - pe) / (1.0 - pe), nil
}

// MeanKappa computes Light's mean-κ across all unordered rater pairs. It
// requires at least MinRaters (3) raters and position-aligned, equal-length
// score vectors. Returns ErrTooFewRaters / ErrRaterLengthMismatch on bad input.
//
// @MX:ANCHOR: [AUTO] Inter-rater agreement gate. A scoring round is marked
// valid IFF MeanKappa >= 0.6; only valid rounds may produce a baseline
// snapshot (REQ-EVAL-004, REQ-EVAL-009). The snapshot writer and round
// orchestration depend on this verdict.
// @MX:REASON: Releasing a baseline from a low-agreement round would enshrine
// noise as the Korean-ranking ground truth and let real regressions hide
// behind rater disagreement. The 0.6 gate + ≥3-rater requirement is a
// release-gate invariant (EC-001 single-rater bypass is rejected here).
// @MX:SPEC: SPEC-EVAL-003
func MeanKappa(raters []RaterScores) (KappaResult, error) {
	if len(raters) < MinRaters {
		return KappaResult{}, fmt.Errorf("%w: got %d", ErrTooFewRaters, len(raters))
	}
	n := len(raters[0].Scores)
	if n == 0 {
		return KappaResult{}, ErrEmptyScores
	}
	for _, r := range raters {
		if len(r.Scores) != n {
			return KappaResult{}, ErrRaterLengthMismatch
		}
	}

	var pairs []PairKappa
	var sum float64
	for i := 0; i < len(raters); i++ {
		for j := i + 1; j < len(raters); j++ {
			k, err := CohenKappa(raters[i].Scores, raters[j].Scores)
			if err != nil {
				return KappaResult{}, err
			}
			pairs = append(pairs, PairKappa{
				RaterA: raters[i].RaterID,
				RaterB: raters[j].RaterID,
				Kappa:  k,
			})
			sum += k
		}
	}

	// Stable ordering for deterministic serialization.
	sort.Slice(pairs, func(x, y int) bool {
		if pairs[x].RaterA != pairs[y].RaterA {
			return pairs[x].RaterA < pairs[y].RaterA
		}
		return pairs[x].RaterB < pairs[y].RaterB
	})

	mean := sum / float64(len(pairs))
	return KappaResult{
		Pairwise:  pairs,
		MeanKappa: mean,
		Valid:     mean >= KappaGateThreshold,
	}, nil
}
