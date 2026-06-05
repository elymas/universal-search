// Package social — tests for X (Twitter) live provider enablement.
// SPEC-ADP-006-XENABLE: REQ-XEN-001..009, NFR-XEN-001..004.
package social

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/elymas/universal-search/pkg/types"
)

// --- fakeProvider: test double for XProvider ---

// fakeProvider implements XProvider for testing.
// All fields are goroutine-safe (atomics or mutex-protected).
type fakeProvider struct {
	name       string
	tweets     []XTweet
	nextCursor string
	err        error // if set, SearchTweets returns this error

	mu         sync.Mutex
	callCount  atomic.Int64
	lastQuery  types.Query
	recordMode bool
}

func (fp *fakeProvider) Name() string { return fp.name }

func (fp *fakeProvider) SearchTweets(_ context.Context, q types.Query) ([]XTweet, string, error) {
	fp.callCount.Add(1)
	if fp.recordMode {
		fp.mu.Lock()
		fp.lastQuery = q
		fp.mu.Unlock()
	}
	if fp.err != nil {
		return nil, "", fp.err
	}
	return fp.tweets, fp.nextCursor, nil
}

// compile-time assertion: fakeProvider satisfies XProvider.
// Test 1: TestXProviderInterfaceShape
var _ XProvider = (*fakeProvider)(nil)

// helper: make N simple XTweet fixtures.
func makeXTweets(n int) []XTweet {
	tweets := make([]XTweet, n)
	for i := range n {
		tweets[i] = XTweet{
			ID:           fmt.Sprintf("tweet-%d", i+1),
			Text:         fmt.Sprintf("Tweet text %d", i+1),
			AuthorHandle: fmt.Sprintf("user%d", i+1),
			URL:          fmt.Sprintf("https://x.com/user%d/status/tweet-%d", i+1, i+1),
			LikeCount:    i * 10,
			RepostCount:  i * 5,
			ReplyCount:   i,
			QuoteCount:   i,
			CreatedAt:    "2026-06-04T12:00:00Z",
		}
	}
	return tweets
}

// helper: construct an X adapter with the given env state and provider.
func newTestXAdapter(envVal string, provider XProvider) *Adapter {
	a, _ := NewX(XOptions{
		EnvLookup: func(string) string { return envVal },
		Provider:  provider,
	})
	return a
}

// --- Test 1: TestXProviderInterfaceShape (compile-time assertion above) ---
// Verified by `var _ XProvider = (*fakeProvider)(nil)` at package level.

