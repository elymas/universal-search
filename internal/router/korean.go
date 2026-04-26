// Package router — deterministic Korean-language detection helpers.
// SPEC-IR-001: REQ-IR-002, REQ-IR-004 + research §1.8 (Hangul block ranges).
package router

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// koreanParticles are the 11 high-frequency Korean postpositions used as a
// boolean-ish signal for short or low-Hangul-ratio queries (research §3.3).
// Values come from https://en.wikipedia.org/wiki/Korean_postpositions.
//
// @MX:NOTE: [AUTO] Korean particle list — 11 high-frequency postpositions.
// Curated from Wikipedia "Korean_postpositions"; no user customisation in v0.
// @MX:SPEC: SPEC-IR-001
var koreanParticles = []string{"을", "를", "이", "가", "은", "는", "에서", "에", "와", "과", "의"}

// isHangulRune reports whether r falls within one of the four Hangul Unicode
// blocks (research §1.8).
//
// @MX:NOTE: [AUTO] Hangul block ranges per Unicode 15.1: U+AC00-D7A3 (modern
// syllables), U+1100-11FF (Jamo), U+3130-318F (Compat Jamo), U+A960-A97F
// (Jamo Extended-A). Updating these ranges is a SPEC-amendment-level decision.
// @MX:SPEC: SPEC-IR-001
func isHangulRune(r rune) bool {
	switch {
	case r >= 0xAC00 && r <= 0xD7A3:
		return true
	case r >= 0x1100 && r <= 0x11FF:
		return true
	case r >= 0x3130 && r <= 0x318F:
		return true
	case r >= 0xA960 && r <= 0xA97F:
		return true
	}
	return false
}

// HangulRatio returns the fraction of Hangul-block runes over total
// non-whitespace runes in s. Range: [0.0, 1.0]. Returns 0.0 when s is empty
// or contains only whitespace.
//
// Single-pass over s; allocations bounded by the input size.
func HangulRatio(s string) float64 {
	if s == "" {
		return 0
	}
	var hangul, total int
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if isHangulRune(r) {
			hangul++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hangul) / float64(total)
}

// ParticleDensity returns the fraction of whitespace-tokens whose suffix is
// one of the koreanParticles. Range: [0.0, 1.0]. Returns 0.0 for empty input.
//
// Tokenisation uses strings.Fields (any Unicode whitespace).
func ParticleDensity(s string) float64 {
	tokens := strings.Fields(s)
	if len(tokens) == 0 {
		return 0
	}
	var hits int
	for _, tok := range tokens {
		for _, p := range koreanParticles {
			if strings.HasSuffix(tok, p) {
				hits++
				break
			}
		}
	}
	return float64(hits) / float64(len(tokens))
}

// KoreanSignals returns the (HangulRatio, ParticleDensity) tuple in a single
// call. Used by Rules.Score to build the per-category raw scores.
func KoreanSignals(s string) (ratio, density float64) {
	return HangulRatio(s), ParticleDensity(s)
}

// _ avoids the unused-import warning for utf8 in some toolchain combinations.
var _ = utf8.RuneCountInString
