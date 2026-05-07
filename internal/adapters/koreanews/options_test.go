// Package koreanews — Options, applyDefaults, capFeeds, parseRSSFeedsEnv, OptionsFromEnv tests.
// SPEC-ADP-009 REQ-ADP9-002.
package koreanews

import (
	"os"
	"testing"
	"time"
)

func TestApplyDefaults_zeroOptions(t *testing.T) {
	t.Parallel()
	opts := Options{}
	opts.applyDefaults()

	if !opts.RSSEnabled {
		t.Error("RSSEnabled should be true for zero Options")
	}
	if opts.RSSPerFeedTimeout != defaultRSSPerFeedTimeout {
		t.Errorf("RSSPerFeedTimeout = %v; want %v", opts.RSSPerFeedTimeout, defaultRSSPerFeedTimeout)
	}
	if opts.KNCBaseURL != defaultKNCBaseURL {
		t.Errorf("KNCBaseURL = %q; want %q", opts.KNCBaseURL, defaultKNCBaseURL)
	}
	if opts.MaxParallelFeeds != defaultMaxParallelFeeds {
		t.Errorf("MaxParallelFeeds = %d; want %d", opts.MaxParallelFeeds, defaultMaxParallelFeeds)
	}
	if opts.UserAgentVersion != defaultUAVersion {
		t.Errorf("UserAgentVersion = %q; want %q", opts.UserAgentVersion, defaultUAVersion)
	}
	if opts.HealthcheckTarget != defaultHealthcheckTarget {
		t.Errorf("HealthcheckTarget = %q; want %q", opts.HealthcheckTarget, defaultHealthcheckTarget)
	}
	if opts.NowFunc == nil {
		t.Error("NowFunc should not be nil after applyDefaults")
	}
}

func TestApplyDefaults_explicitValuesPreserved(t *testing.T) {
	t.Parallel()
	customTimeout := 5 * time.Second
	opts := Options{
		RSSEnabled:        true,
		RSSPerFeedTimeout: customTimeout,
		KNCBaseURL:        "http://custom:9999",
		MaxParallelFeeds:  4,
	}
	opts.applyDefaults()

	if opts.RSSPerFeedTimeout != customTimeout {
		t.Errorf("RSSPerFeedTimeout overridden: got %v; want %v", opts.RSSPerFeedTimeout, customTimeout)
	}
	if opts.KNCBaseURL != "http://custom:9999" {
		t.Errorf("KNCBaseURL overridden: got %q", opts.KNCBaseURL)
	}
	if opts.MaxParallelFeeds != 4 {
		t.Errorf("MaxParallelFeeds overridden: got %d; want 4", opts.MaxParallelFeeds)
	}
}

func TestApplyDefaults_explicitRSSDisabled(t *testing.T) {
	t.Parallel()
	// When caller explicitly sets DaumEnabled=true, the zero-RSS check must not
	// forcibly enable RSS.
	opts := Options{DaumEnabled: true}
	opts.applyDefaults()
	// RSSEnabled should remain false because the caller expressed intent.
	if opts.RSSEnabled {
		t.Error("RSSEnabled should stay false when DaumEnabled is explicitly set")
	}
}

func TestCapFeeds_underLimit(t *testing.T) {
	t.Parallel()
	feeds := make([]string, maxRSSFeeds)
	for i := range feeds {
		feeds[i] = "https://feed.example.com"
	}
	got := capFeeds(feeds)
	if len(got) != maxRSSFeeds {
		t.Errorf("capFeeds: got %d; want %d", len(got), maxRSSFeeds)
	}
}

func TestCapFeeds_overLimit(t *testing.T) {
	t.Parallel()
	feeds := make([]string, maxRSSFeeds+5)
	got := capFeeds(feeds)
	if len(got) != maxRSSFeeds {
		t.Errorf("capFeeds over limit: got %d; want %d", len(got), maxRSSFeeds)
	}
}

func TestParseRSSFeedsEnv_jsonArray(t *testing.T) {
	t.Parallel()
	v := `["https://a.com/feed", "https://b.com/feed"]`
	feeds, err := parseRSSFeedsEnv(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feeds) != 2 {
		t.Errorf("got %d feeds; want 2", len(feeds))
	}
	if feeds[0] != "https://a.com/feed" {
		t.Errorf("feeds[0] = %q; want https://a.com/feed", feeds[0])
	}
}

func TestParseRSSFeedsEnv_commaSeparated(t *testing.T) {
	t.Parallel()
	v := "https://a.com/feed, https://b.com/feed, https://c.com/feed"
	feeds, err := parseRSSFeedsEnv(v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(feeds) != 3 {
		t.Errorf("got %d feeds; want 3", len(feeds))
	}
}

func TestParseRSSFeedsEnv_emptyString(t *testing.T) {
	t.Parallel()
	feeds, err := parseRSSFeedsEnv("")
	if err != nil || len(feeds) != 0 {
		t.Errorf("parseRSSFeedsEnv(\"\") = (%v, %v); want ([], nil)", feeds, err)
	}
}

func TestParseRSSFeedsEnv_invalidJSON(t *testing.T) {
	t.Parallel()
	_, err := parseRSSFeedsEnv("[invalid json")
	if err == nil {
		t.Error("expected error for invalid JSON array, got nil")
	}
}

func TestParseRSSFeedsEnv_whitespaceItems(t *testing.T) {
	t.Parallel()
	v := "https://a.com, , https://b.com, "
	feeds, _ := parseRSSFeedsEnv(v)
	if len(feeds) != 2 {
		t.Errorf("got %d feeds (expected empty strings stripped); want 2", len(feeds))
	}
}

func TestOptionsFromEnv_defaults(t *testing.T) {
	// Not parallel: modifies env vars.
	os.Unsetenv("USEARCH_ADP009_RSS_ENABLED")
	os.Unsetenv("USEARCH_ADP009_RSS_FEEDS")
	os.Unsetenv("USEARCH_ADP009_DAUM_ENABLED")
	os.Unsetenv("USEARCH_ADP009_KNC_ENABLED")
	os.Unsetenv("USEARCH_ADP009_KNC_BASE_URL")

	opts := OptionsFromEnv()
	if !opts.RSSEnabled {
		t.Error("OptionsFromEnv: RSSEnabled should be true when env is unset")
	}
	if opts.DaumEnabled {
		t.Error("OptionsFromEnv: DaumEnabled should be false when env is unset")
	}
	if opts.KNCEnabled {
		t.Error("OptionsFromEnv: KNCEnabled should be false when env is unset")
	}
}

func TestOptionsFromEnv_disableRSS(t *testing.T) {
	t.Setenv("USEARCH_ADP009_RSS_ENABLED", "false")
	opts := OptionsFromEnv()
	if opts.RSSEnabled {
		t.Error("RSSEnabled should be false when USEARCH_ADP009_RSS_ENABLED=false")
	}
}

func TestOptionsFromEnv_enableKNC(t *testing.T) {
	t.Setenv("USEARCH_ADP009_KNC_ENABLED", "true")
	t.Setenv("USEARCH_ADP009_KNC_BASE_URL", "http://sidecar:9000")
	opts := OptionsFromEnv()
	if !opts.KNCEnabled {
		t.Error("KNCEnabled should be true")
	}
	if opts.KNCBaseURL != "http://sidecar:9000" {
		t.Errorf("KNCBaseURL = %q; want http://sidecar:9000", opts.KNCBaseURL)
	}
}