// --- Test 2: TestNewXAcceptsProvider ---
func TestNewXAcceptsProvider(t *testing.T) {
	t.Parallel()
	fp := &fakeProvider{name: "test-provider"}
	a, err := NewX(XOptions{
		Provider: fp,
	})
	if err != nil {
		t.Fatalf("NewX: unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("NewX: returned nil adapter")
	}
	if a.Name() != "x" {
		t.Errorf("Name(): got %q, want %q", a.Name(), "x")
	}
	if a.xProvider == nil {
		t.Error("xProvider: expected non-nil provider")
	}
}

// --- Test 3: TestNewXNilProviderOK ---
func TestNewXNilProviderOK(t *testing.T) {
	t.Parallel()
	a, err := NewX(XOptions{})
	if err != nil {
		t.Fatalf("NewX: unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("NewX: returned nil adapter")
	}
	if a.xProvider != nil {
		t.Error("xProvider: expected nil, got non-nil")
	}
}

// --- Test 4: TestXStillImplementsAdapter (compile-time) ---
// Verified by `var _ types.Adapter = (*Adapter)(nil)` in social.go.

// --- Test 5: TestSearchXLiveHappyPath ---
func TestSearchXLiveHappyPath(t *testing.T) {
	t.Parallel()
	fp := &fakeProvider{
		name:   "test-provider",
		tweets: makeXTweets(5),
	}
	a := newTestXAdapter("true", fp)

	docs, err := a.Search(context.Background(), types.Query{Text: "golang"})
	if err != nil {
		t.Fatalf("Search: unexpected error: %v", err)
	}
	if len(docs) != 5 {
		t.Fatalf("len(docs): got %d, want 5", len(docs))
	}
	for i, doc := range docs {
		if doc.Validate() != nil {
			t.Errorf("doc[%d].Validate(): %v", i, doc.Validate())
		}
		if doc.SourceID != "x" {
			t.Errorf("doc[%d].SourceID: got %q, want %q", i, doc.SourceID, "x")
		}
	}
}

// --- Test 6: TestSearchXLivePassesQueryToProvider ---
func TestSearchXLivePassesQueryToProvider(t *testing.T) {
	t.Parallel()
	fp := &fakeProvider{
		name:       "test-provider",
		tweets:     makeXTweets(1),
		recordMode: true,
	}
	a := newTestXAdapter("true", fp)

	q := types.Query{Text: "golang concurrency", MaxResults: 10}
	_, err := a.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search: unexpected error: %v", err)
	}

	fp.mu.Lock()
	got := fp.lastQuery
	fp.mu.Unlock()

	if got.Text != "golang concurrency" {
		t.Errorf("provider received Text: got %q, want %q", got.Text, "golang concurrency")
	}
	if got.MaxResults != 10 {
		t.Errorf("provider received MaxResults: got %d, want %d", got.MaxResults, 10)
	}
}

// --- Test 7: TestSearchXLiveSurfacesCursor ---
func TestSearchXLiveSurfacesCursor(t *testing.T) {
	t.Parallel()
	fp := &fakeProvider{
		name:       "test-provider",
		tweets:     makeXTweets(3),
		nextCursor: "cursor-page-2",
	}
	a := newTestXAdapter("true", fp)

	docs, err := a.Search(context.Background(), types.Query{Text: "test"})
	if err != nil {
		t.Fatalf("Search: unexpected error: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("len(docs): got %d, want 3", len(docs))
	}

	// First two docs should NOT have next_cursor.
	for i := 0; i < len(docs)-1; i++ {
		if _, ok := docs[i].Metadata["next_cursor"]; ok {
			t.Errorf("doc[%d].Metadata[next_cursor]: expected absent, got present", i)
		}
	}

	// Last doc should have next_cursor.
	last := docs[len(docs)-1]
	nc, ok := last.Metadata["next_cursor"]
	if !ok {
		t.Fatal("last doc Metadata[next_cursor]: expected present, got absent")
	}
	if nc != "cursor-page-2" {
		t.Errorf("last doc Metadata[next_cursor]: got %v, want %q", nc, "cursor-page-2")
	}
}

// --- Test 8: TestSearchXDisabledEvenWithProvider ---
func TestSearchXDisabledEvenWithProvider(t *testing.T) {
	t.Parallel()
	fp := &fakeProvider{
		name:   "test-provider",
		tweets: makeXTweets(5),
	}
	a := newTestXAdapter("", fp) // env = "" (disabled)

	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	if !errors.Is(err, ErrXDisabled) {
		t.Fatalf("expected errors.Is(err, ErrXDisabled), got: %v", err)
	}
	if fp.callCount.Load() != 0 {
		t.Errorf("provider call count: got %d, want 0 (should not be called when disabled)", fp.callCount.Load())
	}
}

// --- Test 9: TestSearchXEnabledNilProvider ---
func TestSearchXEnabledNilProvider(t *testing.T) {
	t.Parallel()
	a := newTestXAdapter("true", nil) // env on, nil provider

	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	if !errors.Is(err, ErrXProviderNotConfigured) {
		t.Fatalf("expected errors.Is(err, ErrXProviderNotConfigured), got: %v", err)
	}
}

// --- Test 10: TestSearchXErrorsArePermanent ---
func TestSearchXErrorsArePermanent(t *testing.T) {
	t.Parallel()

	disabled := newTestXAdapter("", nil)
	_, err1 := disabled.Search(context.Background(), types.Query{Text: "test"})
	if !errors.Is(err1, types.ErrPermanent) {
		t.Errorf("ErrXDisabled: expected errors.Is(err, ErrPermanent), got: %v", err1)
	}

	noProvider := newTestXAdapter("true", nil)
	_, err2 := noProvider.Search(context.Background(), types.Query{Text: "test"})
	if !errors.Is(err2, types.ErrPermanent) {
		t.Errorf("ErrXProviderNotConfigured: expected errors.Is(err, ErrPermanent), got: %v", err2)
	}
}

// --- Test 11: TestSearchXLiveEmptyQueryRejected ---
func TestSearchXLiveEmptyQueryRejected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"spaces", "   "},
		{"tabs_newlines", "\t\n  "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fp := &fakeProvider{
				name:   "test-provider",
				tweets: makeXTweets(1),
			}
			a := newTestXAdapter("true", fp)

			_, err := a.Search(context.Background(), types.Query{Text: tc.text})
			if err == nil {
				t.Fatal("expected error for empty/whitespace query, got nil")
			}
			if !errors.Is(err, types.ErrPermanent) {
				t.Errorf("expected errors.Is(err, ErrPermanent), got: %v", err)
			}
			if !errors.Is(err, ErrInvalidQuery) {
				t.Errorf("expected errors.Is(err, ErrInvalidQuery), got: %v", err)
			}
			if fp.callCount.Load() != 0 {
				t.Errorf("provider call count: got %d, want 0", fp.callCount.Load())
			}
		})
	}
}

