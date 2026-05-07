// Package koreanews — HTML strip helper.
// Duplicated verbatim from internal/adapters/hn/strip.go per SPEC-ADP-002 §11.4
// rule-of-three guidance (consolidation deferred until 3+ consumers).
// NOT a security boundary — output is plain text for synthesis, never rendered.
package koreanews

import "strings"

// stripHTML removes HTML tags and decodes common HTML entities from s.
// Conservative stdlib-only implementation for RSS feed body content.
//
// @MX:NOTE: [AUTO] Conservative stdlib-only HTML-strip. Not a security boundary.
// Duplicated from hn/strip.go per rule-of-three guidance. Consolidate when
// a third adapter needs this helper.
// @MX:SPEC: SPEC-ADP-009
func stripHTML(s string) string {
	if s == "" {
		return ""
	}

	// Pass 1: strip tags by removing '<' ... '>' substrings.
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '<':
			inTag = true
		case c == '>':
			inTag = false
		case !inTag:
			b.WriteByte(c)
		}
	}
	result := b.String()

	// Pass 2: decode common HTML entities.
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&quot;", "\"")
	result = strings.ReplaceAll(result, "&#39;", "'")
	result = strings.ReplaceAll(result, "&nbsp;", " ")

	return result
}
