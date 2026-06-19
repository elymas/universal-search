// Package github provides a GitHub search adapter implementing types.Adapter.
// REQ-ADP4-001: Interface conformance, Name, Capabilities, Healthcheck.
// SPEC-ADP-004: First authenticated adapter; multi-intent routing (code/issues/repos).
//
// Deviation from SPEC: SPEC-ADP-004 references github.com/google/go-github/v85,
// but v73.0.0 is the latest available at implementation time. This adapter
// uses v73; the v73 CodeResult struct lacks Score and Language fields
// present in the v85 spec description. Code search hits use Score=0.5 (neutral).
package github

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	gogithub "github.com/google/go-github/v73/github"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	// defaultUserAgentTemplate is the User-Agent format string.
	// %s is the version from Options.UserAgentVersion.
	defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"

	// defaultUAVersion is the fallback version when Options.UserAgentVersion is empty.
	defaultUAVersion = "v0.1"

	// defaultHealthcheckTarget is the TCP address for Healthcheck.
	defaultHealthcheckTarget = "api.github.com:443"
)

// Options configures the GitHub adapter. All fields are optional; defaults are
// used when a field is the zero value.
type Options struct {
	// BaseURL overrides the GitHub API base URL. Primarily for tests pointing
	// at an httptest.Server. Must have a trailing slash if set.
	BaseURL string

	// HTTPClient overrides the default *http.Client (10s timeout, redirect
	// allowlist, reqid transport). Primarily used in tests.
	HTTPClient *http.Client

	// Token is the GitHub PAT (Personal Access Token).
	// Required when SkipAuthCheck is false and HTTPClient is nil (default).
	Token string

	// UserAgentVersion overrides the version component of the User-Agent header.
	// Default: "v0.1".
	UserAgentVersion string

	// HealthcheckTarget overrides the TCP address for Healthcheck.
	// Default: "api.github.com:443".
	HealthcheckTarget string

	// SkipAuthCheck disables the Token requirement check. Intended for tests
	// that inject a custom HTTPClient or use an unauthenticated stub server.
	SkipAuthCheck bool
}

// Adapter is the GitHub search adapter. It implements types.Adapter.
// All fields are unexported and immutable after construction.
type Adapter struct {
	ghClient          *gogithub.Client
	httpClient        *http.Client
	baseURL           string
	userAgent         string
	healthcheckTarget string
}

// New constructs a GitHub Adapter from the given Options.
//
// Returns *types.SourceError{Category: CategoryPermanent, Cause: ErrMissingToken}
// when Token is empty and SkipAuthCheck is false and HTTPClient is nil.
func New(opts Options) (*Adapter, error) {
	if !opts.SkipAuthCheck && opts.HTTPClient == nil && opts.Token == "" {
		return nil, &types.SourceError{
			Adapter:  "github",
			Category: types.CategoryPermanent,
			Cause:    ErrMissingToken,
		}
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = newDefaultHTTPClient()
	}

	// Build the go-github client.
	ghClient := gogithub.NewClient(httpClient)
	if opts.Token != "" {
		ghClient = ghClient.WithAuthToken(opts.Token)
	}

	// Override BaseURL when provided (tests inject httptest.Server URL).
	if opts.BaseURL != "" {
		baseURL := opts.BaseURL
		if baseURL[len(baseURL)-1] != '/' {
			baseURL += "/"
		}
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return nil, fmt.Errorf("github: invalid BaseURL %q: %w", opts.BaseURL, err)
		}
		ghClient.BaseURL = parsed
	}

	version := opts.UserAgentVersion
	if version == "" {
		version = defaultUAVersion
	}
	ua := fmt.Sprintf(defaultUserAgentTemplate, version)
	ghClient.UserAgent = ua

	target := opts.HealthcheckTarget
	if target == "" {
		target = defaultHealthcheckTarget
	}

	return &Adapter{
		ghClient:          ghClient,
		httpClient:        httpClient,
		baseURL:           opts.BaseURL,
		userAgent:         ua,
		healthcheckTarget: target,
	}, nil
}

// Name returns the stable adapter identifier "github".
// This value is used as the Prometheus label and registry key.
func (a *Adapter) Name() string { return "github" }

// Capabilities returns a deterministic descriptor for the GitHub adapter.
// Called by the Intent Router at startup. The returned value is immutable.
func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:    "github",
		DisplayName: "GitHub",
		DocTypes:    []types.DocType{types.DocTypeRepo, types.DocTypeIssue},
		// SupportedLangs is nil — GitHub search is language-agnostic at the
		// adapter level; kind=code accepts language: qualifier via Filters.
		SupportedLangs:    nil,
		SupportsSince:     true,
		RequiresAuth:      true,
		AuthEnvVars:       []string{"USEARCH_GITHUB_TOKEN"},
		RateLimitPerMin:   30,
		DefaultMaxResults: 25,
		Notes: "GitHub REST search via google/go-github/v73 (SPEC references v85; " +
			"v73.0.0 used at implementation time — deviation documented). " +
			"PAT auth via USEARCH_GITHUB_TOKEN env var (public_repo scope; " +
			"private-repo search out of v0.1 scope). Multi-intent routing " +
			"via Query.Filters[Key=\"kind\"] in {code, issues, repos, commit}; " +
			"default repos when omitted. Rate ceilings: code search 9/min vs " +
			"issues/repos/commit 30/min — Capabilities advertises the conservative 30/min. " +
			"5000 req/hr primary (authenticated). commit search 30/min. Retry-After cap 90s " +
			"(vs 60s in Reddit/HN; GitHub secondary-rate-limit recovery semantics). " +
			"Path A (wrapping github/github-mcp-server) rejected — see SPEC-ADP-004 §2.2.",
	}
}

// Healthcheck probes GitHub API reachability via TCP connect to a.healthcheckTarget.
// The caller's ctx deadline governs the dial timeout. Returns nil on success.
func (a *Adapter) Healthcheck(ctx context.Context) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", a.healthcheckTarget)
	if err != nil {
		return err
	}
	return conn.Close()
}

// Compile-time interface assertion.
var _ types.Adapter = (*Adapter)(nil)
