package reddit

import (
	"context"
	"net"
	"reflect"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

func TestAdapterName(t *testing.T) {
	t.Parallel()
	a, err := New(Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := a.Name(); got != "reddit" {
		t.Errorf("Name() = %q, want %q", got, "reddit")
	}
}

func TestAdapterImplementsInterface(t *testing.T) {
	t.Parallel()
	// The compile-time assertion var _ types.Adapter = (*Adapter)(nil) at the
	// bottom of reddit.go already catches interface drift at build time.
	// This test provides a runtime cross-check.
	a, err := New(Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	var _ types.Adapter = a
}

func TestCapabilitiesDeterministic(t *testing.T) {
	t.Parallel()
	a, err := New(Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	first := a.Capabilities()
	second := a.Capabilities()
	if !reflect.DeepEqual(first, second) {
		t.Errorf("Capabilities() is not deterministic:\nfirst  = %+v\nsecond = %+v", first, second)
	}
}

func TestCapabilitiesShape(t *testing.T) {
	t.Parallel()
	a, err := New(Options{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	c := a.Capabilities()

	if c.SourceID != "reddit" {
		t.Errorf("SourceID = %q, want %q", c.SourceID, "reddit")
	}
	if c.DisplayName != "Reddit" {
		t.Errorf("DisplayName = %q, want %q", c.DisplayName, "Reddit")
	}
	if len(c.DocTypes) != 1 || c.DocTypes[0] != types.DocTypePost {
		t.Errorf("DocTypes = %v, want [%v]", c.DocTypes, types.DocTypePost)
	}
	if c.SupportedLangs != nil {
		t.Errorf("SupportedLangs = %v, want nil", c.SupportedLangs)
	}
	if c.SupportsSince {
		t.Error("SupportsSince = true, want false")
	}
	if c.RequiresAuth {
		t.Error("RequiresAuth = true, want false")
	}
	if c.AuthEnvVars != nil {
		t.Errorf("AuthEnvVars = %v, want nil", c.AuthEnvVars)
	}
	if c.RateLimitPerMin != 10 {
		t.Errorf("RateLimitPerMin = %d, want 10", c.RateLimitPerMin)
	}
	if c.DefaultMaxResults != 25 {
		t.Errorf("DefaultMaxResults = %d, want 25", c.DefaultMaxResults)
	}

	// Notes must contain 4 required substrings.
	requiredSubstrings := []string{
		"public no-auth",
		"NSFW excluded by default",
		"t=all",
		"rate limit discrepancy",
	}
	for _, sub := range requiredSubstrings {
		if !containsSubstring(c.Notes, sub) {
			t.Errorf("Notes missing required substring %q\nNotes = %q", sub, c.Notes)
		}
	}
}

func TestHealthcheckSucceeds(t *testing.T) {
	t.Parallel()

	// Start a TCP listener on a loopback address so Healthcheck can dial it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer ln.Close()
	// Accept connections in the background so the dial completes.
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
		HealthcheckTarget: ln.Addr().String(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := a.Healthcheck(context.Background()); err != nil {
		t.Errorf("Healthcheck() error = %v, want nil", err)
	}
}

// containsSubstring is a simple helper to check substring presence.
func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && findSubstring(s, sub)
}

func findSubstring(s, sub string) bool {
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
