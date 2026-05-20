package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// assertInterface is a compile-time interface assertion.
// If Adapter drifts from types.Adapter, this file will not compile.
var _ types.Adapter = (*Adapter)(nil)

func TestAdapterImplementsInterface(t *testing.T) {
	t.Parallel()
	// The compile-time assertion (var _ types.Adapter = (*Adapter)(nil)) is the real test.
	// This test documents the requirement and ensures it shows up in coverage reports.
	a, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Use the value through the interface to confirm the method set is satisfied.
	// A non-nil concrete value assigned to an interface is always non-nil; staticcheck
	// knows this. We call Name() to exercise the interface without a useless nil check.
	var iface types.Adapter = a
	if iface.Name() == "" {
		t.Error("Adapter.Name() via types.Adapter interface returned empty string")
	}
}

func TestAdapterName(t *testing.T) {
	t.Parallel()
	a, _ := New(Options{})
	if got := a.Name(); got != "youtube" {
		t.Errorf("Name() = %q, want %q", got, "youtube")
	}
}

func TestCapabilitiesDeterministic(t *testing.T) {
	t.Parallel()
	a, _ := New(Options{})
	c1 := a.Capabilities()
	c2 := a.Capabilities()
	if !reflect.DeepEqual(c1, c2) {
		t.Error("Capabilities() not deterministic: two calls differ")
	}
}

func TestCapabilitiesShape(t *testing.T) {
	t.Parallel()
	a, _ := New(Options{})
	caps := a.Capabilities()

	if caps.SourceID != "youtube" {
		t.Errorf("SourceID = %q, want %q", caps.SourceID, "youtube")
	}
	if caps.DisplayName != "YouTube" {
		t.Errorf("DisplayName = %q, want %q", caps.DisplayName, "YouTube")
	}
	if len(caps.DocTypes) != 1 || caps.DocTypes[0] != types.DocTypeVideo {
		t.Errorf("DocTypes = %v, want [%q]", caps.DocTypes, types.DocTypeVideo)
	}
	if caps.SupportedLangs != nil {
		t.Errorf("SupportedLangs = %v, want nil", caps.SupportedLangs)
	}
	if !caps.SupportsSince {
		t.Error("SupportsSince = false, want true")
	}
	if caps.RequiresAuth {
		t.Error("RequiresAuth = true, want false")
	}
	if caps.AuthEnvVars != nil {
		t.Errorf("AuthEnvVars = %v, want nil", caps.AuthEnvVars)
	}
	if caps.RateLimitPerMin != 30 {
		t.Errorf("RateLimitPerMin = %d, want 30", caps.RateLimitPerMin)
	}
	if caps.DefaultMaxResults != 25 {
		t.Errorf("DefaultMaxResults = %d, want 25", caps.DefaultMaxResults)
	}

	requiredSubstrings := []string{
		"yt-dlp Python sidecar",
		"public no-auth",
		"transcript snippet truncated",
		"Korean-locale auto-detection",
		"max_results + cursor offset cap 100",
	}
	for _, sub := range requiredSubstrings {
		if !strings.Contains(caps.Notes, sub) {
			t.Errorf("Notes missing substring %q", sub)
		}
	}
}

func TestHealthcheckSucceeds(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "ytdlp_version": "2026.04.01"})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	if err := a.Healthcheck(context.Background()); err != nil {
		t.Errorf("Healthcheck returned error: %v", err)
	}
}

func TestHealthcheckFailsOn503(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	if err := a.Healthcheck(context.Background()); err == nil {
		t.Error("expected error for 503, got nil")
	}
}

func TestHealthcheckFailsOnMalformedJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json {{{"))
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	if err := a.Healthcheck(context.Background()); err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestHealthcheckFailsOnStatusNotOk(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "degraded"})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	if err := a.Healthcheck(context.Background()); err == nil {
		t.Error("expected error for status=degraded, got nil")
	}
}

func TestHealthcheckFailsOnUnreachable(t *testing.T) {
	t.Parallel()
	// Use a port that's not listening.
	a, _ := New(Options{BaseURL: "http://127.0.0.1:19999"})
	if err := a.Healthcheck(context.Background()); err == nil {
		t.Error("expected error for unreachable sidecar, got nil")
	}
}

func TestNewDefaultsApplied(t *testing.T) {
	t.Parallel()
	a, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if a.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", a.baseURL, defaultBaseURL)
	}
	if !strings.Contains(a.userAgent, "usearch/") {
		t.Errorf("userAgent = %q, want prefix usearch/", a.userAgent)
	}
}
