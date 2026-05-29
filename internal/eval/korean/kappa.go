package korean

import (
	"fmt"
	"math"
)

// CohenKappa computes Cohen's kappa coefficient for inter-rater agreement
// on ordinal rankings (1-5 scale).
// REQ-EVAL-004: Round validity gate uses Light's mean-kappa >= 0.6.
//
// @MX:ANCHOR: [AUTO] Round validity decision function; consumers: snapshot writer, calibration protocol
// @MX:REASON: fan_in >= 3; REQ-EVAL-004 and REQ-EVAL-009 both consume this
func CohenKappa(r1, r2 []int) (float64, error) {
	if len(r1) == 0 || len(r2) == 0 {
		return 0, fmt.Errorf("empty rater data")
	}
	if len(r1) != len(r2) {
		return 0, fmt.Errorf("rater length mismatch: %d vs %d", len(r1), len(r2))
	}

	n := len(r1)

	// Find the rating scale range.
	minVal, maxVal := r1[0], r1[0]
	for _, v := range r1 {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	for _, v := range r2 {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	categories := maxVal - minVal + 1

	// Build confusion matrix.
	matrix := make([][]int, categories)
	for i := range matrix {
		matrix[i] = make([]int, categories)
	}
	for i := range n {
		matrix[r1[i]-minVal][r2[i]-minVal]++
	}

	// Observed agreement (Po).
	po := 0.0
	for i := range categories {
		po += float64(matrix[i][i]) / float64(n)
	}

	// Expected agreement (Pe) by chance.
	pe := 0.0
	for i := range categories {
		rowSum := 0
		colSum := 0
		for j := range categories {
			rowSum += matrix[i][j]
			colSum += matrix[j][i]
		}
		pe += float64(rowSum) * float64(colSum) / float64(n*n)
	}

	if math.Abs(pe-1.0) < 1e-10 {
		// Perfect expected agreement means both raters use only one category.
		// If observed is also perfect, kappa is 1.0; otherwise undefined (return 0).
		if math.Abs(po-1.0) < 1e-10 {
			return 1.0, nil
		}
		return 0.0, nil
	}

	kappa := (po - pe) / (1.0 - pe)
	return kappa, nil
}

// LightMeanKappa computes Light's mean kappa across all pairwise rater combinations.
// REQ-EVAL-004: Round is valid iff mean-kappa >= 0.6 (substantial agreement).
//
// @MX:ANCHOR: [AUTO] Round-level validity aggregator; consumers: snapshot, calibration, CI reporting
// @MX:REASON: fan_in >= 3; single entry point for round validity determination
func LightMeanKappa(sheets [][]int) (float64, error) {
	if len(sheets) < 2 {
		return 0, fmt.Errorf("need at least 2 raters, got %d", len(sheets))
	}

	// Validate all sheets have the same length.
	length := len(sheets[0])
	for i, s := range sheets {
		if len(s) != length {
			return 0, fmt.Errorf("sheet %d length mismatch: %d vs %d", i, len(s), length)
		}
		if len(s) == 0 {
			return 0, fmt.Errorf("sheet %d is empty", i)
		}
	}

	// Compute pairwise kappa for all unique pairs.
	var sum float64
	var count int
	for i := range sheets {
		for j := i + 1; j < len(sheets); j++ {
			k, err := CohenKappa(sheets[i], sheets[j])
			if err != nil {
				return 0, fmt.Errorf("pair (%d,%d): %w", i, j, err)
			}
			sum += k
			count++
		}
	}

	return sum / float64(count), nil
}

// KrippendorffAlphaOrdinal computes Krippendorff's alpha for ordinal data
// as a supplementary reliability metric.
// REQ-EVAL-004: Krippendorff alpha is auxiliary; the gate uses Light's mean-kappa.
func KrippendorffAlphaOrdinal(sheets [][]int) (float64, error) {
	if len(sheets) < 2 {
		return 0, fmt.Errorf("need at least 2 raters, got %d", len(sheets))
	}

	nItems := len(sheets[0])
	for i, s := range sheets {
		if len(s) != nItems {
			return 0, fmt.Errorf("sheet %d length mismatch", i)
		}
	}

	nRaters := len(sheets)

	// Compute observed disagreement (D_o).
	// For ordinal data, the difference metric is (c1 - c2)^2.
	var dObs float64
	pairCount := 0
	for item := range nItems {
		for i := range nRaters {
			for j := i + 1; j < nRaters; j++ {
				diff := float64(sheets[i][item] - sheets[j][item])
				dObs += diff * diff
				pairCount++
			}
		}
	}
	if pairCount == 0 {
		return 1.0, nil
	}
	dObs /= float64(pairCount)

	// Compute expected disagreement (D_e).
	// Pool all values and compute pairwise squared differences.
	var allValues []int
	for _, s := range sheets {
		allValues = append(allValues, s...)
	}
	totalPairs := len(allValues) * (len(allValues) - 1) / 2
	if totalPairs == 0 {
		return 1.0, nil
	}
	var dExp float64
	for i := range allValues {
		for j := i + 1; j < len(allValues); j++ {
			diff := float64(allValues[i] - allValues[j])
			dExp += diff * diff
		}
	}
	dExp /= float64(totalPairs)

	if dExp < 1e-10 {
		return 1.0, nil
	}

	alpha := 1.0 - dObs/dExp
	return alpha, nil
}
