// Package access — additional Phase 3 tests for 5xx and WAF paths.
package access

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPhase3Get_503_UnavailableError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, _, err := phase3Get(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes, RedirectMaxHops: 5},
	)
	if err == nil {
		t.Fatal("503 must return error")
	}
	fe, ok := err.(*FetchError)
	if !ok || fe.Category != CategoryUnavailable {
		t.Errorf("503 must return CategoryUnavailable, got %v", err)
	}
}

func TestPhase3Get_403WAF_WAFSignal(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cf-Ray", "abc123-SFO")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, attempt, err := phase3Get(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{AllowPrivateNetworks: true},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes, RedirectMaxHops: 5},
	)
	if err == nil {
		t.Fatal("403 WAF must return error")
	}
	if attempt == nil || !attempt.hasWAFProfile() {
		t.Errorf("WAF response must set a confident profile hit, attempt=%+v", attempt)
	}
}

func TestPhase3Get_UserAgent_Default(t *testing.T) {
	t.Parallel()
	var receivedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	_, _, err := phase3Get(
		t.Context(),
		srv.URL+"/",
		FetchOptions{AllowPrivateNetworks: true}, // no UserAgent set
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes, RedirectMaxHops: 5},
	)
	if err != nil {
		t.Fatalf("phase3Get error: %v", err)
	}
	if receivedUA == "" {
		t.Error("User-Agent must be sent")
	}
}

func TestPhase3Get_CustomUserAgent(t *testing.T) {
	t.Parallel()
	var receivedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	_, _, err := phase3Get(
		t.Context(),
		srv.URL+"/",
		FetchOptions{AllowPrivateNetworks: true, UserAgent: "CustomBot/2.0"},
		Options{AllowPrivateNetworks: true, MaxBodyBytes: defaultMaxBodyBytes, RedirectMaxHops: 5},
	)
	if err != nil {
		t.Fatalf("phase3Get error: %v", err)
	}
	if receivedUA != "CustomBot/2.0" {
		t.Errorf("User-Agent = %q, want CustomBot/2.0", receivedUA)
	}
}
