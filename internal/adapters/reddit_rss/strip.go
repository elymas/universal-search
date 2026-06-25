// Package redditrss — HTML strip and text helpers.
// Replicated from internal/adapters/koreanews/strip.go and knc.go verbatim.
// reddit-rss is the 3rd consumer of this helper (hn, koreanews, reddit-rss),
// qualifying the rule-of-three; consolidation is OUT OF SCOPE per SPEC-ADP-001b.
//
// @MX:NOTE: [AUTO] Replicated stdlib-only HTML-strip. Third consumer after hn and
// koreanews (rule-of-three met). Consolidation to a shared pkg is deferred per
// SPEC-ADP-001b scope discipline. NOT a security boundary — output is plain text.
package redditrss

import "strings"

// stripHTML removes HTML tags and decodes common HTML entities from s.
// Conservative stdlib-only implementation for RSS feed body content.
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

// truncate returns the first n runes of s, or s unchanged if len(runes) <= n.
// Rune-safe: multi-byte UTF-8 characters are counted as single runes.
// Mirrors koreanews/knc.go truncate exactly (no ellipsis appended).
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
