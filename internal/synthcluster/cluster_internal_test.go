package synthcluster

// Internal coverage for the pure helpers betterRep (representative tiebreaker
// chain) and cosineSimilarity (degenerate-input branches).
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

func TestBetterRep_TiebreakerChain(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	t.Run("higher score wins", func(t *testing.T) {
		cand := types.NormalizedDoc{Score: 0.9}
		cur := types.NormalizedDoc{Score: 0.5}
		if !betterRep(cand, cur) {
			t.Error("higher score should win")
		}
		if betterRep(cur, cand) {
			t.Error("lower score should lose")
		}
	})

	t.Run("later publish time wins on score tie", func(t *testing.T) {
		cand := types.NormalizedDoc{Score: 0.5, PublishedAt: t2}
		cur := types.NormalizedDoc{Score: 0.5, PublishedAt: t1}
		if !betterRep(cand, cur) {
			t.Error("later PublishedAt should win on score tie")
		}
	})

	t.Run("longer body wins on score+time tie", func(t *testing.T) {
		cand := types.NormalizedDoc{Score: 0.5, PublishedAt: t1, Body: "longer body"}
		cur := types.NormalizedDoc{Score: 0.5, PublishedAt: t1, Body: "x"}
		if !betterRep(cand, cur) {
			t.Error("longer body should win on score+time tie")
		}
	})

	t.Run("lexicographically smaller ID wins on full tie", func(t *testing.T) {
		cand := types.NormalizedDoc{Score: 0.5, PublishedAt: t1, Body: "ab", ID: "aaa"}
		cur := types.NormalizedDoc{Score: 0.5, PublishedAt: t1, Body: "cd", ID: "bbb"}
		if !betterRep(cand, cur) {
			t.Error("smaller ID should win on full tie")
		}
		if betterRep(cur, cand) {
			t.Error("larger ID should lose on full tie")
		}
	})
}

func TestClusterMetaToMap(t *testing.T) {
	t.Run("includes cosine_min when set", func(t *testing.T) {
		cm := clusterMeta{
			SchemaVersion: 1,
			Members:       []string{"a", "b"},
			SimHash:       "deadbeef",
			DedupMode:     "cosine",
			ClusterSize:   2,
			CosineMin:     0.87,
		}
		m := cm.toMap()
		if m["cosine_min"] != 0.87 {
			t.Errorf("cosine_min = %v, want 0.87", m["cosine_min"])
		}
		if m["cluster_size"] != 2 {
			t.Errorf("cluster_size = %v, want 2", m["cluster_size"])
		}
	})

	t.Run("omits cosine_min when zero", func(t *testing.T) {
		cm := clusterMeta{SchemaVersion: 1, ClusterSize: 1}
		m := cm.toMap()
		if _, ok := m["cosine_min"]; ok {
			t.Error("cosine_min must be omitted when zero")
		}
	})
}

func TestCosineSimilarity_Branches(t *testing.T) {
	t.Run("mismatched length returns 0", func(t *testing.T) {
		if got := cosineSimilarity([]float64{1, 2}, []float64{1}); got != 0 {
			t.Errorf("mismatched length = %v, want 0", got)
		}
	})
	t.Run("empty returns 0", func(t *testing.T) {
		if got := cosineSimilarity(nil, nil); got != 0 {
			t.Errorf("empty = %v, want 0", got)
		}
	})
	t.Run("zero-magnitude returns 0", func(t *testing.T) {
		if got := cosineSimilarity([]float64{0, 0}, []float64{1, 1}); got != 0 {
			t.Errorf("zero-magnitude = %v, want 0", got)
		}
	})
	t.Run("identical vectors ~1.0", func(t *testing.T) {
		got := cosineSimilarity([]float64{1, 0, 0}, []float64{1, 0, 0})
		if got < 0.99 || got > 1.01 {
			t.Errorf("identical vectors = %v, want ~1.0", got)
		}
	})
	t.Run("orthogonal vectors ~0", func(t *testing.T) {
		got := cosineSimilarity([]float64{1, 0}, []float64{0, 1})
		if got < -0.01 || got > 0.01 {
			t.Errorf("orthogonal vectors = %v, want ~0", got)
		}
	})
}
