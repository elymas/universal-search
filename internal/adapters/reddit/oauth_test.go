package reddit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// tokenEndpointStub creates an httptest.Server that simulates the Reddit OAuth
// token endpoint. It returns the given status code and, on 200, a valid token
// response. It also tracks the number of requests received.
func tokenEndpointStub(t *testing.T, statusCode int, accessToken string, expiresIn int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if statusCode == http.StatusOK {
			resp := tokenResponse{
				AccessToken: accessToken,
				TokenType:   "bearer",
				ExpiresIn:   expiresIn,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(statusCode)
	}))
	t.Cleanup(ts.Close)
	return ts, &count
}

// newAdapterWithStubs creates an adapter with both token and search endpoint stubs.
func newAdapterWithStubs(t *testing.T, tokenTS, searchTS *httptest.Server) *Adapter {
	t.Helper()
	a, err := New(Options{
		BaseURL:      searchTS.URL,
		OAuthURL:     tokenTS.URL,
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return a
}

// --- REQ-ADP-001a-001: Token Acquisition ---

func TestAcquireTokenHappyPath(t *testing.T) {
	t.Parallel()

	ts, _ := tokenEndpointStub(t, http.StatusOK, "tok123", 3600)

	a, err := New(Options{
		SkipAuthCheck: true,
		OAuthURL:      ts.URL,
		ClientID:      "test_client_id",
		ClientSecret:  "test_client_secret",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	token, _, err := a.acquireToken(context.Background())
	if err != nil {
		t.Fatalf("acquireToken() error = %v", err)
	}
	if token != "tok123" {
		t.Errorf("token = %q, want %q", token, "tok123")
	}
}

func TestAcquireTokenBasicAuthHeader(t *testing.T) {
	t.Parallel()

	var capturedAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		resp := tokenResponse{AccessToken: "tok", ExpiresIn: 3600}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	a, err := New(Options{
		SkipAuthCheck: true,
		OAuthURL:      ts.URL,
		ClientID:      "my_id",
		ClientSecret:  "my_secret",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, _, err = a.acquireToken(context.Background())
	if err != nil {
		t.Fatalf("acquireToken() error = %v", err)
	}

	// Decode the Basic auth header: "Basic <base64(client_id:client_secret)>"
	if !strings.HasPrefix(capturedAuth, "Basic ") {
		t.Fatalf("Authorization header = %q, want prefix 'Basic '", capturedAuth)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(capturedAuth, "Basic "))
	if err != nil {
		t.Fatalf("base64 decode error = %v", err)
	}
	if string(decoded) != "my_id:my_secret" {
		t.Errorf("decoded auth = %q, want %q", string(decoded), "my_id:my_secret")
	}
}

func TestAcquireTokenFormBody(t *testing.T) {
	t.Parallel()

	var capturedBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capturedBody = string(body)
		resp := tokenResponse{AccessToken: "tok", ExpiresIn: 3600}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	a, err := New(Options{
		SkipAuthCheck: true,
		OAuthURL:      ts.URL,
		ClientID:      "id",
		ClientSecret:  "secret",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, _, err = a.acquireToken(context.Background())
	if err != nil {
		t.Fatalf("acquireToken() error = %v", err)
	}

	if !strings.Contains(capturedBody, "grant_type=client_credentials") {
		t.Errorf("body = %q, want to contain %q", capturedBody, "grant_type=client_credentials")
	}
}

func TestAcquireTokenSetsUserAgent(t *testing.T) {
	t.Parallel()

	var capturedUA, capturedAccept string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		capturedAccept = r.Header.Get("Accept")
		resp := tokenResponse{AccessToken: "tok", ExpiresIn: 3600}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	a, err := New(Options{
		SkipAuthCheck: true,
		OAuthURL:      ts.URL,
		ClientID:      "id",
		ClientSecret:  "secret",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, _, err = a.acquireToken(context.Background())
	if err != nil {
		t.Fatalf("acquireToken() error = %v", err)
	}

	if !strings.HasPrefix(capturedUA, "usearch/") {
		t.Errorf("User-Agent = %q, want prefix 'usearch/'", capturedUA)
	}
	if capturedAccept != "application/json" {
		t.Errorf("Accept = %q, want %q", capturedAccept, "application/json")
	}
}

// --- REQ-ADP-001a-005: Bad credential token failures ---

func TestAcquireToken401BadCreds(t *testing.T) {
	t.Parallel()

	ts, _ := tokenEndpointStub(t, http.StatusUnauthorized, "", 0)

	a, err := New(Options{
		SkipAuthCheck: true,
		OAuthURL:      ts.URL,
		ClientID:      "bad_id",
		ClientSecret:  "bad_secret",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, _, err = a.acquireToken(context.Background())
	if err == nil {
		t.Fatal("acquireToken() expected error for 401, got nil")
	}
	if !errors.Is(err, types.ErrPermanent) {
		t.Errorf("errors.Is(err, ErrPermanent) = false, want true; err = %v", err)
	}
	if !errors.Is(err, ErrTokenAcquisitionFailed) {
		t.Errorf("errors.Is(err, ErrTokenAcquisitionFailed) = false, want true; err = %v", err)
	}
}

func TestAcquireToken403BadCreds(t *testing.T) {
	t.Parallel()

	ts, _ := tokenEndpointStub(t, http.StatusForbidden, "", 0)

	a, err := New(Options{
		SkipAuthCheck: true,
		OAuthURL:      ts.URL,
		ClientID:      "bad_id",
		ClientSecret:  "bad_secret",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, _, err = a.acquireToken(context.Background())
	if err == nil {
		t.Fatal("acquireToken() expected error for 403, got nil")
	}
	if !errors.Is(err, types.ErrPermanent) {
		t.Errorf("errors.Is(err, ErrPermanent) = false, want true; err = %v", err)
	}
	if !errors.Is(err, ErrTokenAcquisitionFailed) {
		t.Errorf("errors.Is(err, ErrTokenAcquisitionFailed) = false, want true; err = %v", err)
	}
}

func TestAcquireToken5xxUnavailable(t *testing.T) {
	t.Parallel()

	ts, _ := tokenEndpointStub(t, http.StatusInternalServerError, "", 0)

	a, err := New(Options{
		SkipAuthCheck: true,
		OAuthURL:      ts.URL,
		ClientID:      "id",
		ClientSecret:  "secret",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, _, err = a.acquireToken(context.Background())
	if err == nil {
		t.Fatal("acquireToken() expected error for 500, got nil")
	}
	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false, want true; err = %v", err)
	}
}

// --- REQ-ADP-001a-007: OAuth URL Override ---

func TestOAuthURLOverrideUsesEnvValue(t *testing.T) {
	t.Parallel()

	var requestReceived atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived.Add(1)
		resp := tokenResponse{AccessToken: "override_tok", ExpiresIn: 3600}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	a, err := New(Options{
		OAuthURL:     ts.URL,
		ClientID:     "id",
		ClientSecret: "secret",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, _, err = a.acquireToken(context.Background())
	if err != nil {
		t.Fatalf("acquireToken() error = %v", err)
	}
	if requestReceived.Load() != 1 {
		t.Errorf("token endpoint requests = %d, want 1", requestReceived.Load())
	}
}

func TestOAuthURLDefaultWhenUnset(t *testing.T) {
	t.Parallel()

	a, err := New(Options{
		SkipAuthCheck: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if a.oauthURL != "https://www.reddit.com/api/v1/access_token" {
		t.Errorf("oauthURL = %q, want default", a.oauthURL)
	}
}

// --- NFR-ADP-001a-002: No Secret Leakage ---

func TestErrorsDoNotLeakSecrets(t *testing.T) {
	t.Parallel()

	secretID := "SECRET_ID_VALUE"
	secretKey := "SECRET_KEY_VALUE"

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "ErrMissingCredentials",
			err: func() error {
				_, err := New(Options{
					ClientID:     secretID,
					ClientSecret: "", // Empty triggers ErrMissingCredentials
				})
				return err
			}(),
		},
		{
			name: "ErrTokenAcquisitionFailed via 401 token stub",
			err: func() error {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
				}))
				defer ts.Close()
				a, _ := New(Options{
					SkipAuthCheck: true,
					OAuthURL:      ts.URL,
					ClientID:      secretID,
					ClientSecret:  secretKey,
				})
				_, _, err := a.acquireToken(context.Background())
				return err
			}(),
		},
		{
			name: "ErrTokenRefreshExhausted via double-401 search stub",
			err: func() error {
				tokenTS, _ := tokenEndpointStub(t, http.StatusOK, "SECRET_TOKEN", 3600)
				searchTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
				}))
				defer searchTS.Close()
				a, _ := New(Options{
					BaseURL:      searchTS.URL,
					OAuthURL:     tokenTS.URL,
					ClientID:     secretID,
					ClientSecret: secretKey,
				})
				_, err := a.Search(context.Background(), types.Query{Text: "test"})
				return err
			}(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Fatalf("expected error, got nil")
			}
			errStr := tc.err.Error()
			if strings.Contains(errStr, secretID) {
				t.Errorf("error string leaks secret ID: %q", errStr)
			}
			if strings.Contains(errStr, secretKey) {
				t.Errorf("error string leaks secret key: %q", errStr)
			}
			if strings.Contains(errStr, "SECRET_TOKEN") {
				t.Errorf("error string leaks bearer token: %q", errStr)
			}
		})
	}
}

// --- REQ-ADP-001a-002/003: Authenticated Search ---

func TestSearchUsesBearerToken(t *testing.T) {
	t.Parallel()

	tokenTS, _ := tokenEndpointStub(t, http.StatusOK, "test_bearer_tok", 3600)

	var capturedAuth string
	searchTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"after":null,"children":[]}}`))
	}))
	defer searchTS.Close()

	a := newAdapterWithStubs(t, tokenTS, searchTS)
	ctx := testContext(t)
	_, err := a.Search(ctx, types.Query{Text: "golang"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if capturedAuth != "bearer test_bearer_tok" {
		t.Errorf("Authorization = %q, want %q", capturedAuth, "bearer test_bearer_tok")
	}
}

func TestSearchReusesCachedToken(t *testing.T) {
	t.Parallel()

	tokenTS, tokenHits := tokenEndpointStub(t, http.StatusOK, "cached_tok", 3600)

	var searchHits atomic.Int32
	searchTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"after":null,"children":[]}}`))
	}))
	defer searchTS.Close()

	a := newAdapterWithStubs(t, tokenTS, searchTS)
	ctx := testContext(t)

	// First search: acquires token.
	_, err := a.Search(ctx, types.Query{Text: "golang"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	// Second search: should reuse cached token.
	_, err = a.Search(ctx, types.Query{Text: "rust"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if tokenHits.Load() != 1 {
		t.Errorf("token endpoint hits = %d, want 1 (token reused)", tokenHits.Load())
	}
	if searchHits.Load() != 2 {
		t.Errorf("search endpoint hits = %d, want 2", searchHits.Load())
	}
}

func TestSearchHostIsOAuthReddit(t *testing.T) {
	t.Parallel()

	// Verify default base URL is oauth.reddit.com/search
	a, err := New(Options{
		SkipAuthCheck: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if !strings.HasPrefix(a.baseURL, "https://oauth.reddit.com/search") {
		t.Errorf("baseURL = %q, want prefix 'https://oauth.reddit.com/search'", a.baseURL)
	}
}

// --- REQ-ADP-001a-003: Expired Token Refresh ---

func TestSearchRefreshesExpiredToken(t *testing.T) {
	t.Parallel()

	tokenTS, tokenHits := tokenEndpointStub(t, http.StatusOK, "refreshed_tok", 3600)

	searchTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"after":null,"children":[]}}`))
	}))
	defer searchTS.Close()

	a := newAdapterWithStubs(t, tokenTS, searchTS)

	// Manually expire the token.
	a.tokens.mu.Lock()
	a.tokens.token = "expired_tok"
	a.tokens.expiry = time.Now().Add(-1 * time.Hour)
	a.tokens.mu.Unlock()

	ctx := testContext(t)
	_, err := a.Search(ctx, types.Query{Text: "golang"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if tokenHits.Load() != 1 {
		t.Errorf("token endpoint hits = %d, want 1 (refreshed expired token)", tokenHits.Load())
	}
}

// --- REQ-ADP-001a-003 / NFR-ADP-001a-001: Concurrency EXACTLY ONCE ---

func TestSearchConcurrentTokenExactlyOnce(t *testing.T) {
	// Verify token endpoint is hit EXACTLY ONCE under 50 concurrent first-time callers.

	tokenTS, tokenHits := tokenEndpointStub(t, http.StatusOK, "concurrent_tok", 3600)

	body, err := loadFixture("testdata/search_response.json")
	if err != nil {
		t.Fatalf("loadFixture() error = %v", err)
	}

	var searchHits atomic.Int32
	searchTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer searchTS.Close()

	a := newAdapterWithStubs(t, tokenTS, searchTS)

	const numGoroutines = 50
	var wg sync.WaitGroup
	var barrier sync.WaitGroup
	barrier.Add(1)

	type result struct {
		docs []types.NormalizedDoc
		err  error
	}
	results := make([]result, numGoroutines)

	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			barrier.Wait()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			docs, err := a.Search(ctx, types.Query{Text: "golang"})
			results[idx] = result{docs: docs, err: err}
		}(i)
	}

	barrier.Done()
	wg.Wait()

	// EXACTLY ONCE assertion (NFR-ADP-001a-001).
	if tokenHits.Load() != 1 {
		t.Errorf("token endpoint hits = %d, want EXACTLY 1 (no stampede)", tokenHits.Load())
	}
	if searchHits.Load() != numGoroutines {
		t.Errorf("search endpoint hits = %d, want %d", searchHits.Load(), numGoroutines)
	}
	for i, r := range results {
		if r.err != nil {
			t.Errorf("goroutine %d: Search() error = %v", i, r.err)
			continue
		}
		if len(r.docs) != 25 {
			t.Errorf("goroutine %d: got %d docs, want 25", i, len(r.docs))
		}
	}
}

// --- REQ-ADP-001a-004: 401 Refresh + Single Retry ---

func TestSearch401TriggersSingleRefreshRetry(t *testing.T) {
	t.Parallel()

	tokenTS, tokenHits := tokenEndpointStub(t, http.StatusOK, "refresh_tok", 3600)

	var searchHits atomic.Int32
	searchTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit := searchHits.Add(1)
		if hit == 1 {
			// First search: return 401 to trigger retry.
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Second search: return success.
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"after":null,"children":[{"kind":"t3","data":{"name":"t3_1","permalink":"/r/test/comments/1","title":"Test","selftext":"body","created_utc":1700000000.0,"author":"user","score":10,"subreddit":"test","over_18":false,"num_comments":5,"upvote_ratio":0.9,"url":"https://example.com","subreddit_name_prefixed":"r/test","ups":10}}]}}`))
	}))
	defer searchTS.Close()

	a := newAdapterWithStubs(t, tokenTS, searchTS)
	ctx := testContext(t)

	docs, err := a.Search(ctx, types.Query{Text: "golang"})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("docs = %d, want 1", len(docs))
	}

	// Token endpoint should be hit twice: initial + refresh after 401.
	if tokenHits.Load() != 2 {
		t.Errorf("token endpoint hits = %d, want 2 (initial + refresh)", tokenHits.Load())
	}
	// Search endpoint should be hit twice: first (401) + retry (200).
	if searchHits.Load() != 2 {
		t.Errorf("search endpoint hits = %d, want 2", searchHits.Load())
	}
}

func TestSearch401TwiceExhausts(t *testing.T) {
	t.Parallel()

	tokenTS, _ := tokenEndpointStub(t, http.StatusOK, "refresh_tok", 3600)

	var searchHits atomic.Int32
	searchTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchHits.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer searchTS.Close()

	a := newAdapterWithStubs(t, tokenTS, searchTS)
	ctx := testContext(t)

	_, err := a.Search(ctx, types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("Search() expected error for 401 exhaustion, got nil")
	}
	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false, want true; err = %v", err)
	}
	if !errors.Is(err, ErrTokenRefreshExhausted) {
		t.Errorf("errors.Is(err, ErrTokenRefreshExhausted) = false, want true; err = %v", err)
	}
	var se *types.SourceError
	if errors.As(err, &se) && se.HTTPStatus != 401 {
		t.Errorf("HTTPStatus = %d, want 401", se.HTTPStatus)
	}
	// Exactly 2 search attempts (initial + 1 retry), no third.
	if searchHits.Load() != 2 {
		t.Errorf("search endpoint hits = %d, want 2 (no third attempt)", searchHits.Load())
	}
}

