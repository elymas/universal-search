// Package fanout — deduplication of merged NormalizedDoc slices.
// SPEC-FAN-001 §2.3, REQ-FAN-006.
package fanout

import "github.com/elymas/universal-search/pkg/types"

// dedupDocs removes duplicate docs from the input slice using a two-namespace
// keying strategy (url: for parseable URLs, hash: for fallback).
//
// Algorithm (SPEC-FAN-001 §2.3):
//  1. Iterate input in order.
//  2. For each doc, compute dedup key:
//     - PRIMARY: "url:" + canonicalURL(doc.URL) if URL parses successfully.
//     - FALLBACK: "hash:" + doc.CanonicalHash() if URL fails to parse.
//  3. If key not seen, emit doc and record key.
//  4. If key seen, drop doc and increment drop counter.
//  5. Return (deduped, dropCount).
//
// Same-URL different-content: first occurrence wins. Later docs with the same
// canonical URL but different title/body are silently dropped.
//
// Mixed valid/invalid URLs: the two namespaces are DISJOINT. A parseable URL
// and an unparseable URL NEVER produce the same dedup key even if the underlying
// bytes happen to coincide.
//
// @MX:ANCHOR: [AUTO] Every fanout-returned doc passes through this transform.
// @MX:REASON: dedup invariant; first-occurrence-wins semantic must not change
// without a SPEC amendment — bug here corrupts every Result.
// @MX:SPEC: SPEC-FAN-001
func dedupDocs(docs []types.NormalizedDoc) ([]types.NormalizedDoc, int) {
	seen := make(map[string]bool, len(docs))
	out := make([]types.NormalizedDoc, 0, len(docs))
	dropped := 0

	for _, d := range docs {
		key := dedupKey(d)
		if seen[key] {
			dropped++
			continue
		}
		seen[key] = true
		out = append(out, d)
	}
	return out, dropped
}

// dedupKey returns the namespace-prefixed dedup key for a doc.
// The two namespaces are "url:" and "hash:" — disjoint by construction.
func dedupKey(d types.NormalizedDoc) string {
	if canonical, err := canonicalURL(d.URL); err == nil {
		return "url:" + canonical
	}
	return "hash:" + d.CanonicalHash()
}