// --- Test 12: TestSearchXProviderErrorPropagated ---
func TestSearchXProviderErrorPropagated(t *testing.T) {
	t.Parallel()
	providerErr := &types.SourceError{
		Adapter:  "x",
		Category: types.CategoryUnavailable,
		Cause:    fmt.Errorf("upstream down"),
	}
	fp := &fakeProvider{name: "test-provider", err: providerErr}
	a := newTestXAdapter("true", fp)

	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T", err)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category: got %v, want %v", se.Category, types.CategoryUnavailable)
	}
	// Should be the SAME error object, not a wrapper.
	if se != providerErr {
		t.Error("expected the provider's SourceError to be returned unchanged")
	}
}

// --- Test 13: TestSearchXProviderRawErrorWrapped ---
func TestSearchXProviderRawErrorWrapped(t *testing.T) {
	t.Parallel()
	rawErr := fmt.Errorf("boom")
	fp := &fakeProvider{name: "test-provider", err: rawErr}
	a := newTestXAdapter("true", fp)

	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T", err)
	}
	if se.Adapter != "x" {
		t.Errorf("Adapter: got %q, want %q", se.Adapter, "x")
	}
	if !errors.Is(se.Cause, rawErr) {
		t.Errorf("Cause: expected errors.Is(rawErr), got: %v", se.Cause)
	}
}

// --- Test 14: TestXOfficialRateLimit ---
func TestXOfficialRateLimit(t *testing.T) {
	t.Parallel()
	ts := newXStatusServer(429, map[string]string{"Retry-After": "30"}, "")
	defer ts.Close()

	prov, _ := NewXOfficialProvider(XOfficialOptions{
		BearerToken: "test-token",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})

	_, _, err := prov.SearchTweets(context.Background(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("expected error for 429, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T", err)
	}
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Category: got %v, want %v", se.Category, types.CategoryRateLimited)
	}
	if se.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter: got %v, want %v", se.RetryAfter, 30*time.Second)
	}
}

// --- Test 15: TestXOfficialAuthFailure ---
func TestXOfficialAuthFailure(t *testing.T) {
	t.Parallel()
	ts := newXStatusServer(401, nil, "")
	defer ts.Close()

	prov, _ := NewXOfficialProvider(XOfficialOptions{
		BearerToken: "test-token",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})

	_, _, err := prov.SearchTweets(context.Background(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category: got %v, want %v", se.Category, types.CategoryPermanent)
	}
}

// --- Test 16: TestXOfficialUnavailable ---
func TestXOfficialUnavailable(t *testing.T) {
	t.Parallel()
	ts := newXStatusServer(503, nil, "")
	defer ts.Close()

	prov, _ := NewXOfficialProvider(XOfficialOptions{
		BearerToken: "test-token",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})

	_, _, err := prov.SearchTweets(context.Background(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T", err)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category: got %v, want %v", se.Category, types.CategoryUnavailable)
	}
}

// --- Test 26: TestXOfficialRequestFields ---
func TestXOfficialRequestFields(t *testing.T) {
	t.Parallel()
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[],"meta":{}}`))
	}))
	defer ts.Close()

	prov, _ := NewXOfficialProvider(XOfficialOptions{
		BearerToken: "test-token",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})

	_, _, err := prov.SearchTweets(context.Background(), types.Query{Text: "golang"})
	if err != nil {
		t.Fatalf("SearchTweets: unexpected error: %v", err)
	}

	if !containsSubstring(capturedURL, "tweet.fields=public_metrics") {
		t.Errorf("request URL missing tweet.fields=public_metrics: %s", capturedURL)
	}
	if !containsSubstring(capturedURL, "created_at") {
		t.Errorf("request URL missing created_at in tweet.fields: %s", capturedURL)
	}
}

// --- Test 27: TestXOfficialPassesCursor ---
func TestXOfficialPassesCursor(t *testing.T) {
	t.Parallel()
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[],"meta":{}}`))
	}))
	defer ts.Close()

	prov, _ := NewXOfficialProvider(XOfficialOptions{
		BearerToken: "test-token",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})

	_, _, err := prov.SearchTweets(context.Background(), types.Query{
		Text:   "golang",
		Cursor: "t1",
	})
	if err != nil {
		t.Fatalf("SearchTweets: unexpected error: %v", err)
	}

	if !containsSubstring(capturedURL, "next_token=t1") {
		t.Errorf("request URL missing next_token=t1: %s", capturedURL)
	}
}

