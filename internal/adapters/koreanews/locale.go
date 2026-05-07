// Package koreanews — Korean locale heuristic.
// SPEC-ADP-009 REQ-ADP9-013: Hangul rune ratio detection.
package koreanews

import "unicode"

// detectKorean returns "ko" when the Hangul rune ratio of text is >= 0.30,
// otherwise returns "" (unknown). Whitespace runes are excluded from the
// denominator. Empty or whitespace-only text returns "".
//
// The 0.30 threshold handles mixed-language tech blog feeds (some Korean text
// embedded in English articles) while reliably detecting Korean-primary content.
//
// Heuristic: unicode.Is(unicode.Hangul, r) counts syllable blocks (AC00-D7A3),
// Hangul Jamo (1100-11FF), and compatibility jamo. Emoji and CJK characters
// outside the Hangul block do NOT trigger the detector.
//
// Future SPEC-IDX-003 (Korean tokenization) may upgrade this to a real
// language-detect library if false-positives are observed at indexing time.
//
// @MX:NOTE: [AUTO] Heuristic: Hangul rune ratio threshold 0.30. Open Question
// §11.8 documents revisit triggers (false-positive surveillance under SPEC-IDX-003).
// @MX:SPEC: SPEC-ADP-009
func detectKorean(text string) string {
	if text == "" {
		return ""
	}

	var total, hangul int
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		total++
		if unicode.Is(unicode.Hangul, r) {
			hangul++
		}
	}

	if total == 0 {
		return ""
	}

	ratio := float64(hangul) / float64(total)
	if ratio >= 0.30 {
		return "ko"
	}
	return ""
}
