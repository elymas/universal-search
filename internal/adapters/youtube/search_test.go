package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
	"go.uber.org/goleak"
)

// happyPathServer returns an httptest.Server that serves search_response.json.
func happyPathServer(t *testing.T) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		t.Fatalf("load search_response.json: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
}

func TestSearchHappyPath25Videos(t *testing.T) {
	t.Parallel()
	srv := happyPathServer(t)
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	docs, err := a.Search(context.Background(), types.Query{Text: "go tutorials", MaxResults: 25})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) != 25 {
		t.Errorf("len(docs) = %d, want 25", len(docs))
	}
	for i, doc := range docs {
		if doc.ID == "" {
			t.Errorf("docs[%d].ID empty", i)
		}
		if doc.SourceID != "youtube" {
			t.Errorf("docs[%d].SourceID = %q", i, doc.SourceID)
		}
		if doc.URL == "" {
			t.Errorf("docs[%d].URL empty", i)
		}
		if doc.DocType != types.DocTypeVideo {
			t.Errorf("docs[%d].DocType = %q, want video", i, doc.DocType)
		}
	}
}

func TestSearchRequestBodyIncludesAllRequired(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{Text: "golang", MaxResults: 10})

	if captured.Query != "golang" {
		t.Errorf("request body query = %q, want %q", captured.Query, "golang")
	}
	if captured.MaxResults != 10 {
		t.Errorf("request body max_results = %d, want 10", captured.MaxResults)
	}
	if !captured.IncludeTranscripts {
		t.Error("include_transcripts = false, want true")
	}
	if captured.TranscriptLang == "" {
		t.Error("transcript_lang empty, want non-empty")
	}
}

func TestSearchClampsMaxResultsTo100(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{Text: "test", MaxResults: 500})

	if captured.MaxResults != 100 {
		t.Errorf("max_results = %d, want 100 (clamped)", captured.MaxResults)
	}
}

func TestSearchDefaultsMaxResultsTo25(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{Text: "test", MaxResults: 0})

	if captured.MaxResults != 25 {
		t.Errorf("max_results = %d, want 25 (default)", captured.MaxResults)
	}
}

func TestSearchOmitsCursorWhenEmpty(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{Text: "test", Cursor: ""})

	if captured.CursorOffset != 0 {
		t.Errorf("cursor_offset = %d, want 0 when cursor not supplied", captured.CursorOffset)
	}
}

func TestSearchSetsCursorWhenPresent(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{Text: "test", Cursor: "25"})

	if captured.CursorOffset != 25 {
		t.Errorf("cursor_offset = %d, want 25", captured.CursorOffset)
	}
}

func TestSearchSetsContentTypeJSON(t *testing.T) {
	t.Parallel()
	var capturedCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{Text: "test"})

	if capturedCT != "application/json" {
		t.Errorf("Content-Type = %q, want %q", capturedCT, "application/json")
	}
}

func TestSearchSetsCustomUserAgent(t *testing.T) {
	t.Parallel()
	var capturedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{Text: "test"})

	if !contains(capturedUA, "usearch/") {
		t.Errorf("User-Agent = %q, want prefix usearch/", capturedUA)
	}
	if !contains(capturedUA, "(+https://github.com/elymas/universal-search)") {
		t.Errorf("User-Agent = %q, missing URL", capturedUA)
	}
}

func TestSearchSetsAcceptJSON(t *testing.T) {
	t.Parallel()
	var capturedAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{Text: "test"})

	if capturedAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", capturedAccept)
	}
}

func TestSearchUserAgentVersionConfigurable(t *testing.T) {
	t.Parallel()
	var capturedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL, UserAgentVersion: "v0.2-rc1"})
	a.Search(context.Background(), types.Query{Text: "test"})

	if !contains(capturedUA, "usearch/v0.2-rc1") {
		t.Errorf("User-Agent = %q, want usearch/v0.2-rc1", capturedUA)
	}
}

