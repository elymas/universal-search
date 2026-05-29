package korean

// RaterScore represents a single rater's score for one query in a scoring round.
// REQ-EVAL-003: CSV sheet fields.
type RaterScore struct {
	QueryID               string  `json:"query_id"`
	RaterID               string  `json:"rater_id"`
	RankingScore          int     `json:"ranking_score"`           // 1-5
	SourceRelevance       int     `json:"source_relevance"`        // 1-5
	CodeSwitchingHandling *int    `json:"code_switching_handling"` // 1-5, nullable
	TokenizationQuality   int     `json:"tokenization_quality"`    // 1-5
	Top3NaverHit          bool    `json:"top3_naver_hit"`
	MRRTop10              float64 `json:"mrr_top10"` // [0.0, 1.0]
	Notes                 string  `json:"notes,omitempty"`
}

// Round represents a complete scoring round with multiple rater sheets.
type Round struct {
	RaterSheets [][]RaterScore
}

// ScoringResult holds computed metrics from a scoring round.
type ScoringResult struct {
	Top3NaverRecall   float64
	MRRAt10           float64
	PerCategoryRecall map[string]float64
	MeanRankingScore  float64
	QueriesEvaluated  int
}

// Top3NaverRecall computes the top-3 recall@k for Naver-suite adapters over
// the subset of golden queries where expected_naver_relevant is true.
// REQ-EVAL-005: Pass threshold is 0.80 for M3-exit gate.
//
// @MX:ANCHOR: [AUTO] M3 exit gate evidence generator; consumers: snapshot, CI gate, SPEC-REL-001
// @MX:REASON: fan_in >= 3; core ranking metric for Korean-first identity
func Top3NaverRecall(round Round, gold []GoldenQuery) float64 {
	if len(round.RaterSheets) == 0 || len(round.RaterSheets[0]) == 0 {
		return 0.0
	}

	// Build a lookup from the first rater sheet.
	hits := make(map[string]bool)
	for _, score := range round.RaterSheets[0] {
		hits[score.QueryID] = score.Top3NaverHit
	}

	var relevantCount int
	var hitCount int
	for _, q := range gold {
		if !q.ExpectedNaverRelevant {
			continue
		}
		relevantCount++
		if hits[q.QueryID] {
			hitCount++
		}
	}

	if relevantCount == 0 {
		return 0.0
	}
	return float64(hitCount) / float64(relevantCount)
}

// MRRAt10 computes the Mean Reciprocal Rank at 10 over all golden queries.
// REQ-EVAL-005: Supplementary metric, does NOT gate.
func MRRAt10(round Round, gold []GoldenQuery) float64 {
	if len(round.RaterSheets) == 0 || len(round.RaterSheets[0]) == 0 {
		return 0.0
	}

	// Build lookup from the first rater sheet.
	mrrValues := make(map[string]float64)
	for _, score := range round.RaterSheets[0] {
		mrrValues[score.QueryID] = score.MRRTop10
	}

	if len(gold) == 0 {
		return 0.0
	}

	var sum float64
	for _, q := range gold {
		sum += mrrValues[q.QueryID]
	}
	return sum / float64(len(gold))
}

// PerCategoryRecall computes top-3 Naver recall broken down by each category bucket.
// REQ-EVAL-006: Per-category recall surfaced in baseline snapshot JSON.
func PerCategoryRecall(round Round, gold []GoldenQuery) map[string]float64 {
	result := make(map[string]float64)

	if len(round.RaterSheets) == 0 || len(round.RaterSheets[0]) == 0 {
		return result
	}

	// Build lookup from first rater sheet.
	hits := make(map[string]bool)
	for _, score := range round.RaterSheets[0] {
		hits[score.QueryID] = score.Top3NaverHit
	}

	// Count per category.
	type catCount struct {
		relevant int
		hits     int
	}
	counts := make(map[string]*catCount)
	for _, q := range gold {
		if _, ok := counts[q.Category]; !ok {
			counts[q.Category] = &catCount{}
		}
		if !q.ExpectedNaverRelevant {
			continue
		}
		counts[q.Category].relevant++
		if hits[q.QueryID] {
			counts[q.Category].hits++
		}
	}

	for cat, c := range counts {
		if c.relevant == 0 {
			result[cat] = 0.0
		} else {
			result[cat] = float64(c.hits) / float64(c.relevant)
		}
	}

	return result
}
