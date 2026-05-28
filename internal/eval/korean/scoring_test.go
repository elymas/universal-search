package korean

import (
	"fmt"
	"math"
	"testing"
)

// helper: build a slice of GoldenQuery for testing.
func makeTestGoldenQueries(naverRelevantCount, total int) []GoldenQuery {
	var queries []GoldenQuery
	for i := range total {
		id := fmt.Sprintf("KR-%03d", i+1)
		naverRel := i < naverRelevantCount
		cat := "news"
		if i >= 12 {
			cat = "blog"
		}
		queries = append(queries, GoldenQuery{
			QueryID:               id,
			QueryText:             "test " + id,
			Category:              cat,
			ExpectedLang:          "ko",
			ExpectedRouterClass:   "korean",
			ExpectedNaverRelevant: naverRel,
			ExpectedSources:       []string{"naver-news"},
		})
	}
	return queries
}

// ---------- Top-3 Recall tests (RED phase) ----------

func TestTop3Recall_PerfectMatch(t *testing.T) {
	// All 10 Naver-relevant queries have top3_naver_hit = true.
	queries := makeTestGoldenQueries(10, 20)
	sheet := make([]RaterScore, 20)
	for i := range 20 {
		sheet[i] = RaterScore{
			QueryID:      fmt.Sprintf("KR-%03d", i+1),
			RaterID:      "R1",
			Top3NaverHit: i < 10, // first 10 are Naver-relevant and hit
		}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet}}

	recall := Top3NaverRecall(round, queries)
	if math.Abs(recall-1.0) > 1e-6 {
		t.Errorf("Top3NaverRecall = %.6f, want 1.0", recall)
	}
}

func TestTop3Recall_NoMatch(t *testing.T) {
	// All Naver-relevant queries miss.
	queries := makeTestGoldenQueries(10, 20)
	sheet := make([]RaterScore, 20)
	for i := range 20 {
		sheet[i] = RaterScore{
			QueryID:      fmt.Sprintf("KR-%03d", i+1),
			RaterID:      "R1",
			Top3NaverHit: false,
		}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet}}

	recall := Top3NaverRecall(round, queries)
	if math.Abs(recall-0.0) > 1e-6 {
		t.Errorf("Top3NaverRecall = %.6f, want 0.0", recall)
	}
}

func TestTop3Recall_PartialMatch(t *testing.T) {
	// 8 out of 10 Naver-relevant queries hit → recall = 0.8.
	queries := makeTestGoldenQueries(10, 20)
	sheet := make([]RaterScore, 20)
	for i := range 20 {
		sheet[i] = RaterScore{
			QueryID:      fmt.Sprintf("KR-%03d", i+1),
			RaterID:      "R1",
			Top3NaverHit: i < 8, // first 8 hit, next 2 miss
		}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet}}

	recall := Top3NaverRecall(round, queries)
	want := 0.8
	if math.Abs(recall-want) > 1e-6 {
		t.Errorf("Top3NaverRecall = %.6f, want %.6f", recall, want)
	}
}

func TestTop3Recall_NaverSubset(t *testing.T) {
	// Only Naver-relevant queries should count. Non-relevant ones are ignored.
	queries := makeTestGoldenQueries(5, 15)
	sheet := make([]RaterScore, 15)
	for i := range 15 {
		sheet[i] = RaterScore{
			QueryID:      fmt.Sprintf("KR-%03d", i+1),
			RaterID:      "R1",
			Top3NaverHit: i < 4, // 4 out of 5 Naver-relevant hit
		}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet}}

	recall := Top3NaverRecall(round, queries)
	want := 4.0 / 5.0
	if math.Abs(recall-want) > 1e-6 {
		t.Errorf("Top3NaverRecall = %.6f, want %.6f", recall, want)
	}
}

func TestTop3Recall_NoNaverRelevant_ReturnsZero(t *testing.T) {
	// If no queries are Naver-relevant, recall = 0 (avoid divide by zero).
	queries := makeTestGoldenQueries(0, 10)
	sheet := make([]RaterScore, 10)
	for i := range 10 {
		sheet[i] = RaterScore{
			QueryID:      fmt.Sprintf("KR-%03d", i+1),
			RaterID:      "R1",
			Top3NaverHit: false,
		}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet}}

	recall := Top3NaverRecall(round, queries)
	if recall != 0.0 {
		t.Errorf("Top3NaverRecall = %.6f, want 0.0 when no Naver-relevant queries", recall)
	}
}

// ---------- MRR@10 tests (RED phase) ----------

func TestMRRAt10_AllFirstPosition(t *testing.T) {
	queries := makeTestGoldenQueries(10, 10)
	sheet := make([]RaterScore, 10)
	for i := range 10 {
		sheet[i] = RaterScore{
			QueryID:  fmt.Sprintf("KR-%03d", i+1),
			RaterID:  "R1",
			MRRTop10: 1.0, // all at position 1
		}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet}}

	mrr := MRRAt10(round, queries)
	if math.Abs(mrr-1.0) > 1e-6 {
		t.Errorf("MRRAt10 = %.6f, want 1.0", mrr)
	}
}

