// Package naver — HTML strip helper for Naver search result fields.
// REQ-ADP8-006: conservative stdlib-only tag-strip + entity-decode.
// Naver wraps matched keywords in <b>...</b> tags; these must be removed
// to produce clean plain-text NormalizedDoc fields.
// NOT a security boundary — output is plain text for synthesis, never
// rendered as HTML.
package naver

import (
	"strings"
)

// stripHTML removes HTML tags and decodes common HTML entities from s.
// It is a conservative, stdlib-only implementation suitable for Naver's
// highlight markup (<b> tags for matched keywords) and common entities.
//
// Algorithm:
//  1. Remove all content between '<' and '>' (tag stripping).
//  2. Decode the five named HTML entities + &nbsp; and numeric &#39;.
//
// This is NOT a full HTML parser and NOT an XSS sanitizer. Output is plain
// text intended for synthesis input only.
//
// @MX:NOTE: [AUTO] Conservative stdlib-only HTML-strip. Not a security boundary.
// Naver uses <b>...</b> for keyword highlights. For robust parsing,
// revisit golang.org/x/net/html if additional markup patterns emerge.
// @MX:SPEC: SPEC-ADP-008
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
