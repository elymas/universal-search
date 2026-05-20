// Package social — tests for adapter construction, Name, Capabilities, Healthcheck.
// REQ-ADP6-001/002/004/007: interface conformance, Name, Capabilities, Healthcheck.
package social

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// TestBlueskyInterfaceConformance verifies *Adapter implements types.Adapter at compile time.
// The compile-time assertion (var _ types.Adapter = ...) in social.go catches this too,
// but this test makes the failure message human-readable.
func TestBlueskyInterfaceConformance(t *testing.T) {
	t.Parallel()
	a, err := NewBluesky(BlueskyOptions{})
	if err != nil {
		t.Fatalf("NewBluesky: unexpected error: %v", err)
	}
	var _ types.Adapter = a
}

// TestXInterfaceConformance verifies *Adapter from NewX implements types.Adapter.
func TestXInterfaceConformance(t *testing.T) {
	t.Parallel()
	a, err := NewX(XOptions{})
	if err != nil {
		t.Fatalf("NewX: unexpected error: %v", err)
	}
	var _ types.Adapter = a
}

// TestBlueskyName verifies Name() returns "bluesky".
func TestBlueskyName(t *testing.T) {
	t.Parallel()
	a, _ := NewBluesky(BlueskyOptions{})
	if got := a.Name(); got != "bluesky" {
		t.Errorf("Name(): got %q, want %q", got, "bluesky")
	}
}

// TestXName verifies Name() returns "x" for X adapter.
func TestXName(t *testing.T) {
	t.Parallel()
	a, _ := NewX(XOptions{})
	if got := a.Name(); got != "x" {
		t.Errorf("Name(): got %q, want %q", got, "x")
	}
}

// TestBlueskyCapabilities verifies Bluesky adapter Capabilities fields.
func TestBlueskyCapabilities(t *testing.T) {
	t.Parallel()
	a, _ := NewBluesky(BlueskyOptions{})
	caps := a.Capabilities()

	if caps.SourceID != "bluesky" {
		t.Errorf("Capabilities().SourceID: got %q, want %q", caps.SourceID, "bluesky")
	}
	if caps.RequiresAuth {
		t.Error("Capabilities().RequiresAuth: got true, want false (Bluesky is anonymous)")
	}
	if len(caps.DocTypes) == 0 {
		t.Error("Capabilities().DocTypes: expected at least one DocType")
	}
	found := false
	for _, dt := range caps.DocTypes {
		if dt == types.DocTypePost {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Capabilities().DocTypes: expected %q in %v", types.DocTypePost, caps.DocTypes)
	}
	if caps.DefaultMaxResults <= 0 {
		t.Errorf("Capabilities().DefaultMaxResults: got %d, want > 0", caps.DefaultMaxResults)
	}
}

// TestXCapabilities verifies X adapter Capabilities fields.
func TestXCapabilities(t *testing.T) {
	t.Parallel()
	a, _ := NewX(XOptions{})
	caps := a.Capabilities()

	if caps.SourceID != "x" {
		t.Errorf("Capabilities().SourceID: got %q, want %q", caps.SourceID, "x")
	}
}

// TestBlueskyHealthcheckSuccess verifies Healthcheck returns nil when TCP succeeds.
func TestBlueskyHealthcheckSuccess(t *testing.T) {
	t.Parallel()
	// Start a TCP listener on a random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()

	a, _ := NewBluesky(BlueskyOptions{
		HealthcheckTarget: ln.Addr().String(),
	})

	ctx := context.Background()
	if err := a.Healthcheck(ctx); err != nil {
		t.Errorf("Healthcheck: unexpected error: %v", err)
	}
}

// TestBlueskyHealthcheckFailure verifies Healthcheck returns error when TCP fails.
func TestBlueskyHealthcheckFailure(t *testing.T) {
	t.Parallel()
	a, _ := NewBluesky(BlueskyOptions{
		// Port 1 is reserved; connection should be refused.
		HealthcheckTarget: "127.0.0.1:1",
	})

	ctx := context.Background()
	if err := a.Healthcheck(ctx); err == nil {
		t.Error("Healthcheck: expected error on refused port, got nil")
	}
}

// TestBlueskyNewDefaultClient verifies NewBluesky with no options uses the default client.
func TestBlueskyNewDefaultClient(t *testing.T) {
	t.Parallel()
	a, err := NewBluesky(BlueskyOptions{})
	if err != nil {
		t.Fatalf("NewBluesky(): unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("NewBluesky(): returned nil adapter")
	}
}

// TestBlueskyNewCustomHTTPClient verifies NewBluesky accepts an injected http.Client.
func TestBlueskyNewCustomHTTPClient(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	a, err := NewBluesky(BlueskyOptions{
		BaseURL:    ts.URL,
		HTTPClient: ts.Client(),
	})
	if err != nil {
		t.Fatalf("NewBluesky(): unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("NewBluesky(): returned nil adapter")
	}
}
