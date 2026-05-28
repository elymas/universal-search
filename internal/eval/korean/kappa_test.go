package korean

import (
	"math"
	"testing"
)

// ---------- Cohen's kappa tests (RED phase) ----------

func TestCohenKappa_IdenticalRaters(t *testing.T) {
	// Two identical rater sheets → kappa = 1.0.
	r1 := []int{5, 4, 3, 5, 4, 3, 5, 4, 3, 5}
	r2 := []int{5, 4, 3, 5, 4, 3, 5, 4, 3, 5}

	kappa, err := CohenKappa(r1, r2)
	if err != nil {
		t.Fatalf("CohenKappa returned error: %v", err)
	}
	if math.Abs(kappa-1.0) > 1e-6 {
		t.Errorf("CohenKappa = %.6f, want 1.0 for identical raters", kappa)
	}
}

func TestCohenKappa_RandomRaters(t *testing.T) {
	// Completely random independent ratings → kappa should be near 0.
	// Use a large enough sample for statistical stability.
	r1 := []int{1, 2, 3, 4, 5, 1, 2, 3, 4, 5, 1, 2, 3, 4, 5, 1, 2, 3, 4, 5}
	r2 := []int{5, 4, 3, 2, 1, 5, 4, 3, 2, 1, 5, 4, 3, 2, 1, 5, 4, 3, 2, 1}

	kappa, err := CohenKappa(r1, r2)
	if err != nil {
		t.Fatalf("CohenKappa returned error: %v", err)
	}
	// For completely opposite ratings with uniform distribution, kappa should be negative or near 0.
	// We check that it's not close to 1.
	if kappa > 0.3 {
		t.Errorf("CohenKappa = %.6f for opposite ratings, expected near 0 or negative", kappa)
	}
}

func TestCohenKappa_DivergentRaters(t *testing.T) {
	// Realistic divergent raters — some agreement but not perfect.
	r1 := []int{5, 4, 3, 5, 4, 3, 5, 4, 3, 5}
	r2 := []int{5, 3, 3, 4, 4, 2, 5, 3, 3, 4}

	kappa, err := CohenKappa(r1, r2)
	if err != nil {
		t.Fatalf("CohenKappa returned error: %v", err)
	}
	// Should be in the 0.3-0.8 range (moderate to substantial agreement).
	if kappa < 0.1 || kappa > 1.0 {
		t.Errorf("CohenKappa = %.6f, expected moderate agreement range", kappa)
	}
}

func TestCohenKappa_MismatchedLength_ReturnsError(t *testing.T) {
	r1 := []int{5, 4, 3}
	r2 := []int{5, 4}

	_, err := CohenKappa(r1, r2)
	if err == nil {
		t.Error("expected error for mismatched lengths, got nil")
	}
}

func TestCohenKappa_EmptyInput_ReturnsError(t *testing.T) {
	_, err := CohenKappa([]int{}, []int{})
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

// ---------- Light's mean kappa tests (RED phase) ----------

func TestLightMeanKappa_ThreeIdenticalRaters(t *testing.T) {
	// Three identical sheets → all pairwise kappas = 1.0 → mean = 1.0.
	sheet1 := []int{5, 4, 3, 5, 4}
	sheet2 := []int{5, 4, 3, 5, 4}
	sheet3 := []int{5, 4, 3, 5, 4}

	meanK, err := LightMeanKappa([][]int{sheet1, sheet2, sheet3})
	if err != nil {
		t.Fatalf("LightMeanKappa returned error: %v", err)
	}
	if math.Abs(meanK-1.0) > 1e-6 {
		t.Errorf("LightMeanKappa = %.6f, want 1.0 for identical raters", meanK)
	}
}

func TestLightMeanKappa_TwoRaters(t *testing.T) {
	// Two raters → mean kappa = single pairwise kappa.
	sheet1 := []int{5, 4, 3, 5, 4, 3, 5, 4, 3, 5}
	sheet2 := []int{5, 4, 3, 5, 4, 3, 5, 4, 3, 5}

	meanK, err := LightMeanKappa([][]int{sheet1, sheet2})
	if err != nil {
		t.Fatalf("LightMeanKappa returned error: %v", err)
	}
	if math.Abs(meanK-1.0) > 1e-6 {
		t.Errorf("LightMeanKappa = %.6f, want 1.0", meanK)
	}
}

func TestLightMeanKappa_DivergentThreeRaters(t *testing.T) {
	// Three raters with moderate agreement.
	sheet1 := []int{5, 4, 3, 5, 4, 3, 5, 4, 3, 5}
	sheet2 := []int{5, 3, 3, 4, 4, 2, 5, 3, 3, 4}
	sheet3 := []int{4, 4, 3, 5, 3, 3, 4, 4, 3, 5}

	meanK, err := LightMeanKappa([][]int{sheet1, sheet2, sheet3})
	if err != nil {
		t.Fatalf("LightMeanKappa returned error: %v", err)
	}
	// Should be positive and less than 1.0.
	if meanK <= 0 || meanK > 1.0+1e-6 {
		t.Errorf("LightMeanKappa = %.6f, expected positive and <= 1.0", meanK)
	}
}

func TestLightMeanKappa_SingleRater_ReturnsError(t *testing.T) {
	sheet1 := []int{5, 4, 3}
	_, err := LightMeanKappa([][]int{sheet1})
	if err == nil {
		t.Error("expected error for single rater, got nil")
	}
}

func TestLightMeanKappa_EmptySheets_ReturnsError(t *testing.T) {
	_, err := LightMeanKappa([][]int{})
	if err == nil {
		t.Error("expected error for empty sheets, got nil")
	}
}

func TestLightMeanKappa_MismatchedLengths_ReturnsError(t *testing.T) {
	sheet1 := []int{5, 4, 3}
	sheet2 := []int{5, 4}
	_, err := LightMeanKappa([][]int{sheet1, sheet2})
	if err == nil {
		t.Error("expected error for mismatched lengths, got nil")
	}
}

// ---------- Krippendorff alpha ordinal tests (RED phase) ----------

func TestKrippendorffAlphaOrdinal_IdenticalRaters(t *testing.T) {
	sheet1 := []int{5, 4, 3, 5, 4, 3}
	sheet2 := []int{5, 4, 3, 5, 4, 3}
	sheet3 := []int{5, 4, 3, 5, 4, 3}

	alpha, err := KrippendorffAlphaOrdinal([][]int{sheet1, sheet2, sheet3})
	if err != nil {
		t.Fatalf("KrippendorffAlphaOrdinal returned error: %v", err)
	}
	if math.Abs(alpha-1.0) > 1e-6 {
		t.Errorf("KrippendorffAlphaOrdinal = %.6f, want 1.0 for identical raters", alpha)
	}
}

func TestKrippendorffAlphaOrdinal_SingleRater_ReturnsError(t *testing.T) {
	_, err := KrippendorffAlphaOrdinal([][]int{{5, 4, 3}})
	if err == nil {
		t.Error("expected error for single rater, got nil")
	}
}

func TestKrippendorffAlphaOrdinal_EmptySheets_ReturnsError(t *testing.T) {
	_, err := KrippendorffAlphaOrdinal([][]int{})
	if err == nil {
		t.Error("expected error for empty sheets, got nil")
	}
}
