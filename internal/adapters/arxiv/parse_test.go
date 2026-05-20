package arxiv

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

var testRetrievedAt = time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

// TestParseFeedFieldMapping verifies field mapping for a DOI-bearing entry.
func TestParseFeedFieldMapping(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_with_doi.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}
	if len(docs) != 1 {
		t.Fatalf("parseFeed() returned %d docs, want 1", len(docs))
	}
	doc := docs[0]

	// ID: stripped bare arXiv ID.
	if doc.ID != "2402.00001v2" {
		t.Errorf("ID = %q, want %q", doc.ID, "2402.00001v2")
	}
	if doc.SourceID != "arxiv" {
		t.Errorf("SourceID = %q, want %q", doc.SourceID, "arxiv")
	}
	// URL: full ID URL.
	if doc.URL != "http://arxiv.org/abs/2402.00001v2" {
		t.Errorf("URL = %q, want %q", doc.URL, "http://arxiv.org/abs/2402.00001v2")
	}
	if doc.Title != "A Paper With a DOI Link" {
		t.Errorf("Title = %q, want %q", doc.Title, "A Paper With a DOI Link")
	}
	if doc.Body == "" {
		t.Error("Body empty, want non-empty")
	}
	if doc.Snippet == "" {
		t.Error("Snippet empty, want non-empty")
	}
	wantPublishedAt := time.Date(2024, 2, 15, 9, 0, 0, 0, time.UTC)
	if !doc.PublishedAt.Equal(wantPublishedAt) {
		t.Errorf("PublishedAt = %v, want %v", doc.PublishedAt, wantPublishedAt)
	}
	if doc.RetrievedAt != testRetrievedAt {
		t.Errorf("RetrievedAt = %v, want %v", doc.RetrievedAt, testRetrievedAt)
	}
	if doc.Author != "John Doi" {
		t.Errorf("Author = %q, want %q", doc.Author, "John Doi")
	}
	if doc.Score != 0.5 {
		t.Errorf("Score = %v, want 0.5", doc.Score)
	}
	if doc.Lang != "" {
		t.Errorf("Lang = %q, want empty", doc.Lang)
	}
	if doc.DocType != types.DocTypePaper {
		t.Errorf("DocType = %v, want %v", doc.DocType, types.DocTypePaper)
	}
	if doc.Citations != nil {
		t.Errorf("Citations = %v, want nil", doc.Citations)
	}
	if doc.Hash != "" {
		t.Errorf("Hash = %q, want empty", doc.Hash)
	}

	// Validate() must pass.
	if err := doc.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

// TestParseFeedIDStripPrefix verifies the "http://arxiv.org/abs/" prefix is stripped.
func TestParseFeedIDStripPrefix(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_no_doi.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}
	if len(docs) != 1 {
		t.Fatalf("parseFeed() returned %d docs, want 1", len(docs))
	}

	if docs[0].ID != "2402.00002v1" {
		t.Errorf("ID = %q, want %q", docs[0].ID, "2402.00002v1")
	}
}

// TestParseFeedMultiVersionID verifies that a v15 suffix is preserved in the bare ID.
func TestParseFeedMultiVersionID(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_multi_version.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}
	if len(docs) != 1 {
		t.Fatalf("parseFeed() returned %d docs, want 1", len(docs))
	}

	if docs[0].ID != "1706.03762v15" {
		t.Errorf("ID = %q, want %q", docs[0].ID, "1706.03762v15")
	}
}

// TestParseFeedWhitespaceCollapse verifies title/summary whitespace is collapsed.
func TestParseFeedWhitespaceCollapse(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_latex_title.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}
	if len(docs) != 1 {
		t.Fatalf("parseFeed() returned %d docs, want 1", len(docs))
	}

	doc := docs[0]
	// Title should not contain leading/trailing whitespace or multiple consecutive spaces.
	if len(doc.Title) == 0 {
		t.Fatal("Title empty")
	}
	if doc.Title[0] == ' ' || doc.Title[len(doc.Title)-1] == ' ' {
		t.Errorf("Title has leading/trailing space: %q", doc.Title)
	}
	for i := 0; i < len(doc.Title)-1; i++ {
		if doc.Title[i] == ' ' && doc.Title[i+1] == ' ' {
			t.Errorf("Title has consecutive spaces at index %d: %q", i, doc.Title)
			break
		}
	}
	// Body (summary) similarly collapsed.
	if len(doc.Body) > 0 {
		if doc.Body[0] == ' ' || doc.Body[len(doc.Body)-1] == ' ' {
			t.Errorf("Body has leading/trailing space: %q", doc.Body)
		}
	}
}

