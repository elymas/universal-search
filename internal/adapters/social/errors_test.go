// Package social — tests for error sentinels and parseRetryAfter.
// REQ-ADP6-003: parseRetryAfter implements RFC 7231 §7.1.3.
package social

import (
	"errors"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestErrXDisabledIsSentinel verifies ErrXDisabled is a distinct sentinel.
func TestErrXDisabledIsSentinel(t *testing.T) {
	t.Parallel()
	if ErrXDisabled == nil {
		t.Fatal("ErrXDisabled must not be nil")
	}
	if errors.Is(ErrXDisabled, ErrXProviderNotConfigured) {
		t.Fatal("ErrXDisabled and ErrXProviderNotConfigured must be distinct")
	}
}

// TestErrXProviderNotConfiguredIsSentinel verifies ErrXProviderNotConfigured is a distinct sentinel.
func TestErrXProviderNotConfiguredIsSentinel(t *testing.T) {
	t.Parallel()
	if ErrXProviderNotConfigured == nil {
		t.Fatal("ErrXProviderNotConfigured must not be nil")
	}
}

// TestErrXDisabledIsSourceError verifies ErrXDisabled wraps into a *types.SourceError.
func TestErrXDisabledIsSourceError(t *testing.T) {
	t.Parallel()
	var se *types.SourceError
	if !errors.As(ErrXDisabled, &se) {
		t.Fatal("ErrXDisabled must be or wrap *types.SourceError")
	}
	if se.Category != types.CategoryPermanent {
		t.Fatalf("ErrXDisabled category: got %v, want %v", se.Category, types.CategoryPermanent)
	}
}

// TestErrXProviderNotConfiguredIsSourceError verifies ErrXProviderNotConfigured wraps into a *types.SourceError.
func TestErrXProviderNotConfiguredIsSourceError(t *testing.T) {
	t.Parallel()
	var se *types.SourceError
	if !errors.As(ErrXProviderNotConfigured, &se) {
		t.Fatal("ErrXProviderNotConfigured must be or wrap *types.SourceError")
	}
	if se.Category != types.CategoryPermanent {
		t.Fatalf("ErrXProviderNotConfigured category: got %v, want %v", se.Category, types.CategoryPermanent)
	}
}

// TestParseRetryAfterTable tests parseRetryAfter per RFC 7231 §7.1.3.
func TestParseRetryAfterTable(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{
			name:   "empty header returns default",
			header: "",
			want:   defaultRetryAfter,
		},
		{
			name:   "integer 30 returns 30s",
			header: "30",
			want:   30 * time.Second,
		},
		{
			name:   "integer 0 returns default",
			header: "0",
			want:   defaultRetryAfter,
		},
		{
			name:   "negative integer returns default",
			header: "-5",
			want:   defaultRetryAfter,
		},
		{
			name:   "integer exceeding cap returns 60s",
			header: "120",
			want:   maxRetryAfter,
		},
		{
			name:   "exact cap integer returns 60s",
			header: "60",
			want:   maxRetryAfter,
		},
		{
			name:   "HTTP-date 30s in future",
			header: "Wed, 15 Apr 2026 12:00:30 GMT",
			want:   30 * time.Second,
		},
		{
			name:   "HTTP-date in the past returns default",
			header: "Wed, 15 Apr 2026 11:59:00 GMT",
			want:   defaultRetryAfter,
		},
		{
			name:   "HTTP-date exceeds cap returns 60s",
			header: "Wed, 15 Apr 2026 12:02:00 GMT",
			want:   maxRetryAfter,
		},
		{
			name:   "malformed string returns default",
			header: "not-a-duration",
			want:   defaultRetryAfter,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseRetryAfter(tc.header, now)
			if got != tc.want {
				t.Errorf("parseRetryAfter(%q): got %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}
