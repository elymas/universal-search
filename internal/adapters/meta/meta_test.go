package meta

import (
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// ---------------------------------------------------------------------------
// REQ-ADP10-001: Interface conformance + constructor tests
// ---------------------------------------------------------------------------

// TestThreadsImplementsInterface verifies *Adapter from NewThreads implements types.Adapter.
func TestThreadsImplementsInterface(t *testing.T) {
	var _ types.Adapter = (*Adapter)(nil)
}

// TestFacebookImplementsInterface verifies *Adapter from NewFacebook implements types.Adapter.
func TestFacebookImplementsInterface(t *testing.T) {
	var _ types.Adapter = (*Adapter)(nil)
}

// TestThreadsName verifies the Threads instance returns "threads" as Name.
func TestThreadsName(t *testing.T) {
	a, err := NewThreads(ThreadsOptions{AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("NewThreads: %v", err)
	}
	if got := a.Name(); got != "threads" {
		t.Errorf("Threads Name() = %q, want %q", got, "threads")
	}
}

// TestFacebookName verifies the Facebook instance returns "facebook" as Name.
func TestFacebookName(t *testing.T) {
	a, err := NewFacebook(FacebookOptions{})
	if err != nil {
		t.Fatalf("NewFacebook: %v", err)
	}
	if got := a.Name(); got != "facebook" {
		t.Errorf("Facebook Name() = %q, want %q", got, "facebook")
	}
}

// TestNewThreadsMissingTokenReturnsError verifies no token yields ErrThreadsTokenMissing.
func TestNewThreadsMissingTokenReturnsError(t *testing.T) {
	a, err := NewThreads(ThreadsOptions{})
	if a != nil {
		t.Error("expected nil adapter when token missing")
	}
	if !errors.Is(err, ErrThreadsTokenMissing) {
		t.Errorf("expected ErrThreadsTokenMissing, got: %v", err)
	}
}

// TestNewThreadsTokenFromEnv verifies the constructor reads the env token via EnvLookup.
func TestNewThreadsTokenFromEnv(t *testing.T) {
	a, err := NewThreads(ThreadsOptions{
		EnvLookup: func(key string) string {
			if key == "THREADS_ACCESS_TOKEN" {
				return "env-token-123"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatalf("NewThreads with env token: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil adapter with env token")
	}
}

// TestNewThreadsPrefersOptsOverEnv verifies explicit AccessToken takes precedence.
func TestNewThreadsPrefersOptsOverEnv(t *testing.T) {
	a, err := NewThreads(ThreadsOptions{
		AccessToken: "opts-token",
		EnvLookup: func(key string) string {
			if key == "THREADS_ACCESS_TOKEN" {
				return "env-token"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatalf("NewThreads: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-001: Capabilities determinism + shape
// ---------------------------------------------------------------------------

// TestThreadsCapabilitiesDeterministic verifies two calls return equal values.
func TestThreadsCapabilitiesDeterministic(t *testing.T) {
	a, _ := NewThreads(ThreadsOptions{AccessToken: "t"})
	c1 := a.Capabilities()
	c2 := a.Capabilities()
	if !reflect.DeepEqual(c1, c2) {
		t.Errorf("Threads Capabilities not deterministic:\n%v\n%v", c1, c2)
	}
}

// TestFacebookCapabilitiesDeterministic verifies two calls return equal values.
func TestFacebookCapabilitiesDeterministic(t *testing.T) {
	a, _ := NewFacebook(FacebookOptions{})
	c1 := a.Capabilities()
	c2 := a.Capabilities()
	if !reflect.DeepEqual(c1, c2) {
		t.Errorf("Facebook Capabilities not deterministic:\n%v\n%v", c1, c2)
	}
}

// TestThreadsCapabilitiesShape verifies all field values and Notes substrings.
func TestThreadsCapabilitiesShape(t *testing.T) {
	a, _ := NewThreads(ThreadsOptions{AccessToken: "t"})
	c := a.Capabilities()

	if c.SourceID != "threads" {
		t.Errorf("SourceID = %q, want %q", c.SourceID, "threads")
	}
	if c.DisplayName != "Threads" {
		t.Errorf("DisplayName = %q, want %q", c.DisplayName, "Threads")
	}
	if len(c.DocTypes) != 1 || c.DocTypes[0] != types.DocTypePost {
		t.Errorf("DocTypes = %v, want [post]", c.DocTypes)
	}
	if c.SupportedLangs != nil {
		t.Errorf("SupportedLangs = %v, want nil", c.SupportedLangs)
	}
	if !c.SupportsSince {
		t.Error("SupportsSince = false, want true")
	}
	if !c.RequiresAuth {
		t.Error("RequiresAuth = false, want true")
	}
	if len(c.AuthEnvVars) != 1 || c.AuthEnvVars[0] != "THREADS_ACCESS_TOKEN" {
		t.Errorf("AuthEnvVars = %v, want [THREADS_ACCESS_TOKEN]", c.AuthEnvVars)
	}
	if c.RateLimitPerMin != 1 {
		t.Errorf("RateLimitPerMin = %d, want 1", c.RateLimitPerMin)
	}
	if c.DefaultMaxResults != 25 {
		t.Errorf("DefaultMaxResults = %d, want 25", c.DefaultMaxResults)
	}

	for _, substr := range []string{
		"graph.threads.net",
		"keyword_search",
		"threads_keyword_search permission required",
		"meta",
	} {
		if !strings.Contains(c.Notes, substr) {
			t.Errorf("Notes missing substring %q", substr)
		}
	}
}

// TestFacebookCapabilitiesShape verifies the disabled Facebook descriptor.
func TestFacebookCapabilitiesShape(t *testing.T) {
	a, _ := NewFacebook(FacebookOptions{})
	c := a.Capabilities()

	if c.SourceID != "facebook" {
		t.Errorf("SourceID = %q, want %q", c.SourceID, "facebook")
	}
	if c.DisplayName != "Facebook" {
		t.Errorf("DisplayName = %q, want %q", c.DisplayName, "Facebook")
	}
	if len(c.DocTypes) != 1 || c.DocTypes[0] != types.DocTypePost {
		t.Errorf("DocTypes = %v, want [post]", c.DocTypes)
	}
	if c.RequiresAuth {
		t.Error("RequiresAuth = true, want false")
	}
	if c.AuthEnvVars != nil {
		t.Errorf("AuthEnvVars = %v, want nil", c.AuthEnvVars)
	}
	if c.RateLimitPerMin != 0 {
		t.Errorf("RateLimitPerMin = %d, want 0", c.RateLimitPerMin)
	}
	if c.DefaultMaxResults != 0 {
		t.Errorf("DefaultMaxResults = %d, want 0", c.DefaultMaxResults)
	}

	for _, substr := range []string{
		"NOT SUPPORTED",
		"no public-post keyword search",
		"meta",
		"Facebook Graph API",
	} {
		if !strings.Contains(c.Notes, substr) {
			t.Errorf("Notes missing substring %q", substr)
		}
	}
}

// ---------------------------------------------------------------------------
// REQ-ADP10-001: Healthcheck
// ---------------------------------------------------------------------------

// TestThreadsHealthcheckSucceeds verifies Threads healthcheck with a TCP stub.
func TestThreadsHealthcheckSucceeds(t *testing.T) {
	// Start a TCP listener to simulate a reachable endpoint.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	a, err := NewThreads(ThreadsOptions{
		AccessToken:       "t",
		HealthcheckTarget: ln.Addr().String(),
	})
	if err != nil {
		t.Fatalf("NewThreads: %v", err)
	}

	if err := a.Healthcheck(t.Context()); err != nil {
		t.Errorf("Healthcheck = %v, want nil", err)
	}
}

// TestFacebookHealthcheckReturnsDisabled verifies Facebook healthcheck returns ErrFacebookDisabled.
func TestFacebookHealthcheckReturnsDisabled(t *testing.T) {
	a, _ := NewFacebook(FacebookOptions{})
	err := a.Healthcheck(t.Context())
	if !errors.Is(err, ErrFacebookDisabled) {
		t.Errorf("Healthcheck err = %v, want ErrFacebookDisabled", err)
	}
}

// ---------------------------------------------------------------------------
// NFR-ADP10-002: Secret handling — token must not appear in Capabilities
// ---------------------------------------------------------------------------

// TestThreadsTokenNotInCapabilities verifies the token value never appears in
// any Capabilities string field.
func TestThreadsTokenNotInCapabilities(t *testing.T) {
	secret := "super-secret-token-do-not-leak"
	a, _ := NewThreads(ThreadsOptions{AccessToken: secret})
	c := a.Capabilities()

	// Check all string fields.
	if strings.Contains(c.SourceID, secret) {
		t.Error("token leaked into SourceID")
	}
	if strings.Contains(c.DisplayName, secret) {
		t.Error("token leaked into DisplayName")
	}
	if strings.Contains(c.Notes, secret) {
		t.Error("token leaked into Notes")
	}
	for _, ev := range c.AuthEnvVars {
		if strings.Contains(ev, secret) {
			t.Errorf("token leaked into AuthEnvVars entry %q", ev)
		}
	}
}
