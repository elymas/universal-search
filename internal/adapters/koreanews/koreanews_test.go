// Package koreanews — Adapter construction, Name, Capabilities, Healthcheck tests.
// SPEC-ADP-009 REQ-ADP9-001.
package koreanews_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/adapters/koreanews"
	"github.com/elymas/universal-search/pkg/types"
)

// TestNew_defaults verifies that New with a zero Options creates a working adapter.
func TestNew_defaults(t *testing.T) {
	t.Parallel()
	a, err := koreanews.New(koreanews.Options{})
	if err != nil {
		t.Fatalf("New(zero) error: %v", err)
	}
	if a == nil {
		t.Fatal("New(zero) returned nil adapter")
	}
}

// TestName returns the stable "koreanews" identifier.
func TestName(t *testing.T) {
	t.Parallel()
	a, _ := koreanews.New(koreanews.Options{})
	if got := a.Name(); got != "koreanews" {
		t.Errorf("Name() = %q; want %q", got, "koreanews")
	}
}

// TestCapabilities verifies the static Capabilities descriptor.
func TestCapabilities(t *testing.T) {
	t.Parallel()
	a, _ := koreanews.New(koreanews.Options{})
	caps := a.Capabilities()

	if caps.SourceID != "koreanews" {
		t.Errorf("SourceID = %q; want %q", caps.SourceID, "koreanews")
	}
	if len(caps.DocTypes) == 0 {
		t.Error("DocTypes should not be empty")
	}
	if caps.DocTypes[0] != types.DocTypeArticle {
		t.Errorf("DocTypes[0] = %q; want DocTypeArticle", caps.DocTypes[0])
	}
	if len(caps.SupportedLangs) == 0 || caps.SupportedLangs[0] != "ko" {
		t.Errorf("SupportedLangs = %v; want [ko]", caps.SupportedLangs)
	}
}

// TestCapabilities_deterministic verifies that Capabilities is idempotent.
func TestCapabilities_deterministic(t *testing.T) {
	t.Parallel()
	a, _ := koreanews.New(koreanews.Options{})
	c1 := a.Capabilities()
	c2 := a.Capabilities()
	if c1.SourceID != c2.SourceID || c1.DisplayName != c2.DisplayName {
		t.Error("Capabilities() is not deterministic")
	}
}

// TestInterfaceAssertion verifies compile-time types.Adapter implementation.
func TestInterfaceAssertion(t *testing.T) {
	t.Parallel()
	var _ types.Adapter = (*koreanews.Adapter)(nil)
}

// TestHealthcheck_success verifies Healthcheck returns nil on HTTP 200.
func TestHealthcheck_success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a, err := koreanews.New(koreanews.Options{
		HTTPClient:        srv.Client(),
		HealthcheckTarget: srv.URL,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := a.Healthcheck(context.Background()); err != nil {
		t.Errorf("Healthcheck: unexpected error: %v", err)
	}
}

// TestHealthcheck_server503 verifies Healthcheck returns CategoryUnavailable on HTTP 503.
func TestHealthcheck_server503(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	a, err := koreanews.New(koreanews.Options{
		HTTPClient:        srv.Client(),
		HealthcheckTarget: srv.URL,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Healthcheck(context.Background())
	if err == nil {
		t.Fatal("expected error on 503, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) || se.Category != types.CategoryUnavailable {
		t.Errorf("want CategoryUnavailable SourceError; got %T %v", err, err)
	}
}

// TestHealthcheck_contextCancel verifies Healthcheck respects context cancellation.
func TestHealthcheck_contextCancel(t *testing.T) {
	t.Parallel()

	// Use a server that blocks indefinitely.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a, err := koreanews.New(koreanews.Options{
		HTTPClient:        srv.Client(),
		HealthcheckTarget: srv.URL,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = a.Healthcheck(ctx)
	if err == nil {
		t.Fatal("expected error on context timeout, got nil")
	}
}
