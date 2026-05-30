package korean

import (
	"errors"
	"math"
	"testing"
)

func TestCohenKappa_Identical(t *testing.T) {
	t.Parallel()
	a := []int{1, 2, 3, 4, 5, 1, 2, 3}
	k, err := CohenKappa(a, a)
	if err != nil {
		t.Fatalf("CohenKappa: %v", err)
	}
	if math.Abs(k-1.0) > 1e-9 {
		t.Errorf("identical scores: κ = %v, want 1.0", k)
	}
}

func TestCohenKappa_ConstantIdentical(t *testing.T) {
	t.Parallel()
	// Both raters give all 3s → pe == 1, perfect agreement → κ = 1.0.
	a := []int{3, 3, 3, 3}
	k, err := CohenKappa(a, a)
	if err != nil {
		t.Fatalf("CohenKappa: %v", err)
	}
	if math.Abs(k-1.0) > 1e-9 {
		t.Errorf("constant identical: κ = %v, want 1.0", k)
	}
}

func TestCohenKappa_NearZero(t *testing.T) {
	t.Parallel()
	// Constructed so observed agreement ≈ chance agreement → κ ≈ 0.
	// Rater A: 1,2,1,2,... Rater B: 2,1,2,1,... no positional agreement, but
	// balanced marginals make pe ≈ 0.5; po = 0 → κ ≈ -1. Use a milder mix.
	a := []int{1, 1, 2, 2, 1, 1, 2, 2}
	b := []int{1, 2, 2, 1, 2, 1, 1, 2}
	k, err := CohenKappa(a, b)
	if err != nil {
		t.Fatalf("CohenKappa: %v", err)
	}
	if math.Abs(k) > 0.35 {
		t.Errorf("near-random scores: κ = %v, want within ±0.35 of 0", k)
	}
}

func TestCohenKappa_LengthMismatch(t *testing.T) {
	t.Parallel()
	_, err := CohenKappa([]int{1, 2}, []int{1})
	if !errors.Is(err, ErrRaterLengthMismatch) {
		t.Errorf("want ErrRaterLengthMismatch, got %v", err)
	}
}

func TestCohenKappa_Empty(t *testing.T) {
	t.Parallel()
	_, err := CohenKappa(nil, nil)
	if !errors.Is(err, ErrEmptyScores) {
		t.Errorf("want ErrEmptyScores, got %v", err)
	}
}

func TestMeanKappa_ThreeIdentical_Valid(t *testing.T) {
	t.Parallel()
	scores := []int{1, 2, 3, 4, 5, 1, 2, 3, 4, 5}
	raters := []RaterScores{
		{RaterID: "R1", Scores: scores},
		{RaterID: "R2", Scores: scores},
		{RaterID: "R3", Scores: scores},
	}
	res, err := MeanKappa(raters)
	if err != nil {
		t.Fatalf("MeanKappa: %v", err)
	}
	if math.Abs(res.MeanKappa-1.0) > 1e-9 {
		t.Errorf("mean-κ = %v, want 1.0", res.MeanKappa)
	}
	if !res.Valid {
		t.Errorf("identical raters should mark round valid")
	}
	if len(res.Pairwise) != 3 {
		t.Errorf("3 raters → 3 pairwise κ; got %d", len(res.Pairwise))
	}
}

func TestMeanKappa_Divergent_Invalid(t *testing.T) {
	t.Parallel()
	// Realistic divergent raters → mean-κ in the 0.3–0.5 band → invalid.
	raters := []RaterScores{
		{RaterID: "R1", Scores: []int{5, 4, 5, 3, 4, 5, 2, 4, 5, 3}},
		{RaterID: "R2", Scores: []int{4, 4, 5, 4, 3, 5, 3, 4, 4, 3}},
		{RaterID: "R3", Scores: []int{5, 3, 4, 3, 4, 4, 2, 5, 5, 2}},
	}
	res, err := MeanKappa(raters)
	if err != nil {
		t.Fatalf("MeanKappa: %v", err)
	}
	if res.MeanKappa >= KappaGateThreshold {
		t.Errorf("divergent raters: mean-κ = %v, expected < 0.6 (invalid)", res.MeanKappa)
	}
	if res.Valid {
		t.Errorf("divergent round should be invalid")
	}
}

func TestMeanKappa_TooFewRaters(t *testing.T) {
	t.Parallel()
	raters := []RaterScores{
		{RaterID: "R1", Scores: []int{1, 2, 3}},
		{RaterID: "R2", Scores: []int{1, 2, 3}},
	}
	_, err := MeanKappa(raters)
	if !errors.Is(err, ErrTooFewRaters) {
		t.Errorf("2 raters should error ErrTooFewRaters, got %v", err)
	}
}

func TestMeanKappa_SingleRaterRejected(t *testing.T) {
	t.Parallel()
	// EC-001: single-rater κ-gate bypass attempt.
	_, err := MeanKappa([]RaterScores{{RaterID: "R1", Scores: []int{1, 2, 3}}})
	if !errors.Is(err, ErrTooFewRaters) {
		t.Errorf("single rater should be rejected, got %v", err)
	}
}

func TestMeanKappa_LengthMismatch(t *testing.T) {
	t.Parallel()
	raters := []RaterScores{
		{RaterID: "R1", Scores: []int{1, 2, 3}},
		{RaterID: "R2", Scores: []int{1, 2}},
		{RaterID: "R3", Scores: []int{1, 2, 3}},
	}
	_, err := MeanKappa(raters)
	if !errors.Is(err, ErrRaterLengthMismatch) {
		t.Errorf("want ErrRaterLengthMismatch, got %v", err)
	}
}
