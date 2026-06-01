package github

// Coverage for the nil-safe accessor helpers' nil branches (the dereference
// branches are reached by the parse-level tests; the nil branches are not).
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"testing"
	"time"

	gogithub "github.com/google/go-github/v73/github"
)

func TestSafeHelpers_NilReturnsZero(t *testing.T) {
	if got := safeStr(nil); got != "" {
		t.Errorf("safeStr(nil) = %q, want empty", got)
	}
	if got := safeInt(nil); got != 0 {
		t.Errorf("safeInt(nil) = %d, want 0", got)
	}
	if got := safeInt64(nil); got != 0 {
		t.Errorf("safeInt64(nil) = %d, want 0", got)
	}
	if got := safeTime(nil); !got.IsZero() {
		t.Errorf("safeTime(nil) = %v, want zero time", got)
	}
}

func TestSafeHelpers_NonNilReturnsValue(t *testing.T) {
	s := "hello"
	if got := safeStr(&s); got != "hello" {
		t.Errorf("safeStr = %q, want hello", got)
	}
	i := 42
	if got := safeInt(&i); got != 42 {
		t.Errorf("safeInt = %d, want 42", got)
	}
	i64 := int64(99)
	if got := safeInt64(&i64); got != 99 {
		t.Errorf("safeInt64 = %d, want 99", got)
	}
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ts := &gogithub.Timestamp{Time: now}
	if got := safeTime(ts); !got.Equal(now) {
		t.Errorf("safeTime = %v, want %v", got, now)
	}
}

func TestTruncateRunes_GithubVariant(t *testing.T) {
	if got := truncateRunes("short", 100); got != "short" {
		t.Errorf("under-limit truncateRunes changed input: %q", got)
	}
	long := "abcdefghij" // 10 runes
	if got := truncateRunes(long, 4); got != "abcd" {
		t.Errorf("truncateRunes(%q, 4) = %q, want abcd", long, got)
	}
	// Multibyte: 5 Korean syllables truncated to 2 must keep rune boundaries.
	if got := truncateRunes("가나다라마", 2); got != "가나" {
		t.Errorf("multibyte truncate = %q, want 가나", got)
	}
}
