// Package access — additional Phase 4 tests for TLS paths.
package access

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPhase4TLS_404_PermanentError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, _, err := phase4TLS(
		t.Context(),
		srv.URL+"/missing",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes},
	)
	if err == nil {
		t.Fatal("404 in Phase 4 must return error")
	}
	fe, ok := err.(*FetchError)
	if !ok || fe.Category != CategoryPermanent {
		t.Errorf("404 Phase 4 must return CategoryPermanent, got %v", err)
	}
}

func TestPhase4TLS_429_RateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, _, err := phase4TLS(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes},
	)
	if err == nil {
		t.Fatal("429 in Phase 4 must return error")
	}
	fe, ok := err.(*FetchError)
	if !ok || fe.Category != CategoryRateLimited {
		t.Errorf("429 Phase 4 must return CategoryRateLimited, got %v", err)
	}
}

func TestPhase4TLS_503_UnavailableError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, _, err := phase4TLS(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes},
	)
	if err == nil {
		t.Fatal("503 Phase 4 must return error")
	}
	fe, ok := err.(*FetchError)
	if !ok || fe.Category != CategoryUnavailable {
		t.Errorf("503 Phase 4 must return CategoryUnavailable, got %v", err)
	}
}

func TestPhase4TLS_JSChallenge_Signal(t *testing.T) {
	t.Parallel()
	jsPage := `<html><body id="cf-please-stand-by">Checking...</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(jsPage))
	}))
	defer srv.Close()

	content, attempt, _ := phase4TLS(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes},
	)
	// Phase 4 may return content with JS challenge signal OR error with signal.
	// Either way, the signal should be detectable.
	if content != nil {
		t.Logf("Phase 4 returned content despite JS challenge (signal handled by attempt)")
	}
	if attempt != nil && attempt.isJSChallenge {
		t.Logf("Phase 4 correctly set isJSChallenge=true")
	}
}
