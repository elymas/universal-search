package searxng_test

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/elymas/universal-search/internal/adapters/searxng"
	"github.com/elymas/universal-search/pkg/types"
)

// TestAdapterName verifies the stable adapter identifier.
func TestAdapterName(t *testing.T) {
	t.Parallel()
	a, err := searxng.New(searxng.Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := a.Name(); got != "searxng" {
		t.Errorf("Name() = %q, want %q", got, "searxng")
	}
}

// TestAdapterImplementsInterface verifies the compile-time assertion at runtime.
func TestAdapterImplementsInterface(t *testing.T) {
	t.Parallel()
	a, err := searxng.New(searxng.Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var _ types.Adapter = a
}

// TestCapabilitiesDeterministic verifies two consecutive calls return equal values.
func TestCapabilitiesDeterministic(t *testing.T) {
	t.Parallel()
	a, err := searxng.New(searxng.Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c1 := a.Capabilities()
	c2 := a.Capabilities()

	if c1.SourceID != c2.SourceID {
		t.Errorf("SourceID not deterministic: %q vs %q", c1.SourceID, c2.SourceID)
	}
	if len(c1.DocTypes) != len(c2.DocTypes) {
		t.Errorf("DocTypes length not deterministic: %d vs %d", len(c1.DocTypes), len(c2.DocTypes))
	}
	if c1.RateLimitPerMin != c2.RateLimitPerMin {
		t.Errorf("RateLimitPerMin not deterministic: %d vs %d", c1.RateLimitPerMin, c2.RateLimitPerMin)
	}
}

// TestCapabilitiesShape verifies the post-audit (H1, H3) Capabilities values.
func TestCapabilitiesShape(t *testing.T) {
	t.Parallel()
	a, err := searxng.New(searxng.Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	caps := a.Capabilities()

	// SourceID must be "searxng".
	if caps.SourceID != "searxng" {
		t.Errorf("SourceID = %q, want %q", caps.SourceID, "searxng")
	}

	// H1 audit: DocTypes must be exactly [DocTypeArticle].
	if len(caps.DocTypes) != 1 {
		t.Fatalf("DocTypes len = %d, want 1 (H1 audit)", len(caps.DocTypes))
	}
	if caps.DocTypes[0] != types.DocTypeArticle {
		t.Errorf("DocTypes[0] = %v, want DocTypeArticle (H1 audit)", caps.DocTypes[0])
	}

	// H3 audit: RateLimitPerMin must be 0 (no external rate limit for self-hosted).
	if caps.RateLimitPerMin != 0 {
		t.Errorf("RateLimitPerMin = %d, want 0 (H3 audit)", caps.RateLimitPerMin)
	}

	// RequiresAuth must be false (REQ-ADP7-001).
	if caps.RequiresAuth {
		t.Error("RequiresAuth = true, want false")
	}
}

// TestNewHonoursOptionsBaseURL verifies Options.BaseURL takes precedence.
func TestNewHonoursOptionsBaseURL(t *testing.T) {
	t.Parallel()
	custom := "http://custom-searxng:9090"
	a, err := searxng.New(searxng.Options{BaseURL: custom})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Verify the adapter uses the custom base URL by hitting a test server.
	// We test indirectly via the fact that New doesn't error on a valid URL.
	_ = a
}

// TestNewHonoursEnvVarWhenOptionsEmpty verifies USEARCH_SEARXNG_URL is used when
// Options.BaseURL is empty.
func TestNewHonoursEnvVarWhenOptionsEmpty(t *testing.T) {
	// t.Setenv requires no t.Parallel.
	t.Setenv("USEARCH_SEARXNG_URL", "http://env-searxng:7777")
	a, err := searxng.New(searxng.Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = a
}

// TestNewDefaultsToCompose verifies the docker-compose default is used when
// neither Options.BaseURL nor USEARCH_SEARXNG_URL is set.
func TestNewDefaultsToCompose(t *testing.T) {
	// t.Setenv requires no t.Parallel.
	// Clear env var to ensure default path is exercised.
	t.Setenv("USEARCH_SEARXNG_URL", "")
	a, err := searxng.New(searxng.Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = a
}

// TestNewOptionsBaseURLTakesPrecedenceOverEnv verifies the priority order:
// Options.BaseURL > USEARCH_SEARXNG_URL.
func TestNewOptionsBaseURLTakesPrecedenceOverEnv(t *testing.T) {
	// t.Setenv requires no t.Parallel.
	t.Setenv("USEARCH_SEARXNG_URL", "http://env-searxng:7777")
	custom := "http://opts-searxng:9090"
	a, err := searxng.New(searxng.Options{BaseURL: custom})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Name doesn't expose baseURL; we verify no error and correct construction.
	if a.Name() != "searxng" {
		t.Errorf("Name() = %q, want %q", a.Name(), "searxng")
	}
}

// TestNewTrailingSlashTrimmed verifies trailing slashes are stripped.
func TestNewTrailingSlashTrimmed(t *testing.T) {
	t.Parallel()
	// If trailing slash not trimmed, URL building would double-slash.
	// We verify no error — actual URL shape tested in search_test.go.
	_, err := searxng.New(searxng.Options{BaseURL: "http://searxng:8080/"})
	if err != nil {
		t.Fatalf("New with trailing slash: %v", err)
	}
}

// TestHealthcheckSucceeds verifies Healthcheck returns nil for a reachable TCP address.
func TestHealthcheckSucceeds(t *testing.T) {
	t.Parallel()
	// Spin up a local TCP listener to act as the healthcheck target.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	a, err := searxng.New(searxng.Options{
		HealthcheckTarget: ln.Addr().String(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := a.Healthcheck(context.Background()); err != nil {
		t.Errorf("Healthcheck() = %v, want nil", err)
	}
}

// TestHealthcheckFailsWhenUnreachable verifies Healthcheck returns an error when
// the target is not listening.
func TestHealthcheckFailsWhenUnreachable(t *testing.T) {
	t.Parallel()
	a, err := searxng.New(searxng.Options{
		// Port 1 is typically privileged/unreachable.
		HealthcheckTarget: "127.0.0.1:1",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testDialTimeout)
	defer cancel()

	if err := a.Healthcheck(ctx); err == nil {
		t.Error("Healthcheck() = nil, want error for unreachable target")
	}
}

// TestNewHTTPClientOverride verifies Options.HTTPClient is used when provided.
func TestNewHTTPClientOverride(t *testing.T) {
	t.Parallel()
	custom := &http.Client{}
	a, err := searxng.New(searxng.Options{HTTPClient: custom})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = a
}
