// Package hn provides a Hacker News search adapter implementing types.Adapter.
// REQ-ADP2-001: Interface conformance, Name, Capabilities, Healthcheck.
// SPEC-ADP-002: Algolia HN Search public no-auth endpoint (https://hn.algolia.com/api).
package hn

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	// defaultBaseURL is the Algolia HN Search relevance-ranked endpoint.
	defaultBaseURL = "https://hn.algolia.com/api/v1/search"

	// defaultUserAgentTemplate is the User-Agent header format string.
	// %s is replaced with the version string from Options.UserAgentVersion.
	defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"

	// defaultUAVersion is the fallback version when Options.UserAgentVersion is empty.
	defaultUAVersion = "v0.1"

	// defaultHealthcheckTarget is the TCP address for the Healthcheck dial.
	// Tests inject a loopback address via Options.HealthcheckTarget.
	defaultHealthcheckTarget = "hn.algolia.com:443"
)

// Options configures the HN adapter. All fields are optional; defaults are
// used when a field is the zero value.
type Options struct {
	// BaseURL overrides the Algolia HN search endpoint. Primarily used in tests
	// to redirect requests to an httptest.Server.
	BaseURL string

	// HTTPClient overrides the default *http.Client (10s timeout, redirect
	// allowlist, reqid transport). Primarily used in tests.
	HTTPClient *http.Client

	// UserAgentVersion overrides the version component of the User-Agent header.
	// Default: "v0.1".
	UserAgentVersion string

	// HealthcheckTarget overrides the TCP address for Healthcheck.
	// Default: "hn.algolia.com:443". Tests inject a loopback address.
	HealthcheckTarget string
}

// Adapter is the Hacker News search adapter. It implements types.Adapter.
// All fields are unexported and immutable after construction (REQ-ADP2-010).
type Adapter struct {
	httpClient        *http.Client
	baseURL           string
	userAgent         string
	healthcheckTarget string
}

// New constructs an HN Adapter from the given Options. Returns an error if
// the Options are inconsistent (currently always nil; reserved for future
// validation).
func New(opts Options) (*Adapter, error) {
	base := opts.BaseURL
	if base == "" {
		base = defaultBaseURL
	}

	version := opts.UserAgentVersion
	if version == "" {
		version = defaultUAVersion
	}
	ua := fmt.Sprintf(defaultUserAgentTemplate, version)

	client := opts.HTTPClient
	if client == nil {
		client = newDefaultClient()
	}

	target := opts.HealthcheckTarget
	if target == "" {
		target = defaultHealthcheckTarget
	}

	return &Adapter{
		httpClient:        client,
		baseURL:           base,
		userAgent:         ua,
		healthcheckTarget: target,
	}, nil
}

// Name returns the stable adapter identifier "hackernews".
// This value is used as the Prometheus label and registry key.
func (a *Adapter) Name() string { return "hackernews" }

// Capabilities returns a deterministic descriptor for the HN adapter.
// Called by the Intent Router at startup. The returned value is immutable.
func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:    "hackernews",
		DisplayName: "Hacker News",
		DocTypes:    []types.DocType{types.DocTypePost},
		// SupportedLangs is nil — HN is language-agnostic; the Intent Router
		// (SPEC-IR-001 REQ-IR-008) treats nil as "matches any language query".
		SupportedLangs:    nil,
		SupportsSince:     true, // numericFilters=created_at_i>=... supported.
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   60,
		DefaultMaxResults: 25,
		Notes: "Algolia HN Search public no-auth endpoint " +
			"(https://hn.algolia.com/api). Stories only (tags=story " +
			"hardcoded; comments / polls deferred). self-posts use " +
			"news.ycombinator.com permalink as URL. Body and Snippet " +
			"are HTML-stripped from story_text. Filter keys: 'since' " +
			"(Unix seconds, maps to created_at_i>=) and 'min_points' " +
			"(integer, maps to points>=). Retry-After defaults to 5s " +
			"when Algolia omits the header on 429.",
	}
}

// Healthcheck probes HN reachability via TCP connect to a.healthcheckTarget.
// The caller's ctx deadline governs the dial timeout. Returns nil on success.
// Tests inject a loopback address via Options.HealthcheckTarget.
func (a *Adapter) Healthcheck(ctx context.Context) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", a.healthcheckTarget)
	if err != nil {
		return err
	}
	return conn.Close()
}

// Compile-time interface assertion.
// If types.Adapter gains a new method, this line fails to compile until the
// method is also added to *Adapter.
var _ types.Adapter = (*Adapter)(nil)
