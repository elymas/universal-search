package metrics

// Coverage for statusClass: maps HTTP status codes to their class label, across
// all four buckets plus the 5xx default branch.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import "testing"

func TestStatusClass(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{200, "2xx"},
		{204, "2xx"},
		{301, "3xx"},
		{399, "3xx"},
		{404, "4xx"},
		{429, "4xx"},
		{500, "5xx"},
		{503, "5xx"},
		{100, "5xx"}, // below 200 falls into the default branch
		{600, "5xx"}, // above 599 falls into the default branch
	}
	for _, tt := range tests {
		if got := statusClass(tt.code); got != tt.want {
			t.Errorf("statusClass(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}
