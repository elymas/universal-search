// Package hn — HTML strip helper for HN story_text field.
// REQ-ADP2-005 (in-scope §2.1e): conservative stdlib-only tag-strip + entity-decode.
// NOT a security boundary — output is plain text consumed by synthesis, never
// rendered as HTML.
package hn

import (
	"strings"
)

// stripHTML removes HTML tags and decodes common HTML entities from s.
// It is a conservative, stdlib-only implementation suitable for HN's shallow
// body markup (<p>, <a>, <i>, <b>, <br>, <code>, <pre>, and similar).
//
// Algorithm:
//  1. Remove all content between '<' and '>' (tag stripping).
//  2. Decode the five named HTML entities + &nbsp; and numeric &#39;.
//
// This is NOT a full HTML parser and NOT an XSS sanitizer. Output is plain
// text intended for synthesis input only.
//
// @MX:NOTE: [AUTO] Conservative stdlib-only HTML-strip. Not a security boundary.
// Adding a third HN body markup pattern requires updating strip_test.go fixture
// set first. For robust parsing, revisit golang.org/x/net/html per Open Question §11.1.
// @MX:SPEC: SPEC-ADP-002
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
