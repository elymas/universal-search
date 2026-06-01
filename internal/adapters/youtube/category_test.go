package youtube

// Coverage for sidecarCategoryToType: maps each sidecar error-category string
// to the canonical types.Category, defaulting to Unavailable for unknown values.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

func TestSidecarCategoryToType(t *testing.T) {
	tests := []struct {
		in   string
		want types.Category
	}{
		{"rate_limited", types.CategoryRateLimited},
		{"permanent", types.CategoryPermanent},
		{"transient", types.CategoryTransient},
		{"unavailable", types.CategoryUnavailable},
		{"", types.CategoryUnavailable},        // default branch
		{"garbage", types.CategoryUnavailable}, // default branch
	}
	for _, tt := range tests {
		if got := sidecarCategoryToType(tt.in); got != tt.want {
			t.Errorf("sidecarCategoryToType(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