// ---- HTTP 429 Rate-Limit Tests ----

func TestSearchHTTP429WithIntegerRetryAfter(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	requireRateLimited(t, err, 30*time.Second)
}

func TestSearchHTTP429WithHTTPDateRetryAfter(t *testing.T) {
	t.Parallel()
	future := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", future)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	_, err := a.Search(context.Background(), types.Query{Text: "test"})

	se := requireSourceError(t, err)
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Category = %q, want rate_limited", se.Category)
	}
	if se.RetryAfter < 25*time.Second || se.RetryAfter > 35*time.Second {
		t.Errorf("RetryAfter = %v, want in (25s, 35s)", se.RetryAfter)
	}
}

func TestSearchHTTP429NoRetryAfterDefaults30s(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	requireRateLimited(t, err, 30*time.Second)
}

func TestSearchHTTP429RetryAfterCapped60s(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "999")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	_, err := a.Search(context.Background(), types.Query{Text: "test"})
	requireRateLimited(t, err, 60*time.Second)
}

func TestSearchHTTP429NoInternalRetry(t *testing.T) {
	t.Parallel()
	var count int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&count, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{Text: "test"})

	if count != 1 {
		t.Errorf("request count = %d, want 1 (no internal retry)", count)
	}
}

// ---- HTTP 4xx/5xx and Error Mapping ----

func TestSearchHTTP4xx(t *testing.T) {
	t.Parallel()
	for _, status := range []int{401, 403, 404} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			a, _ := New(Options{BaseURL: srv.URL})
			_, err := a.Search(context.Background(), types.Query{Text: "test"})
			if !errors.Is(err, types.ErrPermanent) {
				t.Errorf("status %d: errors.Is(err, ErrPermanent) = false, err = %v", status, err)
			}
			se := requireSourceError(t, err)
			if se.HTTPStatus != status {
				t.Errorf("HTTPStatus = %d, want %d", se.HTTPStatus, status)
			}
		})
	}
}

func TestSearchHTTP5xx(t *testing.T) {
	t.Parallel()
	for _, status := range []int{500, 503, 504} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer srv.Close()

			a, _ := New(Options{BaseURL: srv.URL})
			_, err := a.Search(context.Background(), types.Query{Text: "test"})
			if !errors.Is(err, types.ErrSourceUnavailable) {
				t.Errorf("status %d: errors.Is(err, ErrSourceUnavailable) = false, err = %v", status, err)
			}
			se := requireSourceError(t, err)
			if se.HTTPStatus != status {
				t.Errorf("HTTPStatus = %d, want %d", se.HTTPStatus, status)
			}
		})
	}
}

func TestSearchSidecarUnreachable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // close immediately

	a, _ := New(Options{BaseURL: srv.URL})
	_, err := a.Search(context.Background(), types.Query{Text: "test"})

	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false, err = %v", err)
	}
	se := requireSourceError(t, err)
	if se.HTTPStatus != 0 {
		t.Errorf("HTTPStatus = %d, want 0 for network error", se.HTTPStatus)
	}
}

func TestSearchSidecarYtdlpChallenge(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"category": "unavailable",
				"message":  "yt-dlp signed-in challenge",
			},
		})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	_, err := a.Search(context.Background(), types.Query{Text: "test"})

	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false, err = %v", err)
	}
	se := requireSourceError(t, err)
	if se.Cause == nil || !contains(se.Cause.Error(), "yt-dlp signed-in challenge") {
		t.Errorf("Cause = %v, want to contain 'yt-dlp signed-in challenge'", se.Cause)
	}
}

func TestSearchUnavailablePreservesUnderlyingError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	_, err := a.Search(context.Background(), types.Query{Text: "test"})

	se := requireSourceError(t, err)
	if se.Cause == nil {
		t.Error("Cause is nil, want non-nil inner error")
	}
}

// ---- Validation: Empty Query, Invalid Cursor, Cursor Over Cap ----

