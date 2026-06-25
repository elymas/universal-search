// Package social provides Bluesky and X (Twitter) search adapters implementing
// types.Adapter. Both constructors return *Adapter; dispatch is via subSource.
//
// REQ-ADP6-001: NewBluesky and NewX constructors, interface conformance.
// SPEC-ADP-006: Reference implementation for social adapter pattern.
package social

import (
	"context"
	"errors"
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
	xProvider         XProvider           // nil = disabled; non-nil = live (SPEC-ADP-006-XENABLE)
	// @MX:NOTE: [AUTO] The live/disabled discriminator for X. Nil = disabled; non-nil = live.
	// @MX:SPEC: SPEC-ADP-006-XENABLE
	bearerToken string // Bluesky session accessJwt; "" = unauthenticated (searchPosts now 403s without it)
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

	// Handle and AppPassword authenticate the adapter via createSession.
	// Required since searchPosts rejects unauthenticated requests (HTTP 403).
	// When either is empty, the adapter stays unauthenticated.
	Handle      string
	AppPassword string
}

// XOptions configures the X (Twitter) adapter.
type XOptions struct {
	// EnvLookup overrides os.Getenv for X_ENABLED env var lookups.
	// MUST be injected in concurrent tests (goroutine-safe replacement for
	// t.Setenv which is unsafe under -race). Default: os.Getenv.
	EnvLookup func(string) string

	// Provider is the pluggable X search backend. When nil, the adapter
	// operates in disabled/stub mode (ErrXDisabled / ErrXProviderNotConfigured).
	// SPEC-ADP-006-XENABLE REQ-XEN-001.
	Provider XProvider
}

// NewBluesky constructs a Bluesky Adapter from the given BlueskyOptions.
//
// @MX:ANCHOR: [AUTO] Bluesky adapter constructor; public API boundary.
// @MX:REASON: called by registry at startup; fan_in >= 3 from registry + tests.
// @MX:SPEC: SPEC-ADP-006
func NewBluesky(opts BlueskyOptions) (*Adapter, error) {
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

	// Authenticate when credentials are supplied. searchPosts returns HTTP 403
	// for unauthenticated requests, so without creds the adapter still builds
	// but its searches will fail at query time (parity with pre-auth behavior).
	var token string
	if opts.Handle != "" && opts.AppPassword != "" {
		t, err := createBlueskySession(context.Background(), client, opts.Handle, opts.AppPassword)
		if err != nil {
			return nil, err
		}
		token = t
	}

	// Select endpoint. The public AppView (public.api.bsky.app) only serves
	// unauthenticated requests, but searchPosts now rejects those (403). An
	// authenticated session JWT is only accepted by the PDS host (bsky.social),
	// so route authenticated searches there. Explicit BaseURL always wins.
	base := opts.BaseURL
	if base == "" {
		if token != "" {
			base = authedBlueskyBaseURL
		} else {
			base = defaultBlueskyBaseURL
		}
	}

	return &Adapter{
		httpClient:        client,
		baseURL:           base,
		userAgent:         ua,
		healthcheckTarget: target,
		subSource:         "bluesky",
		envLookup:         os.Getenv,
		bearerToken:       token,
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
		xProvider:         opts.Provider,
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
		if a.xProvider != nil {
			return xCapabilitiesLive(a.xProvider)
		}
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

// xCapabilitiesLive returns the LIVE Capabilities descriptor when a provider is configured.
func xCapabilitiesLive(prov XProvider) types.Capabilities {
	return types.Capabilities{
		SourceID:       "x",
		DisplayName:    "X (Twitter)",
		DocTypes:       []types.DocType{types.DocTypePost},
		SupportedLangs: nil,
		SupportsSince:  false,
		RequiresAuth:   true,
		// X_BEARER_TOKEN is the exact key the production provider reads
		// (cmd/usearch-mcp buildXProvider). Declaring it lets the registry
		// auth gate (registry.go) and admin `sources status` reflect X's
		// real credential requirement (SPEC-SEC-002 cred-01).
		AuthEnvVars:       []string{"X_BEARER_TOKEN"},
		RateLimitPerMin:   0,
		DefaultMaxResults: 25,
		Notes: "X (Twitter) social LIVE via configured XProvider. Enabled by " +
			"USEARCH_X_ENABLED=true + provider creds. Provider is pluggable " +
			"(X official API or twitterapi.io). ToS-grey third-party providers " +
			"require explicit ToS acknowledgement at deployment (tech.md:147). " +
			"Provider: " + prov.Name() + ".",
	}
}

// xProviderHealthcheck probes the live provider's reachability.
func (a *Adapter) xProviderHealthcheck(ctx context.Context) error {
	if a.xProvider == nil {
		return ErrXDisabled
	}
	// Perform a lightweight search with an empty query to probe reachability.
	// The provider should return quickly or error.
	_, _, err := a.xProvider.SearchTweets(ctx, types.Query{Text: "healthcheck", MaxResults: 1})
	if err == nil {
		return nil
	}
	var se *types.SourceError
	if errors.As(err, &se) {
		return se
	}
	return &types.SourceError{
		Adapter:  "x",
		Category: types.CategoryUnavailable,
		Cause:    err,
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
		if a.xProvider != nil {
			return a.xProviderHealthcheck(ctx)
		}
		return ErrXDisabled
	default:
		return fmt.Errorf("social: unknown subSource %q", a.subSource)
	}
}

// Compile-time interface assertion.
var _ types.Adapter = (*Adapter)(nil)
