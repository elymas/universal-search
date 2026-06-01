package korean

// Direct coverage for the pure validation/mapping helpers: isValidQueryID
// (KR-NNN format) and verticalForDocType (DocType → Naver vertical mapping).
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

func TestIsValidQueryID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"KR-001", true},
		{"KR-999", true},
		{"KR-01", false},   // too few digits
		{"KR-0001", false}, // too many digits
		{"KR-12a", false},  // non-digit
		{"XX-001", false},  // wrong prefix
		{"001", false},     // no prefix
		{"", false},        // empty
		{"KR-", false},     // missing digits
	}
	for _, tt := range tests {
		if got := isValidQueryID(tt.id); got != tt.want {
			t.Errorf("isValidQueryID(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestVerticalForDocType(t *testing.T) {
	tests := []struct {
		dt           types.DocType
		wantVertical string
		wantUnambig  bool
	}{
		{types.DocTypePost, "blog", true},
		{types.DocTypeArticle, "news", true},
		{types.DocTypeOther, "", false},
		{types.DocTypePaper, "", false}, // any non-post/article -> ambiguous
	}
	for _, tt := range tests {
		gotV, gotU := verticalForDocType(tt.dt)
		if gotV != tt.wantVertical || gotU != tt.wantUnambig {
			t.Errorf("verticalForDocType(%v) = (%q, %v), want (%q, %v)",
				tt.dt, gotV, gotU, tt.wantVertical, tt.wantUnambig)
		}
	}
}