func TestSearchEmptyQueryRejectedNoHTTP(t *testing.T) {
	t.Parallel()
	var count int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&count, 1)
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	for _, text := range []string{"", "   ", "\t\n  \r"} {
		_, err := a.Search(context.Background(), types.Query{Text: text})
		if !errors.Is(err, types.ErrPermanent) {
			t.Errorf("text=%q: errors.Is(err, ErrPermanent) = false, err = %v", text, err)
		}
		if !errors.Is(err, ErrInvalidQuery) {
			t.Errorf("text=%q: errors.Is(err, ErrInvalidQuery) = false, err = %v", text, err)
		}
	}
	if count != 0 {
		t.Errorf("request count = %d, want 0 (no HTTP for invalid query)", count)
	}
}

func TestSearchInvalidCursorRejectedNoHTTP(t *testing.T) {
	t.Parallel()
	var count int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&count, 1)
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	for _, cursor := range []string{"abc", "-1", "1.5", "1e3", " 25"} {
		_, err := a.Search(context.Background(), types.Query{Text: "test", Cursor: cursor})
		if !errors.Is(err, types.ErrPermanent) {
			t.Errorf("cursor=%q: errors.Is(err, ErrPermanent) = false, err = %v", cursor, err)
		}
		if !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("cursor=%q: errors.Is(err, ErrInvalidCursor) = false, err = %v", cursor, err)
		}
	}
	if count != 0 {
		t.Errorf("request count = %d, want 0 (no HTTP for invalid cursor)", count)
	}
}

func TestSearchCursorOverCapRejected(t *testing.T) {
	t.Parallel()
	var count int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&count, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})

	// 50 + 60 = 110 > 100: rejected.
	_, err := a.Search(context.Background(), types.Query{Text: "test", MaxResults: 50, Cursor: "60"})
	if !errors.Is(err, ErrCursorOverCap) {
		t.Errorf("50+60: errors.Is(err, ErrCursorOverCap) = false, err = %v", err)
	}

	// 25 + 75 = 100 == 100: allowed (inclusive boundary).
	_, err = a.Search(context.Background(), types.Query{Text: "test", MaxResults: 25, Cursor: "75"})
	if errors.Is(err, ErrCursorOverCap) {
		t.Error("25+75=100: should be allowed but got ErrCursorOverCap")
	}

	// MaxResults=0 (defaults to 25) + 76 = 101 > 100: rejected.
	_, err = a.Search(context.Background(), types.Query{Text: "test", MaxResults: 0, Cursor: "76"})
	if !errors.Is(err, ErrCursorOverCap) {
		t.Errorf("0(=25)+76: errors.Is(err, ErrCursorOverCap) = false, err = %v", err)
	}
}

// ---- Context Cancellation ----

func TestSearchCtxAlreadyCancelled(t *testing.T) {
	t.Parallel()
	var count int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&count, 1)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	a, _ := New(Options{BaseURL: srv.URL})
	_, err := a.Search(ctx, types.Query{Text: "test"})

	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false, err = %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false, err = %v", err)
	}
	if count != 0 {
		t.Errorf("request count = %d, want 0 for pre-cancelled ctx", count)
	}
}

func TestSearchCtxCancelledMidFlight(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay so cancellation fires mid-flight.
		select {
		case <-r.Context().Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	a, _ := New(Options{BaseURL: srv.URL})
	_, err := a.Search(ctx, types.Query{Text: "test"})

	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false, err = %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false, err = %v", err)
	}
}

func TestSearchCtxDeadlineExceeded(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	a, _ := New(Options{BaseURL: srv.URL})
	_, err := a.Search(ctx, types.Query{Text: "test"})

	if !errors.Is(err, types.ErrSourceUnavailable) {
		t.Errorf("errors.Is(err, ErrSourceUnavailable) = false, err = %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false, err = %v", err)
	}
}

