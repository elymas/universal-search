// Package hn — stripHTML helper tests.
// TestStripHTMLTable covers 8 input shapes per SPEC-ADP-002 §2.1l.
package hn

import (
	"strings"
	"testing"
)

// TestStripHTMLTable validates stripHTML over a comprehensive set of inputs.
func TestStripHTMLTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
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
			name:  "plain text no tags",
			input: "Hello world, no tags here.",
			want:  "Hello world, no tags here.",
		},
		{
			name:  "single paragraph tag",
			input: "<p>Hello world</p>",
			want:  "Hello world",
		},
		{
			name:  "nested tags",
			input: "<p>Hello <b>world</b> and <i>everyone</i>.</p>",
			want:  "Hello world and everyone.",
		},
		{
			name:  "malformed unclosed tag",
			input: "<p>Unclosed tag",
			want:  "Unclosed tag",
		},
		{
			name:  "entity decoding amp lt gt quot",
			input: "a &amp; b &lt; c &gt; d &quot;quoted&quot;",
			want:  `a & b < c > d "quoted"`,
		},
		{
			name:  "mixed tags and entities",
			input: "<p>Hello <b>world</b>&amp; goodbye</p>",
			want:  "Hello world& goodbye",
		},
		{
			name:  "very long body",
			input: "<p>" + strings.Repeat("x", 500) + "</p>",
			want:  strings.Repeat("x", 500),
		},
		{
			name:  "anchor tag with href",
			input: `<p>Check <a href="https://example.com">this link</a>.</p>`,
			want:  "Check this link.",
		},
		{
			name:  "numeric entity apostrophe",
			input: "It&#39;s a test",
			want:  "It's a test",
		},
		{
			name:  "nbsp entity",
			input: "word&nbsp;word",
			want:  "word word",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := stripHTML(tc.input)
			if got != tc.want {
				t.Errorf("stripHTML(%q)\n  got:  %q\n  want: %q", tc.input, got, tc.want)
			}
		})
	}
}
