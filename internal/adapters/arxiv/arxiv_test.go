package arxiv

import (
	"context"
	"net"
	"reflect"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// TestAdapterName verifies the adapter name is "arxiv".
func TestAdapterName(t *testing.T) {
	t.Parallel()
	a, err := New(Options{MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := a.Name(); got != "arxiv" {
		t.Errorf("Name() = %q, want %q", got, "arxiv")
	}
}

// TestAdapterImplementsInterface verifies runtime compliance with types.Adapter.
func TestAdapterImplementsInterface(t *testing.T) {
	t.Parallel()
	a, err := New(Options{MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	var _ types.Adapter = a
}

// TestCapabilitiesDeterministic verifies that two Capabilities() calls return equal values.
func TestCapabilitiesDeterministic(t *testing.T) {
	t.Parallel()
	a, err := New(Options{MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	first := a.Capabilities()
	second := a.Capabilities()
	if !reflect.DeepEqual(first, second) {
		t.Errorf("Capabilities() not deterministic:\nfirst  = %+v\nsecond = %+v", first, second)
	}
}

// TestCapabilitiesShape verifies the static fields of the Capabilities descriptor.
func TestCapabilitiesShape(t *testing.T) {
	t.Parallel()
	a, err := New(Options{MinRequestInterval: 0})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	c := a.Capabilities()

	if c.SourceID != "arxiv" {
		t.Errorf("SourceID = %q, want %q", c.SourceID, "arxiv")
	}
	if c.DisplayName != "arXiv" {
		t.Errorf("DisplayName = %q, want %q", c.DisplayName, "arXiv")
	}
	if len(c.DocTypes) != 1 || c.DocTypes[0] != types.DocTypePaper {
		t.Errorf("DocTypes = %v, want [%v]", c.DocTypes, types.DocTypePaper)
	}
	if c.SupportedLangs != nil {
		t.Errorf("SupportedLangs = %v, want nil", c.SupportedLangs)
	}
	if !c.SupportsSince {
		t.Error("SupportsSince = false, want true")
	}
	if c.RequiresAuth {
		t.Error("RequiresAuth = true, want false")
	}
	if c.AuthEnvVars != nil {
		t.Errorf("AuthEnvVars = %v, want nil", c.AuthEnvVars)
	}
	if c.RateLimitPerMin != 20 {
		t.Errorf("RateLimitPerMin = %d, want 20", c.RateLimitPerMin)
	}
	if c.DefaultMaxResults != 25 {
		t.Errorf("DefaultMaxResults = %d, want 25", c.DefaultMaxResults)
	}

	// Notes must contain required substrings.
	requiredSubstrings := []string{
		"public no-auth",
		"Score=0.5",
		"sortBy=relevance",
	}
	for _, sub := range requiredSubstrings {
		if !containsSubstring(c.Notes, sub) {
			t.Errorf("Notes missing required substring %q\nNotes = %q", sub, c.Notes)
		}
	}
}

// TestNewDefaultsApplied verifies that zero-value Options gets all defaults applied.
func TestNewDefaultsApplied(t *testing.T) {
	t.Parallel()
	a, err := New(Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if a.httpClient == nil {
		t.Error("httpClient = nil, want non-nil default")
	}
	if a.baseURL == "" {
		t.Error("baseURL empty, want defaultBaseURL")
	}
	if a.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", a.baseURL, defaultBaseURL)
	}
	if a.userAgent == "" {
		t.Error("userAgent empty, want non-empty")
	}
	if a.healthcheckTarget == "" {
		t.Error("healthcheckTarget empty, want non-empty")
	}
}

// TestHealthcheckSucceeds verifies that a TCP listener on loopback satisfies Healthcheck.
func TestHealthcheckSucceeds(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	a, err := New(Options{
		HealthcheckTarget:  ln.Addr().String(),
		MinRequestInterval: 0,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := a.Healthcheck(context.Background()); err != nil {
		t.Errorf("Healthcheck() error = %v, want nil", err)
	}
}

// TestHealthcheckFailsWhenNoListener verifies Healthcheck fails if nothing is listening.
func TestHealthcheckFailsWhenNoListener(t *testing.T) {
	t.Parallel()

	// Allocate a port then immediately close it — nothing will listen there.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	a, err := New(Options{
		HealthcheckTarget:  addr,
		MinRequestInterval: 0,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := a.Healthcheck(context.Background()); err == nil {
		t.Error("Healthcheck() = nil, want error (nothing listening)")
	}
}

// containsSubstring checks whether s contains sub.
func containsSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
