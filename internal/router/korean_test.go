// Package router_test validates the deterministic Korean detection helpers.
package router_test

import (
	"math"
	"testing"

	"github.com/elymas/universal-search/internal/router"
)

// TestHangulRatioPureKorean asserts a fully-Korean string returns ratio ≈ 1.0.
func TestHangulRatioPureKorean(t *testing.T) {
	t.Parallel()
	got := router.HangulRatio("안녕하세요")
	if !approxEqual(got, 1.0, 0.001) {
		t.Errorf("HangulRatio pure korean: got %v, want 1.0", got)
	}
}

// TestHangulRatioPureEnglish asserts a fully-ASCII string returns 0.
func TestHangulRatioPureEnglish(t *testing.T) {
	t.Parallel()
	got := router.HangulRatio("transformer paper review")
	if got != 0.0 {
		t.Errorf("HangulRatio pure english: got %v, want 0.0", got)
	}
}

// TestHangulRatioEmptyString returns 0 with no panic.
func TestHangulRatioEmptyString(t *testing.T) {
	t.Parallel()
	got := router.HangulRatio("")
	if got != 0.0 {
		t.Errorf("HangulRatio empty: got %v, want 0.0", got)
	}
}

// TestHangulRatioMixed asserts a known-mixed string returns the expected
// fraction. "ChatGPT 사용법" — 7 ASCII + 3 Hangul, 10 non-ws runes →
// ratio = 3/10 = 0.30.
func TestHangulRatioMixed(t *testing.T) {
	t.Parallel()
	got := router.HangulRatio("ChatGPT 사용법")
	if !approxEqual(got, 0.30, 0.005) {
		t.Errorf("HangulRatio mixed: got %v, want %v", got, 0.30)
	}
}

// TestHangulRatioCompatibilityJamo covers the U+3130-U+318F block.
func TestHangulRatioCompatibilityJamo(t *testing.T) {
	t.Parallel()
	// ㄱ is U+3131 (compatibility jamo).
	got := router.HangulRatio("ㄱABC")
	if !approxEqual(got, 0.25, 0.005) {
		t.Errorf("HangulRatio compat jamo: got %v, want 0.25", got)
	}
}

// TestHangulRatioJamoExtendedA covers the U+A960-A97F block.
func TestHangulRatioJamoExtendedA(t *testing.T) {
	t.Parallel()
	// U+A960 is the first rune in Hangul Jamo Extended-A.
	got := router.HangulRatio("ꥠX")
	if !approxEqual(got, 0.5, 0.005) {
		t.Errorf("HangulRatio jamo-ext-A: got %v, want 0.5", got)
	}
}

// TestKoreanSignalsDetectsParticles asserts the particle suffix detector
// fires on a token ending in 과.
func TestKoreanSignalsDetectsParticles(t *testing.T) {
	t.Parallel()
	ratio, density := router.KoreanSignals("ChatGPT 사용법과 팁")
	if ratio <= 0.0 {
		t.Errorf("ratio: got %v, want > 0", ratio)
	}
	if density == 0.0 {
		t.Error("particle density should be > 0 because token ends in 과")
	}
}

// TestKoreanSignalsHandlesEmoji asserts emoji runes do not crash and do not
// inflate the Hangul count.
func TestKoreanSignalsHandlesEmoji(t *testing.T) {
	t.Parallel()
	ratio, _ := router.KoreanSignals("hello 안녕 🚀 world")
	if ratio == 0.0 {
		t.Error("expected non-zero ratio when Hangul is present")
	}
}

// TestParticleDensityNoTokens returns 0 for empty input.
func TestParticleDensityNoTokens(t *testing.T) {
	t.Parallel()
	if d := router.ParticleDensity(""); d != 0.0 {
		t.Errorf("empty: got %v, want 0", d)
	}
}

// TestParticleDensityFraction asserts the density formula returns
// matched-tokens / total-tokens.
func TestParticleDensityFraction(t *testing.T) {
	t.Parallel()
	// "사용법과 팁이 좋다" — 3 tokens, 2 end in particles (과 and 이).
	got := router.ParticleDensity("사용법과 팁이 좋다")
	if !approxEqual(got, 2.0/3.0, 0.01) {
		t.Errorf("ParticleDensity: got %v, want %v", got, 2.0/3.0)
	}
}

func approxEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}
