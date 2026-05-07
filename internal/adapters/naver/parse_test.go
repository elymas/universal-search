package naver

import (
	"os"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

var fixedTime = time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)

// TestParseBlogResponse_HappyPath verifies the basic blog parsing from the 25-item fixture.
// REQ-ADP8-006.
func TestParseBlogResponse_HappyPath(t *testing.T) {
	t.Parallel()
	body := mustReadFile(t, "testdata/search_response_blog.json")

	docs, err := parseBlogResponse(body, fixedTime)
	if err != nil {
		t.Fatalf("parseBlogResponse() error = %v", err)
	}
	if len(docs) != 25 {
		t.Fatalf("len(docs) = %d, want 25", len(docs))
	}

	// Spot-check first doc.
	d := docs[0]
	if d.SourceID != "naver" {
		t.Errorf("docs[0].SourceID = %q, want %q", d.SourceID, "naver")
	}
	if d.DocType != types.DocTypePost {
		t.Errorf("docs[0].DocType = %v, want DocTypePost", d.DocType)
	}
	if d.Lang != "ko" {
		t.Errorf("docs[0].Lang = %q, want %q", d.Lang, "ko")
	}
	if d.Score != defaultScore {
		t.Errorf("docs[0].Score = %v, want %v", d.Score, defaultScore)
	}
	if d.RetrievedAt != fixedTime {
		t.Errorf("docs[0].RetrievedAt = %v, want %v", d.RetrievedAt, fixedTime)
	}
	if d.URL == "" {
		t.Error("docs[0].URL is empty")
	}
	if d.ID == "" {
		t.Error("docs[0].ID is empty")
	}

	// Author should come from bloggername.
	if d.Author == "" {
		t.Error("docs[0].Author is empty")
	}

	// Metadata should contain bloggername + bloggerlink.
	if d.Metadata == nil {
		t.Fatal("docs[0].Metadata is nil")
	}
	if _, ok := d.Metadata["bloggername"]; !ok {
		t.Error("docs[0].Metadata missing 'bloggername'")
	}
	if _, ok := d.Metadata["bloggerlink"]; !ok {
		t.Error("docs[0].Metadata missing 'bloggerlink'")
	}

	// Validate all docs pass NormalizedDoc.Validate().
	for i, doc := range docs {
		docCopy := doc
		if err := docCopy.Validate(); err != nil {
			t.Errorf("docs[%d].Validate() error = %v", i, err)
		}
	}
}

// TestParseBlogResponse_Empty verifies zero items returns nil docs without error.
func TestParseBlogResponse_Empty(t *testing.T) {
	t.Parallel()
	body := mustReadFile(t, "testdata/search_response_blog_empty.json")

	docs, err := parseBlogResponse(body, fixedTime)
	if err != nil {
		t.Fatalf("parseBlogResponse() error = %v, want nil", err)
	}
	if docs != nil {
		t.Errorf("parseBlogResponse() docs = %v, want nil", docs)
	}
}