// --- REQ-ADP-001a-005: 403 Permanent ---

func TestSearch403StaysPermanent(t *testing.T) {
	t.Parallel()

	tokenTS, tokenHits := tokenEndpointStub(t, http.StatusOK, "tok123", 3600)

	searchTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer searchTS.Close()

	a := newAdapterWithStubs(t, tokenTS, searchTS)
	ctx := testContext(t)

	_, err := a.Search(ctx, types.Query{Text: "golang"})
	if err == nil {
		t.Fatal("Search() expected error for 403, got nil")
	}
	if !errors.Is(err, types.ErrPermanent) {
		t.Errorf("errors.Is(err, ErrPermanent) = false, want true; err = %v", err)
	}
	// Token endpoint should NOT be re-hit (no refresh on 403).
	if tokenHits.Load() != 1 {
		t.Errorf("token endpoint hits = %d, want 1 (no refresh on 403)", tokenHits.Load())
	}
}

// --- REQ-ADP-001a-006b: Credential Gate ---

func TestNewMissingClientIDReturnsErr(t *testing.T) {
	t.Parallel()

	_, err := New(Options{
		ClientID:     "",
		ClientSecret: "secret",
	})
	if err == nil {
		t.Fatal("New() expected error for missing ClientID, got nil")
	}
	if !errors.Is(err, ErrMissingCredentials) {
		t.Errorf("errors.Is(err, ErrMissingCredentials) = false, want true; err = %v", err)
	}
}

func TestNewMissingClientSecretReturnsErr(t *testing.T) {
	t.Parallel()

	_, err := New(Options{
		ClientID:     "id",
		ClientSecret: "",
	})
	if err == nil {
		t.Fatal("New() expected error for missing ClientSecret, got nil")
	}
	if !errors.Is(err, ErrMissingCredentials) {
		t.Errorf("errors.Is(err, ErrMissingCredentials) = false, want true; err = %v", err)
	}
}

func TestNewSkipAuthCheckSucceeds(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	a, err := New(Options{
		BaseURL:       ts.URL,
		SkipAuthCheck: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if a == nil {
		t.Fatal("New() returned nil adapter")
	}
}

// loadFixture reads a test fixture file.
func loadFixture(path string) ([]byte, error) {
	return os.ReadFile(path)
}
