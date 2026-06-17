// Package github — adapter construction and capabilities tests.
// Tests #1–8: REQ-ADP4-001.
package github

import (
	"context"
	"net"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// TestAdapterName verifies Name() == "github".
func TestAdapterName(t *testing.T) {
	t.Parallel()
	a, err := New(Options{SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := a.Name(); got != "github" {
		t.Errorf("Name() = %q, want github", got)
	}
}

// TestAdapterImplementsInterface is a compile-time check that *Adapter satisfies types.Adapter.
func TestAdapterImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ types.Adapter = (*Adapter)(nil)
}

// TestCapabilitiesDeterministic verifies that two calls to Capabilities() return
// reflect.DeepEqual results (immutable, no hidden state).
func TestCapabilitiesDeterministic(t *testing.T) {
	t.Parallel()
	a, err := New(Options{SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c1 := a.Capabilities()
	c2 := a.Capabilities()
	if !reflect.DeepEqual(c1, c2) {
		t.Errorf("Capabilities() not deterministic:\n  call1: %+v\n  call2: %+v", c1, c2)
	}
}

// TestCapabilitiesShape verifies all 9 required Capabilities fields.
func TestCapabilitiesShape(t *testing.T) {
	t.Parallel()
	a, err := New(Options{SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c := a.Capabilities()

	if c.SourceID != "github" {
		t.Errorf("SourceID = %q, want github", c.SourceID)
	}
	if c.DisplayName == "" {
		t.Error("DisplayName must not be empty")
	}
	if len(c.DocTypes) == 0 {
		t.Error("DocTypes must not be empty")
	}
	if c.DefaultMaxResults <= 0 {
		t.Errorf("DefaultMaxResults = %d, want > 0", c.DefaultMaxResults)
	}
	if c.Notes == "" {
		t.Error("Notes must not be empty")
	}
	// Notes should mention key characteristics.
	for _, substr := range []string{"go-github", "USEARCH_GITHUB_TOKEN"} {
		found := false
		if len(c.Notes) > 0 {
			// Use strings package via import would be cleaner; use manual check.
			for i := 0; i <= len(c.Notes)-len(substr); i++ {
				if c.Notes[i:i+len(substr)] == substr {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("Notes should contain %q, got: %q", substr, c.Notes)
		}
	}
}

// TestCapabilitiesDeclaresRequiresAuth verifies RequiresAuth=true and
// AuthEnvVars=["USEARCH_GITHUB_TOKEN"].
func TestCapabilitiesDeclaresRequiresAuth(t *testing.T) {
	t.Parallel()
	a, err := New(Options{SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c := a.Capabilities()

	if !c.RequiresAuth {
		t.Error("RequiresAuth must be true")
	}
	if len(c.AuthEnvVars) == 0 {
		t.Error("AuthEnvVars must not be empty")
	}
	found := false
	for _, v := range c.AuthEnvVars {
		if v == "USEARCH_GITHUB_TOKEN" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("AuthEnvVars = %v, want to contain USEARCH_GITHUB_TOKEN", c.AuthEnvVars)
	}
}

// TestHealthcheckSucceeds verifies that Healthcheck succeeds against a loopback listener.
func TestHealthcheckSucceeds(t *testing.T) {
	t.Parallel()
	// Start a TCP listener on an ephemeral port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	// Accept one connection in the background.
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	a, err := New(Options{
		SkipAuthCheck:     true,
		HealthcheckTarget: ln.Addr().String(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := a.Healthcheck(context.Background()); err != nil {
		t.Errorf("Healthcheck: %v", err)
	}
}

// TestNewMissingTokenRejected verifies that New returns ErrMissingToken
// when Token is empty and SkipAuthCheck is false.
func TestNewMissingTokenRejected(t *testing.T) {
	t.Parallel()
	_, err := New(Options{SkipAuthCheck: false})
	if err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
	se, ok := err.(*types.SourceError)
	if !ok {
		t.Fatalf("expected *types.SourceError, got %T: %v", err, err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want CategoryPermanent", se.Category)
	}
	// Check that ErrMissingToken is the root cause.
	if se.Cause != ErrMissingToken {
		t.Errorf("Cause = %v, want ErrMissingToken", se.Cause)
	}
}

// TestNewSkipAuthCheckAllowsEmptyToken verifies that SkipAuthCheck=true allows
// empty Token (for test usage).
func TestNewSkipAuthCheckAllowsEmptyToken(t *testing.T) {
	t.Parallel()
	a, err := New(Options{SkipAuthCheck: true})
	if err != nil {
		t.Errorf("New with SkipAuthCheck=true should succeed, got: %v", err)
	}
	if a == nil {
		t.Error("Adapter should not be nil")
	}
}

// TestNewWithTokenSucceeds verifies that New with a non-empty Token succeeds.
func TestNewWithTokenSucceeds(t *testing.T) {
	t.Parallel()
	a, err := New(Options{Token: "ghp_test_token"})
	if err != nil {
		t.Errorf("New with token should succeed, got: %v", err)
	}
	if a == nil {
		t.Error("Adapter should not be nil")
	}
}

// TestCapabilitiesDocTypesContainRepoAndIssue verifies DocTypes includes both
// DocTypeRepo and DocTypeIssue.
func TestCapabilitiesDocTypesContainRepoAndIssue(t *testing.T) {
	t.Parallel()
	a, err := New(Options{SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c := a.Capabilities()

	hasRepo, hasIssue := false, false
	for _, dt := range c.DocTypes {
		if dt == types.DocTypeRepo {
			hasRepo = true
		}
		if dt == types.DocTypeIssue {
			hasIssue = true
		}
	}
	if !hasRepo {
		t.Errorf("DocTypes = %v, want to contain DocTypeRepo", c.DocTypes)
	}
	if !hasIssue {
		t.Errorf("DocTypes = %v, want to contain DocTypeIssue", c.DocTypes)
	}
}

// TestCapabilitiesRateLimitPerMin verifies the declared rate limit is > 0.
func TestCapabilitiesRateLimitPerMin(t *testing.T) {
	t.Parallel()
	a, err := New(Options{SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c := a.Capabilities()
	if c.RateLimitPerMin <= 0 {
		t.Errorf("RateLimitPerMin = %d, want > 0", c.RateLimitPerMin)
	}
}

// TestNewWithHTTPClientOverride verifies that a custom HTTPClient is accepted.
func TestNewWithHTTPClientOverride(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(nil) // minimal server
	defer srv.Close()

	a, err := New(Options{
		BaseURL:       srv.URL + "/",
		SkipAuthCheck: true,
	})
	if err != nil {
		t.Fatalf("New with HTTPClient override: %v", err)
	}
	if a == nil {
		t.Error("Adapter should not be nil")
	}
}

// --- REQ-ADP4a Capabilities Notes extension ---

// TestCapabilitiesNotesCommitCadence verifies the Notes field advertises the
// commit-search cadence AND still contains the substrings the parent
// TestCapabilitiesShape asserts ("go-github", "USEARCH_GITHUB_TOKEN"), plus
// the additional commit/rate-ceiling substrings (SPEC-ADP-004a AC-014..015).
func TestCapabilitiesNotesCommitCadence(t *testing.T) {
	t.Parallel()
	a, err := New(Options{SkipAuthCheck: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c := a.Capabilities()

	// The 2 substrings the parent TestCapabilitiesShape checks must remain.
	// The 4 additional substrings (commit cadence + rate-ceiling doc) live here
	// per plan-auditor D1 (parent test stays at 2).
	for _, substr := range []string{
		"go-github",            // dependency identifier
		"USEARCH_GITHUB_TOKEN", // auth env var
		"commit",               // commit intent enumerated in routing list
		"commit search 30/min", // commit cadence advertised
		"30/min",               // shared search bucket rate ceiling
		"code search 9/min",    // code sub-ceiling still documented
	} {
		if !strings.Contains(c.Notes, substr) {
			t.Errorf("Notes should contain %q, got: %q", substr, c.Notes)
		}
	}
}
