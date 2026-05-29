// Package naver provides a Naver suite adapter implementing types.Adapter.
// REQ-ADP8-001: Interface conformance, Name, Capabilities, Healthcheck.
// SPEC-ADP-008: Multi-vertical Naver search adapter (blog, news, web, shop, DataLab).
//
// Authentication: requires NAVER_CLIENT_ID and NAVER_CLIENT_SECRET environment
// variables declared in Capabilities.AuthEnvVars and resolved in New().
package naver

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/elymas/universal-search/internal/security/secretstore"
	"github.com/elymas/universal-search/pkg/types"
)

// secretEnv resolves NAVER_* credentials (REQ-SEC-016). It is the default env
// backend (os.Getenv semantics); behaviour at the call site is identical — an
// unset variable yields an empty value that triggers the existing "not set"
// error path. The resolver is immutable and stateless.
var secretEnv secretstore.Resolver = secretstore.NewEnvResolver()

const (
	// baseURLBlog is the Naver Blog search API endpoint.
	baseURLBlog = "https://openapi.naver.com/v1/search/blog.json"

	// baseURLNews is the Naver News search API endpoint.
	baseURLNews = "https://openapi.naver.com/v1/search/news.json"

	// baseURLWeb is the Naver Web search API endpoint.
	baseURLWeb = "https://openapi.naver.com/v1/search/webkr.json"

	// baseURLShop is the Naver Shopping search API endpoint.
	baseURLShop = "https://openapi.naver.com/v1/search/shop.json"

	// baseURLDataLab is the Naver DataLab trends POST endpoint.
	baseURLDataLab = "https://openapi.naver.com/v1/datalab/search"

	// defaultHealthcheckTarget is the TCP address for the Healthcheck dial.
	defaultHealthcheckTarget = "openapi.naver.com:443"

	// verticalBlog is the filter value for the blog search vertical.
	verticalBlog = "blog"

	// verticalNews is the filter value for the news search vertical.
	verticalNews = "news"

	// verticalWeb is the filter value for the web search vertical.
	verticalWeb = "web"

	// verticalShop is the filter value for the shopping search vertical.
	verticalShop = "shop"

	// verticalDataLab is the filter value for the DataLab trend API.
	verticalDataLab = "datalab"

	// filterKeyVertical is the Query.Filters key used to select the search vertical.
	filterKeyVertical = "naver_vertical"
)

// Options configures the Naver adapter. All fields are optional; defaults are
// used when a field is the zero value. Credentials are resolved from environment
// variables when Options.ClientID / ClientSecret are empty.
type Options struct {
	// ClientID overrides the NAVER_CLIENT_ID environment variable. Tests inject
	// a dummy value here to avoid requiring live credentials.
	ClientID string

	// ClientSecret overrides the NAVER_CLIENT_SECRET environment variable.
	ClientSecret string

	// BaseURLBlog overrides the blog search endpoint. Used in tests.
	BaseURLBlog string

	// BaseURLNews overrides the news search endpoint. Used in tests.
	BaseURLNews string

	// BaseURLWeb overrides the web search endpoint. Used in tests.
	BaseURLWeb string

	// BaseURLShop overrides the shopping search endpoint. Used in tests.
	BaseURLShop string

	// BaseURLDataLab overrides the DataLab POST endpoint. Used in tests.
	BaseURLDataLab string

	// HTTPClient overrides the default *http.Client. Used in tests.
	HTTPClient *http.Client

	// HealthcheckTarget overrides the TCP address for Healthcheck.
	// Default: "openapi.naver.com:443". Tests inject a loopback address.
	HealthcheckTarget string
}

// Adapter is the Naver suite search adapter. It implements types.Adapter.
// All fields are unexported and immutable after construction (REQ-ADP8-011).
type Adapter struct {
	httpClient        *http.Client
	clientID          string
	clientSecret      string
	baseURLBlog       string
	baseURLNews       string
	baseURLWeb        string
	baseURLShop       string
	baseURLDataLab    string
	healthcheckTarget string
}

// New constructs a Naver Adapter from the given Options.
// Credentials are resolved (in order): Options fields → environment variables.
// Returns an error if neither source provides non-empty credentials.
//
// @MX:ANCHOR: [AUTO] Constructor; called by registry, tests, and CLI.
// @MX:REASON: fan_in >= 3; sole construction path for the Naver adapter.
// @MX:SPEC: SPEC-ADP-008
func New(opts Options) (*Adapter, error) {
	clientID := opts.ClientID
	if clientID == "" {
		clientID, _ = secretEnv.Get(context.Background(), "NAVER_CLIENT_ID")
	}
	if clientID == "" {
		return nil, fmt.Errorf("naver: NAVER_CLIENT_ID not set")
	}

	clientSecret := opts.ClientSecret
	if clientSecret == "" {
		clientSecret, _ = secretEnv.Get(context.Background(), "NAVER_CLIENT_SECRET")
	}
	if clientSecret == "" {
		return nil, fmt.Errorf("naver: NAVER_CLIENT_SECRET not set")
	}

	client := opts.HTTPClient
	if client == nil {
		client = newDefaultClient()
	}

	urlBlog := opts.BaseURLBlog
	if urlBlog == "" {
		urlBlog = baseURLBlog
	}
	urlNews := opts.BaseURLNews
	if urlNews == "" {
		urlNews = baseURLNews
	}
	urlWeb := opts.BaseURLWeb
	if urlWeb == "" {
		urlWeb = baseURLWeb
	}
	urlShop := opts.BaseURLShop
	if urlShop == "" {
		urlShop = baseURLShop
	}
	urlDataLab := opts.BaseURLDataLab
	if urlDataLab == "" {
		urlDataLab = baseURLDataLab
	}

	target := opts.HealthcheckTarget
	if target == "" {
		target = defaultHealthcheckTarget
	}

	return &Adapter{
		httpClient:        client,
		clientID:          clientID,
		clientSecret:      clientSecret,
		baseURLBlog:       urlBlog,
		baseURLNews:       urlNews,
		baseURLWeb:        urlWeb,
		baseURLShop:       urlShop,
		baseURLDataLab:    urlDataLab,
		healthcheckTarget: target,
	}, nil
}

// Name returns the stable adapter identifier "naver".
// This value is used as the Prometheus label and registry key.
func (a *Adapter) Name() string { return "naver" }

// Capabilities returns a deterministic descriptor for the Naver adapter.
// Called by the Intent Router at startup. The returned value is immutable.
func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:    "naver",
		DisplayName: "Naver",
		DocTypes: []types.DocType{
			types.DocTypePost,    // blog
			types.DocTypeArticle, // news
			types.DocTypeOther,   // web, shop, datalab
		},
		SupportedLangs:    []string{"ko"},
		SupportsSince:     false,
		RequiresAuth:      true,
		AuthEnvVars:       []string{"NAVER_CLIENT_ID", "NAVER_CLIENT_SECRET"},
		RateLimitPerMin:   10,
		DefaultMaxResults: 25,
		Notes: "Naver multi-vertical search adapter (blog, news, web, shop, datalab). " +
			"Auth via NAVER_CLIENT_ID + NAVER_CLIENT_SECRET. " +
			"Select vertical with Query.Filters[{naver_vertical, <vertical>}]; default=blog. " +
			"DataLab: Query.Text must be JSON-encoded request body.",
	}
}

// Healthcheck probes Naver API reachability via TCP connect to a.healthcheckTarget.
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
var _ types.Adapter = (*Adapter)(nil)