func TestMRRAt10_FixedFixture(t *testing.T) {
	// 4 queries: MRR values 1.0, 0.5, 0.333, 0.0 → mean = (1.0+0.5+0.333+0)/4 = 0.4583
	queries := makeTestGoldenQueries(4, 4)
	sheet := []RaterScore{
		{QueryID: "KR-001", RaterID: "R1", MRRTop10: 1.0},
		{QueryID: "KR-002", RaterID: "R1", MRRTop10: 0.5},
		{QueryID: "KR-003", RaterID: "R1", MRRTop10: 1.0 / 3.0},
		{QueryID: "KR-004", RaterID: "R1", MRRTop10: 0.0},
	}
	round := Round{RaterSheets: [][]RaterScore{sheet}}

	mrr := MRRAt10(round, queries)
	want := (1.0 + 0.5 + 1.0/3.0 + 0.0) / 4.0
	if math.Abs(mrr-want) > 1e-4 {
		t.Errorf("MRRAt10 = %.6f, want %.6f", mrr, want)
	}
}

func TestMRRAt10_EmptyRound_ReturnsZero(t *testing.T) {
	queries := makeTestGoldenQueries(5, 5)
	round := Round{RaterSheets: [][]RaterScore{{}}}

	mrr := MRRAt10(round, queries)
	if mrr != 0.0 {
		t.Errorf("MRRAt10 = %.6f, want 0.0 for empty round", mrr)
	}
}

// ---------- Per-category recall tests (RED phase) ----------

func TestPerCategoryRecall(t *testing.T) {
	// 2 categories: news (3 Naver-relevant, 2 hit) and blog (2 Naver-relevant, 2 hit).
	queries := []GoldenQuery{
		{QueryID: "KR-001", Category: "news", ExpectedNaverRelevant: true},
		{QueryID: "KR-002", Category: "news", ExpectedNaverRelevant: true},
		{QueryID: "KR-003", Category: "news", ExpectedNaverRelevant: true},
		{QueryID: "KR-004", Category: "blog", ExpectedNaverRelevant: true},
		{QueryID: "KR-005", Category: "blog", ExpectedNaverRelevant: true},
	}
	sheet := []RaterScore{
		{QueryID: "KR-001", RaterID: "R1", Top3NaverHit: true},
		{QueryID: "KR-002", RaterID: "R1", Top3NaverHit: false},
		{QueryID: "KR-003", RaterID: "R1", Top3NaverHit: true},
		{QueryID: "KR-004", RaterID: "R1", Top3NaverHit: true},
		{QueryID: "KR-005", RaterID: "R1", Top3NaverHit: true},
	}
	round := Round{RaterSheets: [][]RaterScore{sheet}}

	result := PerCategoryRecall(round, queries)
	if math.Abs(result["news"]-2.0/3.0) > 1e-6 {
		t.Errorf("news recall = %.6f, want %.6f", result["news"], 2.0/3.0)
	}
	if math.Abs(result["blog"]-1.0) > 1e-6 {
		t.Errorf("blog recall = %.6f, want 1.0", result["blog"])
	}
}

func TestPerCategoryRecall_NoRelevantInCategory_ReturnsZero(t *testing.T) {
	queries := []GoldenQuery{
		{QueryID: "KR-001", Category: "news", ExpectedNaverRelevant: false},
	}
	sheet := []RaterScore{
		{QueryID: "KR-001", RaterID: "R1", Top3NaverHit: false},
	}
	round := Round{RaterSheets: [][]RaterScore{sheet}}

	result := PerCategoryRecall(round, queries)
	if result["news"] != 0.0 {
		t.Errorf("news recall = %.6f, want 0.0 when no Naver-relevant queries", result["news"])
	}
}

// ---------- Multi-rater aggregation tests ----------

func TestTop3Recall_MultiRater_UsesFirstSheet(t *testing.T) {
	// When multiple rater sheets exist, uses the first sheet for recall computation.
	queries := makeTestGoldenQueries(5, 10)
	sheet1 := make([]RaterScore, 10)
	sheet2 := make([]RaterScore, 10)
	for i := range 10 {
		sheet1[i] = RaterScore{
			QueryID:      fmt.Sprintf("KR-%03d", i+1),
			RaterID:      "R1",
			Top3NaverHit: i < 4, // 4/5 hit
		}
		sheet2[i] = RaterScore{
			QueryID:      fmt.Sprintf("KR-%03d", i+1),
			RaterID:      "R2",
			Top3NaverHit: i < 5, // 5/5 hit (different rater)
		}
	}
	round := Round{RaterSheets: [][]RaterScore{sheet1, sheet2}}

	recall := Top3NaverRecall(round, queries)
	want := 4.0 / 5.0
	if math.Abs(recall-want) > 1e-6 {
		t.Errorf("Top3NaverRecall = %.6f, want %.6f (first sheet)", recall, want)
	}
}