func TestSearchCtxPrecedenceOverValidation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel; query is also empty

	a, _ := New(Options{BaseURL: "http://127.0.0.1:19998"})
	_, err := a.Search(ctx, types.Query{Text: ""})

	// ctx cancellation takes precedence over empty query rejection.
	if errors.Is(err, ErrInvalidQuery) {
		t.Error("got ErrInvalidQuery, but ctx cancel should take precedence")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false, err = %v", err)
	}
}

func TestSearchNoGoroutineLeakOnCancel(t *testing.T) {
	defer goleak.VerifyNone(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(300 * time.Millisecond):
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(ctx, types.Query{Text: "test"})
	// Allow any background goroutines to wind down.
	time.Sleep(20 * time.Millisecond)
}

// ---- Filter Tests ----

func TestSearchExplicitLangFilterWins(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{
		Text:    "안녕하세요 이것은 한국어 쿼리입니다", // Korean text
		Filters: []types.Filter{{Key: "lang", Value: "ja"}},
	})

	if captured.TranscriptLang != "ja" {
		t.Errorf("transcript_lang = %q, want %q (explicit filter wins over auto-detect)", captured.TranscriptLang, "ja")
	}
}

func TestSearchKoreanAutoDetection(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{
		Text: "안녕하세요 이것은 한국어 쿼리입니다",
	})

	if captured.TranscriptLang != "ko" {
		t.Errorf("transcript_lang = %q, want %q (Korean auto-detect)", captured.TranscriptLang, "ko")
	}
}

func TestSearchEnglishDefaultForLatinScript(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{Text: "hello world golang tutorial"})

	if captured.TranscriptLang != "en" {
		t.Errorf("transcript_lang = %q, want %q", captured.TranscriptLang, "en")
	}
}

func TestSearchKoreanThresholdBoundary(t *testing.T) {
	t.Parallel()
	// Build queries at just below and just above the 30% Hangul threshold.
	// 22 ASCII + 10 Hangul = 32 runes → 10/32 ≈ 31.25% → Korean.
	aboveQuery := "aaaaaaaaaaaaaaaaaaaaaa" + "안녕하세요가나다라마"
	// 17 ASCII + 4 Hangul = 21 runes → 4/21 ≈ 19% → English.
	belowQuery := "aaaaaaaaaaaaaaaaa" + "안녕"

	for _, tc := range []struct {
		name string
		text string
		want string
	}{
		{"above-30pct-korean", aboveQuery, "ko"},
		{"below-30pct-english", belowQuery, "en"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var captured searchRequestBody
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				json.Unmarshal(body, &captured)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(ytSearchResponse{})
			}))
			defer srv.Close()

			a, _ := New(Options{BaseURL: srv.URL})
			a.Search(context.Background(), types.Query{Text: tc.text})

			if captured.TranscriptLang != tc.want {
				t.Errorf("transcript_lang = %q, want %q", captured.TranscriptLang, tc.want)
			}
		})
	}
}

func TestSearchSinceFilterAdded(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{
		Text:    "test",
		Filters: []types.Filter{{Key: "since", Value: "1700000000"}},
	})

	if captured.Since == nil || *captured.Since != 1700000000 {
		t.Errorf("since = %v, want 1700000000", captured.Since)
	}
}

func TestSearchSinceFilterMalformedDropped(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{
		Text:    "test",
		Filters: []types.Filter{{Key: "since", Value: "abc"}},
	})

	if captured.Since != nil {
		t.Errorf("since = %v, want nil (malformed dropped)", captured.Since)
	}
}

func TestSearchSinceFilterNegativeDropped(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{
		Text:    "test",
		Filters: []types.Filter{{Key: "since", Value: "-100"}},
	})

	if captured.Since != nil {
		t.Errorf("since = %v, want nil (negative dropped)", captured.Since)
	}
}

