package arxiv

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestCategorizeStatusTable verifies the HTTP status -> Category truth table.
func TestCategorizeStatusTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status           int
		expectedCategory types.Category
	}{
		{200, types.CategoryUnknown}, // unexpected (Search drains 200)
		{400, types.CategoryPermanent},
		{401, types.CategoryPermanent},
		{403, types.CategoryPermanent},
		{404, types.CategoryPermanent},
		{429, types.CategoryRateLimited},
		{500, types.CategoryUnavailable},
		{503, types.CategoryUnavailable},
		{0, types.CategoryUnavailable}, // network-layer error
	}

	for _, tc := range tests {
		se := categorizeStatus(tc.status, 5*time.Second, errors.New("cause"))
		if se.Category != tc.expectedCategory {
			t.Errorf("categorizeStatus(%d) category = %v, want %v", tc.status, se.Category, tc.expectedCategory)
		}
		if se.Adapter != "arxiv" {
			t.Errorf("categorizeStatus(%d) Adapter = %q, want %q", tc.status, se.Adapter, "arxiv")
		}
		if se.HTTPStatus != tc.status {
			t.Errorf("categorizeStatus(%d) HTTPStatus = %d, want %d", tc.status, se.HTTPStatus, tc.status)
		}
	}

	// 429 with retryAfter
	se429 := categorizeStatus(429, 30*time.Second, errors.New("rate limited"))
	if se429.RetryAfter != 30*time.Second {
		t.Errorf("categorizeStatus(429) RetryAfter = %v, want 30s", se429.RetryAfter)
	}
}

// TestParseRetryAfterTable verifies the Retry-After parser over various input shapes.
func TestParseRetryAfterTable(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 21, 7, 27, 30, 0, time.UTC) // 30s before the HTTP-date below

	tests := []struct {
		name    string
		header  string
		wantMin time.Duration
		wantMax time.Duration
	}{
		{
			name:    "integer seconds",
			header:  "30",
			wantMin: 30 * time.Second,
			wantMax: 30 * time.Second,
		},
		{
			name:    "HTTP date 30s in future",
			header:  "Wed, 21 Oct 2026 07:28:00 GMT",
			wantMin: 25 * time.Second,
			wantMax: 35 * time.Second,
		},
		{
			name:    "missing header",
			header:  "",
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
		{
			name:    "malformed header",
			header:  "not-a-date-or-number",
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
		{
			name:    "value exceeds 60s cap",
			header:  "999",
			wantMin: 60 * time.Second,
			wantMax: 60 * time.Second,
		},
		{
			name:    "negative value",
			header:  "-10",
			wantMin: 5 * time.Second,
			wantMax: 5 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRetryAfter(tc.header, now)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("parseRetryAfter(%q) = %v, want [%v, %v]", tc.header, got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

// TestRedirectAllowlistAllowedHosts verifies allowed hosts pass the check.
func TestRedirectAllowlistAllowedHosts(t *testing.T) {
	t.Parallel()

	allowed := []string{
		"export.arxiv.org",
		"arxiv.org",
	}
	fn := redirectAllowlist()
	for _, host := range allowed {
		if err := fn(host); err != nil {
			t.Errorf("redirectAllowlist()(%q) = %v, want nil", host, err)
		}
	}
}

// TestRedirectAllowlistBlockedHosts verifies disallowed hosts produce errors.
func TestRedirectAllowlistBlockedHosts(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"attacker.com",
		"evil.org",
		"export.arxiv.org.evil.com",
	}
	fn := redirectAllowlist()
	for _, host := range blocked {
		if err := fn(host); err == nil {
			t.Errorf("redirectAllowlist()(%q) = nil, want error", host)
		} else if !strings.Contains(err.Error(), "cross-domain redirect") {
			t.Errorf("redirectAllowlist()(%q) error = %q, want to contain %q", host, err.Error(), "cross-domain redirect")
		}
	}
}
