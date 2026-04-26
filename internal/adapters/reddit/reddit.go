// Package reddit provides a Reddit search adapter implementing types.Adapter.
// REQ-ADP-001: Interface conformance, Name, Capabilities, Healthcheck.
// SPEC-ADP-001: Reference implementation for the adapter pattern.
package reddit

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	// defaultBaseURL is the Reddit public search JSON endpoint.
	defaultBaseURL = "https://www.reddit.com/search.json"

	// defaultUserAgentTemplate is the User-Agent header format string.
	// %s is replaced with the version string from Options.UserAgentVersion.
	defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"

	// defaultUAVersion is the fallback version when Options.UserAgentVersion is empty.
	defaultUAVersion = "v0.1"

	// defaultHealthcheckTarget is the TCP address for the Healthcheck dial.
	// Tests inject a loopback address via Options.HealthcheckTarget.
	defaultHealthcheckTarget = "www.reddit.com:443"
)

// Options configures the Reddit adapter. All fields are optional; defaults are
// used when a field is the zero value.
type Options struct {
	// BaseURL overrides the Reddit search endpoint. Primarily used in tests to
	// redirect requests to an httptest.Server.
	BaseURL string

	// HTTPClient overrides the default *http.Client (10s timeout, redirect
	// allowlist, reqid transport). Primarily used in tests.
	HTTPClient *http.Client

	// UserAgentVersion overrides the version component of the User-Agent header.
	// Default: "v0.1".
	UserAgentVersion string

	// HealthcheckTarget overrides the TCP address for Healthcheck.
	// Default: "www.reddit.com:443". Tests inject a loopback address.
	HealthcheckTarget string
}

// Adapter is the Reddit search adapter. It implements types.Adapter.
// All fields are unexported and immutable after construction (REQ-ADP-011).
type Adapter struct {
	httpClient        *http.Client
	baseURL           string
	userAgent         string
	healthcheckTarget string
}

// New constructs a Reddit Adapter from the given Options. Returns an error if
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

// Name returns the stable adapter identifier "reddit".
// This value is used as the Prometheus label and registry key.
func (a *Adapter) Name() string { return "reddit" }

// Capabilities returns a deterministic descriptor for the Reddit adapter.
// Called by the Intent Router at startup. The returned value is immutable.
func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:    "reddit",
		DisplayName: "Reddit",
		DocTypes:    []types.DocType{types.DocTypePost},
		// SupportedLangs is nil — Reddit is language-agnostic; the Intent Router
		// (SPEC-IR-001 REQ-IR-008) treats nil as "matches any language query".
		SupportedLangs:    nil,
		SupportsSince:     false, // v0.1 hardcodes t=all; time-range filter deferred.
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   10, // Conservative unauth figure (research.md §1.7).
		DefaultMaxResults: 25,
		Notes: "Reddit public no-auth search.json endpoint. " +
			"NSFW excluded by default; set Query.Filters[{nsfw, true}] to include. " +
			"t=all hardcoded (time-range filter deferred to future SPEC). " +
			"rate limit discrepancy: 10/min unauth (this SPEC) vs 60/min in tech.md " +
			"(follow-up sync task recommended).",
	}
}

// Healthcheck probes Reddit reachability via TCP connect to a.healthcheckTarget.
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
