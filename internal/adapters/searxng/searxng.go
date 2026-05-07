// Package searxng provides a SearXNG Bridge adapter implementing types.Adapter.
// REQ-ADP7-001: Interface conformance, Name, Capabilities, Healthcheck.
// SPEC-ADP-007: SearXNG self-hosted meta-search bridge adapter.
package searxng

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	// defaultBaseURL is the SearXNG docker-compose service URL.
	// USEARCH_SEARXNG_URL env or Options.BaseURL override this.
	defaultBaseURL = "http://searxng:8080"

	// defaultUserAgentTemplate is the User-Agent header format string.
	// %s is replaced with the version string from Options.UserAgentVersion.
	defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"

	// defaultUAVersion is the fallback version when Options.UserAgentVersion is empty.
	defaultUAVersion = "v0.1"

	// envBaseURL is the environment variable name for the SearXNG base URL override.
	envBaseURL = "USEARCH_SEARXNG_URL"
)

// Options configures the SearXNG adapter. All fields are optional; defaults are
// used when a field is the zero value.
type Options struct {
	// BaseURL overrides the SearXNG instance URL. Primarily used in tests to
	// redirect requests to an httptest.Server.
	// Resolution order: Options.BaseURL → USEARCH_SEARXNG_URL env → "http://searxng:8080".
	BaseURL string

	// HTTPClient overrides the default *http.Client (10s timeout, redirect
	// allowlist, reqid transport). Primarily used in tests.
	HTTPClient *http.Client

	// UserAgentVersion overrides the version component of the User-Agent header.
	// Default: "v0.1".
	UserAgentVersion string

	// HealthcheckTarget overrides the TCP address for Healthcheck.
	// Default: derived from BaseURL host+port. Tests inject a loopback address.
	HealthcheckTarget string
}

// Adapter is the SearXNG search adapter. It implements types.Adapter.
// All fields are unexported and immutable after construction (REQ-ADP7-010).
//
// @MX:ANCHOR: [AUTO] Root struct for all SearXNG adapter behaviour. New() is the
// only constructor; Search, Healthcheck, Capabilities, Name all fan in here.
// @MX:REASON: Any field addition or removal affects the entire adapter surface
// (Search hot path, Healthcheck, Capabilities, reqid transport chain).
// @MX:SPEC: SPEC-ADP-007
type Adapter struct {
	httpClient        *http.Client
	baseURL           string
	userAgent         string
	healthcheckTarget string
}

// New constructs a SearXNG Adapter from the given Options.
//
// BaseURL resolution order (REQ-ADP7-001):
//  1. Options.BaseURL (non-empty)
//  2. USEARCH_SEARXNG_URL environment variable (non-empty)
//  3. "http://searxng:8080" (docker-compose default)
//
// Trailing slashes on BaseURL are trimmed. Returns a non-nil error only if
// BaseURL is syntactically invalid (unparseable by url.Parse).
//
// @MX:ANCHOR: [AUTO] Sole construction path for *Adapter. Fan_in: registry,
// tests, CLI-001.
// @MX:REASON: signature change requires updating all call sites; Options struct
// extension must remain backward compatible.
// @MX:SPEC: SPEC-ADP-007
func New(opts Options) (*Adapter, error) {
	// Resolve BaseURL: Options → env → docker-compose default.
	base := opts.BaseURL
	if base == "" {
		base = os.Getenv(envBaseURL)
	}
	if base == "" {
		base = defaultBaseURL
	}
	// Trim trailing slash for consistent URL building.
	base = strings.TrimRight(base, "/")

	// Validate that base is a parseable URL.
	if _, err := url.Parse(base); err != nil {
		return nil, fmt.Errorf("searxng: invalid base URL %q: %w", base, err)
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
		target = healthcheckHostFromBase(base)
	}

	return &Adapter{
		httpClient:        client,
		baseURL:           base,
		userAgent:         ua,
		healthcheckTarget: target,
	}, nil
}

// Name returns the stable adapter identifier "searxng".
// This value is used as the Prometheus label and registry key (REQ-ADP7-001).
func (a *Adapter) Name() string { return "searxng" }

// Capabilities returns a deterministic descriptor for the SearXNG adapter.
// Called by the Intent Router at startup. The returned value is immutable.
//
// REQ-ADP7-001 (post-audit H1): DocTypes restricted to [DocTypeArticle].
// REQ-ADP7-001 (post-audit H3): RateLimitPerMin=0 (no external rate limit for
// self-hosted instance; individual SearXNG engine limits are not surfaced here).
func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:    "searxng",
		DisplayName: "SearXNG",
		// H1 audit fix: DocTypes restricted to Article only. SearXNG is a
		// meta-search engine aggregating web results; all results normalise to
		// DocTypeArticle regardless of the underlying engine category.
		DocTypes: []types.DocType{types.DocTypeArticle},
		// SupportedLangs: nil — SearXNG is language-agnostic; the Intent Router
		// (SPEC-IR-001 REQ-IR-008) treats nil as "matches any language query".
		SupportedLangs: nil,
		SupportsSince:  false, // time-range filter deferred to future SPEC.
		RequiresAuth:   false, // self-hosted, no auth required (REQ-ADP7-001).
		AuthEnvVars:    nil,
		// H3 audit fix: 0 means no externally-visible rate limit. Individual
		// backing-engine limits are handled server-side by SearXNG itself.
		RateLimitPerMin:   0,
		DefaultMaxResults: 10,
		Notes: "Self-hosted SearXNG meta-search bridge. Aggregates results from " +
			"multiple search engines (Google, Bing, DuckDuckGo, etc.). " +
			"Base URL resolves via Options.BaseURL → USEARCH_SEARXNG_URL env → " +
			"http://searxng:8080 (docker-compose default). " +
			"DocTypes restricted to Article (H1 audit). " +
			"No external rate limit (H3 audit, RateLimitPerMin=0).",
	}
}

// Healthcheck probes SearXNG reachability via TCP connect to a.healthcheckTarget.
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

// healthcheckHostFromBase derives the TCP healthcheck target (host:port) from
// the adapter's base URL. Falls back to "searxng:8080" if the URL cannot be
// parsed or has no host component.
func healthcheckHostFromBase(base string) string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return "searxng:8080"
	}
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		// Default port by scheme.
		switch u.Scheme {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}
	return net.JoinHostPort(host, port)
}

// Compile-time interface assertion.
// If types.Adapter gains a new method, this line fails to compile until the
// method is also added to *Adapter.
var _ types.Adapter = (*Adapter)(nil)
