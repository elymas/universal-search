// Package koreanews — Options configuration for the composite adapter.
// SPEC-ADP-009: Options, defaults, env-var loaders, RSS feed list parsing.
package koreanews

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// maxRSSFeeds caps the number of feed URLs the adapter will fetch.
	// Entries beyond this limit are truncated with a slog WARN.
	maxRSSFeeds = 32

	// defaultRSSPerFeedTimeout is the per-feed fetch timeout.
	defaultRSSPerFeedTimeout = 30 * time.Second

	// defaultMaxParallelFeeds is the maximum concurrent feed fetches.
	defaultMaxParallelFeeds = 8

	// defaultKNCBaseURL is the default KoreaNewsCrawler sidecar HTTP address.
	defaultKNCBaseURL = "http://localhost:8002"

	// defaultUAVersion is the fallback version string.
	defaultUAVersion = "v0.1"

	// defaultHealthcheckTarget is the default healthcheck endpoint.
	defaultHealthcheckTarget = "https://www.yna.co.kr/rss/all.xml"
)

// Options configures the Korean news composite adapter.
// All fields are optional; zero values fall back to documented defaults.
type Options struct {
	// RSSEnabled enables the RSS sub-source (default true).
	// Set USEARCH_ADP009_RSS_ENABLED=false to disable.
	RSSEnabled bool

	// RSSFeeds is the operator-configured list of feed URLs (max 32).
	// Populated via USEARCH_ADP009_RSS_FEEDS env var (JSON array or comma-list).
	RSSFeeds []string

	// RSSPerFeedTimeout is the per-feed fetch timeout (default 30s).
	RSSPerFeedTimeout time.Duration

	// DaumEnabled enables the Daum sub-source (default false).
	// In v0.1, enabling Daum only routes to the stub returning ErrDaumDisabled.
	// Set USEARCH_ADP009_DAUM_ENABLED=true to plumb (not activate) the path.
	DaumEnabled bool

	// KNCEnabled enables the KoreaNewsCrawler sub-source (default false).
	// Set USEARCH_ADP009_KNC_ENABLED=true to enable sidecar calls.
	KNCEnabled bool

	// KNCBaseURL is the sidecar HTTP base address (default "http://localhost:8002").
	// Set USEARCH_ADP009_KNC_BASE_URL to override.
	KNCBaseURL string

	// MaxParallelFeeds caps concurrent feed fetches (default 8).
	MaxParallelFeeds int

	// HTTPClient overrides the default *http.Client. Primarily used in tests.
	HTTPClient *http.Client

	// UserAgentVersion overrides the version component of the User-Agent header.
	// Default: "v0.1".
	UserAgentVersion string

	// HealthcheckTarget overrides the healthcheck URL. Primarily used in tests.
	HealthcheckTarget string

	// NowFunc overrides time.Now() for deterministic test timestamps.
	NowFunc func() time.Time
}

// applyDefaults fills zero fields with their documented defaults.
func (o *Options) applyDefaults() {
	if !o.RSSEnabled && o.RSSFeeds == nil && !o.DaumEnabled && !o.KNCEnabled {
		// If caller passed a zero Options, default RSS to enabled.
		// This is the typical construction path for a default adapter.
		o.RSSEnabled = true
	}
	if o.RSSPerFeedTimeout == 0 {
		o.RSSPerFeedTimeout = defaultRSSPerFeedTimeout
	}
	if o.KNCBaseURL == "" {
		o.KNCBaseURL = defaultKNCBaseURL
	}
	if o.MaxParallelFeeds == 0 {
		o.MaxParallelFeeds = defaultMaxParallelFeeds
	}
	if o.UserAgentVersion == "" {
		o.UserAgentVersion = defaultUAVersion
	}
	if o.HealthcheckTarget == "" {
		o.HealthcheckTarget = defaultHealthcheckTarget
	}
	if o.NowFunc == nil {
		o.NowFunc = func() time.Time { return time.Now().UTC() }
	}
}

// capFeeds enforces the maxRSSFeeds ceiling, logging a WARN when truncation
// occurs per REQ-ADP9-002 (D3 RSS feed list configuration surface).
func capFeeds(feeds []string) []string {
	if len(feeds) <= maxRSSFeeds {
		return feeds
	}
	slog.Warn("koreanews: RSS feed list truncated to max",
		slog.Int("original", len(feeds)),
		slog.Int("capped", maxRSSFeeds),
		slog.Int("max", maxRSSFeeds),
	)
	return feeds[:maxRSSFeeds]
}

// OptionsFromEnv constructs Options by reading the four USEARCH_ADP009_*
// environment variables. It is a convenience helper for production construction;
// tests usually pass Options directly.
func OptionsFromEnv() Options {
	opts := Options{}

	// USEARCH_ADP009_RSS_ENABLED (default true when unset)
	if v := os.Getenv("USEARCH_ADP009_RSS_ENABLED"); v == "false" {
		opts.RSSEnabled = false
	} else {
		opts.RSSEnabled = true
	}

	// USEARCH_ADP009_RSS_FEEDS — JSON array or comma-separated list
	if v := os.Getenv("USEARCH_ADP009_RSS_FEEDS"); v != "" {
		feeds, err := parseRSSFeedsEnv(v)
		if err != nil {
			slog.Warn("koreanews: failed to parse USEARCH_ADP009_RSS_FEEDS",
				slog.String("error", err.Error()),
			)
		} else {
			opts.RSSFeeds = capFeeds(feeds)
		}
	}

	// USEARCH_ADP009_DAUM_ENABLED
	opts.DaumEnabled = os.Getenv("USEARCH_ADP009_DAUM_ENABLED") == "true"

	// USEARCH_ADP009_KNC_ENABLED
	opts.KNCEnabled = os.Getenv("USEARCH_ADP009_KNC_ENABLED") == "true"

	// USEARCH_ADP009_KNC_BASE_URL
	if v := os.Getenv("USEARCH_ADP009_KNC_BASE_URL"); v != "" {
		opts.KNCBaseURL = v
	}

	return opts
}

// parseRSSFeedsEnv parses the USEARCH_ADP009_RSS_FEEDS value.
// It tries JSON array first, then falls back to comma-separated list.
func parseRSSFeedsEnv(v string) ([]string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}

	// Try JSON array first.
	if strings.HasPrefix(v, "[") {
		var feeds []string
		if err := json.Unmarshal([]byte(v), &feeds); err != nil {
			return nil, fmt.Errorf("koreanews: JSON parse failed for RSS feeds: %w", err)
		}
		var result []string
		for _, f := range feeds {
			f = strings.TrimSpace(f)
			if f != "" {
				result = append(result, f)
			}
		}
		return result, nil
	}

	// Comma-separated fallback.
	parts := strings.Split(v, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result, nil
}
