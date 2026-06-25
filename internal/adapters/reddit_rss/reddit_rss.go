// Package redditrss implements the credential-free Reddit RSS adapter.
// SPEC-ADP-001b: Name()="reddit-rss", always-on (no credentials required),
// fetches https://www.reddit.com/search.rss?q=<query>&sort=relevance.
//
// This adapter is a SEPARATE independent package that shares NO code with
// the OAuth reddit adapter (internal/adapters/reddit/). It is a near-mirror
// of the single-feed slice of internal/adapters/koreanews/rss.go, specialised
// to one feed URL and mapping to DocTypePost instead of DocTypeArticle.
//
// Sole-emitter discipline (REQ-ADP1B-021): no Prometheus metrics, OTel spans,
// or slog records are emitted from within this package. Observability is
// provided by the registry's wrappedAdapter layer.
package redditrss

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// Adapter is the Reddit RSS search adapter. All fields are immutable after
// construction (set by New).
//
// @MX:ANCHOR: [AUTO] Adapter entry point; callers: registry, fanout, healthcheck, tests, pipeline
// @MX:REASON: fan_in >= 3; implements types.Adapter — all method additions break callers.
// @MX:SPEC: SPEC-ADP-001b
type Adapter struct {
	opts       Options
	httpClient *http.Client
	userAgent  string

	// cooldown + maxAttempts govern the 429 retry loop (SPEC-ADP-001b REQ-ADP1B-012).
	// Initialised from defaults in New; overridable in tests via export_test.go.
	cooldown    time.Duration
	maxAttempts int
}

// New constructs a reddit-rss Adapter from opts, applying defaults to all
// zero-value fields. Returns (*Adapter, nil) always in v0.1.
//
// @MX:ANCHOR: [AUTO] Constructor; callers: pipeline.go, integration tests, sources_cmd
// @MX:REASON: fan_in >= 3; sole entry point for building a valid Adapter.
// @MX:SPEC: SPEC-ADP-001b REQ-ADP1B-001
func New(opts Options) (*Adapter, error) {
	opts.applyDefaults()

	client := opts.HTTPClient
	if client == nil {
		client = newClientForBase(opts.BaseURL, opts.Timeout)
	}

	ua := opts.UserAgent
	if ua == "" {
		ua = fmt.Sprintf(userAgentTemplate, opts.UserAgentVersion)
	}

	return &Adapter{
		opts:        opts,
		httpClient:  client,
		userAgent:   ua,
		cooldown:    defaultCooldown,
		maxAttempts: defaultMaxAttempts,
	}, nil
}

// Name returns the stable adapter identifier "reddit-rss".
// MUST equal Capabilities().SourceID (REQ-ADP1B-018).
func (a *Adapter) Name() string { return "reddit-rss" }

// Capabilities returns the static descriptor for this adapter (REQ-ADP1B-018).
// Called once at startup by the Intent Router. Always returns the same value.
func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:     "reddit-rss",
		DisplayName:  "Reddit (RSS)",
		DocTypes:     []types.DocType{types.DocTypePost},
		RequiresAuth: false,
		AuthEnvVars:  nil,
		Notes: "Credential-free public-RSS fallback to the OAuth 'reddit' adapter. " +
			"Fetches https://www.reddit.com/search.rss?q=<query>&sort=relevance. " +
			"No credentials required; always registered. " +
			"Result count capped ~25 items (RSS limitation). " +
			"SPEC-ADP-001b.",
	}
}

// Healthcheck probes the configured base host with a lightweight GET request
// and returns nil on HTTP 2xx/3xx (REQ-ADP1B-019).
// Returns *types.SourceError{CategoryUnavailable} on transport failure or 5xx.
func (a *Adapter) Healthcheck(ctx context.Context) error {
	// Probe with a trivial query to the search.rss endpoint.
	probeURL := buildSearchURL(a.opts.BaseURL, "healthcheck")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return &types.SourceError{
			Adapter:  a.Name(),
			Category: types.CategoryUnavailable,
			Cause:    err,
		}
	}
	req.Header.Set("User-Agent", a.userAgent)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return &types.SourceError{
			Adapter:  a.Name(),
			Category: types.CategoryUnavailable,
			Cause:    err,
		}
	}
	_ = resp.Body.Close()

	// 5xx → unavailable; 2xx/3xx → healthy.
	if resp.StatusCode >= 500 {
		return &types.SourceError{
			Adapter:    a.Name(),
			Category:   types.CategoryUnavailable,
			HTTPStatus: resp.StatusCode,
			Cause:      fmt.Errorf("healthcheck: HTTP %d", resp.StatusCode),
		}
	}
	return nil
}

// Compile-time interface assertion. If types.Adapter gains a new method this
// line fails to compile until the method is also added to *Adapter.
var _ types.Adapter = (*Adapter)(nil)
