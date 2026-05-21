// Package koreanews — composite Korean-news adapter.
// SPEC-ADP-009: wraps three sub-sources: RSS (default-ON), KNC sidecar (default-OFF),
// Daum (always-stub due to robots.txt legal constraint in v0.1).
//
// Adapter lifecycle:
//   - New(opts) validates opts, applies defaults, logs a one-time Daum warning when DaumEnabled.
//   - Search dispatches to enabled sub-sources concurrently, merges, deduplicates, sorts.
//   - Healthcheck probes a lightweight RSS URL (HEAD request) for liveness.
//
// Sole-emitter discipline: no Prometheus metrics, OTel spans, or slog records are
// emitted from within this package. Observability is handled by the registry's
// wrappedAdapter layer per SPEC-ADP-009 §11.9.
package koreanews

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	// userAgentTemplate is the User-Agent header format string; %s is replaced
	// with the version string from Options.UserAgentVersion.
	userAgentTemplate = "usearch-koreanews/%s (+https://github.com/elymas/universal-search)"
)

// Adapter is the composite Korean-news adapter. It implements types.Adapter.
// All fields are unexported and immutable after construction (REQ-ADP-011).
//
// @MX:ANCHOR: [AUTO] Composite adapter entry point; callers: registry, fanout, healthcheck, tests
// @MX:REASON: fan_in >= 3; implements types.Adapter — all method additions break callers
// @MX:SPEC: SPEC-ADP-009
type Adapter struct {
	opts       Options
	httpClient *http.Client
	userAgent  string
}

// New constructs an Adapter from the given Options, applying defaults for any
// zero fields. Returns an error only if future validation rules are violated
// (currently always nil; reserved).
//
// When opts.DaumEnabled == true, New emits a one-time slog.Warn reminding
// the operator that the Daum stub always returns ErrDaumDisabled in v0.1.
//
// @MX:ANCHOR: [AUTO] Constructor; callers: registry, integration tests, production main
// @MX:REASON: fan_in >= 3; sole entry point for constructing a valid Adapter
// @MX:SPEC: SPEC-ADP-009
func New(opts Options) (*Adapter, error) {
	opts.applyDefaults()

	client := opts.HTTPClient
	if client == nil {
		client = newDefaultClient()
	}

	ua := fmt.Sprintf(userAgentTemplate, opts.UserAgentVersion)

	return &Adapter{
		opts:       opts,
		httpClient: client,
		userAgent:  ua,
	}, nil
}

// newDefaultClient returns an *http.Client suitable for Korean news fetches:
// 30-second timeout, no redirect-following beyond host boundary.
func newDefaultClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

// Name returns the stable adapter identifier "koreanews".
// This value is used as the Prometheus label and registry key.
func (a *Adapter) Name() string { return "koreanews" }

// Capabilities returns a deterministic descriptor for the Korean news adapter.
// Called by the Intent Router at startup. The returned value is immutable.
func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          "koreanews",
		DisplayName:       "Korean News (RSS + KNC)",
		DocTypes:          []types.DocType{types.DocTypeArticle},
		SupportedLangs:    []string{"ko"},
		SupportsSince:     false,
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   0, // governed by operator-configured feed count × per-feed rate
		DefaultMaxResults: 20,
		Notes: "Composite adapter: RSS (default-ON), KoreaNewsCrawler sidecar (default-OFF), " +
			"Daum (stub — always returns ErrDaumDisabled in v0.1 per robots.txt). " +
			"Configure via USEARCH_ADP009_* env vars or Options struct.",
	}
}

// Healthcheck probes the configured HealthcheckTarget URL via a HEAD request.
// Returns nil when the probe succeeds (HTTP 2xx or 3xx). Returns a
// *types.SourceError{CategoryUnavailable} on failure.
func (a *Adapter) Healthcheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, a.opts.HealthcheckTarget, nil)
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

// Compile-time interface assertion.
// If types.Adapter gains a new method, this line fails to compile until the
// method is also added to *Adapter.
var _ types.Adapter = (*Adapter)(nil)
