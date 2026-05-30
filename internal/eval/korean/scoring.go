package korean

import (
	"sort"

	"github.com/elymas/universal-search/pkg/types"
)

// NaverSourceID is the single registered Naver adapter identifier. Verticals
// are NOT separate adapters — they are derived from DocType + the
// naver_vertical request filter (internal/adapters/naver/naver.go:181).
const NaverSourceID = "naver"

// M3RecallGateThreshold is the top-3 Naver recall pass line for the M3-exit
// regression gate (REQ-EVAL-005 D4). At least 80% of Naver-relevant queries
// must return a naver result in the top-3.
const M3RecallGateThreshold = 0.80

// verticalForDocType maps a result DocType to the Naver vertical it implies,
// per the live mapping in REQ-EVAL-005:
//
//	DocTypePost    → blog
//	DocTypeArticle → news
//	DocTypeOther   → {web, shop, datalab}  (ambiguous → unverified)
//
// The boolean return reports whether the mapping is UNAMBIGUOUS. When false,
// the caller falls back to a SourceID-only match and records the vertical as
// "unverified" (D1 finding).
func verticalForDocType(dt types.DocType) (vertical string, unambiguous bool) {
	switch dt {
	case types.DocTypePost:
		return "blog", true
	case types.DocTypeArticle:
		return "news", true
	default:
		// DocTypeOther covers web/shop/datalab — not separable from DocType
		// alone, so the vertical is unverified.
		return "", false
	}
}

// naverHitAtK reports whether results (already in rank order, top-first)
// contain a Naver hit within the first k positions for the given query.
//
// A Naver hit requires SourceID == "naver". When the query specifies
// ExpectedNaverVertical, an unambiguous DocType→vertical mapping must match;
// when the DocType is ambiguous (DocTypeOther), the match falls back to
// SourceID-only and is treated as a hit (recorded "unverified" by callers).
func naverHitAtK(q GoldenQuery, results []types.NormalizedDoc, k int) bool {
	limit := k
	if len(results) < limit {
		limit = len(results)
	}
	for i := 0; i < limit; i++ {
		r := results[i]
		if r.SourceID != NaverSourceID {
			continue
		}
		if q.ExpectedNaverVertical == "" {
			return true
		}
		vertical, unambiguous := verticalForDocType(r.DocType)
		if !unambiguous {
			// Ambiguous DocType → SourceID-only fallback (unverified hit).
			return true
		}
		if vertical == q.ExpectedNaverVertical {
			return true
		}
	}
	return false
}

// RecallReport is the output of TopThreeNaverRecall: the aggregate recall over
// Naver-relevant queries plus a per-category breakdown and the supplementary
// MRR@10 metric.
type RecallReport struct {
	// Aggregate is recall@3 over all queries with ExpectedNaverRelevant==true.
	// It is the ONLY V1 gating metric (REQ-EVAL-005).
	Aggregate float64
	// PerCategory is recall@3 keyed by category — observational, non-gating
	// (REQ-EVAL-006). Categories with no Naver-relevant queries map to 0 with
	// a corresponding 0 denominator (see PerCategoryDenominator).
	PerCategory map[Category]float64
	// PerCategoryDenominator records the Naver-relevant query count per
	// category so callers can distinguish "0 recall" from "no eligible query".
	PerCategoryDenominator map[Category]int
	// MRRTop10 is the mean reciprocal rank of the first Naver hit within the
	// top-10, averaged over Naver-relevant queries. Supplementary; NOT a gate.
	MRRTop10 float64
	// EligibleCount is the number of ExpectedNaverRelevant queries (recall
	// denominator).
	EligibleCount int
}

// QueryResults pairs a golden-set query with the ranked result list the live
// stack produced for it (top-first).
type QueryResults struct {
	Query   GoldenQuery
	Results []types.NormalizedDoc
}

// TopThreeNaverRecall computes the top-3 Naver recall (aggregate + per
// category) and the supplementary MRR@10 over the supplied query/result
// pairs. Only queries with ExpectedNaverRelevant==true contribute to recall
// and MRR (REQ-EVAL-005). The function is pure: same input → same output.
//
// @MX:ANCHOR: [AUTO] Top-3 Naver recall is the M3-exit release gate metric.
// SPEC-REL-001 (V1 ship) is blocked until this returns Aggregate >= 0.80 on
// the curated golden set against the live M3 stack.
// @MX:REASON: This single number is the measurable implementation of the
// product.md "Korean results ranked first" SLO. A miscomputation here would
// either block a healthy release or ship a silent Korean-ranking regression —
// it is a release-gate invariant.
// @MX:SPEC: SPEC-EVAL-003
func TopThreeNaverRecall(pairs []QueryResults) RecallReport {
	perCatHit := make(map[Category]int)
	perCatDenom := make(map[Category]int)
	var hits, eligible int
	var mrrSum float64

	for _, p := range pairs {
		if !p.Query.ExpectedNaverRelevant {
			continue
		}
		eligible++
		perCatDenom[p.Query.Category]++

		if naverHitAtK(p.Query, p.Results, 3) {
			hits++
			perCatHit[p.Query.Category]++
		}
		mrrSum += reciprocalRankNaver(p.Query, p.Results, 10)
	}

	report := RecallReport{
		PerCategory:            make(map[Category]float64),
		PerCategoryDenominator: perCatDenom,
		EligibleCount:          eligible,
	}
	if eligible > 0 {
		report.Aggregate = float64(hits) / float64(eligible)
		report.MRRTop10 = mrrSum / float64(eligible)
	}
	for cat, denom := range perCatDenom {
		if denom > 0 {
			report.PerCategory[cat] = float64(perCatHit[cat]) / float64(denom)
		}
	}
	return report
}

// reciprocalRankNaver returns 1/rank of the first Naver hit within the top-k
// (rank is 1-based), or 0.0 if no Naver hit appears in the top-k.
func reciprocalRankNaver(q GoldenQuery, results []types.NormalizedDoc, k int) float64 {
	limit := k
	if len(results) < limit {
		limit = len(results)
	}
	for i := 0; i < limit; i++ {
		r := results[i]
		if r.SourceID != NaverSourceID {
			continue
		}
		if q.ExpectedNaverVertical == "" {
			return 1.0 / float64(i+1)
		}
		vertical, unambiguous := verticalForDocType(r.DocType)
		if !unambiguous || vertical == q.ExpectedNaverVertical {
			return 1.0 / float64(i+1)
		}
	}
	return 0.0
}

// PassesM3Gate reports whether the aggregate recall meets the M3-exit
// threshold (REQ-EVAL-005).
func (r RecallReport) PassesM3Gate() bool {
	return r.Aggregate >= M3RecallGateThreshold
}

// OrderedCategories returns the snapshot category order used for deterministic
// per-category serialization.
func OrderedCategories() []Category {
	cats := []Category{
		CategoryNews, CategoryBlog, CategoryShopping,
		CategoryAcademicTech, CategoryCodeMixed, CategoryCultural,
	}
	sort.Slice(cats, func(i, j int) bool { return cats[i] < cats[j] })
	return cats
}
