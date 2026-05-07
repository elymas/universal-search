// Package koreanews — dedupDocs and canonicalizeURL unit tests.
// SPEC-ADP-009 / SPEC-FAN-001 §2.4 (8-rule URL canonicalization).
package koreanews

import (
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// makeDoc builds a minimal NormalizedDoc for dedup tests.
func makeDoc(url, title string) types.NormalizedDoc {
	return types.NormalizedDoc{
		ID:          "id-" + url,
		SourceID:    "koreanews",
		URL:         url,
		Title:       title,
		RetrievedAt: time.Now(),
	}
}

func TestDedupDocs_empty(t *testing.T) {
	t.Parallel()
	docs, dropped := dedupDocs(nil)
	if len(docs) != 0 || dropped != 0 {
		t.Errorf("dedupDocs(nil) = (%d, %d); want (0, 0)", len(docs), dropped)
	}
}

func TestDedupDocs_noDuplicates(t *testing.T) {
	t.Parallel()
	input := []types.NormalizedDoc{
		makeDoc("https://example.com/a", "A"),
		makeDoc("https://example.com/b", "B"),
		makeDoc("https://example.com/c", "C"),
	}
	got, dropped := dedupDocs(input)
	if len(got) != 3 || dropped != 0 {
		t.Errorf("got %d docs, %d dropped; want 3, 0", len(got), dropped)
	}
}

func TestDedupDocs_duplicatesDropped(t *testing.T) {
	t.Parallel()
	input := []types.NormalizedDoc{
		makeDoc("https://example.com/a", "A"),
		makeDoc("https://example.com/a", "A duplicate"),
		makeDoc("https://example.com/b", "B"),
	}
	got, dropped := dedupDocs(input)
	if len(got) != 2 || dropped != 1 {
		t.Errorf("got %d docs, %d dropped; want 2, 1", len(got), dropped)
	}
}

func TestDedupDocs_firstOccurrenceWins(t *testing.T) {
	t.Parallel()
	input := []types.NormalizedDoc{
		makeDoc("https://example.com/a", "First"),
		makeDoc("https://example.com/a", "Second"),
	}
	got, _ := dedupDocs(input)
	if got[0].Title != "First" {
		t.Errorf("first-occurrence-wins violated; got title %q", got[0].Title)
	}
}

func TestDedupDocs_trackingParamsCanonicalised(t *testing.T) {
	t.Parallel()
	// URL with and without tracking params should map to same key.
	input := []types.NormalizedDoc{
		makeDoc("https://news.example.com/article?utm_source=twitter&id=1", "A"),
		makeDoc("https://news.example.com/article?id=1", "A duplicate sans tracking"),
	}
	got, dropped := dedupDocs(input)
	if len(got) != 1 || dropped != 1 {
		t.Errorf("tracking-param dedup failed: got %d docs, %d dropped; want 1, 1", len(got), dropped)
	}
}

// ---- canonicalizeURL tests ----

func TestCanonicalizeURL_lowercase(t *testing.T) {
	t.Parallel()
	got, err := canonicalizeURL("HTTPS://News.Example.COM/Article")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://news.example.com/Article"
	if got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestCanonicalizeURL_removeDefaultHTTPSPort(t *testing.T) {
	t.Parallel()
	got, _ := canonicalizeURL("https://example.com:443/path")
	if want := "https://example.com/path"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestCanonicalizeURL_removeDefaultHTTPPort(t *testing.T) {
	t.Parallel()
	got, _ := canonicalizeURL("http://example.com:80/path")
	if want := "http://example.com/path"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestCanonicalizeURL_removeTrailingSlash(t *testing.T) {
	t.Parallel()
	got, _ := canonicalizeURL("https://example.com/path/to/article/")
	if want := "https://example.com/path/to/article"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestCanonicalizeURL_rootSlashPreserved(t *testing.T) {
	t.Parallel()
	got, _ := canonicalizeURL("https://example.com/")
	// Root slash should not be removed.
	if want := "https://example.com/"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestCanonicalizeURL_sortQueryParams(t *testing.T) {
	t.Parallel()
	got, _ := canonicalizeURL("https://example.com/s?z=1&a=2&m=3")
	if want := "https://example.com/s?a=2&m=3&z=1"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestCanonicalizeURL_removeTrackingParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		param string
		url   string
	}{
		{"utm_source", "https://ex.com/?utm_source=google&id=1"},
		{"utm_medium", "https://ex.com/?utm_medium=cpc&id=1"},
		{"utm_campaign", "https://ex.com/?utm_campaign=spring&id=1"},
		{"utm_term", "https://ex.com/?utm_term=keyword&id=1"},
		{"utm_content", "https://ex.com/?utm_content=banner&id=1"},
		{"fbclid", "https://ex.com/?fbclid=abc123&id=1"},
		{"gclid", "https://ex.com/?gclid=xyz&id=1"},
		{"ref", "https://ex.com/?ref=homepage&id=1"},
		{"source", "https://ex.com/?source=newsletter&id=1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.param, func(t *testing.T) {
			t.Parallel()
			got, _ := canonicalizeURL(tc.url)
			if want := "https://ex.com/?id=1"; got != want {
				t.Errorf("tracking %q not removed: got %q; want %q", tc.param, got, want)
			}
		})
	}
}

func TestCanonicalizeURL_removeEmptyQueryParams(t *testing.T) {
	t.Parallel()
	got, _ := canonicalizeURL("https://example.com/path?a=1&b=&c=3")
	if want := "https://example.com/path?a=1&c=3"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestCanonicalizeURL_removeFragment(t *testing.T) {
	t.Parallel()
	got, _ := canonicalizeURL("https://example.com/article#section2")
	if want := "https://example.com/article"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestCanonicalizeURL_emptyURL(t *testing.T) {
	t.Parallel()
	got, err := canonicalizeURL("")
	if err != nil || got != "" {
		t.Errorf("canonicalizeURL(%q) = (%q, %v); want (\"\", nil)", "", got, err)
	}
}
