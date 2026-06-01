package orchestrator

// Direct coverage for intersectSources: empty filter returns the full set,
// non-empty filter intersects (with whitespace trimming), and a disjoint filter
// yields an empty result.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"reflect"
	"testing"
)

func TestIntersectSources(t *testing.T) {
	tests := []struct {
		name       string
		adapterSet []string
		filter     []string
		want       []string
	}{
		{
			name:       "empty filter returns full set",
			adapterSet: []string{"reddit", "github"},
			filter:     nil,
			want:       []string{"reddit", "github"},
		},
		{
			name:       "filter intersects with set",
			adapterSet: []string{"reddit", "github", "arxiv"},
			filter:     []string{"github", "arxiv"},
			want:       []string{"github", "arxiv"},
		},
		{
			name:       "whitespace in filter is trimmed",
			adapterSet: []string{"reddit", "github"},
			filter:     []string{" github "},
			want:       []string{"github"},
		},
		{
			name:       "disjoint filter yields empty",
			adapterSet: []string{"reddit"},
			filter:     []string{"github"},
			want:       nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intersectSources(tt.adapterSet, tt.filter)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("intersectSources(%v, %v) = %v, want %v", tt.adapterSet, tt.filter, got, tt.want)
			}
		})
	}
}
