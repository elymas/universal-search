// Package koreanews — detectKorean unit tests.
// SPEC-ADP-009 REQ-ADP9-013.
package koreanews

import (
	"testing"
)

func TestDetectKorean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "empty string returns empty",
			text: "",
			want: "",
		},
		{
			name: "whitespace only returns empty",
			text: "   \t\n",
			want: "",
		},
		{
			name: "pure Korean text returns ko",
			text: "한국 뉴스 최신 기사 속보 오늘",
			want: "ko",
		},
		{
			name: "pure English text returns empty",
			text: "Breaking news from Seoul today technology startup",
			want: "",
		},
		{
			name: "mixed Korean above threshold returns ko",
			// 한국어뉴스기사 = 7 hangul runes, mixed with a few ASCII → ratio > 0.30
			text: "한국어뉴스기사 hi",
			want: "ko",
		},
		{
			name: "mixed Korean below threshold returns empty",
			// 1 hangul rune out of many ASCII → ratio << 0.30
			text: "This is a very long English sentence with one Korean char 안",
			want: "",
		},
		{
			name: "exactly at threshold boundary (>= 0.30)",
			// 3 hangul + 7 ASCII non-space = 10 total, 3/10 = 0.30 >= 0.30 → ko
			text: "abc가나다defg",
			want: "ko",
		},
		{
			name: "Japanese hiragana does not trigger Korean detector",
			text: "これは日本語のテキストです",
			want: "",
		},
		{
			name: "Chinese CJK does not trigger Korean detector",
			text: "这是中文文本内容测试",
			want: "",
		},
		{
			name: "whitespace excluded from denominator",
			// "가 나 다" — 3 hangul, 2 spaces excluded = 3/3 = 1.0 → ko
			text: "가 나 다",
			want: "ko",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := detectKorean(tc.text)
			if got != tc.want {
				t.Errorf("detectKorean(%q) = %q; want %q", tc.text, got, tc.want)
			}
		})
	}
}
