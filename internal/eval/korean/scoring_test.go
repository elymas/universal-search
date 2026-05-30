package korean

import (
	"math"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

func doc(sourceID string, dt types.DocType) types.NormalizedDoc {
	return types.NormalizedDoc{SourceID: sourceID, DocType: dt}
}

func gq(id string, cat Category, naverRel bool, vertical string) GoldenQuery {
	return GoldenQuery{
		QueryID:               id,
		Category:              cat,
		ExpectedNaverRelevant: naverRel,
		ExpectedNaverVertical: vertical,
		ExpectedSources:       []string{"naver"},
	}
}

func TestTopThreeNaverRecall_KnownFixture(t *testing.T) {
	t.Parallel()
	// 5 Naver-relevant queries; 4 have a naver hit in top-3 → recall 0.80.
	pairs := []QueryResults{
		{Query: gq("KR-001", CategoryNews, true, ""), Results: []types.NormalizedDoc{
			doc("naver", types.DocTypeArticle), doc("arxiv", types.DocTypePaper),
		}},
		{Query: gq("KR-002", CategoryBlog, true, ""), Results: []types.NormalizedDoc{
			doc("github", types.DocTypeRepo), doc("naver", types.DocTypePost),
		}},
		{Query: gq("KR-003", CategoryShopping, true, ""), Results: []types.NormalizedDoc{
			doc("hackernews", types.DocTypeArticle), doc("naver", types.DocTypeOther),
		}},
		{Query: gq("KR-004", CategoryCultural, true, ""), Results: []types.NormalizedDoc{
			doc("koreanews", types.DocTypeArticle), doc("naver", types.DocTypeOther),
		}},
		{Query: gq("KR-005", CategoryNews, true, ""), Results: []types.NormalizedDoc{
			// No naver in top-3 → miss.
			doc("github", types.DocTypeRepo), doc("arxiv", types.DocTypePaper),
			doc("hackernews", types.DocTypeArticle), doc("naver", types.DocTypeArticle),
		}},
	}
	rep := TopThreeNaverRecall(pairs)
	if math.Abs(rep.Aggregate-0.80) > 1e-9 {
		t.Errorf("aggregate recall = %v, want 0.80", rep.Aggregate)
	}
	if !rep.PassesM3Gate() {
		t.Errorf("recall 0.80 should pass M3 gate")
	}
	if rep.EligibleCount != 5 {
		t.Errorf("eligible = %d, want 5", rep.EligibleCount)
	}
}

func TestTopThreeNaverRecall_ExcludesNonRelevant(t *testing.T) {
	t.Parallel()
	pairs := []QueryResults{
		// academic-tech: not naver-relevant → must NOT contribute to denominator.
		{Query: gq("KR-001", CategoryAcademicTech, false, ""), Results: []types.NormalizedDoc{
			doc("arxiv", types.DocTypePaper),
		}},
		{Query: gq("KR-002", CategoryNews, true, ""), Results: []types.NormalizedDoc{
			doc("naver", types.DocTypeArticle),
		}},
	}
	rep := TopThreeNaverRecall(pairs)
	if rep.EligibleCount != 1 {
		t.Fatalf("eligible = %d, want 1 (academic-tech excluded)", rep.EligibleCount)
	}
	if math.Abs(rep.Aggregate-1.0) > 1e-9 {
		t.Errorf("aggregate = %v, want 1.0", rep.Aggregate)
	}
}

func TestTopThreeNaverRecall_VerticalMatch(t *testing.T) {
	t.Parallel()
	// Query expects blog vertical. DocTypePost→blog (unambiguous) matches.
	hit := QueryResults{
		Query:   gq("KR-001", CategoryBlog, true, "blog"),
		Results: []types.NormalizedDoc{doc("naver", types.DocTypePost)},
	}
	// Query expects news vertical but result is DocTypePost→blog → mismatch.
	miss := QueryResults{
		Query:   gq("KR-002", CategoryNews, true, "news"),
		Results: []types.NormalizedDoc{doc("naver", types.DocTypePost)},
	}
	rep := TopThreeNaverRecall([]QueryResults{hit, miss})
	if math.Abs(rep.Aggregate-0.5) > 1e-9 {
		t.Errorf("aggregate = %v, want 0.5 (one vertical match, one mismatch)", rep.Aggregate)
	}
}

func TestTopThreeNaverRecall_AmbiguousVerticalFallback(t *testing.T) {
	t.Parallel()
	// Query expects shop vertical; result is DocTypeOther (ambiguous) →
	// SourceID-only fallback counts as a hit (D1 unverified).
	pairs := []QueryResults{{
		Query:   gq("KR-001", CategoryShopping, true, "shop"),
		Results: []types.NormalizedDoc{doc("naver", types.DocTypeOther)},
	}}
	rep := TopThreeNaverRecall(pairs)
	if math.Abs(rep.Aggregate-1.0) > 1e-9 {
		t.Errorf("ambiguous DocTypeOther should fall back to SourceID-only hit; got %v", rep.Aggregate)
	}
}

func TestTopThreeNaverRecall_PerCategory(t *testing.T) {
	t.Parallel()
	pairs := []QueryResults{
		{Query: gq("KR-001", CategoryNews, true, ""), Results: []types.NormalizedDoc{doc("naver", types.DocTypeArticle)}},
		{Query: gq("KR-002", CategoryNews, true, ""), Results: []types.NormalizedDoc{doc("arxiv", types.DocTypePaper)}},
		{Query: gq("KR-003", CategoryBlog, true, ""), Results: []types.NormalizedDoc{doc("naver", types.DocTypePost)}},
	}
	rep := TopThreeNaverRecall(pairs)
	if math.Abs(rep.PerCategory[CategoryNews]-0.5) > 1e-9 {
		t.Errorf("news per-category = %v, want 0.5", rep.PerCategory[CategoryNews])
	}
	if math.Abs(rep.PerCategory[CategoryBlog]-1.0) > 1e-9 {
		t.Errorf("blog per-category = %v, want 1.0", rep.PerCategory[CategoryBlog])
	}
	if rep.PerCategoryDenominator[CategoryNews] != 2 {
		t.Errorf("news denom = %d, want 2", rep.PerCategoryDenominator[CategoryNews])
	}
}

func TestTopThreeNaverRecall_MRR(t *testing.T) {
	t.Parallel()
	// naver at rank 1 → RR 1.0; naver at rank 2 → RR 0.5. Mean = 0.75.
	pairs := []QueryResults{
		{Query: gq("KR-001", CategoryNews, true, ""), Results: []types.NormalizedDoc{
			doc("naver", types.DocTypeArticle), doc("arxiv", types.DocTypePaper),
		}},
		{Query: gq("KR-002", CategoryNews, true, ""), Results: []types.NormalizedDoc{
			doc("arxiv", types.DocTypePaper), doc("naver", types.DocTypeArticle),
		}},
	}
	rep := TopThreeNaverRecall(pairs)
	if math.Abs(rep.MRRTop10-0.75) > 1e-9 {
		t.Errorf("MRR@10 = %v, want 0.75", rep.MRRTop10)
	}
}

func TestTopThreeNaverRecall_BelowGate(t *testing.T) {
	t.Parallel()
	// 0 of 2 hits → 0.0 recall, gate fails.
	pairs := []QueryResults{
		{Query: gq("KR-001", CategoryNews, true, ""), Results: []types.NormalizedDoc{doc("arxiv", types.DocTypePaper)}},
		{Query: gq("KR-002", CategoryNews, true, ""), Results: []types.NormalizedDoc{doc("github", types.DocTypeRepo)}},
	}
	rep := TopThreeNaverRecall(pairs)
	if rep.PassesM3Gate() {
		t.Errorf("0.0 recall should fail M3 gate")
	}
}

func TestTopThreeNaverRecall_EmptyInput(t *testing.T) {
	t.Parallel()
	rep := TopThreeNaverRecall(nil)
	if rep.Aggregate != 0 || rep.EligibleCount != 0 {
		t.Errorf("empty input: got aggregate=%v eligible=%d, want 0/0", rep.Aggregate, rep.EligibleCount)
	}
}
