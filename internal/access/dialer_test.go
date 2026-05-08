// Package access — unit tests for pinnedDialContext and dialContextWithPinnedIP.
//
// REQ-CACHE-013: DNS-rebind mitigation via IP pinning.
package access

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPinnedDialContext_AllowPrivate_Bypasses(t *testing.T) {
	t.Parallel()
	// With AllowPrivateNetworks, no DNS resolution occurs — plain dialer returned.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	opts := Options{AllowPrivateNetworks: true}
	fopts := FetchOptions{AllowPrivateNetworks: true}

	dialFn, err := pinnedDialContext(t.Context(), "127.0.0.1", opts, fopts)
	if err != nil {
		t.Fatalf("pinnedDialContext error: %v", err)
	}
	if dialFn == nil {
		t.Fatal("dialFn must not be nil")
	}
}

func TestDialContextWithPinnedIP_ConnectsToRealServer(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pinned ok"))
	}))
	defer srv.Close()

	// Extract host IP and port from the test server.
	// srv.URL = "http://127.0.0.1:PORT"
	host := srv.URL[len("http://"):]
	dialFn := dialContextWithPinnedIP("127.0.0.1")

	conn, err := dialFn(context.Background(), "tcp", host)
	if err != nil {
		t.Fatalf("dialContextWithPinnedIP error: %v", err)
	}
	_ = conn.Close()
}

func TestPinnedDialContext_FetchOptions_AllowPrivate(t *testing.T) {
	t.Parallel()
	opts := Options{}
	fopts := FetchOptions{AllowPrivateNetworks: true}

	dialFn, err := pinnedDialContext(t.Context(), "localhost", opts, fopts)
	if err != nil {
		t.Fatalf("pinnedDialContext with FetchOptions.AllowPrivate error: %v", err)
	}
	if dialFn == nil {
		t.Fatal("dialFn must not be nil")
	}
}
