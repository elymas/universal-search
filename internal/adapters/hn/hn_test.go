// Package hn — adapter interface conformance and Capabilities tests.
// REQ-ADP2-001: TestAdapterName, TestCapabilitiesDeterministic, TestCapabilitiesShape,
// TestHealthcheckSucceeds.
package hn

import (
	"context"
	"net"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// TestAdapterImplementsInterface verifies the compile-time assertion holds at runtime.
// The var _ types.Adapter = (*Adapter)(nil) at the bottom of hn.go is the primary check;
// this test is a belt-and-suspenders runtime confirmation.
func TestAdapterImplementsInterface(t *testing.T) {
	t.Parallel()
	a, err := New(Options{})
	if err != nil {
		t.Fatalf("New() returned unexpected error: %v", err)
	}
	var _ types.Adapter = a // compile-time; also a runtime assignment.
}

// TestAdapterName verifies Name() returns the stable "hackernews" identifier.
func TestAdapterName(t *testing.T) {
	t.Parallel()
	a, err := New(Options{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if got := a.Name(); got != "hackernews" {
		t.Errorf("Name() = %q; want %q", got, "hackernews")
	}
}

// TestCapabilitiesDeterministic verifies two consecutive Capabilities() calls return
// reflect.DeepEqual results.
func TestCapabilitiesDeterministic(t *testing.T) {
	t.Parallel()
	a, err := New(Options{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	c1 := a.Capabilities()
	c2 := a.Capabilities()
	if !reflect.DeepEqual(c1, c2) {
		t.Errorf("Capabilities() not deterministic:\nfirst:  %+v\nsecond: %+v", c1, c2)
	}
}

// TestCapabilitiesShape verifies all 9 documented field values and Notes substrings.
func TestCapabilitiesShape(t *testing.T) {
	t.Parallel()
	a, err := New(Options{})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	cap := a.Capabilities()

	if cap.SourceID != "hackernews" {
		t.Errorf("SourceID = %q; want %q", cap.SourceID, "hackernews")
	}
	if cap.DisplayName != "Hacker News" {
		t.Errorf("DisplayName = %q; want %q", cap.DisplayName, "Hacker News")
	}
	if len(cap.DocTypes) != 1 || cap.DocTypes[0] != types.DocTypePost {
		t.Errorf("DocTypes = %v; want [DocTypePost]", cap.DocTypes)
	}
	if cap.SupportedLangs != nil {
		t.Errorf("SupportedLangs = %v; want nil", cap.SupportedLangs)
	}
	if !cap.SupportsSince {
		t.Error("SupportsSince = false; want true")
	}
	if cap.RequiresAuth {
		t.Error("RequiresAuth = true; want false")
	}
	if cap.AuthEnvVars != nil {
		t.Errorf("AuthEnvVars = %v; want nil", cap.AuthEnvVars)
	}
	if cap.RateLimitPerMin != 60 {
		t.Errorf("RateLimitPerMin = %d; want 60", cap.RateLimitPerMin)
	}
	if cap.DefaultMaxResults != 25 {
		t.Errorf("DefaultMaxResults = %d; want 25", cap.DefaultMaxResults)
	}

	// Notes substring checks.
	notesSubstrings := []string{
		"Algolia HN Search",
		"public no-auth",
		"tags=story",
		"self-posts use news.ycombinator.com permalink",
	}
	for _, sub := range notesSubstrings {
		if !strings.Contains(cap.Notes, sub) {
			t.Errorf("Notes missing substring %q\nNotes: %s", sub, cap.Notes)
		}
	}
}

// TestHealthcheckSucceeds verifies Healthcheck dials a test server loopback address.
func TestHealthcheckSucceeds(t *testing.T) {
	t.Parallel()

	// Start a TCP listener that we can dial (httptest.Server binds to a real port).
	srv := httptest.NewServer(nil)
	defer srv.Close()

	// Extract host:port from the test server URL.
	addr := srv.Listener.Addr().String()

	a, err := New(Options{HealthcheckTarget: addr})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := a.Healthcheck(context.Background()); err != nil {
		t.Errorf("Healthcheck() returned error: %v", err)
	}
}

// TestHealthcheckFailsWhenTargetUnreachable verifies Healthcheck returns an error
// when the target host cannot be reached.
func TestHealthcheckFailsWhenTargetUnreachable(t *testing.T) {
	t.Parallel()

	// Use a port that nothing is listening on.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close() // Close immediately so the port is not reachable.

	a, err := New(Options{HealthcheckTarget: addr})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := a.Healthcheck(context.Background()); err == nil {
		t.Error("Healthcheck() succeeded; want error on unreachable target")
	}
}
