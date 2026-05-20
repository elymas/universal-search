package router_test

import (
	"testing"

	"github.com/elymas/universal-search/internal/index/router"
	"github.com/elymas/universal-search/pkg/types"
)

// ---------------------------------------------------------------------------
// IndexShardForDoc — 5 routing cases from SPEC-IDX-003 §2.1
// ---------------------------------------------------------------------------

func TestIndexShardForDoc_KoLang(t *testing.T) {
	t.Parallel()
	// Case 1: Lang == "ko" AND HangulRatio >= 0.10 → ShardKo only.
	doc := types.NormalizedDoc{Lang: "ko", Title: "안녕하세요 서울 날씨"}
	shards := router.IndexShardForDoc(doc)
	if len(shards) != 1 || shards[0] != router.ShardKo {
		t.Errorf("expected [ShardKo], got %v", shards)
	}
}

func TestIndexShardForDoc_KoLangLowHangul(t *testing.T) {
	t.Parallel()
	// Case 2: Lang == "ko" AND HangulRatio < 0.10 → both shards (dual-write).
	doc := types.NormalizedDoc{Lang: "ko", Title: "hello world"}
	shards := router.IndexShardForDoc(doc)
	if len(shards) != 2 {
		t.Fatalf("expected 2 shards for dual-write, got %d: %v", len(shards), shards)
	}
	found := map[router.Shard]bool{}
	for _, s := range shards {
		found[s] = true
	}
	if !found[router.ShardKo] || !found[router.ShardDefault] {
		t.Errorf("dual-write must include both shards, got %v", shards)
	}
}

func TestIndexShardForDoc_HighHangulRatio(t *testing.T) {
	t.Parallel()
	// Case 3: Lang != "ko" but HangulRatio >= RatioHigh (0.30) → ShardKo only.
	doc := types.NormalizedDoc{Lang: "", Title: "안녕하세요 서울 날씨 한국어 형태소"}
	shards := router.IndexShardForDoc(doc)
	if len(shards) != 1 || shards[0] != router.ShardKo {
		t.Errorf("expected [ShardKo] for high hangul ratio, got %v", shards)
	}
}

func TestIndexShardForDoc_MediumHangulRatio(t *testing.T) {
	t.Parallel()
	// Case 4: Lang != "ko" AND RatioLow <= HangulRatio < RatioHigh → both shards.
	// Title with ~15% Hangul: "안녕하세요 ai" — 5 Hangul / ~8 total letter runes ≈ 0.20.
	// (spaces are typically counted in denominator by HangulRatio)
	// Use many Hangul chars to be solidly in [0.10, 0.30) with Lang="" appended.
	doc := types.NormalizedDoc{Lang: "", Title: "안녕하세요 hello world ai"}
	shards := router.IndexShardForDoc(doc)
	if len(shards) != 2 {
		t.Fatalf("expected dual-write in ambiguous band, got %d: %v", len(shards), shards)
	}
}

func TestIndexShardForDoc_NoHangul(t *testing.T) {
	t.Parallel()
	// Case 5: HangulRatio < RatioLow AND Lang != "ko" → ShardDefault only.
	doc := types.NormalizedDoc{Lang: "en", Title: "hello world english text"}
	shards := router.IndexShardForDoc(doc)
	if len(shards) != 1 || shards[0] != router.ShardDefault {
		t.Errorf("expected [ShardDefault] for non-Korean, got %v", shards)
	}
}

// ---------------------------------------------------------------------------
// QueryShardsForText — 8 routing cases from SPEC-IDX-003 §2.2
// ---------------------------------------------------------------------------

func TestQueryShardsForText_PureKorean(t *testing.T) {
	t.Parallel()
	// HangulRatio >= RatioHigh (0.30) → ShardKo only.
	shards := router.QueryShardsForText("안녕하세요 서울 날씨 오늘")
	if len(shards) != 1 || shards[0] != router.ShardKo {
		t.Errorf("expected [ShardKo], got %v", shards)
	}
}

func TestQueryShardsForText_MixedHighHangul(t *testing.T) {
	t.Parallel()
	// RatioLow <= HangulRatio < RatioHigh (ambiguous band) → both shards.
	// "안녕 programming" — ~2/14 chars are Hangul = ~14%; in [0.10, 0.30).
	shards := router.QueryShardsForText("안녕 programming")
	if len(shards) != 2 {
		t.Fatalf("expected dual-shard in ambiguous band, got %d: %v", len(shards), shards)
	}
}

func TestQueryShardsForText_NoHangul(t *testing.T) {
	t.Parallel()
	// HangulRatio < RatioLow (0.10) → ShardDefault only.
	shards := router.QueryShardsForText("machine learning search engine")
	if len(shards) != 1 || shards[0] != router.ShardDefault {
		t.Errorf("expected [ShardDefault] for no-hangul query, got %v", shards)
	}
}

func TestQueryShardsForText_EmptyQuery(t *testing.T) {
	t.Parallel()
	// Empty text → ShardDefault (no-op fallback).
	shards := router.QueryShardsForText("")
	if len(shards) != 1 || shards[0] != router.ShardDefault {
		t.Errorf("expected [ShardDefault] for empty query, got %v", shards)
	}
}
