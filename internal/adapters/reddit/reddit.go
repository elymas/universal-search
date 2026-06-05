// Package reddit provides a Reddit search adapter implementing types.Adapter.
// REQ-ADP-001: Interface conformance, Name, Capabilities, Healthcheck.
// SPEC-ADP-001: Reference implementation for the adapter pattern.
// SPEC-ADP-001a: App-only OAuth (client_credentials) amendment.
package reddit

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	// defaultBaseURL is the Reddit authenticated search endpoint.
	// Changed from www.reddit.com/search.json to oauth.reddit.com/search per ADP-001a.
	defaultBaseURL = "https://oauth.reddit.com/search"

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

	// ClientID is the Reddit app client ID for OAuth2 app-only authentication.
	// Required unless SkipAuthCheck is true. Read from REDDIT_CLIENT_ID env var.
	ClientID string

	// ClientSecret is the Reddit app client secret for OAuth2 app-only authentication.
	// Required unless SkipAuthCheck is true. Read from REDDIT_CLIENT_SECRET env var.
	ClientSecret string

	// OAuthURL overrides the token acquisition endpoint.
	// Default: "https://www.reddit.com/api/v1/access_token".
	// Read from REDDIT_OAUTH_URL env var (REQ-ADP-001a-007).
	OAuthURL string

	// SkipAuthCheck bypasses the credential validation in New.
	// Used by tests to construct an adapter without real credentials.
	// Mirrors the GitHub adapter's SkipAuthCheck seam (REQ-ADP-001a-006b).
	SkipAuthCheck bool
}

// Adapter is the Reddit search adapter. It implements types.Adapter.
// Fields are unexported and immutable after construction except for the
// tokenCache which is guarded by sync.Mutex (REQ-ADP-011, NFR-ADP-001a-001).
type Adapter struct {
	httpClient        *http.Client
	baseURL           string
	userAgent         string
	healthcheckTarget string
	clientID          string
	clientSecret      string
	oauthURL          string
	tokens            *tokenCache
}

// New constructs a Reddit Adapter from the given Options.
//
// Credential validation (REQ-ADP-001a-006b): returns ErrMissingCredentials
// when (ClientID == "" || ClientSecret == "") && !SkipAuthCheck && HTTPClient == nil.
// The HTTPClient != nil escape lets tests inject a custom transport without
// providing real credentials.
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

	oauthURL := opts.OAuthURL
	if oauthURL == "" {
		oauthURL = defaultOAuthURL
	}

	// Credential gate (REQ-ADP-001a-006b).
	// Parentheses matter: (ID=="" OR Secret=="") AND !SkipAuthCheck AND HTTPClient==nil.
	// An empty credential must NOT error when SkipAuthCheck=true or HTTPClient is injected.
	if (opts.ClientID == "" || opts.ClientSecret == "") && !opts.SkipAuthCheck && opts.HTTPClient == nil {
		return nil, &types.SourceError{
			Adapter:  "reddit",
			Category: types.CategoryPermanent,
			Cause:    ErrMissingCredentials,
		}
	}

	return &Adapter{
		httpClient:        client,
		baseURL:           base,
		userAgent:         ua,
		healthcheckTarget: target,
		clientID:          opts.ClientID,
		clientSecret:      opts.ClientSecret,
		oauthURL:          oauthURL,
		tokens:            &tokenCache{},
	}, nil
}

// Name returns the stable adapter identifier "reddit".
// This value is used as the Prometheus label and registry key.
func (a *Adapter) Name() string { return "reddit" }

// Capabilities returns a deterministic descriptor for the Reddit adapter.
// Called by the Intent Router at startup. The returned value is immutable.
//
// Updated per SPEC-ADP-001a: RequiresAuth=true, AuthEnvVars set,
// RateLimitPerMin=60 (authenticated ceiling), Notes describe OAuth.
func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:    "reddit",
		DisplayName: "Reddit",
		DocTypes:    []types.DocType{types.DocTypePost},
		// SupportedLangs is nil — Reddit is language-agnostic; the Intent Router
		// (SPEC-IR-001 REQ-IR-008) treats nil as "matches any language query".
		SupportedLangs:    nil,
		SupportsSince:     false, // v0.1 hardcodes t=all; time-range filter deferred.
		RequiresAuth:      true,
		AuthEnvVars:       []string{"REDDIT_CLIENT_ID", "REDDIT_CLIENT_SECRET"},
		RateLimitPerMin:   60, // Authenticated ceiling (60/min OAuth app-only).
		DefaultMaxResults: 25,
		Notes: "Reddit OAuth app-only search (client_credentials grant) via " +
			"oauth.reddit.com. Rate limit 60/min (authenticated). " +
			"Set REDDIT_CLIENT_ID and REDDIT_CLIENT_SECRET env vars. " +
			"NSFW excluded by default; set Query.Filters[{nsfw, true}] to include. " +
			"t=all hardcoded (time-range filter deferred to future SPEC).",
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