// --- Test 28: TestXOfficialMapsPublicMetrics ---
func TestXOfficialMapsPublicMetrics(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "123",
				"text": "Hello",
				"public_metrics": {
					"like_count": 42,
					"retweet_count": 13,
					"reply_count": 7,
					"quote_count": 3
				},
				"created_at": "2026-06-04T12:00:00Z"
			}],
			"meta": {"result_count": 1}
		}`))
	}))
	defer ts.Close()

	prov, _ := NewXOfficialProvider(XOfficialOptions{
		BearerToken: "test-token",
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})

	tweets, _, err := prov.SearchTweets(context.Background(), types.Query{Text: "test"})
	if err != nil {
		t.Fatalf("SearchTweets: unexpected error: %v", err)
	}
	if len(tweets) != 1 {
		t.Fatalf("len(tweets): got %d, want 1", len(tweets))
	}
	tw := tweets[0]
	if tw.LikeCount != 42 {
		t.Errorf("LikeCount: got %d, want 42", tw.LikeCount)
	}
	if tw.RepostCount != 13 {
		t.Errorf("RepostCount: got %d, want 13", tw.RepostCount)
	}
	if tw.ReplyCount != 7 {
		t.Errorf("ReplyCount: got %d, want 7", tw.ReplyCount)
	}
	if tw.QuoteCount != 3 {
		t.Errorf("QuoteCount: got %d, want 3", tw.QuoteCount)
	}
	if tw.CreatedAt != "2026-06-04T12:00:00Z" {
		t.Errorf("CreatedAt: got %q, want %q", tw.CreatedAt, "2026-06-04T12:00:00Z")
	}
}

// helper: containsSubstring reports whether substr is in s.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Test 22: TestXHealthcheckNilProvider ---
func TestXHealthcheckNilProvider(t *testing.T) {
	t.Parallel()
	a := newTestXAdapter("true", nil)

	err := a.Healthcheck(context.Background())
	if !errors.Is(err, ErrXDisabled) {
		t.Errorf("expected errors.Is(err, ErrXDisabled), got: %v", err)
	}
}

// --- Test 23: TestXHealthcheckLive ---
func TestXHealthcheckLive(t *testing.T) {
	t.Parallel()
	fp := &fakeProvider{
		name:   "test-provider",
		tweets: makeXTweets(1),
	}
	a := newTestXAdapter("true", fp)

	err := a.Healthcheck(context.Background())
	if err != nil {
		t.Errorf("Healthcheck: unexpected error: %v", err)
	}
}

// --- Test 24: TestXHealthcheckLiveFailure ---
func TestXHealthcheckLiveFailure(t *testing.T) {
	t.Parallel()
	fp := &fakeProvider{
		name: "test-provider",
		err:  fmt.Errorf("upstream unreachable"),
	}
	a := newTestXAdapter("true", fp)

	err := a.Healthcheck(context.Background())
	if err == nil {
		t.Fatal("Healthcheck: expected error for failing provider, got nil")
	}
	var se *types.SourceError
	if !errors.As(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T", err)
	}
}

// --- Test 25: TestXHealthcheckCtxCancel ---
func TestXHealthcheckCtxCancel(t *testing.T) {
	t.Parallel()
	fp := &fakeProvider{
		name: "test-provider",
		err:  context.Canceled,
	}
	a := newTestXAdapter("true", fp)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := a.Healthcheck(ctx)
	if err == nil {
		t.Fatal("Healthcheck: expected error for cancelled ctx, got nil")
	}
}

// --- Test 29: TestSearchXLiveConcurrentSafe ---
func TestSearchXLiveConcurrentSafe(t *testing.T) {
	t.Parallel()
	fp := &fakeProvider{
		name:   "test-provider",
		tweets: makeXTweets(3),
	}
	a := newTestXAdapter("true", fp)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errs := make(chan error, goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			docs, err := a.Search(context.Background(), types.Query{Text: "concurrent"})
			if err != nil {
				errs <- err
				return
			}
			if len(docs) != 3 {
				errs <- fmt.Errorf("expected 3 docs, got %d", len(docs))
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Search: %v", err)
	}
}

// --- Test 30: TestSearchBothSubSourcesLiveConcurrent ---
func TestSearchBothSubSourcesLiveConcurrent(t *testing.T) {
	t.Parallel()

	// Bluesky adapter with httptest server.
	body, err := os.ReadFile(testdataPath + "bluesky_search_response.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	blueskyTS := newXStatusServer(200, map[string]string{"Content-Type": "application/json"}, string(body))
	defer blueskyTS.Close()

	blueskyAdapter, _ := NewBluesky(BlueskyOptions{
		BaseURL:    blueskyTS.URL,
		HTTPClient: blueskyTS.Client(),
	})

	// X adapter with fake provider.
	fp := &fakeProvider{
		name:   "test-provider",
		tweets: makeXTweets(3),
	}
	xAdapter := newTestXAdapter("true", fp)

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)
	errs := make(chan error, goroutines*2)

	for range goroutines {
		go func() {
			defer wg.Done()
			docs, err := blueskyAdapter.Search(context.Background(), types.Query{Text: "test"})
			if err != nil {
				errs <- fmt.Errorf("bluesky: %w", err)
			} else if len(docs) == 0 {
				errs <- fmt.Errorf("bluesky: expected docs, got 0")
			}
		}()
		go func() {
			defer wg.Done()
			docs, err := xAdapter.Search(context.Background(), types.Query{Text: "test"})
			if err != nil {
				errs <- fmt.Errorf("x: %w", err)
			} else if len(docs) != 3 {
				errs <- fmt.Errorf("x: expected 3 docs, got %d", len(docs))
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent cross-source: %v", err)
	}
}

// --- Test 31: TestSearchXProviderErrorCategoryParity ---
func TestSearchXProviderErrorCategoryParity(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		statusCode int
		want       types.Category
	}{
		{"429 RateLimited", 429, types.CategoryRateLimited},
		{"401 Permanent", 401, types.CategoryPermanent},
		{"403 Permanent", 403, types.CategoryPermanent},
		{"404 Permanent", 404, types.CategoryPermanent},
		{"500 Unavailable", 500, types.CategoryUnavailable},
		{"503 Unavailable", 503, types.CategoryUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			headers := map[string]string{}
			if tc.statusCode == 429 {
				headers["Retry-After"] = "10"
			}
			ts := newXStatusServer(tc.statusCode, headers, "")
			defer ts.Close()

			prov, _ := NewXOfficialProvider(XOfficialOptions{
				BearerToken: "test-token",
				BaseURL:     ts.URL,
				HTTPClient:  ts.Client(),
			})

			_, _, err := prov.SearchTweets(context.Background(), types.Query{Text: "test"})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var se *types.SourceError
			if !errors.As(err, &se) {
				t.Fatalf("expected *types.SourceError, got %T", err)
			}
			if se.Category != tc.want {
				t.Errorf("Category: got %v, want %v", se.Category, tc.want)
			}
		})
	}
}

// --- Test 32: TestXNoSecretInError ---
func TestXNoSecretInError(t *testing.T) {
	t.Parallel()
	ts := newXStatusServer(401, nil, "")
	defer ts.Close()

	secret := "super-secret-bearer-token-12345"
	prov, _ := NewXOfficialProvider(XOfficialOptions{
		BearerToken: secret,
		BaseURL:     ts.URL,
		HTTPClient:  ts.Client(),
	})

	_, _, err := prov.SearchTweets(context.Background(), types.Query{Text: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, secret) {
		t.Errorf("error message contains secret: %q", errMsg)
	}
}

// --- Test 34: TestSearchXLiveNoGoroutineLeakOnCancel ---
func TestSearchXLiveNoGoroutineLeakOnCancel(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	// Provider that blocks until context is done.
	fp := &blockingFakeProvider{}
	a := newTestXAdapter("true", fp)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay to simulate mid-flight cancellation.
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, _ = a.Search(ctx, types.Query{Text: "test"})
	// goleak.VerifyNone will check for leaked goroutines.
}

// --- Helpers ---

// blockingFakeProvider blocks on SearchTweets until context is cancelled.
type blockingFakeProvider struct{}

func (p *blockingFakeProvider) Name() string { return "blocking" }

func (p *blockingFakeProvider) SearchTweets(ctx context.Context, _ types.Query) ([]XTweet, string, error) {
	<-ctx.Done()
	return nil, "", ctx.Err()
}

// newXStatusServer creates an httptest.Server that returns the given status code
// and headers. If body is empty, no body is written.
func newXStatusServer(status int, headers map[string]string, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
}