func TestSearchUnknownFilterIgnored(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{
		Text:    "hello",
		Filters: []types.Filter{{Key: "nsfw", Value: "true"}},
	})

	// Unknown filter must not cause crash and must not change transcript_lang.
	if captured.TranscriptLang != "en" {
		t.Errorf("transcript_lang = %q, expected en (unknown filter ignored)", captured.TranscriptLang)
	}
}

func TestSearchEmptyLangValueDropsToDefault(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	a.Search(context.Background(), types.Query{
		Text:    "hello world",
		Filters: []types.Filter{{Key: "lang", Value: ""}},
	})

	if captured.TranscriptLang != "en" {
		t.Errorf("transcript_lang = %q, want en (empty lang value drops to default)", captured.TranscriptLang)
	}
}

func TestSearchInvalidLangFormatRejected(t *testing.T) {
	t.Parallel()
	var captured searchRequestBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ytSearchResponse{})
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	// "verylongstring" is > 8 chars → treated as invalid.
	a.Search(context.Background(), types.Query{
		Text:    "hello world",
		Filters: []types.Filter{{Key: "lang", Value: "verylongstring"}},
	})

	if captured.TranscriptLang != "en" {
		t.Errorf("transcript_lang = %q, want en (too-long lang rejected)", captured.TranscriptLang)
	}
}

// ---- Concurrency and E2E Performance ----

func TestSearchConcurrentSafe(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	var requestCount int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	results := make([][]types.NormalizedDoc, N)
	errs := make([]error, N)

	var barrier sync.WaitGroup
	barrier.Add(1)

	for i := range N {
		i := i
		go func() {
			defer wg.Done()
			barrier.Wait() // synchronize start
			docs, err := a.Search(context.Background(), types.Query{Text: "golang", MaxResults: 25})
			results[i] = docs
			errs[i] = err
		}()
	}
	barrier.Done() // release all goroutines simultaneously
	wg.Wait()

	if atomic.LoadInt64(&requestCount) != N {
		t.Errorf("request count = %d, want %d", requestCount, N)
	}
	for i := range N {
		if errs[i] != nil {
			t.Errorf("goroutine %d error: %v", i, errs[i])
		}
		if len(results[i]) != 25 {
			t.Errorf("goroutine %d: len(docs) = %d, want 25", i, len(results[i]))
		}
	}
}

func TestSearchE2ELatencyStubP95(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}
	t.Parallel()
	body, err := os.ReadFile("testdata/search_response.json")
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	a, _ := New(Options{BaseURL: srv.URL})
	const iterations = 100
	durations := make([]time.Duration, iterations)
	for i := range iterations {
		start := time.Now()
		_, err := a.Search(context.Background(), types.Query{Text: "test", MaxResults: 25})
		if err != nil {
			t.Fatalf("Search[%d]: %v", i, err)
		}
		durations[i] = time.Since(start)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[94]
	if p95 > 200*time.Millisecond {
		t.Errorf("p95 latency = %v, want ≤ 200ms", p95)
	}
}

// ---- Helpers ----

func requireSourceError(t *testing.T, err error) *types.SourceError {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	se, ok := err.(*types.SourceError)
	if !ok {
		// Try unwrapping
		var se2 *types.SourceError
		if errors.As(err, &se2) {
			return se2
		}
		t.Fatalf("error type = %T, want *types.SourceError", err)
	}
	return se
}

func requireRateLimited(t *testing.T, err error, wantRetryAfter time.Duration) {
	t.Helper()
	se := requireSourceError(t, err)
	if se.Category != types.CategoryRateLimited {
		t.Errorf("Category = %q, want rate_limited", se.Category)
	}
	if se.HTTPStatus != http.StatusTooManyRequests {
		t.Errorf("HTTPStatus = %d, want 429", se.HTTPStatus)
	}
	if se.RetryAfter != wantRetryAfter {
		t.Errorf("RetryAfter = %v, want %v", se.RetryAfter, wantRetryAfter)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
