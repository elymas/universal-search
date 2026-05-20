// Package access — Options struct with defaults and validation.
//
// REQ-CACHE-001 §6.6: documented defaults and validation in New.
// D4: Phase1=100ms, Phase2=200ms, Phase3=10s, Phase4=15s, Phase5=30s
package access

import "time"

const (
	defaultMaxBrowsers     = 2
	defaultRobotsTTL       = 24 * time.Hour
	defaultMaxBodyBytes    = 10 * 1024 * 1024 // 10 MB
	defaultRedirectMaxHops = 5
)

// defaultPerPhaseTimeout is the D4 budget table from SPEC-CACHE-001 §6.6.
//
// @MX:NOTE: [AUTO] Magic constants — per-phase timeout defaults from SPEC-CACHE-001 §6.6 D4.
// Operators tune via .moai/config/sections/access.yaml.
var defaultPerPhaseTimeout = map[int]time.Duration{
	1: 100 * time.Millisecond,  // Phase 1: local index lookup
	2: 200 * time.Millisecond,  // Phase 2: HEAD probe + robots.txt
	3: 10 * time.Second,        // Phase 3: standard HTTP GET
	4: 15 * time.Second,        // Phase 4: TLS-aware HTTP GET
	5: 30 * time.Second,        // Phase 5: Playwright headless
}

// PlaywrightConfig holds Playwright-specific construction parameters.
type PlaywrightConfig struct {
	// BrowserType is one of "chromium", "firefox", "webkit". Default "chromium".
	BrowserType string
}

// Options holds the fetcher-level (constructor-time) configuration.
// Zero values are replaced by the documented defaults in New.
type Options struct {
	// Playwright holds Playwright-specific configuration.
	Playwright PlaywrightConfig
	// IndexLookup is the optional port to the document index (SPEC-IDX-001).
	// Nil → Phase 1 is skipped.
	IndexLookup IndexLookup

	// Obs is the observability bundle. Nil → all metrics/logging are no-ops.
	// @MX:NOTE: [AUTO] Obs is nil-safe; all emission helpers guard against nil.
	Obs ObsAdapter

	// MaxBrowsers is the browser pool size for Phase 5.
	MaxBrowsers int
	// PerPhaseTimeout overrides the D4 defaults. Nil → defaults applied.
	PerPhaseTimeout map[int]time.Duration
	// RobotsTTL is the per-host robots.txt cache TTL. Zero → 24h.
	RobotsTTL time.Duration
	// MaxBodyBytes caps the response body size. Zero → 10 MB.
	MaxBodyBytes int64
	// RedirectMaxHops caps redirect chains. Zero → 5.
	RedirectMaxHops int
	// AllowPrivateNetworks permits fetching RFC1918/loopback addresses.
	// MUST be false in production; true only for httptest.Server stubs.
	AllowPrivateNetworks bool
	// CacheWriteThrough enables async index upsert after Phase 3-5 success.
	CacheWriteThrough bool
	// PlaywrightEnabled activates Phase 5. Default false (opt-in).
	PlaywrightEnabled bool
	// AutoInstallPlaywright calls playwright.Install() automatically at New.
	AutoInstallPlaywright bool
}

// applyDefaults fills zero-valued fields with documented defaults.
func (o *Options) applyDefaults() {
	if o.MaxBrowsers <= 0 {
		o.MaxBrowsers = defaultMaxBrowsers
	}
	if len(o.PerPhaseTimeout) == 0 {
		o.PerPhaseTimeout = make(map[int]time.Duration, 5)
		for k, v := range defaultPerPhaseTimeout {
			o.PerPhaseTimeout[k] = v
		}
	}
	if o.RobotsTTL == 0 {
		o.RobotsTTL = defaultRobotsTTL
	}
	if o.MaxBodyBytes == 0 {
		o.MaxBodyBytes = defaultMaxBodyBytes
	}
	if o.RedirectMaxHops == 0 {
		o.RedirectMaxHops = defaultRedirectMaxHops
	}
	if o.Playwright.BrowserType == "" {
		o.Playwright.BrowserType = "chromium"
	}
}