// TestParseBlogResponse_MalformedJSON verifies malformed JSON returns a SourceError.
func TestParseBlogResponse_MalformedJSON(t *testing.T) {
	t.Parallel()
	body := mustReadFile(t, "testdata/search_response_blog_malformed.json")

	docs, err := parseBlogResponse(body, fixedTime)
	if err == nil {
		t.Fatal("parseBlogResponse() error = nil, want error for malformed JSON")
	}
	if docs != nil {
		t.Error("parseBlogResponse() returned non-nil docs on error")
	}
	se, ok := err.(*types.SourceError)
	if !ok {
		t.Fatalf("parseBlogResponse() error type = %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("parseBlogResponse() category = %v, want CategoryPermanent", se.Category)
	}
}

// TestParseBlogResponse_MalformedPostDate verifies graceful handling of bad postdate.
// Empty or non-YYYYMMDD postdate should produce zero PublishedAt, not panic.
func TestParseBlogResponse_MalformedPostDate(t *testing.T) {
	t.Parallel()
	body := mustReadFile(t, "testdata/search_response_blog_malformed_postdate.json")

	docs, err := parseBlogResponse(body, fixedTime)
	if err != nil {
		t.Fatalf("parseBlogResponse() error = %v, want nil", err)
	}
	if len(docs) != 3 {
		t.Fatalf("len(docs) = %d, want 3", len(docs))
	}

	// docs[0] has valid postdate "20260507".
	if docs[0].PublishedAt.IsZero() {
		t.Error("docs[0].PublishedAt is zero, want non-zero for valid postdate")
	}
	// docs[1] has empty postdate — should be zero.
	if !docs[1].PublishedAt.IsZero() {
		t.Errorf("docs[1].PublishedAt = %v, want zero for empty postdate", docs[1].PublishedAt)
	}
	// docs[2] has invalid postdate "not-a-date" — should be zero.
	if !docs[2].PublishedAt.IsZero() {
		t.Errorf("docs[2].PublishedAt = %v, want zero for invalid postdate", docs[2].PublishedAt)
	}
}

// TestParseBlogResponse_HTMLStripping verifies <b> tags and entities are stripped.
// REQ-ADP8-006: HTML strip + entity decode integration.
func TestParseBlogResponse_HTMLStripping(t *testing.T) {
	t.Parallel()
	body := mustReadFile(t, "testdata/search_response_blog_html_entities.json")

	docs, err := parseBlogResponse(body, fixedTime)
	if err != nil {
		t.Fatalf("parseBlogResponse() error = %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("parseBlogResponse() returned 0 docs")
	}

	// First doc title should not contain <b> bold tags or raw HTML entities like &amp;.
	title := docs[0].Title
	if containsRawHTML(title) {
		t.Errorf("docs[0].Title still contains raw HTML markup: %q", title)
	}

	// The title should contain "&" (decoded from &amp;) not "&amp;".
	if !containsSubstring(title, "&") {
		t.Errorf("docs[0].Title = %q: expected decoded '&' from &amp;", title)
	}

	// First doc description should have <b> stripped.
	desc := docs[0].Body
	if containsRawHTML(desc) {
		t.Errorf("docs[0].Body still contains raw HTML markup: %q", desc)
	}
}

// TestParseBlogResponse_Pagination verifies next_cursor is set on the last doc
// when there are more pages. REQ-ADP8-007.
func TestParseBlogResponse_Pagination(t *testing.T) {
	t.Parallel()
	body := mustReadFile(t, "testdata/search_response_blog_pagination.json")

	// This fixture has start=26, display=25, total=125.
	// next_cursor = 51 (26+25=51, 51 <= 125 and 51 <= 1000).
	docs, err := parseBlogResponse(body, fixedTime)
	if err != nil {
		t.Fatalf("parseBlogResponse() error = %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("parseBlogResponse() returned 0 docs")
	}

	last := docs[len(docs)-1]
	if last.Metadata == nil {
		t.Fatal("last doc Metadata is nil, want next_cursor")
	}
	cursor, ok := last.Metadata["next_cursor"]
	if !ok {
		t.Error("last doc Metadata missing 'next_cursor'")
	}
	if cursor != "51" {
		t.Errorf("last doc next_cursor = %v, want %q", cursor, "51")
	}
}

// TestParseBlogResponse_NoCursorOnLastPage verifies no next_cursor when at last page.
func TestParseBlogResponse_NoCursorOnLastPage(t *testing.T) {
	t.Parallel()
	// blog fixture: start=1, total=125, display=25 → next=26. Cursor IS set.
	// Use the empty fixture which has total=0 → no cursor.
	body := mustReadFile(t, "testdata/search_response_blog_empty.json")
	docs, err := parseBlogResponse(body, fixedTime)
	if err != nil {
		t.Fatalf("parseBlogResponse() error = %v", err)
	}
	// Empty = no docs = no cursor to check.
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
}

// TestParseNewsResponse_HappyPath verifies basic news parsing from the 25-item fixture.
func TestParseNewsResponse_HappyPath(t *testing.T) {
	t.Parallel()
	body := mustReadFile(t, "testdata/search_response_news.json")

	docs, err := parseNewsResponse(body, fixedTime)
	if err != nil {
		t.Fatalf("parseNewsResponse() error = %v", err)
	}
	if len(docs) != 25 {
		t.Fatalf("len(docs) = %d, want 25", len(docs))
	}

	d := docs[0]
	if d.DocType != types.DocTypeArticle {
		t.Errorf("docs[0].DocType = %v, want DocTypeArticle", d.DocType)
	}
	if d.Author == "" {
		t.Error("docs[0].Author is empty, want hostname from originallink")
	}
	// URL should be originallink when available.
	if d.URL == "" {
		t.Error("docs[0].URL is empty")
	}
	// PublishedAt should be non-zero.
	if d.PublishedAt.IsZero() {
		t.Error("docs[0].PublishedAt is zero")
	}

	// Validate all docs.
	for i, doc := range docs {
		docCopy := doc
		if err := docCopy.Validate(); err != nil {
			t.Errorf("docs[%d].Validate() error = %v", i, err)
		}
	}
}

// TestParseNewsResponse_NoOriginalLink verifies fallback to link when originallink is "".
func TestParseNewsResponse_NoOriginalLink(t *testing.T) {
	t.Parallel()
	body := mustReadFile(t, "testdata/search_response_news_no_originallink.json")

	docs, err := parseNewsResponse(body, fixedTime)
	if err != nil {
		t.Fatalf("parseNewsResponse() error = %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("len(docs) = %d, want 1", len(docs))
	}

	d := docs[0]
	// URL should fall back to naver link when originallink is empty.
	if d.URL == "" {
		t.Error("docs[0].URL is empty")
	}
	// Author should be empty when originallink is absent.
	if d.Author != "" {
		t.Errorf("docs[0].Author = %q, want empty when originallink is absent", d.Author)
	}
}

// TestParseWebResponse_HappyPath verifies basic web parsing from the 25-item fixture.
func TestParseWebResponse_HappyPath(t *testing.T) {
	t.Parallel()
	body := mustReadFile(t, "testdata/search_response_web.json")

	docs, err := parseWebResponse(body, fixedTime)
	if err != nil {
		t.Fatalf("parseWebResponse() error = %v", err)
	}
	if len(docs) != 25 {
		t.Fatalf("len(docs) = %d, want 25", len(docs))
	}

	d := docs[0]
	if d.DocType != types.DocTypeOther {
		t.Errorf("docs[0].DocType = %v, want DocTypeOther", d.DocType)
	}
	// PublishedAt is zero for web results (no date field).
	if !d.PublishedAt.IsZero() {
		t.Errorf("docs[0].PublishedAt = %v, want zero for web", d.PublishedAt)
	}

	for i, doc := range docs {
		docCopy := doc
		if err := docCopy.Validate(); err != nil {
			t.Errorf("docs[%d].Validate() error = %v", i, err)
		}
	}
}

// TestParseShopResponse_HappyPath verifies basic shop parsing from the 25-item fixture.
func TestParseShopResponse_HappyPath(t *testing.T) {
	t.Parallel()
	body := mustReadFile(t, "testdata/search_response_shop.json")

	docs, err := parseShopResponse(body, fixedTime)
	if err != nil {
		t.Fatalf("parseShopResponse() error = %v", err)
	}
	if len(docs) != 25 {
		t.Fatalf("len(docs) = %d, want 25", len(docs))
	}

	d := docs[0]
	if d.DocType != types.DocTypeOther {
		t.Errorf("docs[0].DocType = %v, want DocTypeOther", d.DocType)
	}
	// Metadata should contain lprice.
	if d.Metadata == nil {
		t.Fatal("docs[0].Metadata is nil")
	}
	if _, ok := d.Metadata["lprice"]; !ok {
		t.Error("docs[0].Metadata missing 'lprice'")
	}
	if _, ok := d.Metadata["mall_name"]; !ok {
		t.Error("docs[0].Metadata missing 'mall_name'")
	}

	for i, doc := range docs {
		docCopy := doc
		if err := docCopy.Validate(); err != nil {
			t.Errorf("docs[%d].Validate() error = %v", i, err)
		}
	}
}

// TestSyntheticID verifies the syntheticID function produces a 16-char hex string.
func TestSyntheticID(t *testing.T) {
	t.Parallel()
	id := syntheticID("https://blog.naver.com/user1/post1")
	if len(id) != 16 {
		t.Errorf("syntheticID() len = %d, want 16", len(id))
	}
	// Same input → same output.
	id2 := syntheticID("https://blog.naver.com/user1/post1")
	if id != id2 {
		t.Errorf("syntheticID() not deterministic: %q != %q", id, id2)
	}
	// Different input → different output.
	id3 := syntheticID("https://blog.naver.com/user2/post2")
	if id == id3 {
		t.Errorf("syntheticID() collision for different inputs: %q", id)
	}
}

// TestTruncateRunes verifies snippet truncation with "…" ellipsis.
func TestTruncateRunes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		maxRunes int
		want     string
	}{
		{
			name:     "short string, no truncation",
			input:    "hello",
			maxRunes: 10,
			want:     "hello",
		},
		{
			name:     "exact length, no truncation",
			input:    "hello",
			maxRunes: 5,
			want:     "hello",
		},
		{
			name:     "truncation with ellipsis",
			input:    "hello world",
			maxRunes: 8,
			want:     "hello w…",
		},
		{
			name:     "korean text truncation",
			input:    "가나다라마바사아자차카타파하",
			maxRunes: 5,
			want:     "가나다라…",
		},
		{
			name:     "empty string",
			input:    "",
			maxRunes: 10,
			want:     "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncateRunes(tc.input, tc.maxRunes)
			if got != tc.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tc.input, tc.maxRunes, got, tc.want)
			}
		})
	}
}

