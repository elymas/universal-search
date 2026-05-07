// Package fanout — deterministic sorting of NormalizedDoc slices.
// SPEC-FAN-001 REQ-FAN-007, D5.
package fanout

import (
	"sort"

	"github.com/elymas/universal-search/pkg/types"
)

// sortDocs sorts the slice in-place using sort.SliceStable with a 3-key compare:
//  1. PRIMARY: Score descending (higher score first).
//  2. SECONDARY tie-breaker: SourceID ascending (= adapter name, lexicographic).
//     IR-001 REQ-IR-008 guarantees AdapterSet is lexicographically sorted, so
//     this tie-breaker preserves the input adapter order for equal-scored docs.
//  3. TERTIARY tie-breaker: RetrievedAt descending (newer first).
//
// sort.SliceStable preserves input order for equal 3-key tuples
// (TestSortStableForEqualKeys asserts this).
func sortDocs(docs []types.NormalizedDoc) {
	sort.SliceStable(docs, func(i, j int) bool {
		di, dj := docs[i], docs[j]
		if di.Score != dj.Score {
			return di.Score > dj.Score // descending
		}
		if di.SourceID != dj.SourceID {
			return di.SourceID < dj.SourceID // ascending
		}
		return di.RetrievedAt.After(dj.RetrievedAt) // descending
	})
}
