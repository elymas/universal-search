package naver

import (
	"testing"
)

// TestStripHTML tests the stripHTML function with table-driven cases.
// REQ-ADP8-006: Naver uses <b>...</b> highlights and HTML entities.
func TestStripHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "no HTML",
			input: "plain text only",
			want:  "plain text only",
		},
		{
			name:  "naver bold highlight",
			input: "<b>강조된</b> 텍스트",
			want:  "강조된 텍스트",
		},
		{
			name:  "amp entity",
			input: "Tom &amp; Jerry",
			want:  "Tom & Jerry",
		},
		{
			name:  "lt gt entities",
			input: "&lt;script&gt;alert(1)&lt;/script&gt;",
			want:  "<script>alert(1)</script>",
		},
		{
			name:  "quot entity",
			input: "&quot;hello&quot;",
			want:  `"hello"`,
		},
		{
			name:  "apos numeric entity",
			input: "it&#39;s a test",
			want:  "it's a test",
		},
		{
			name:  "nbsp entity",
			input: "hello&nbsp;world",
			want:  "hello world",
		},
		{
			name:  "multiple bold tags and entities",
			input: "<b>검색어</b>가 포함된 &amp; 설명 &lt;태그&gt;",
			want:  "검색어가 포함된 & 설명 <태그>",
		},
		{
			name:  "tag with attributes",
			input: `<a href="https://example.com">링크</a>`,
			want:  "링크",
		},
		{
			name:  "only tag",
			input: "<br/>",
			want:  "",
		},
		{
			name:  "nested tags",
			input: "<b><i>nested</i></b>",
			want:  "nested",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := stripHTML(tc.input)
			if got != tc.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
