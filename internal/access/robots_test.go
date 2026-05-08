// Package access — unit tests for robots.txt cache and RFC 9309 semantics.
//
// REQ-CACHE-003: robots.txt check with RFC 9309 semantics.
// RFC 9309: 4xx → allow all; 5xx/network → disallow all; 2xx → parse.
package access

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// robotsHost extracts scheme and host from a test server URL.
func robotsHost(srvURL string) (scheme, host string) {
	// srvURL is http://127.0.0.1:PORT
	scheme = "http"
	host = srvURL[len("http://"):]
	return
}

func TestRobotsCache_AllowAll_On4xx(t *testing.T) {
	t.Parallel()
	// RFC 9309 §2.3.1: 4xx → treat as allow-all.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound) // 404
	}))
	defer srv.Close()

	cache := newRobotsCache(1 * time.Minute)
	scheme, host := robotsHost(srv.URL)
	allowed, err := cache.isAllowed(t.Context(), scheme, host, "/page", "MoAI-Bot/1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("4xx robots.txt must allow all")
	}
}

func TestRobotsCache_DisallowAll_On5xx(t *testing.T) {
	t.Parallel()
	// RFC 9309 §2.3.1: 5xx → treat as disallow-all (transient).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cache := newRobotsCache(1 * time.Minute)
	scheme, host := robotsHost(srv.URL)
	allowed, _ := cache.isAllowed(t.Context(), scheme, host, "/page", "MoAI-Bot/1.0")
	// 5xx → disallow all; allowed must be false (err may be non-nil with CategoryBlocked).
	if allowed {
		t.Error("5xx robots.txt must disallow all")
	}
}

func TestRobotsCache_ParseRules_DisallowPath(t *testing.T) {
	t.Parallel()
	robots := "User-agent: *\nDisallow: /private/\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(robots))
	}))
	defer srv.Close()

	cache := newRobotsCache(1 * time.Minute)
	scheme, host := robotsHost(srv.URL)

	// Disallowed path: returns (false, FetchError{CategoryBlocked}).
	allowed, disallowErr := cache.isAllowed(t.Context(), scheme, host, "/private/secret", "MoAI-Bot/1.0")
	if allowed {
		t.Error("/private/ must be disallowed")
	}
	if disallowErr == nil {
		t.Error("disallowed path must return non-nil error")
	}
	fe, ok := disallowErr.(*FetchError)
	if !ok || fe.Category != CategoryBlocked {
		t.Errorf("disallowed must return CategoryBlocked, got %v", disallowErr)
	}

	// Allowed path: returns (true, nil).
	allowed, allowErr := cache.isAllowed(t.Context(), scheme, host, "/public/page", "MoAI-Bot/1.0")
	if allowErr != nil {
		t.Fatalf("allowed path must return nil error: %v", allowErr)
	}
	if !allowed {
		t.Error("/public/ must be allowed")
	}
}

func TestRobotsCache_CacheHit(t *testing.T) {
	t.Parallel()
	// Verify the cache serves subsequent requests without re-fetching.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			callCount++
		}
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
	}))
	defer srv.Close()

	cache := newRobotsCache(1 * time.Minute)
	scheme, host := robotsHost(srv.URL)
	for range 3 {
		_, _ = cache.isAllowed(t.Context(), scheme, host, "/page", "MoAI-Bot/1.0")
	}
	if callCount > 1 {
		t.Errorf("robots.txt fetched %d times, want at most 1 (cache hit)", callCount)
	}
}

func TestRobotsCache_TTL_Expiry(t *testing.T) {
	t.Parallel()
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			callCount++
		}
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
	}))
	defer srv.Close()

	// TTL of 1 nanosecond — immediately expired.
	cache := newRobotsCache(1 * time.Nanosecond)
	scheme, host := robotsHost(srv.URL)

	_, _ = cache.isAllowed(t.Context(), scheme, host, "/page", "MoAI-Bot/1.0")
	time.Sleep(5 * time.Millisecond) // ensure TTL expired
	_, _ = cache.isAllowed(t.Context(), scheme, host, "/page", "MoAI-Bot/1.0")

	if callCount < 2 {
		t.Errorf("robots.txt should be re-fetched after TTL expiry, got %d fetches", callCount)
	}
}

func TestRobotsCache_AllowAll_WhenNoRobotsFile(t *testing.T) {
	t.Parallel()
	// 404 means allow all per RFC 9309.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	cache := newRobotsCache(1 * time.Minute)
	scheme, host := robotsHost(srv.URL)
	allowed, err := cache.isAllowed(t.Context(), scheme, host, "/anything", "bot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("missing robots.txt (404) must allow all per RFC 9309")
	}
}
