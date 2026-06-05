// Package youtube provides a YouTube search adapter implementing types.Adapter.
// The adapter is an HTTP client to a yt-dlp Python sidecar (port 8084 by default).
// REQ-ADP5-001: Interface conformance, Name, Capabilities, Healthcheck.
// SPEC-ADP-005: YouTube video search via yt-dlp sidecar.
package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	// defaultBaseURL is the default sidecar HTTP base URL.
	defaultBaseURL = "http://localhost:8084"

	// defaultHealthcheckPath is the path for the sidecar health endpoint.
	defaultHealthcheckPath = "/health"

	// defaultUserAgentTemplate is the User-Agent header format string.
	// %s is replaced with the version string from Options.UserAgentVersion.
	defaultUserAgentTemplate = "usearch/%s (+https://github.com/elymas/universal-search)"

	// defaultUAVersion is the fallback version when Options.UserAgentVersion is empty.
	defaultUAVersion = "v0.1"
)

// capabilitiesNotes documents the adapter's key operational characteristics.
const capabilitiesNotes = "yt-dlp Python sidecar at port 8084 (default); public no-auth; " +
	"transcript snippet truncated to 500 runes; Korean-locale auto-detection via 30% Hangul threshold; " +
	"max_results + cursor offset cap 100; score=Tanh-of-log10(view_count+1)/5; 30s default Retry-After"

// Options configures the YouTube adapter. All fields are optional; defaults are
// used when a field is the zero value.
type Options struct {
	// BaseURL overrides the sidecar base URL. Primarily used in tests to
	// redirect requests to an httptest.Server. Default: "http://localhost:8084".
	BaseURL string

	// HTTPClient overrides the default *http.Client (30s timeout, reqid transport).
	// Primarily used in tests.
	HTTPClient *http.Client

	// UserAgentVersion overrides the version component of the User-Agent header.
	// Default: "v0.1".
	UserAgentVersion string
}

// Adapter is the YouTube search adapter. It implements types.Adapter.
// All fields are unexported and immutable after construction (REQ-ADP5-010).
type Adapter struct {
	httpClient      *http.Client
	baseURL         string
	userAgent       string
	healthcheckPath string
}

// New constructs a YouTube Adapter from the given Options. Returns an error if
// Options are invalid (reserved for future validation; currently always nil).
func New(opts Options) (*Adapter, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	version := opts.UserAgentVersion
	if version == "" {
		version = defaultUAVersion
	}
	ua := fmt.Sprintf(defaultUserAgentTemplate, version)

	client := opts.HTTPClient
	if client == nil {
		client = newDefaultClient()
	}

	return &Adapter{
		httpClient:      client,
		baseURL:         baseURL,
		userAgent:       ua,
		healthcheckPath: defaultHealthcheckPath,
	}, nil
}

// Name returns the canonical adapter identifier "youtube".
func (a *Adapter) Name() string { return "youtube" }

// Capabilities returns a deterministic descriptor for the YouTube adapter.
// Two consecutive calls return reflect.DeepEqual results.
func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          "youtube",
		DisplayName:       "YouTube",
		DocTypes:          []types.DocType{types.DocTypeVideo},
		SupportedLangs:    nil,
		SupportsSince:     true,
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   30,
		DefaultMaxResults: 25,
		Notes:             capabilitiesNotes,
	}
}

// Healthcheck issues an HTTP GET to the sidecar /health endpoint using the
// caller-supplied ctx. Returns nil when the sidecar responds 200 with
// {"status":"ok",...}. Returns a non-nil error on any failure.
func (a *Adapter) Healthcheck(ctx context.Context) error {
	url := a.baseURL + a.healthcheckPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("youtube: healthcheck request build: %w", err)
	}

	resp, err := a.doRequest(req)
	if err != nil {
		return fmt.Errorf("youtube: healthcheck request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // body close error is non-actionable after read

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("youtube: healthcheck status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("youtube: healthcheck body read: %w", err)
	}

	var health struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &health); err != nil {
		return fmt.Errorf("youtube: healthcheck JSON parse: %w", err)
	}
	if health.Status != "ok" {
		return fmt.Errorf("youtube: healthcheck status field %q (expected \"ok\")", health.Status)
	}
	return nil
}

// Compile-time interface assertion: fails to compile when Adapter drifts from types.Adapter.
var _ types.Adapter = (*Adapter)(nil)
