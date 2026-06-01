package social

// Direct coverage for truncateRunes, including the rune-boundary truncation
// path (which the parse-level tests only reach indirectly) and the sentinel
// error type's Error method.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateRunes(t *testing.T) {
	t.Run("under limit is unchanged", func(t *testing.T) {
		in := "short"
		if got := truncateRunes(in, 280); got != in {
			t.Errorf("truncateRunes(%q) = %q, want unchanged", in, got)
		}
	})

	t.Run("at limit is unchanged", func(t *testing.T) {
		in := strings.Repeat("a", 10)
		if got := truncateRunes(in, 10); got != in {
			t.Errorf("truncateRunes at exact limit changed input: %q", got)
		}
	})

	t.Run("over limit is truncated with ellipsis", func(t *testing.T) {
		in := strings.Repeat("a", 50)
		got := truncateRunes(in, 10)
		if !strings.HasSuffix(got, "...") {
			t.Errorf("truncated output %q must end with ...", got)
		}
		if utf8.RuneCountInString(got) > 10 {
			t.Errorf("truncated output has %d runes, want <= 10", utf8.RuneCountInString(got))
		}
	})

	t.Run("multibyte runes counted correctly", func(t *testing.T) {
		// 20 Korean syllables (each 3 bytes) — must truncate by rune count, not bytes.
		in := strings.Repeat("가", 20)
		got := truncateRunes(in, 10)
		if utf8.RuneCountInString(got) > 10 {
			t.Errorf("multibyte truncation = %d runes, want <= 10", utf8.RuneCountInString(got))
		}
		if !utf8.ValidString(got) {
			t.Errorf("truncation produced invalid UTF-8: %q", got)
		}
	})
}

func TestXSentinelError(t *testing.T) {
	const e xSentinelError = "x: boom"
	if e.Error() != "x: boom" {
		t.Errorf("xSentinelError.Error() = %q, want %q", e.Error(), "x: boom")
	}
}