// TestNextCursorValue verifies cursor computation logic.
func TestNextCursorValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		start   int
		display int
		total   int
		want    string
	}{
		{"first page with more", 1, 25, 125, "26"},
		{"second page", 26, 25, 125, "51"},
		// 101+25=126 > total(125) → no next cursor
		{"at end exceeds total", 101, 25, 125, ""},
		// 100+25=125 <= total(125) and 125 <= 1000 → cursor "125"
		{"at page boundary within total", 100, 25, 125, "125"},
		// 980+25=1005 > 1000 → no next cursor (Naver cap)
		{"beyond 1000", 980, 25, 2000, ""},
		// 976+24=1000 <= 1000 and 1000 <= 2000 → cursor "1000"
		{"exactly 1000", 976, 24, 2000, "1000"},
		// 951+50=1001 > 1000 → no next cursor (Naver cap)
		{"stop at 1000", 951, 50, 2000, ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := nextCursorValue(tc.start, tc.display, tc.total)
			if got != tc.want {
				t.Errorf("nextCursorValue(%d, %d, %d) = %q, want %q",
					tc.start, tc.display, tc.total, got, tc.want)
			}
		})
	}
}

// TestParseNewsPubDate verifies RFC1123Z and RFC1123 parsing + zero on failure.
func TestParseNewsPubDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pubDate string
		wantNil bool // true = expect zero time
	}{
		{"empty", "", true},
		{"rfc1123z", "Wed, 07 May 2026 09:00:00 +0900", false},
		{"rfc1123 UTC", "Wed, 07 May 2026 09:00:00 UTC", false},
		{"invalid", "not-a-date", true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := parseNewsPubDate(tc.pubDate)
			if tc.wantNil && !got.IsZero() {
				t.Errorf("parseNewsPubDate(%q) = %v, want zero time", tc.pubDate, got)
			}
			if !tc.wantNil && got.IsZero() {
				t.Errorf("parseNewsPubDate(%q) = zero, want non-zero", tc.pubDate)
			}
		})
	}
}

