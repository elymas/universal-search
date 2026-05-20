// Package youtube — Korean-locale auto-detection and transcript language selection.
// REQ-ADP5-007 / D6: ≥30% Hangul runes triggers ko transcript request.
package youtube

import (
	"unicode/utf8"

	"github.com/elymas/universal-search/pkg/types"
)

// hangulFirst is the first codepoint in the Hangul Syllables block (U+AC00).
const hangulFirst = 0xAC00

// hangulLast is the last codepoint in the Hangul Syllables block (U+D7AF).
const hangulLast = 0xD7AF

// koreanThreshold is the minimum fraction of Hangul runes to trigger Korean
// auto-detection. Empirical value; revisit if Korean-content recall regresses.
//
// @MX:NOTE: [AUTO] 30% Hangul threshold. If Korean-content recall regresses
// post-M3, lower the threshold or add jamo/compatibility Hangul ranges.
// @MX:SPEC: SPEC-ADP-005
const koreanThreshold = 0.30

// maxLangLen is the maximum length of a valid BCP-47 language tag accepted by
// the adapter. "verylongstring" (>8) is rejected per REQ-ADP5-007.
const maxLangLen = 8

// detectKoreanQuery returns true when at least 30% of the runes in text
// fall in the Hangul Syllables block (U+AC00..U+D7AF). Pure function.
func detectKoreanQuery(text string) bool {
	if text == "" {
		return false
	}
	total := utf8.RuneCountInString(text)
	if total == 0 {
		return false
	}
	hangul := 0
	for _, r := range text {
		if r >= hangulFirst && r <= hangulLast {
			hangul++
		}
	}
	return float64(hangul)/float64(total) >= koreanThreshold
}

// selectTranscriptLang returns the preferred transcript language for the
// sidecar request, applying the priority order:
//  1. Explicit Filters[Key="lang"].Value (when set and valid BCP-47 length).
//  2. Korean auto-detection (detectKoreanQuery on text).
//  3. Default "en".
func selectTranscriptLang(text string, filters []types.Filter) string {
	for _, f := range filters {
		if f.Key == "lang" {
			v := f.Value
			if v != "" && len(v) <= maxLangLen {
				return v
			}
			// Invalid or empty lang value — fall through to detection.
			break
		}
	}
	if detectKoreanQuery(text) {
		return "ko"
	}
	return "en"
}
