// Package access — unit tests for Phase 2 HEAD probe + robots.txt.
//
// REQ-CACHE-003: Phase 2 executes robots.txt check then HEAD probe.
package access

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPhase2Probe_BothSkipped_ReturnsNotApplicable(t *testing.T) {
	t.Parallel()
	// When both SkipHEADProbe and SkipRobotsTxt are set → ErrPhaseNotApplicable.
	cache := newRobotsCache(0)
	_, err := phase2Probe(
		t.Context(),
		"http://example.com/page",
		FetchOptions{SkipHEADProbe: true, SkipRobotsTxt: true},
		Options{AllowPrivateNetworks: true},
		cache,
	)
	if err != ErrPhaseNotApplicable {
		t.Errorf("both skip must return ErrPhaseNotApplicable, got %v", err)
	}
}

func TestPhase2Probe_RobotsBlocked_ReturnsBlocked(t *testing.T) {
	t.Parallel()
	// Serve robots.txt that disallows everything.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
			return
		}
		_, _ = w.Write([]byte("secret"))
	}))
	defer srv.Close()

	cache := newRobotsCache(0)
	_, err := phase2Probe(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{},
		Options{AllowPrivateNetworks: true},
		cache,
	)
	if err == nil {
		t.Fatal("robots.txt disallow must return error")
	}
	fe, ok := err.(*FetchError)
	if !ok || fe.Category != CategoryBlocked {
		t.Errorf("robots disallow must return CategoryBlocked, got %v", err)
	}
}

func TestPhase2Probe_HeadProbe_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r) // 404 → allow all
			return
		}
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write([]byte("body"))
	}))
	defer srv.Close()

	cache := newRobotsCache(0)
	content, err := phase2Probe(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{},
		Options{AllowPrivateNetworks: true},
		cache,
	)
	if err != nil {
		t.Fatalf("phase2Probe error: %v", err)
	}
	// Phase 2 doesn't return body content (HEAD probe only).
	_ = content
}

func TestPhase2Probe_SkipRobots_SkipProbe_HeadOnly(t *testing.T) {
	t.Parallel()
	// SkipRobotsTxt=true but SkipHEADProbe=false → HEAD probe runs.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "text/html")
			return
		}
	}))
	defer srv.Close()

	cache := newRobotsCache(0)
	_, err := phase2Probe(
		t.Context(),
		srv.URL+"/page",
		FetchOptions{SkipRobotsTxt: true},
		Options{AllowPrivateNetworks: true},
		cache,
	)
	if err != nil {
		t.Errorf("SkipRobots + HEAD probe must succeed: %v", err)
	}
}

func TestDoHEADProbe_404_ReturnsStatusCode(t *testing.T) {
	t.Parallel()
	// doHEADProbe returns the status code regardless — phase2Probe interprets it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	content, err := doHEADProbe(t.Context(), srv.URL+"/missing", "MoAI-Bot/1.0")
	if err != nil {
		t.Fatalf("doHEADProbe network error: %v", err)
	}
	if content == nil {
		t.Fatal("doHEADProbe must return content even for 404")
	}
	if content.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", content.StatusCode)
	}
}

func TestDoHEADProbe_200_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	content, err := doHEADProbe(t.Context(), srv.URL+"/page", "MoAI-Bot/1.0")
	if err != nil {
		t.Fatalf("doHEADProbe 200 error: %v", err)
	}
	// Phase 2 may return nil content (HEAD probe only).
	_ = content
}
