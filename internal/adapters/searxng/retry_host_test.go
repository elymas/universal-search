package searxng

// Coverage for the HTTP-date branch of parseRetryAfter and for
// healthcheckHostFromBase port/scheme derivation. The integer-seconds branches
// of parseRetryAfter are exercised through the 429 response path in
// client_test.go; these tests reach the date-parsing and host-derivation paths
// directly.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("future date within cap", func(t *testing.T) {
		future := now.Add(20 * time.Second)
		hdr := future.UTC().Format(http.TimeFormat)
		got := parseRetryAfter(hdr, now)
		// Allow ±2s slop from second-granularity formatting.
		if got < 18*time.Second || got > 22*time.Second {
			t.Errorf("parseRetryAfter(date+20s) = %v, want ~20s", got)
		}
	})

	t.Run("far-future date capped at max", func(t *testing.T) {
		future := now.Add(10 * time.Minute)
		hdr := future.UTC().Format(http.TimeFormat)
		if got := parseRetryAfter(hdr, now); got != maxRetryAfter {
			t.Errorf("parseRetryAfter(date+10m) = %v, want cap %v", got, maxRetryAfter)
		}
	})

	t.Run("past date returns default", func(t *testing.T) {
		past := now.Add(-1 * time.Hour)
		hdr := past.UTC().Format(http.TimeFormat)
		if got := parseRetryAfter(hdr, now); got != defaultRetryAfter {
			t.Errorf("parseRetryAfter(past date) = %v, want default %v", got, defaultRetryAfter)
		}
	})

	t.Run("unparseable header returns default", func(t *testing.T) {
		if got := parseRetryAfter("not-a-date", now); got != defaultRetryAfter {
			t.Errorf("parseRetryAfter(garbage) = %v, want default %v", got, defaultRetryAfter)
		}
	})
}

func TestHealthcheckHostFromBase(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{"explicit host:port", "http://searxng:8080", "searxng:8080"},
		{"https default port", "https://search.example.com", "search.example.com:443"},
		{"http default port", "http://search.example.com", "search.example.com:80"},
		{"unparseable falls back", "://bad-url", "searxng:8080"},
		{"empty host falls back", "no-scheme-no-host", "searxng:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := healthcheckHostFromBase(tt.base); got != tt.want {
				t.Errorf("healthcheckHostFromBase(%q) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}
