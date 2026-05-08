// Package index — unit tests for RRF fusion (REQ-IDX-007).
package index

import (
	"math"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

func makeRanked(id string) Ranked {
	return Ranked{DocID: id, Doc: types.NormalizedDoc{ID: id}}
}

func TestFuseRRF_EmptyRankLists(t *testing.T) {
	t.Parallel()
	result := fuseRRF(map[string][]Ranked{}, map[string]float64{}, 60)
	if len(result) != 0 {
		t.Fatalf("expected empty, got %d items", len(result))
	}
}

func TestFuseRRF_SingleStore(t *testing.T) {
	t.Parallel()
	lists := map[string][]Ranked{
		"qdrant": {makeRanked("a"), makeRanked("b"), makeRanked("c")},
	}
	weights := map[string]float64{"qdrant": 1.0}
	result := fuseRRF(lists, weights, 60)
	if len(result) != 3 {
		t.Fatalf("expected 3 fused docs, got %d", len(result))
	}
	// Rank 1 (index 0) must have higher score than rank 2.
	if result[0].Score <= result[1].Score {
		t.Fatalf("score not descending: %v <= %v", result[0].Score, result[1].Score)
	}
}

func TestFuseRRF_ScoreFormula(t *testing.T) {
	t.Parallel()
	// For a single doc at rank 1 with weight=1.0 and k=60: score = 1/(60+1) = 1/61.
	lists := map[string][]Ranked{
		"qdrant": {makeRanked("only")},
	}
	weights := map[string]float64{"qdrant": 1.0}
	result := fuseRRF(lists, weights, 60)
	want := 1.0 / (60.0 + 1.0)
	if math.Abs(result[0].Score-want) > 1e-9 {
		t.Fatalf("score = %v, want %v", result[0].Score, want)
	}
}

func TestFuseRRF_MultiStore_Merge(t *testing.T) {
	t.Parallel()
	// docA in qdrant (rank 1) and meili (rank 1) → higher score than docB only in pg.
	lists := map[string][]Ranked{
		"qdrant": {makeRanked("docA"), makeRanked("docB")},
		"meili":  {makeRanked("docA"), makeRanked("docC")},
		"pg":     {makeRanked("docB")},
	}
	weights := map[string]float64{"qdrant": 1.0, "meili": 1.0, "pg": 1.0}
	result := fuseRRF(lists, weights, 60)

	// docA must be first.
	if result[0].DocID != "docA" {
		t.Fatalf("expected docA first, got %q", result[0].DocID)
	}
}

func TestFuseRRF_Ordering_Descending(t *testing.T) {
	t.Parallel()
	lists := map[string][]Ranked{
		"qdrant": {makeRanked("x1"), makeRanked("x2"), makeRanked("x3")},
		"meili":  {makeRanked("x3"), makeRanked("x1"), makeRanked("x2")},
	}
	weights := map[string]float64{"qdrant": 1.0, "meili": 1.0}
	result := fuseRRF(lists, weights, 60)
	for i := 1; i < len(result); i++ {
		if result[i-1].Score < result[i].Score {
			t.Fatalf("not descending at idx %d: %v < %v", i, result[i-1].Score, result[i].Score)
		}
	}
}

func TestFuseRRF_WeightScaling(t *testing.T) {
	t.Parallel()
	// With weight=2.0, score for rank-1 doc = 2/(60+1) = 2/61.
	lists := map[string][]Ranked{
		"qdrant": {makeRanked("doc")},
	}
	weights := map[string]float64{"qdrant": 2.0}
	result := fuseRRF(lists, weights, 60)
	want := 2.0 / 61.0
	if math.Abs(result[0].Score-want) > 1e-9 {
		t.Fatalf("score with weight=2 = %v, want %v", result[0].Score, want)
	}
}

func TestFuseRRF_TieBreakByDocID(t *testing.T) {
	t.Parallel()
	// Two docs with identical scores must be stable-sorted by doc ID.
	lists := map[string][]Ranked{
		"qdrant": {makeRanked("bbb"), makeRanked("aaa")},
	}
	// Swap to ensure same score for different docs doesn't happen (they have different ranks),
	// but same score IS possible if both appear at same rank from different stores.
	lists2 := map[string][]Ranked{
		"qdrant": {makeRanked("aaa")},
		"meili":  {makeRanked("bbb")},
	}
	weights := map[string]float64{"qdrant": 1.0, "meili": 1.0}
	result := fuseRRF(lists2, weights, 60)
	// Both have score 1/61; tie-break by docID → "aaa" < "bbb".
	if result[0].DocID != "aaa" {
		t.Fatalf("expected 'aaa' first (tie-break by docID), got %q", result[0].DocID)
	}
	_ = lists
}

func TestFuseRRF_UnknownStoreWeight_DefaultsToZero(t *testing.T) {
	t.Parallel()
	// Store not in weights map → weight 0 → contributes nothing.
	lists := map[string][]Ranked{
		"qdrant": {makeRanked("a")},
		"ghost":  {makeRanked("b")},
	}
	weights := map[string]float64{"qdrant": 1.0}
	result := fuseRRF(lists, weights, 60)
	// "b" should have score 0 → after "a".
	if result[0].DocID != "a" {
		t.Fatalf("expected 'a' first, got %q", result[0].DocID)
	}
}
