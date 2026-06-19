//go:build integration

// Package main_test — end-to-end integration test per SPEC-CLI-001 NFR-CLI-001.
//
// Spins up httptest stubs for the Reddit, Hacker News, and researcher sidecar
// HTTP surfaces, builds the usearch binary fresh, runs `usearch query` against
// the stubs, and asserts the binary exits with code 0 or 3 and emits a single
// valid JSON object on stdout.
//
// Run via: go test -tags=integration ./cmd/usearch/...
package main_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// redditStubResponse is a minimal Reddit search.json response body shaped to
// pass the Reddit adapter's parser (1 child, NSFW=false, non-empty fields).
const redditStubResponse = `{
  "kind": "Listing",
  "data": {
    "after": null,
    "dist": 1,
    "children": [
      {
        "kind": "t3",
        "data": {
          "name": "t3_int_001",
          "permalink": "/r/test/comments/int_001/integration_test_post/",
          "title": "Integration Test Post on Hello World",
          "selftext": "This stubbed Reddit post mentions hello world for the integration test.",
          "created_utc": 1700000000,
          "author": "integration_tester",
          "score": 100,
          "subreddit": "test",
          "over_18": false,
          "num_comments": 5,
          "upvote_ratio": 0.95,
          "url": "https://example.com/integration/hello-world",
          "subreddit_name_prefixed": "r/test",
          "ups": 100
        }
      }
    ]
  }
}`

// hnStubResponse is a minimal Algolia HN response shaped to pass the HN
// adapter's parser (1 hit, story type, non-empty fields).
const hnStubResponse = `{
  "hits": [
    {
      "objectID": "int_hn_001",
      "title": "Integration Test HN Story on Hello World",
      "url": "https://example.com/hn/integration/hello-world",
      "author": "hn_integration_tester",
      "points": 250,
      "story_text": null,
      "num_comments": 12,
      "created_at_i": 1700000100,
      "_tags": ["story", "author_hn_integration_tester", "story_int_hn_001"]
    }
  ],
  "hitsPerPage": 25,
  "nbHits": 1
}`

// researcherSynthesizeResponse is a minimal SynthesizeResponse shaped to pass
// the synthesis client's parser. Two citations covering both stub adapters.
const researcherSynthesizeResponse = `{
  "request_id": "integration-test",
  "text": "Hello world is a classic programming greeting [1] [2].",
  "citations": [
    {"marker": 1, "doc_id": "reddit:t3_int_001", "url": "https://example.com/integration/hello-world", "title": "Integration Test Post on Hello World"},
    {"marker": 2, "doc_id": "hackernews:int_hn_001", "url": "https://example.com/hn/integration/hello-world", "title": "Integration Test HN Story on Hello World"}
  ],
  "model": "stub-haiku",
  "provider": "stub",
  "cost_usd": 0.0001,
  "prompt_tokens": 50,
  "completion_tokens": 20,
  "latency_ms": 12.5,
  "degraded": false,
  "notice": ""
}`

// TestQueryE2EWithStubs is the NFR-CLI-001 end-to-end integration test.
// It exercises the full query pipeline: subcommand dispatch -> flag parsing
// -> adapter registry -> rule-based router (because --no-llm is passed)
// -> CLI-internal fanout -> synthesis sidecar -> JSON output.
func TestQueryE2EWithStubs(t *testing.T) {
	// --- Reddit stub ---
	reddit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(redditStubResponse))
	}))
	defer reddit.Close()

	// --- HN stub ---
	hn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(hnStubResponse))
	}))
	defer hn.Close()

	// --- Researcher sidecar stub ---
	researcher := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/health"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","version":"0.1.0"}`))
		case r.URL.Path == "/synthesize":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(researcherSynthesizeResponse))
		default:
			http.NotFound(w, r)
		}
	}))
	defer researcher.Close()

	// --- Build binary fresh ---
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "usearch")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("go build failed: %v", err)
	}

	// --- Run binary against stubs ---
	cmd := exec.Command(binPath, "query", "--no-llm", "--timeout", "10s", "--format", "json", "hello world")
	cmd.Env = append(os.Environ(),
		// Adapter base URLs: include the path the adapter expects.
		"REDDIT_BASE_URL="+reddit.URL+"/search.json",
		"REDDIT_CLIENT_ID=test_integration_id",
		"REDDIT_CLIENT_SECRET=test_integration_secret",
		"HN_BASE_URL="+hn.URL+"/api/v1/search",
		"RESEARCHER_BASE_URL="+researcher.URL,
		"RESEARCHER_REQUEST_TIMEOUT_SECONDS=5",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)

	exitCode := 0
	if exitErr, ok := runErr.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if runErr != nil {
		t.Fatalf("binary execution failed: %v\nstderr:\n%s", runErr, stderr.String())
	}

	t.Logf("binary exit=%d elapsed=%s", exitCode, elapsed)
	t.Logf("stdout (%d bytes): %s", stdout.Len(), stdout.String())
	t.Logf("stderr (%d bytes): %s", stderr.Len(), stderr.String())

	// --- Assertions per NFR-CLI-001 ---

	// Exit code must be 0 (full success) or 3 (partial — e.g. router failed
	// to classify a stubbed query and selected no adapters; still counts).
	if exitCode != 0 && exitCode != 3 {
		t.Errorf("exit %d, want 0 or 3", exitCode)
	}

	// stdout MUST be non-empty and parseable as a single JSON object.
	if stdout.Len() == 0 {
		t.Fatal("stdout is empty; expected JSON payload")
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout not valid JSON: %v\nstdout:\n%s", err, stdout.String())
	}

	// Schema sanity: the documented top-level keys MUST be present.
	for _, key := range []string{"schema_version", "query", "adapters", "summary", "citations", "stats"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("stdout JSON missing required key %q", key)
		}
	}

	// schema_version MUST be "1" (REQ-CLI-004).
	if got, _ := payload["schema_version"].(string); got != "1" {
		t.Errorf("schema_version = %q, want %q", got, "1")
	}

	// stats.request_id MUST be a non-empty string (REQ-CLI-011).
	stats, _ := payload["stats"].(map[string]any)
	if stats == nil {
		t.Fatal("stats key missing or not an object")
	}
	if rid, _ := stats["request_id"].(string); rid == "" {
		t.Error("stats.request_id is empty")
	}

	// adapters MUST list at least one of the registered adapters (reddit + HN).
	adaptersField, _ := payload["adapters"].([]any)
	if len(adaptersField) == 0 {
		t.Error("adapters list is empty; expected reddit and/or hackernews")
	}

	// On full-success exit (0), summary and citations MUST be non-empty
	// (the synthesis stub returned valid content). On exit 3, the synthesis
	// path failed so summary may be empty — still pass.
	if exitCode == 0 {
		if summary, _ := payload["summary"].(string); summary == "" {
			t.Error("exit 0: summary is empty (expected synthesized text)")
		}
		if cites, _ := payload["citations"].([]any); len(cites) == 0 {
			t.Error("exit 0: citations is empty (expected at least 1)")
		}
	}
}
