package naver

import (
	"testing"
	"time"
)

// TestParseRetryAfter tests the parseRetryAfter helper with various header values.
// REQ-ADP8-003: Retry-After header parsing.
func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"empty header", "", defaultRetryAfter},
		{"integer 30s", "30", 30 * time.Second},
		{"integer 0 uses default", "0", defaultRetryAfter},
		{"integer negative uses default", "-1", defaultRetryAfter},
		{"integer exceeds cap", "120", maxRetryAfter},
		{"integer at cap", "60", maxRetryAfter},
		{"integer below cap", "45", 45 * time.Second},
		{"malformed string", "abc", defaultRetryAfter},
		// HTTP-date form: 30 seconds in the future.
		// http.ParseTime requires "GMT" suffix, not "UTC".
		{"http-date future", now.Add(30 * time.Second).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"), 30 * time.Second},
		// HTTP-date form: past date uses default.
		{"http-date past", now.Add(-10 * time.Second).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"), defaultRetryAfter},
		// HTTP-date form: exceeds cap.
		{"http-date exceeds cap", now.Add(2 * time.Minute).UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"), maxRetryAfter},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseRetryAfter(tc.header, now)
			if got != tc.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}
