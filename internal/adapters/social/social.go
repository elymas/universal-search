// Package social provides Bluesky and X (Twitter) search adapters implementing
// types.Adapter. Both constructors return *Adapter; dispatch is via subSource.
//
// REQ-ADP6-001: NewBluesky and NewX constructors, interface conformance.
// SPEC-ADP-006: Reference implementation for social adapter pattern.
package social

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	// defaultUserAgentTemplate is the User-Agent header format string.
	defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"

	// defaultUAVersion is the fallback version when Options.UserAgentVersion is empty.
	defaultUAVersion = "v0.1"

	// defaultBlueskyHealthcheckTarget is the TCP address for Bluesky Healthcheck.
	defaultBlueskyHealthcheckTarget = "public.api.bsky.app:443"
)

// Adapter is the social search adapter. It implements types.Adapter.
// Both Bluesky and X instances use this same struct; subSource dispatches Search.
//
// All fields are unexported and immutable after construction (REQ-ADP6-001).
type Adapter struct {
	httpClient        *http.Client
	baseURL           string
	userAgent         string
	healthcheckTarget string
	subSource         string              // "bluesky" or "x"
	envLookup         func(string) string // env var lookup; injected for test isolation
}

// BlueskyOptions configures the Bluesky adapter. All fields are optional;
// defaults are used when a field is the zero value.
type BlueskyOptions struct {
	// BaseURL overrides the Bluesky AppView search endpoint. Used in tests.
	BaseURL string

	// HTTPClient overrides the default *http.Client. Used in tests.
	HTTPClient *http.Client

	// UserAgentVersion overrides the version component of the User-Agent header.
	// Default: "v0.1".
	UserAgentVersion string

	// HealthcheckTarget overrides the TCP address for Healthcheck.
	// Default: "public.api.bsky.app:443".
	HealthcheckTarget string
}

// XOptions configures the X (Twitter) adapter stub.
type XOptions struct {
	// EnvLookup overrides os.Getenv for X_ENABLED env var lookups.
	// MUST be injected in concurrent tests (goroutine-safe replacement for
	// t.Setenv which is unsafe under -race). Default: os.Getenv.
	EnvLookup func(string) string
}

// NewBluesky constructs a Bluesky Adapter from the given BlueskyOptions.
//
// @MX:ANCHOR: [AUTO] Bluesky adapter constructor; public API boundary.
// @MX:REASON: called by registry at startup; fan_in >= 3 from registry + tests.
// @MX:SPEC: SPEC-ADP-006
func NewBluesky(opts BlueskyOptions) (*Adapter, error) {
	base := opts.BaseURL
	if base == "" {
		base = defaultBlueskyBaseURL
	}

	version := opts.UserAgentVersion
	if version == "" {
		version = defaultUAVersion
	}
	ua := fmt.Sprintf(defaultUserAgentTemplate, version)

	client := opts.HTTPClient
	if client == nil {
		client = newDefaultBlueskyClient()
	}

	target := opts.HealthcheckTarget
	if target == "" {
		target = defaultBlueskyHealthcheckTarget
	}

	return &Adapter{
		httpClient:        client,
		baseURL:           base,
		userAgent:         ua,
		healthcheckTarget: target,
		subSource:         "bluesky",
		envLookup:         os.Getenv,
	}, nil
}

// NewX constructs an X (Twitter) stub Adapter from the given XOptions.
//
// @MX:ANCHOR: [AUTO] X adapter constructor; public API boundary.
// @MX:REASON: called by registry at startup; fan_in >= 3 from registry + tests.
// @MX:SPEC: SPEC-ADP-006
func NewX(opts XOptions) (*Adapter, error) {
	lookup := opts.EnvLookup
	if lookup == nil {
		lookup = os.Getenv
	}

	return &Adapter{
		httpClient:        &http.Client{},
		baseURL:           "",
		userAgent:         fmt.Sprintf(defaultUserAgentTemplate, defaultUAVersion),
		healthcheckTarget: "",
		subSource:         "x",
		envLookup:         lookup,
	}, nil
}

// Name returns the stable adapter identifier ("bluesky" or "x").
// This value is used as the Prometheus label and registry key.
func (a *Adapter) Name() string { return a.subSource }

// Capabilities returns a deterministic descriptor for this adapter.
// Called by the Intent Router at startup. The returned value is immutable.
func (a *Adapter) Capabilities() types.Capabilities {
	switch a.subSource {
	case "bluesky":
		return blueskyCapabilities()
	case "x":
		return xCapabilities()
	default:
		return types.Capabilities{SourceID: a.subSource}
	}
}

// blueskyCapabilities returns the Bluesky adapter Capabilities descriptor.
func blueskyCapabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          "bluesky",
		DisplayName:       "Bluesky",
		DocTypes:          []types.DocType{types.DocTypePost},
		SupportedLangs:    nil, // multi-lingual; Intent Router treats nil as "matches any"
		SupportsSince:     true,
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   600,
		DefaultMaxResults: 25,
		Notes: "Bluesky social public AppView search via app.bsky.feed.searchPosts. " +
			"Anonymous access, sort=top hardcoded. " +
			"lang filter supported. since filter supported (RFC 3339). " +
			"social DocType=post (latest deferred). Lang filter from Query.Lang; since " +
			"from Query.Filters[since]. SSRF guard: {public.api.bsky.app, api.bsky.app, bsky.app}.",
	}
}

// xCapabilities returns the X (Twitter) adapter Capabilities descriptor.
func xCapabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          "x",
		DisplayName:       "X (Twitter)",
		DocTypes:          []types.DocType{types.DocTypePost},
		SupportedLangs:    nil,
		SupportsSince:     false,
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   0,
		DefaultMaxResults: 0,
		Notes: "DISABLED in v0. Set USEARCH_X_ENABLED=true to enable. " +
			"SPEC-ADP-006-XENABLE pending. social sub-source stub; no live path wired.",
	}
}

// Search dispatches to the appropriate sub-source search implementation.
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	switch a.subSource {
	case "bluesky":
		return a.searchBluesky(ctx, q)
	case "x":
		return a.searchX(ctx, q)
	default:
		return nil, &types.SourceError{
			Adapter:  a.subSource,
			Category: types.CategoryPermanent,
			Cause:    fmt.Errorf("social: unknown subSource %q", a.subSource),
		}
	}
}

// Healthcheck probes adapter reachability.
// For Bluesky: TCP connect to a.healthcheckTarget.
// For X: returns ErrXDisabled (no endpoint to probe in v0).
func (a *Adapter) Healthcheck(ctx context.Context) error {
	switch a.subSource {
	case "bluesky":
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", a.healthcheckTarget)
		if err != nil {
			return err
		}
		return conn.Close()
	case "x":
		return ErrXDisabled
	default:
		return fmt.Errorf("social: unknown subSource %q", a.subSource)
	}
}

// Compile-time interface assertion.
var _ types.Adapter = (*Adapter)(nil)