// TestCollapseWSTable verifies collapseWS helper directly.
func TestCollapseWSTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"  hello   world  ", "hello world"},
		{"\n\t multiple \n\n spaces\t\t", "multiple spaces"},
		{"no change", "no change"},
		{"", ""},
		{"   ", ""},
		{"a  b  c", "a b c"},
	}

	for _, tc := range tests {
		got := collapseWS(tc.input)
		if got != tc.want {
			t.Errorf("collapseWS(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestParseFeedScoreConstant verifies every doc has Score == 0.5.
func TestParseFeedScoreConstant(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}

	for i, doc := range docs {
		if doc.Score != 0.5 {
			t.Errorf("docs[%d].Score = %v, want 0.5", i, doc.Score)
		}
	}
}

// TestParseFeedDOIInArxivNamespace verifies <arxiv:doi> is parsed into Metadata["doi"].
func TestParseFeedDOIInArxivNamespace(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_with_doi.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}
	if len(docs) != 1 {
		t.Fatalf("parseFeed() returned %d docs, want 1", len(docs))
	}

	got, ok := docs[0].Metadata["doi"]
	if !ok {
		t.Fatal("Metadata missing key \"doi\"")
	}
	if got != "10.1234/example.2024.001" {
		t.Errorf("Metadata[\"doi\"] = %q, want %q", got, "10.1234/example.2024.001")
	}
}

// TestParseFeedNoDOIOmitsKey verifies absence of <arxiv:doi> means no "doi" key in Metadata.
func TestParseFeedNoDOIOmitsKey(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_no_doi.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}
	if len(docs) != 1 {
		t.Fatalf("parseFeed() returned %d docs, want 1", len(docs))
	}

	if _, ok := docs[0].Metadata["doi"]; ok {
		t.Error("Metadata contains \"doi\" key, want absent for entry without DOI")
	}
}

// TestParseFeedAuthorsList verifies multi-author entries join all names.
func TestParseFeedAuthorsList(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_multi_author.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}
	if len(docs) != 1 {
		t.Fatalf("parseFeed() returned %d docs, want 1", len(docs))
	}

	doc := docs[0]
	if doc.Author != "First Author" {
		t.Errorf("Author = %q, want %q (first author only)", doc.Author, "First Author")
	}

	authorsRaw, ok := doc.Metadata["authors"]
	if !ok {
		t.Fatal("Metadata missing key \"authors\"")
	}
	authors, ok := authorsRaw.([]string)
	if !ok {
		t.Fatalf("Metadata[\"authors\"] type = %T, want []string", authorsRaw)
	}
	if len(authors) != 5 {
		t.Errorf("len(authors) = %d, want 5", len(authors))
	}
	wantAuthors := []string{"First Author", "Second Author", "Third Author", "Fourth Author", "Fifth Author"}
	for i, want := range wantAuthors {
		if i >= len(authors) {
			break
		}
		if authors[i] != want {
			t.Errorf("authors[%d] = %q, want %q", i, authors[i], want)
		}
	}
}

// TestParseFeedPaginationCursor verifies next_cursor on last doc when more results exist.
func TestParseFeedPaginationCursor(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_pagination.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	// start=0, totalResults=100, 5 entries → cursor = "5"
	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}
	if len(docs) == 0 {
		t.Fatal("parseFeed() returned 0 docs")
	}

	last := docs[len(docs)-1]
	got, ok := last.Metadata["next_cursor"]
	if !ok {
		t.Fatal("last doc Metadata missing \"next_cursor\"")
	}
	if got != "5" {
		t.Errorf("Metadata[\"next_cursor\"] = %v, want %q", got, "5")
	}

	// Earlier docs must NOT have next_cursor.
	for i, doc := range docs[:len(docs)-1] {
		if _, ok := doc.Metadata["next_cursor"]; ok {
			t.Errorf("docs[%d] has next_cursor, want absent", i)
		}
	}
}

// TestParseFeedNoCursorOnLastPage verifies no next_cursor when start+len==totalResults.
func TestParseFeedNoCursorOnLastPage(t *testing.T) {
	t.Parallel()

	// Use overshoot fixture: start=1000, totalResults=50, 0 entries.
	body, err := os.ReadFile("testdata/search_response_overshoot.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}
	// Zero entries → no cursor.
	for i, doc := range docs {
		if _, ok := doc.Metadata["next_cursor"]; ok {
			t.Errorf("docs[%d] has next_cursor, want absent", i)
		}
	}
}

// TestParseFeedHashEmpty verifies Hash is always "".
func TestParseFeedHashEmpty(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}

	for i, doc := range docs {
		if doc.Hash != "" {
			t.Errorf("docs[%d].Hash = %q, want empty", i, doc.Hash)
		}
	}
}

