package router

import (
	"sort"

	"github.com/elymas/universal-search/pkg/types"
)

// rrfK is the rank-constant in the RRF formula: score = 1 / (k + rank).
// k=60 is the standard value from Cormack et al. (SIGIR 2009).
const rrfK = 60

// MergeRRF fuses per-shard result lists using Reciprocal Rank Fusion (k=60).
//
// Each document's RRF score is the sum of 1/(rrfK+rank) across the shards it
// appears in. Documents sharing the same CanonicalHash are deduplicated: the
// copy with the highest individual score is retained with the accumulated RRF
// score.
//
// The returned slice is sorted by descending RRF score.
//
// # @MX:ANCHOR: [AUTO] Cross-shard RRF fusion; callers: index.Search, router tests, bench
// # @MX:REASON: fan_in >= 3; all multi-shard queries funnel through here
// # @MX:SPEC: SPEC-IDX-003
func MergeRRF(shardResults map[Shard][]types.NormalizedDoc) []types.NormalizedDoc {
	if len(shardResults) == 0 {
		return nil
	}

	// Accumulate RRF scores keyed by CanonicalHash.
	type entry struct {
		doc   types.NormalizedDoc
		score float64
	}
	byHash := map[string]*entry{}

	for _, docs := range shardResults {
		for rank, doc := range docs {
			rrfScore := 1.0 / float64(rrfK+rank+1) // rank is 0-indexed; +1 for 1-indexed formula
			// Use cached Hash field; fall back to calling CanonicalHash() if empty.
			hash := doc.Hash
			if hash == "" {
				hash = doc.CanonicalHash()
			}
			if hash == "" {
				hash = doc.ID // last-resort fallback
			}
			if e, ok := byHash[hash]; ok {
				e.score += rrfScore
				// Keep the doc copy with the higher original store score.
				if doc.Score > e.doc.Score {
					e.doc = doc
				}
			} else {
				byHash[hash] = &entry{doc: doc, score: rrfScore}
			}
		}
	}

	// Flatten and sort by descending RRF score.
	result := make([]types.NormalizedDoc, 0, len(byHash))
	for _, e := range byHash {
		e.doc.Score = e.score
		result = append(result, e.doc)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score > result[j].Score
		}
		return result[i].ID < result[j].ID // deterministic tie-break
	})

	return result
}
