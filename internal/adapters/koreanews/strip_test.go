// Package koreanews — stripHTML unit tests.
// SPEC-ADP-009 §11.4: conservative stdlib-only HTML strip.
package koreanews

import (
	"testing"
)

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
			name:  "plain text unchanged",
			input: "Hello World",
			want:  "Hello World",
		},
		{
			name:  "simple paragraph tag removed",
			input: "<p>Hello</p>",
			want:  "Hello",
		},
		{
			name:  "anchor tag with href removed",
			input: `<a href="https://example.com">click here</a>`,
			want:  "click here",
		},
		{
			name:  "nested tags removed",
			input: "<div><p><strong>Bold text</strong></p></div>",
			want:  "Bold text",
		},
		{
			name:  "amp entity decoded",
			input: "A &amp; B",
			want:  "A & B",
		},
		{
			name:  "lt gt entities decoded",
			input: "&lt;tag&gt;",
			want:  "<tag>",
		},
		{
			name:  "quot entity decoded",
			input: "say &quot;hello&quot;",
			want:  `say "hello"`,
		},
		{
			name:  "apos entity decoded",
			input: "it&#39;s fine",
			want:  "it's fine",
		},
		{
			name:  "nbsp entity decoded",
			input: "word1&nbsp;word2",
			want:  "word1 word2",
		},
		{
			name:  "mixed tags and entities",
			input: "<p>K&amp;R style &lt;code&gt;</p>",
			want:  "K&R style <code>",
		},
		{
			name: "script tag removed but content kept",
			// stripHTML only removes tags, not tag content — it is not a security boundary.
			// The function removes <...> delimiters; content between tags is preserved.
			input: "<script>var x = 1;</script>after",
			want:  "var x = 1;after",
		},
		{
			name:  "Korean text with HTML preserved correctly",
			input: "<p>한국어 &amp; 영어</p>",
			want:  "한국어 & 영어",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := stripHTML(tc.input)
			if got != tc.want {
				t.Errorf("stripHTML(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}
