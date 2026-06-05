// Package meta provides Threads and Facebook (disabled) search adapters
// implementing types.Adapter. Both constructors return *Adapter; dispatch is
// via subSource.
//
// REQ-ADP10-001: NewThreads and NewFacebook constructors, interface conformance.
// SPEC-ADP-010: Meta adapter — Threads live path + Facebook disabled stub.
package meta

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/elymas/universal-search/internal/obs/reqid"
	"github.com/elymas/universal-search/pkg/types"
)

const (
	// defaultUserAgentTemplate is the User-Agent header format string.
	defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"

	// defaultUAVersion is the fallback version when Options.UserAgentVersion is empty.
	defaultUAVersion = "v0.1"

	// defaultThreadsBaseURL is the Threads keyword_search endpoint.
	defaultThreadsBaseURL = "https://graph.threads.net/v1.0/keyword_search"

	// defaultThreadsHealthcheckTarget is the TCP address for Threads healthcheck.
	defaultThreadsHealthcheckTarget = "graph.threads.net:443"
)

// Adapter is the meta search adapter. It implements types.Adapter.
// Both Threads and Facebook instances use this same struct; subSource dispatches Search.
//
// All fields are unexported and immutable after construction (REQ-ADP10-001).
type Adapter struct {
	httpClient        *http.Client
	baseURL           string
	accessToken       string // @MX:WARN: [AUTO] Secret. @MX:REASON: must never be logged/metric-labelled; Authorization header only (NFR-ADP10-002). @MX:SPEC: SPEC-ADP-010
	userAgent         string
	healthcheckTarget string
	subSource         string              // "threads" or "facebook"
	envLookup         func(string) string // env var lookup; injected for test isolation
}

// ThreadsOptions configures the Threads adapter. All fields are optional;
// defaults are used when a field is the zero value.
type ThreadsOptions struct {
	// BaseURL overrides the Threads keyword_search endpoint. Used in tests.
	BaseURL string

	// HTTPClient overrides the default *http.Client. Used in tests.
	HTTPClient *http.Client

	// AccessToken provides the OAuth 2.0 Bearer token directly.
	// Falls back to EnvLookup("THREADS_ACCESS_TOKEN") when empty.
	AccessToken string

	// UserAgentVersion overrides the version component of the User-Agent header.
	// Default: "v0.1".
	UserAgentVersion string

	// HealthcheckTarget overrides the TCP address for Healthcheck.
	// Default: "graph.threads.net:443".
	HealthcheckTarget string

	// EnvLookup overrides os.Getenv for THREADS_ACCESS_TOKEN lookups.
	// MUST be injected in concurrent tests (goroutine-safe replacement for
	// t.Setenv which is unsafe under -race). Default: os.Getenv.
	EnvLookup func(string) string
}

// FacebookOptions configures the Facebook (disabled) adapter.
type FacebookOptions struct {
	// EnvLookup overrides os.Getenv. Default: os.Getenv.
	EnvLookup func(string) string
}

// NewThreads constructs a Threads Adapter from the given ThreadsOptions.
// Returns (nil, ErrThreadsTokenMissing) when no token is available.
//
// @MX:ANCHOR: [AUTO] Threads adapter constructor; public API boundary.
// @MX:REASON: called by registry at startup; fan_in >= 3 from registry + tests.
// @MX:SPEC: SPEC-ADP-010
func NewThreads(opts ThreadsOptions) (*Adapter, error) {
	lookup := opts.EnvLookup
	if lookup == nil {
		lookup = os.Getenv
	}

	token := opts.AccessToken
	if token == "" {
		token = lookup("THREADS_ACCESS_TOKEN")
	}
	if token == "" {
		return nil, ErrThreadsTokenMissing
	}

	base := opts.BaseURL
	if base == "" {
		base = defaultThreadsBaseURL
	}

	version := opts.UserAgentVersion
	if version == "" {
		version = defaultUAVersion
	}
	ua := fmt.Sprintf(defaultUserAgentTemplate, version)

	client := opts.HTTPClient
	if client == nil {
		client = newDefaultThreadsClient()
	}

	target := opts.HealthcheckTarget
	if target == "" {
		target = defaultThreadsHealthcheckTarget
	}

	return &Adapter{
		httpClient:        client,
		baseURL:           base,
		accessToken:       token,
		userAgent:         ua,
		healthcheckTarget: target,
		subSource:         "threads",
		envLookup:         lookup,
	}, nil
}

