// Package router implements Korean-aware shard routing for SPEC-IDX-003.
//
// Two exported functions determine which Meilisearch index shards to target:
//   - IndexShardForDoc: index-time routing (5 cases, §2.1)
//   - QueryShardsForText: query-time routing (8 cases, §2.2)
//
// Both functions are pure (no I/O, no state) and reuse HangulRatio,
// RatioHigh, and RatioLow from SPEC-IR-001 without duplication.
package router

import (
	"github.com/elymas/universal-search/internal/router"
	"github.com/elymas/universal-search/pkg/types"
)

// Shard identifies a Meilisearch index shard.
type Shard string

const (
	// ShardDefault is the primary (non-Korean) Meilisearch index "usearch".
	ShardDefault Shard = "default"
	// ShardKo is the Korean-dedicated Meilisearch index "usearch-ko".
	ShardKo Shard = "ko"
)

// IndexShardForDoc returns the ordered list of shards a document should be
// written to during indexing.
//
// Routing table (SPEC-IDX-003 §2.1):
//
//	Case 1: doc.Lang == "ko" AND HangulRatio(title) >= RatioLow → [ShardKo]
//	Case 2: doc.Lang == "ko" AND HangulRatio(title) <  RatioLow → [ShardKo, ShardDefault] (dual-write)
//	Case 3: doc.Lang != "ko" AND HangulRatio >= RatioHigh       → [ShardKo]
//	Case 4: doc.Lang != "ko" AND RatioLow <= HangulRatio < RatioHigh → [ShardKo, ShardDefault]
//	Case 5: HangulRatio < RatioLow AND doc.Lang != "ko"         → [ShardDefault]
//
// # @MX:ANCHOR: [AUTO] Index-time shard routing decision; callers: index.Upsert, router tests, bench
// # @MX:REASON: fan_in >= 3; all document writes pass through this function
// # @MX:SPEC: SPEC-IDX-003
func IndexShardForDoc(doc types.NormalizedDoc) []Shard {
	// Use the document title as the representative text for Hangul ratio.
	text := doc.Title + " " + doc.Lang
	ratio := router.HangulRatio(text)

	if doc.Lang == "ko" {
		if ratio >= router.RatioLow {
			// Case 1: explicit Korean with sufficient Hangul content.
			return []Shard{ShardKo}
		}
		// Case 2: explicit Korean but low Hangul (e.g. English title, ko metadata).
		// Dual-write to both shards.
		return []Shard{ShardKo, ShardDefault}
	}

	// Non-Korean lang (or empty).
	if ratio >= router.RatioHigh {
		// Case 3: high Hangul ratio implies Korean content regardless of lang tag.
		return []Shard{ShardKo}
	}
	if ratio >= router.RatioLow {
		// Case 4: ambiguous band — write to both shards.
		return []Shard{ShardKo, ShardDefault}
	}
	// Case 5: non-Korean, low Hangul.
	return []Shard{ShardDefault}
}

// QueryShardsForText returns the list of shards to query for the given text.
//
// Routing table (SPEC-IDX-003 §2.2):
//
//	HangulRatio >= RatioHigh          → [ShardKo]
//	RatioLow <= HangulRatio < RatioHigh → [ShardKo, ShardDefault]
//	HangulRatio < RatioLow or empty   → [ShardDefault]
//
// # @MX:ANCHOR: [AUTO] Query-time shard selection; callers: index.Search, router tests, bench
// # @MX:REASON: fan_in >= 3; every search request routes through this function
// # @MX:SPEC: SPEC-IDX-003
func QueryShardsForText(text string) []Shard {
	if text == "" {
		return []Shard{ShardDefault}
	}

	ratio := router.HangulRatio(text)

	if ratio >= router.RatioHigh {
		return []Shard{ShardKo}
	}
	if ratio >= router.RatioLow {
		return []Shard{ShardKo, ShardDefault}
	}
	return []Shard{ShardDefault}
}
