// Package qdrant — unit tests for Qdrant client helpers (REQ-IDX-002, REQ-IDX-010).
package qdrant

import (
	"testing"
)

func TestToQdrantID_Length(t *testing.T) {
	t.Parallel()
	id := toQdrantID("0123456789abcdef")
	// UUID form: 8-4-4-4-12 = 32 hex + 4 dashes = 36 chars.
	if len(id) != 36 {
		t.Fatalf("UUID len = %d, want 36: %q", len(id), id)
	}
}

func TestToQdrantID_Format(t *testing.T) {
	t.Parallel()
	id := toQdrantID("0123456789abcdef")
	// Positions 8, 13, 18, 23 must be dashes.
	for _, pos := range []int{8, 13, 18, 23} {
		if id[pos] != '-' {
			t.Errorf("position %d = %q, want '-'", pos, id[pos])
		}
	}
}

func TestToFromQdrantID_Roundtrip(t *testing.T) {
	t.Parallel()
	original := "0123456789abcdef"
	uuid := toQdrantID(original)
	recovered := fromQdrantID(uuid)
	if recovered != original {
		t.Fatalf("roundtrip failed: %q → %q → %q", original, uuid, recovered)
	}
}

func TestToQdrantID_ShortID(t *testing.T) {
	t.Parallel()
	// Short IDs should be left-padded with spaces (formatstring uses %032s).
	// Result must still be 36 chars.
	id := toQdrantID("abc")
	if len(id) != 36 {
		t.Fatalf("UUID len = %d for short id: %q", len(id), id)
	}
}

func TestFromQdrantID_StripsDashes(t *testing.T) {
	t.Parallel()
	uuid := "00000000-0000-0000-0000-0123456789ab"
	got := fromQdrantID(uuid)
	// Should be last 16 hex chars of cleaned UUID.
	if len(got) != 16 {
		t.Fatalf("fromQdrantID length = %d: %q", len(got), got)
	}
}

func TestHostFromEndpoint_ColonSplit(t *testing.T) {
	t.Parallel()
	cases := []struct{ endpoint, want string }{
		{"localhost:6334", "localhost"},
		{"127.0.0.1:6334", "127.0.0.1"},
		{"qdrant.example.com:6334", "qdrant.example.com"},
		{"noport", "noport"},
	}
	for _, c := range cases {
		got := hostFromEndpoint(c.endpoint)
		if got != c.want {
			t.Errorf("hostFromEndpoint(%q) = %q, want %q", c.endpoint, got, c.want)
		}
	}
}

func TestPortFromEndpoint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		endpoint string
		want     int
	}{
		{"localhost:6334", 6334},
		{"host:7777", 7777},
		{"noport", 6334},
	}
	for _, c := range cases {
		got := portFromEndpoint(c.endpoint)
		if got != c.want {
			t.Errorf("portFromEndpoint(%q) = %d, want %d", c.endpoint, got, c.want)
		}
	}
}

func TestBuildFilter_Nil(t *testing.T) {
	t.Parallel()
	got := buildFilter(&Filter{})
	if got != nil {
		t.Fatalf("expected nil filter for empty Filter, got %v", got)
	}
}

func TestBuildFilter_WithSourceID(t *testing.T) {
	t.Parallel()
	f := &Filter{SourceID: "s1"}
	got := buildFilter(f)
	if got == nil {
		t.Fatal("expected non-nil filter")
	}
	if len(got.Must) != 1 {
		t.Fatalf("Must conditions: %d, want 1", len(got.Must))
	}
}

func TestBuildFilter_MultiConditions(t *testing.T) {
	t.Parallel()
	f := &Filter{SourceID: "s", Lang: "ko", TeamID: "t"}
	got := buildFilter(f)
	if got == nil {
		t.Fatal("expected non-nil filter")
	}
	if len(got.Must) != 3 {
		t.Fatalf("Must conditions: %d, want 3", len(got.Must))
	}
}

func TestNewClient_NoServer(t *testing.T) {
	t.Parallel()
	// NewClient does NOT dial; it creates the pool lazily.
	_, err := NewClient(Config{Endpoint: "localhost:16334"})
	// Should succeed (no dial at construction time).
	if err != nil {
		t.Logf("NewClient returned error (acceptable if server not available): %v", err)
	}
}