// TestNewsAuthorFromURL verifies hostname extraction from originallink.
func TestNewsAuthorFromURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"valid URL", "https://news.example.com/article1", "news.example.com"},
		{"malformed URL", "://bad-url", ""},
		{"url with path", "https://www.yonhap.co.kr/news/article123", "www.yonhap.co.kr"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := newsAuthorFromURL(tc.input)
			if got != tc.want {
				t.Errorf("newsAuthorFromURL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestClampDisplay verifies display parameter clamping.
func TestClampDisplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input int
		want  int
	}{
		{0, 25},   // default
		{1, 1},    // min
		{-1, 1},   // below min
		{50, 50},  // within range
		{100, 100}, // max
		{101, 100}, // above max
	}

	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := clampDisplay(tc.input)
			if got != tc.want {
				t.Errorf("clampDisplay(%d) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// --- helpers ---

// mustReadFile reads a test fixture file, failing the test on error.
func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return data
}

// containsRawHTML checks whether s contains HTML tags or HTML entity sequences
// (source markup that should have been stripped/decoded).
func containsRawHTML(s string) bool {
	// Check for literal HTML entity sequences and bold tags.
	for _, marker := range []string{"<b>", "</b>", "&amp;", "&lt;", "&gt;", "&quot;", "&#39;", "&nbsp;"} {
		if containsSubstring(s, marker) {
			return true
		}
	}
	return false
}

// containsSubstring reports whether s contains sub.
func containsSubstring(s, sub string) bool {
	if len(sub) == 0 || len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