// NewFacebook constructs a Facebook (disabled) Adapter from the given FacebookOptions.
//
// @MX:ANCHOR: [AUTO] Facebook adapter constructor; public API boundary.
// @MX:REASON: called by registry at startup; fan_in >= 3 from registry + tests.
// @MX:SPEC: SPEC-ADP-010
func NewFacebook(opts FacebookOptions) (*Adapter, error) {
	lookup := opts.EnvLookup
	if lookup == nil {
		lookup = os.Getenv
	}

	return &Adapter{
		httpClient:        &http.Client{},
		baseURL:           "",
		accessToken:       "",
		userAgent:         fmt.Sprintf(defaultUserAgentTemplate, defaultUAVersion),
		healthcheckTarget: "",
		subSource:         "facebook",
		envLookup:         lookup,
	}, nil
}

// Name returns the stable adapter identifier ("threads" or "facebook").
func (a *Adapter) Name() string { return a.subSource }

// Capabilities returns a deterministic descriptor for this adapter.
func (a *Adapter) Capabilities() types.Capabilities {
	switch a.subSource {
	case "threads":
		return threadsCapabilities()
	case "facebook":
		return facebookCapabilities()
	default:
		return types.Capabilities{SourceID: a.subSource}
	}
}

// threadsCapabilities returns the Threads adapter Capabilities descriptor.
func threadsCapabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          "threads",
		DisplayName:       "Threads",
		DocTypes:          []types.DocType{types.DocTypePost},
		SupportedLangs:    nil,
		SupportsSince:     true,
		RequiresAuth:      true,
		AuthEnvVars:       []string{"THREADS_ACCESS_TOKEN"},
		RateLimitPerMin:   1,
		DefaultMaxResults: 25,
		Notes: "Threads (Meta) via graph.threads.net keyword_search. " +
			"meta. OAuth 2.0 Bearer token (THREADS_ACCESS_TOKEN). " +
			"threads_keyword_search permission required for full public-post " +
			"search; without it only the authed user's own posts are returned " +
			"(research §1.1). search_type=TOP, search_mode=KEYWORD hardcoded. " +
			"since/until filters from Query.Filters. RATE LIMIT: 2200 queries " +
			"per 24h PER USER (per token), across all apps — this is a daily " +
			"budget, NOT a per-minute cap. RateLimitPerMin=1 is a coarse floor " +
			"only; consumers (SPEC-FAN-001) MUST treat this as a daily token " +
			"budget, not a 1-call/min global limit. No engagement counts in " +
			"response → Score=0.5 neutral (research §1.3).",
	}
}

// facebookCapabilities returns the Facebook (disabled) Capabilities descriptor.
func facebookCapabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          "facebook",
		DisplayName:       "Facebook",
		DocTypes:          []types.DocType{types.DocTypePost},
		SupportedLangs:    nil,
		SupportsSince:     false,
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   0,
		DefaultMaxResults: 0,
		Notes: "Facebook (Meta) meta. NOT SUPPORTED. The official Facebook " +
			"Graph API exposes no public-post keyword search endpoint " +
			"(verified: canonical Graph API search reference returns HTTP 404; " +
			"research §2.1). No official endpoint exists to keyword-search " +
			"public posts/Pages you do not own. Scraping is excluded per " +
			"tech.md:147 (ToS risk). All Search calls return " +
			"ErrFacebookNotSupported (permanent). Surface reserved for future " +
			"SPEC-ADP-010-FBSCRAPE only if a ToS-compliant path emerges; no " +
			"opt-in env provided in v0.",
	}
}

// Healthcheck probes adapter reachability.
// For Threads: TCP connect to a.healthcheckTarget.
// For Facebook: returns ErrFacebookDisabled (no endpoint to probe).
func (a *Adapter) Healthcheck(ctx context.Context) error {
	switch a.subSource {
	case "threads":
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", a.healthcheckTarget)
		if err != nil {
			return err
		}
		return conn.Close()
	case "facebook":
		return ErrFacebookDisabled
	default:
		return fmt.Errorf("meta: unknown subSource %q", a.subSource)
	}
}

// newDefaultThreadsClient constructs the default HTTP client for the Threads adapter.
func newDefaultThreadsClient() *http.Client {
	return &http.Client{
		Timeout:       10 * (1e9), // 10 seconds
		Transport:     reqid.NewTransport(http.DefaultTransport),
		CheckRedirect: threadsRedirectAllowlist,
	}
}

// threadsAllowedRedirectHosts is the SSRF-guard allowlist for Threads redirect targets.
var threadsAllowedRedirectHosts = map[string]struct{}{
	"graph.threads.net": {},
}

// threadsRedirectAllowlist is the CheckRedirect policy for the Threads HTTP client.
func threadsRedirectAllowlist(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return fmt.Errorf("meta: too many redirects (max 3)")
	}
	host := req.URL.Hostname()
	if _, ok := threadsAllowedRedirectHosts[host]; !ok {
		return fmt.Errorf("meta: cross-domain redirect rejected: %s", host)
	}
	return nil
}

// Compile-time interface assertion.
var _ types.Adapter = (*Adapter)(nil)