// TestParseFeedMetadataKeys verifies all 6 required metadata keys are present.
func TestParseFeedMetadataKeys(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}

	requiredKeys := []string{"arxiv_id", "authors", "primary_category", "categories", "published_at", "updated_at"}
	for i, doc := range docs {
		for _, key := range requiredKeys {
			if _, ok := doc.Metadata[key]; !ok {
				t.Errorf("docs[%d].Metadata missing required key %q", i, key)
			}
		}
	}
}

// TestParseFeedMalformedXML verifies truncated XML returns a SourceError with CategoryPermanent.
func TestParseFeedMalformedXML(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response_malformed.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	_, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr == nil {
		t.Fatal("parseFeed() expected error for malformed XML, got nil")
	}

	var se *types.SourceError
	if !errors.As(parseErr, &se) {
		t.Fatalf("parseErr is not *types.SourceError: %T = %v", parseErr, parseErr)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("SourceError.Category = %v, want CategoryPermanent", se.Category)
	}
}

// TestTruncateRunesNoTruncationNeeded verifies strings <= maxRunes are returned unchanged.
func TestTruncateRunesNoTruncationNeeded(t *testing.T) {
	t.Parallel()

	short := "hello"
	got := truncateRunes(short, 10)
	if got != short {
		t.Errorf("truncateRunes(%q, 10) = %q, want %q", short, got, short)
	}
}

// TestTruncateRunesUnicode verifies truncation is rune-aware for multi-byte characters.
func TestTruncateRunesUnicode(t *testing.T) {
	t.Parallel()

	s := "한한한한한한한한한한" // 10 Korean runes, 30 bytes
	got := truncateRunes(s, 5)
	if got == s {
		t.Fatal("truncateRunes() did not truncate")
	}
	wantRunes := 5
	if gotCount := len([]rune(got)); gotCount != wantRunes {
		t.Errorf("truncateRunes() rune count = %d, want %d; got = %q", gotCount, wantRunes, got)
	}
	if got[len(got)-3:] != "..." {
		t.Errorf("truncateRunes() does not end with '...': %q", got)
	}
}

// TestSnippetFallsBackToTitle verifies empty body falls back to title for snippet.
func TestSnippetFallsBackToTitle(t *testing.T) {
	t.Parallel()

	// Build a feed with empty summary — snippet should fall back to title.
	xmlBody := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"
      xmlns:opensearch="http://a9.com/-/spec/opensearch/1.1/"
      xmlns:arxiv="http://arxiv.org/schemas/atom">
  <opensearch:totalResults>1</opensearch:totalResults>
  <opensearch:startIndex>0</opensearch:startIndex>
  <opensearch:itemsPerPage>1</opensearch:itemsPerPage>
  <entry>
    <id>http://arxiv.org/abs/2404.99001v1</id>
    <updated>2024-04-01T00:00:00Z</updated>
    <published>2024-04-01T00:00:00Z</published>
    <title>Fallback Title for Snippet</title>
    <summary></summary>
    <author><name>Test Author</name></author>
    <link href="http://arxiv.org/abs/2404.99001v1" rel="alternate" type="text/html"/>
    <arxiv:primary_category xmlns:arxiv="http://arxiv.org/schemas/atom" term="cs.AI" scheme="http://arxiv.org/schemas/atom"/>
    <category term="cs.AI" scheme="http://arxiv.org/schemas/atom"/>
  </entry>
</feed>`)

	docs, parseErr := parseFeed(xmlBody, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}
	if len(docs) != 1 {
		t.Fatalf("parseFeed() returned %d docs, want 1", len(docs))
	}

	if docs[0].Snippet != "Fallback Title for Snippet" {
		t.Errorf("Snippet = %q, want title fallback %q", docs[0].Snippet, "Fallback Title for Snippet")
	}
}

// TestParseFeedTotalResultsOnFirstDoc verifies total_results metadata on first doc only.
func TestParseFeedTotalResultsOnFirstDoc(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("testdata/search_response.xml")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseFeed(body, testRetrievedAt)
	if parseErr != nil {
		t.Fatalf("parseFeed() error = %v", parseErr)
	}
	if len(docs) < 2 {
		t.Fatalf("need at least 2 docs, got %d", len(docs))
	}

	// First doc should have total_results.
	if _, ok := docs[0].Metadata["total_results"]; !ok {
		t.Error("docs[0].Metadata missing \"total_results\"")
	}

	// Subsequent docs should NOT have total_results.
	for i := 1; i < len(docs); i++ {
		if _, ok := docs[i].Metadata["total_results"]; ok {
			t.Errorf("docs[%d].Metadata has \"total_results\", want absent", i)
		}
	}
}
