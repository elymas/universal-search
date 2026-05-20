// Package index — Reciprocal Rank Fusion implementation.
// SPEC-IDX-001 REQ-IDX-007 §2.5 (scope item j).
// Reference: Cormack, Clarke, Buettcher SIGIR 2009.
// http://cormack.uwaterloo.ca/cormacksigir09-rrf.pdf
package index

import (
	"sort"

	"github.com/elymas/universal-search/pkg/types"
)

// Ranked is a single entry in a per-store rank list.
type Ranked struct {
	DocID string
	Doc   types.NormalizedDoc
}

// FusedDoc is the output of fuseRRF; carries the fused RRF score.
type FusedDoc struct {
	DocID string
	Doc   types.NormalizedDoc
	Score float64
}

// fuseRRF implements Reciprocal Rank Fusion across multiple ranked lists.
//
// Formula: score(d) = sum_{store} weights[store] / (k + rank_store(d))
// where rank is 1-indexed and ties are broken by DocID ascending.
//
// Properties:
//   - O(N) time and space, N = sum of len(list) across stores.
//   - Pure: no I/O, no time, no randomness.
//   - Deterministic: same input → byte-equal output (map iteration is not
//     observed; output is built into a slice and sorted stably).
//
// @MX:ANCHOR: [AUTO] Every fused result passes through this single transform. fan_in >= 1 but invariant-bearing.
// @MX:REASON: RRF formula and weighting invariant must not change without SPEC amendment.
// @MX:SPEC: SPEC-IDX-001
func fuseRRF(rankLists map[string][]Ranked, weights map[string]float64, k int) []FusedDoc {
	scores := make(map[string]float64, 64)
	docs := make(map[string]types.NormalizedDoc, 64)

	for store, list := range rankLists {
		w, ok := weights[store]
		if !ok || w <= 0 {
			w = 1.0
		}
		for i, r := range list {
			rank := i + 1 // 1-indexed per the paper
			scores[r.DocID] += w / (float64(k) + float64(rank))
			if _, seen := docs[r.DocID]; !seen {
				docs[r.DocID] = r.Doc
			}
		}
	}

	out := make([]FusedDoc, 0, len(scores))
	for id, s := range scores {
		out = append(out, FusedDoc{DocID: id, Doc: docs[id], Score: s})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].DocID < out[j].DocID
	})

	return out
}
