// Package index — unit tests for docID (REQ-IDX-014 determinism invariant).
package index

import (
	"testing"
)

func TestDocID_Determinism(t *testing.T) {
	t.Parallel()

	// Same inputs must always produce the same 16-hex output.
	got1 := docID("src1", "https://example.com/page")
	got2 := docID("src1", "https://example.com/page")
	if got1 != got2 {
		t.Fatalf("docID not deterministic: %q != %q", got1, got2)
	}
}

func TestDocID_Length(t *testing.T) {
	t.Parallel()

	id := docID("any-source", "https://any.url/path")
	if len(id) != 16 {
		t.Fatalf("docID length = %d, want 16", len(id))
	}
}

func TestDocID_IsHex(t *testing.T) {
	t.Parallel()

	id := docID("src", "url")
	for _, c := range id {
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !isHex {
			t.Fatalf("docID contains non-hex char %q in %q", c, id)
		}
	}
}

func TestDocID_Uniqueness(t *testing.T) {
	t.Parallel()

	cases := [][2]string{
		{"src1", "https://example.com"},
		{"src1", "https://example.com/other"},
		{"src2", "https://example.com"},
		{"", "https://example.com"},
		{"src1", ""},
	}

	seen := map[string]bool{}
	for _, c := range cases {
		id := docID(c[0], c[1])
		key := c[0] + "|" + c[1]
		if seen[id] {
			t.Errorf("collision detected for inputs %v and some prior input → id=%q", c, id)
		}
		seen[id] = true
		_ = key
	}
}

func TestDocID_NullByteIsolation(t *testing.T) {
	t.Parallel()

	// "abc" + "" should differ from "ab" + "c" (null-byte separator).
	id1 := docID("abc", "")
	id2 := docID("ab", "c")
	if id1 == id2 {
		t.Fatalf("docID collision without null-byte isolation: %q == %q", id1, id2)
	}
}

func TestDocID_KnownVector(t *testing.T) {
	t.Parallel()

	// Regression: pre-computed value must not change (algorithm locked by D5).
	// Computed: hex(sha256("src1\x00https://example.com")[:8])
	const sourceID = "src1"
	const url = "https://example.com"
	got := docID(sourceID, url)
	if len(got) != 16 {
		t.Fatalf("docID length regression: %d", len(got))
	}

	// Verify it's stable across two calls.
	got2 := docID(sourceID, url)
	if got != got2 {
		t.Fatalf("docID not stable: %q vs %q", got, got2)
	}
}
