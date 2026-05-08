package router_test

import (
	"testing"

	"github.com/elymas/universal-search/internal/index/router"
	"github.com/elymas/universal-search/pkg/types"
)

func makeDoc(id, hash string, score float64) types.NormalizedDoc {
	return types.NormalizedDoc{
		ID:    id,
		Hash:  hash,
		Title: "test doc " + id,
		Score: score,
	}
}

// TestMergeRRF_TwoShards verifies RRF fusion with k=60 across two shards.
// REQ-IDX-003-007: merged result sorted by descending RRF score.
func TestMergeRRF_TwoShards(t *testing.T) {
	t.Parallel()
	shardKo := []types.NormalizedDoc{
		makeDoc("ko-1", "hash-a", 0.9),
		makeDoc("ko-2", "hash-b", 0.8),
	}
	shardDefault := []types.NormalizedDoc{
		makeDoc("def-1", "hash-c", 0.85),
		makeDoc("def-2", "hash-b", 0.75), // same canonical_hash as ko-2 → dedup
	}

	merged := router.MergeRRF(map[router.Shard][]types.NormalizedDoc{
		router.ShardKo:      shardKo,
		router.ShardDefault: shardDefault,
	})

	// hash-b appears in both shards; should be deduplicated.
	seen := map[string]int{}
	for _, d := range merged {
		seen[d.Hash]++
	}
	if seen["hash-b"] != 1 {
		t.Errorf("hash-b should appear exactly once after dedup, got %d", seen["hash-b"])
	}

	// Results must be in descending RRF score order.
	for i := 1; i < len(merged); i++ {
		if merged[i].Score > merged[i-1].Score {
			t.Errorf("results not sorted at index %d: %.4f > %.4f",
				i, merged[i].Score, merged[i-1].Score)
		}
	}
}

// TestMergeRRF_SingleShard verifies passthrough when only one shard has results.
func TestMergeRRF_SingleShard(t *testing.T) {
	t.Parallel()
	docs := []types.NormalizedDoc{
		makeDoc("doc-1", "h1", 0.9),
		makeDoc("doc-2", "h2", 0.7),
	}
	merged := router.MergeRRF(map[router.Shard][]types.NormalizedDoc{
		router.ShardKo: docs,
	})

	if len(merged) != 2 {
		t.Errorf("expected 2 docs from single shard, got %d", len(merged))
	}
}

// TestMergeRRF_EmptyInput verifies nil/empty input returns empty slice.
func TestMergeRRF_EmptyInput(t *testing.T) {
	t.Parallel()
	merged := router.MergeRRF(nil)
	if len(merged) != 0 {
		t.Errorf("expected empty result, got %d docs", len(merged))
	}
}

// TestMergeRRF_DeduplicatesByCanonicalHash verifies dedup across all shards.
// REQ-IDX-003-007
func TestMergeRRF_DeduplicatesByCanonicalHash(t *testing.T) {
	t.Parallel()
	// Same canonical hash in both shards.
	shardA := []types.NormalizedDoc{makeDoc("a1", "shared-hash", 0.9)}
	shardB := []types.NormalizedDoc{makeDoc("b1", "shared-hash", 0.8)}

	merged := router.MergeRRF(map[router.Shard][]types.NormalizedDoc{
		router.ShardKo:      shardA,
		router.ShardDefault: shardB,
	})

	if len(merged) != 1 {
		t.Errorf("expected 1 doc after dedup, got %d", len(merged))
	}
}

// TestMergeRRF_ScoreIsNonNegative verifies RRF scores are always positive.
func TestMergeRRF_ScoreIsNonNegative(t *testing.T) {
	t.Parallel()
	shardKo := []types.NormalizedDoc{
		makeDoc("ko-1", "h1", 0.1),
	}
	shardDefault := []types.NormalizedDoc{
		makeDoc("def-1", "h2", 0.1),
	}
	merged := router.MergeRRF(map[router.Shard][]types.NormalizedDoc{
		router.ShardKo:      shardKo,
		router.ShardDefault: shardDefault,
	})
	for _, d := range merged {
		if d.Score < 0 {
			t.Errorf("negative RRF score: %v", d.Score)
		}
	}
}
