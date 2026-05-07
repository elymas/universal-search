// Package arxiv implements the types.Adapter interface for the arXiv public
// Atom API (https://export.arxiv.org/api/query). It provides paper search
// with category filtering, rate-limit enforcement, and cursor-based pagination.
//
// The adapter is sole-emitter-clean: it emits zero metrics, logs, or spans.
// All observability is handled by the registry's wrappedAdapter at
// internal/adapters/registry.go.
package arxiv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	defaultBaseURL           = "https://export.arxiv.org/api/query"
	defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"
	defaultUAVersion         = "v0.1"
	defaultHealthcheckTarget = "export.arxiv.org:443"
	defaultMinInterval       = 3 * time.Second
	constantScore            = 0.5
)

// Options configures an arXiv adapter instance. All fields are optional;
// zero values receive safe defaults.
type Options struct {
	// BaseURL overrides the default arXiv API endpoint. Useful in tests.
	BaseURL string

	// HTTPClient replaces the default *http.Client. Useful in tests.
	HTTPClient *http.Client

	// UserAgentVersion overrides the default version segment in the User-Agent.
	UserAgentVersion string

	// HealthcheckTarget overrides the default TCP dial target.
	HealthcheckTarget string

	// MinRequestInterval enforces a minimum time between successive requests.
	// Set to 0 in tests to disable the rate gate.
	// Default: 3 seconds (arXiv courtesy guideline).
	MinRequestInterval time.Duration
}

// Adapter implements types.Adapter for the arXiv public API.
//
// // @MX:ANCHOR: [AUTO] Public API entry point implementing types.Adapter.
// // @MX:REASON: fan_in >= 3 (New callers: registry, tests, benchmark)
// // @MX:SPEC: SPEC-ADP-003
type Adapter struct {
	httpClient        *http.Client
	baseURL           string
	userAgent         string
	healthcheckTarget string

	// Rate-limit state (REQ-ADP3-012). Per-instance; mutex-guarded.
	// @MX:WARN: [AUTO] rateMu guards nextRequest; acquire briefly, sleep outside.
	// @MX:REASON: timer sleep must be outside mutex to allow ctx cancellation.
	rateMu      sync.Mutex
	nextRequest time.Time
	minInterval time.Duration
}

// New constructs a new Adapter with the given options. All zero-value option
// fields receive safe defaults.
//
// // @MX:ANCHOR: [AUTO] Constructor exported to registry and all callers.
// // @MX:REASON: fan_in >= 3 (registry, arxiv_test.go, rate_test.go, search_test.go)
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
	// MinRequestInterval: apply the 3-second default only for production callers
	// (identified by no BaseURL override). Tests always supply BaseURL and set
	// MinRequestInterval: 0 explicitly to disable the rate gate.
	interval := opts.MinRequestInterval
	if interval == 0 && opts.BaseURL == "" {
		interval = defaultMinInterval
	}
	return &Adapter{
		httpClient:        client,
		baseURL:           base,
		userAgent:         ua,
		healthcheckTarget: target,
		minInterval:       interval,
	}, nil
}

// Name returns the canonical adapter name used as SourceID in NormalizedDoc.
func (a *Adapter) Name() string { return "arxiv" }

// Capabilities returns a static descriptor for the arXiv adapter.
func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          "arxiv",
		DisplayName:       "arXiv",
		DocTypes:          []types.DocType{types.DocTypePaper},
		SupportedLangs:    nil,
		SupportsSince:     true,
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   20,
		DefaultMaxResults: 25,
		Notes: "arXiv public no-auth API endpoint " +
			"(https://export.arxiv.org/api/query). 3-second courtesy " +
			"interval enforced per arXiv guideline (configurable via " +
			"Options.MinRequestInterval). sortBy=relevance hardcoded; " +
			"Score=0.5 constant (arXiv has no per-paper relevance score). " +
			"LaTeX pass-through (mathematical notation in titles and " +
			"abstracts is preserved as plain text). Filter keys: " +
			"'category' (e.g., 'cs.AI', 'math.GT'). " +
			"SupportsSince=true is forward-compat for date-range " +
			"translation (deferred to P2 enhancement).",
	}
}

// Healthcheck verifies reachability of the arXiv API host via a TCP dial.
// Returns nil on success, non-nil on failure.
func (a *Adapter) Healthcheck(ctx context.Context) error {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", a.healthcheckTarget)
	if err != nil {
		return fmt.Errorf("arxiv: healthcheck dial %q: %w", a.healthcheckTarget, err)
	}
	_ = conn.Close()
	return nil
}

// Compile-time assertion that *Adapter satisfies types.Adapter.
var _ types.Adapter = (*Adapter)(nil)
