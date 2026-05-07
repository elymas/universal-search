package searxng_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// loadTestdata reads a testdata file by name. Fatal on read error.
func loadTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("loadTestdata(%q): %v", name, err)
	}
	return b
}

// TestParseSearchFieldMapping verifies that all NormalizedDoc fields are
// populated correctly from the happy-path fixture.
func TestParseSearchFieldMapping(t *testing.T) {
	t.Parallel()
	body := loadTestdata(t, "search_response.json")
	now := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)

	docs, nextCursor, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("parseSearch returned empty slice for happy-path fixture")
	}

	doc := docs[0]

	// ID must be deterministic "searxng:" prefix.
	if len(doc.ID) == 0 || doc.ID[:8] != "searxng:" {
		t.Errorf("ID = %q, want prefix %q", doc.ID, "searxng:")
	}
	if doc.SourceID != "searxng" {
		t.Errorf("SourceID = %q, want %q", doc.SourceID, "searxng")
	}
	if doc.URL == "" {
		t.Error("URL is empty")
	}
	if doc.Title == "" {
		t.Error("Title is empty")
	}
	// Body must match the raw content field.
	if doc.Body == "" {
		t.Error("Body is empty")
	}
	// RetrievedAt must match the injected now.
	if !doc.RetrievedAt.Equal(now) {
		t.Errorf("RetrievedAt = %v, want %v", doc.RetrievedAt, now)
	}
	// DocType must be Article (H1 audit).
	if doc.DocType != types.DocTypeArticle {
		t.Errorf("DocType = %v, want DocTypeArticle", doc.DocType)
	}
	// Hash must be empty (consumers compute via CanonicalHash).
	if doc.Hash != "" {
		t.Errorf("Hash = %q, want empty", doc.Hash)
	}
	// Author must be empty (SearXNG does not surface per-result authorship).
	if doc.Author != "" {
		t.Errorf("Author = %q, want empty", doc.Author)
	}
	// Score must be in [0.0, 1.0].
	if doc.Score < 0.0 || doc.Score > 1.0 {
		t.Errorf("Score = %v, want [0.0, 1.0]", doc.Score)
	}
	// nextCursor must be non-empty.
	if nextCursor == "" {
		t.Error("nextCursor is empty, want non-empty for non-empty results")
	}
}

// TestParseSearchEmptyResultsReturnsNilNoError verifies (nil, "", nil) on zero results.
func TestParseSearchEmptyResultsReturnsNilNoError(t *testing.T) {
	t.Parallel()
	body := loadTestdata(t, "search_response_empty.json")
	now := time.Now().UTC()

	docs, nextCursor, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	if docs != nil {
		t.Errorf("docs = %v, want nil", docs)
	}
	if nextCursor != "" {
		t.Errorf("nextCursor = %q, want empty", nextCursor)
	}
}

// TestParseSearchMalformedJSON verifies SourceError{Permanent} on malformed JSON.
func TestParseSearchMalformedJSON(t *testing.T) {
	t.Parallel()
	body := loadTestdata(t, "search_response_malformed.json")
	now := time.Now().UTC()

	_, _, err := exportedParseSearch(body, now, 1)
	if err == nil {
		t.Fatal("parseSearch returned nil error for malformed JSON, want error")
	}
	var se *types.SourceError
	if !asSourceError(err, &se) {
		t.Fatalf("error type = %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want CategoryPermanent", se.Category)
	}
}

// TestParseSearchPublishedDateParsed verifies RFC3339 publishedDate is parsed.
func TestParseSearchPublishedDateParsed(t *testing.T) {
	t.Parallel()
	body := loadTestdata(t, "search_response.json")
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}

	// At least one doc in happy-path fixture should have a non-zero PublishedAt.
	found := false
	for _, d := range docs {
		if !d.PublishedAt.IsZero() {
			found = true
			break
		}
	}
	if !found {
		t.Error("no doc has non-zero PublishedAt; want at least one parsed publishedDate")
	}
}

// TestParseSearchPublishedDateMissing verifies PublishedAt is zero when absent.
func TestParseSearchPublishedDateMissing(t *testing.T) {
	t.Parallel()
	body := loadTestdata(t, "search_response_no_published_date.json")
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	for i, d := range docs {
		if !d.PublishedAt.IsZero() {
			t.Errorf("docs[%d].PublishedAt = %v, want zero (no publishedDate in fixture)", i, d.PublishedAt)
		}
	}
}

// TestParseSearchPublishedDateMalformed verifies PublishedAt is zero on parse failure.
func TestParseSearchPublishedDateMalformed(t *testing.T) {
	t.Parallel()
	body := []byte(`{"query":"test","results":[{"url":"https://example.com","title":"T","content":"C","publishedDate":"not-a-date"}]}`)
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected 1 doc")
	}
	if !docs[0].PublishedAt.IsZero() {
		t.Errorf("PublishedAt = %v, want zero for malformed date", docs[0].PublishedAt)
	}
}

// TestParseSearchSnippetTruncated280Runes verifies content >280 runes is truncated.
func TestParseSearchSnippetTruncated280Runes(t *testing.T) {
	t.Parallel()
	// Generate a 300-rune content string.
	content := ""
	for i := 0; i < 300; i++ {
		content += "x"
	}
	body := []byte(`{"query":"test","results":[{"url":"https://example.com","title":"T","content":"` + content + `"}]}`)
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected 1 doc")
	}
	runeLen := len([]rune(docs[0].Snippet))
	if runeLen > 280 {
		t.Errorf("Snippet rune length = %d, want <= 280", runeLen)
	}
	if runeLen == 0 {
		t.Error("Snippet is empty after truncation")
	}
}

// TestParseSearchSnippetNotTruncatedWhenShort verifies short content is unchanged.
func TestParseSearchSnippetNotTruncatedWhenShort(t *testing.T) {
	t.Parallel()
	short := "Short content"
	body := []byte(`{"query":"test","results":[{"url":"https://example.com","title":"T","content":"` + short + `"}]}`)
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	if docs[0].Snippet != short {
		t.Errorf("Snippet = %q, want %q (short content must not be modified)", docs[0].Snippet, short)
	}
}

// TestParseSearchPaginationCursor verifies nextCursor = strconv.Itoa(currentPage+1).
func TestParseSearchPaginationCursor(t *testing.T) {
	t.Parallel()
	body := loadTestdata(t, "search_response_pagination.json")
	now := time.Now().UTC()

	tests := []struct {
		page       int
		wantCursor string
	}{
		{1, "2"},
		{3, "4"},
		{10, "11"},
	}
	for _, tc := range tests {
		docs, cursor, err := exportedParseSearch(body, now, tc.page)
		if err != nil {
			t.Fatalf("page=%d: parseSearch: %v", tc.page, err)
		}
		if len(docs) == 0 {
			t.Fatalf("page=%d: no docs", tc.page)
		}
		if cursor != tc.wantCursor {
			t.Errorf("page=%d: nextCursor = %q, want %q", tc.page, cursor, tc.wantCursor)
		}
		// next_cursor must be set on the last doc.
		last := docs[len(docs)-1]
		if last.Metadata["next_cursor"] != tc.wantCursor {
			t.Errorf("page=%d: last.Metadata[next_cursor] = %v, want %q", tc.page, last.Metadata["next_cursor"], tc.wantCursor)
		}
	}
}

// TestParseSearchHashEmpty verifies Hash is always empty string (consumers compute via CanonicalHash).
func TestParseSearchHashEmpty(t *testing.T) {
	t.Parallel()
	body := loadTestdata(t, "search_response.json")
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	for i, d := range docs {
		if d.Hash != "" {
			t.Errorf("docs[%d].Hash = %q, want empty", i, d.Hash)
		}
	}
}

// TestParseSearchMetadataKeys verifies required metadata keys are present.
func TestParseSearchMetadataKeys(t *testing.T) {
	t.Parallel()
	body := loadTestdata(t, "search_response.json")
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("no docs")
	}
	doc := docs[0]

	required := []string{"engine", "engines", "category", "score_raw"}
	for _, k := range required {
		if _, ok := doc.Metadata[k]; !ok {
			t.Errorf("Metadata missing key %q", k)
		}
	}
}

// TestParseSearchSingleEngineMetadata verifies engine-of-origin metadata for
// single-engine results.
func TestParseSearchSingleEngineMetadata(t *testing.T) {
	t.Parallel()
	body := []byte(`{"query":"test","results":[{"url":"https://example.com","title":"T","content":"C","engine":"google","engines":["google"],"score":0.8}]}`)
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	doc := docs[0]
	if doc.Metadata["engine"] != "google" {
		t.Errorf("engine = %v, want %q", doc.Metadata["engine"], "google")
	}
	engines, ok := doc.Metadata["engines"].([]string)
	if !ok {
		t.Fatalf("engines type = %T, want []string", doc.Metadata["engines"])
	}
	if len(engines) != 1 || engines[0] != "google" {
		t.Errorf("engines = %v, want [google]", engines)
	}
}

// TestParseSearchMultiEngineMetadata verifies engine-of-origin metadata for
// multi-engine aggregated results.
func TestParseSearchMultiEngineMetadata(t *testing.T) {
	t.Parallel()
	body := loadTestdata(t, "search_response_multi_engine.json")
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	for i, d := range docs {
		engines, ok := d.Metadata["engines"].([]string)
		if !ok {
			t.Errorf("docs[%d]: engines type = %T, want []string", i, d.Metadata["engines"])
			continue
		}
		if len(engines) == 0 {
			t.Errorf("docs[%d]: engines is empty", i)
		}
	}
}

// TestParseSearchEnginesFallback verifies M4 fix: when engines is empty/null,
// single-element list [engine] is used as fallback.
func TestParseSearchEnginesFallback(t *testing.T) {
	t.Parallel()
	// engines field is missing; engine field is present.
	body := []byte(`{"query":"test","results":[{"url":"https://example.com","title":"T","content":"C","engine":"bing"}]}`)
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	doc := docs[0]

	engines, ok := doc.Metadata["engines"].([]string)
	if !ok {
		t.Fatalf("engines type = %T, want []string", doc.Metadata["engines"])
	}
	if len(engines) != 1 || engines[0] != "bing" {
		t.Errorf("engines = %v, want [bing] (M4 fallback)", engines)
	}
}

// TestParseSearchEnginesFallbackBothEmpty verifies M4 fallback: when both engine
// and engines are empty, engines is set to empty slice (not nil panic).
func TestParseSearchEnginesFallbackBothEmpty(t *testing.T) {
	t.Parallel()
	body := []byte(`{"query":"test","results":[{"url":"https://example.com","title":"T","content":"C"}]}`)
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	doc := docs[0]

	engines, ok := doc.Metadata["engines"].([]string)
	if !ok {
		t.Fatalf("engines type = %T, want []string", doc.Metadata["engines"])
	}
	if engines == nil {
		t.Error("engines is nil, want empty slice")
	}
}

// TestParseSearchIDDeterminism verifies M1: same URL always produces same ID.
func TestParseSearchIDDeterminism(t *testing.T) {
	t.Parallel()
	const testURL = "https://example.com/determinism"
	body := []byte(`{"query":"test","results":[{"url":"` + testURL + `","title":"T","content":"C"}]}`)
	now := time.Now().UTC()

	docs1, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	docs2, _, err := exportedParseSearch(body, now.Add(time.Hour), 2)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if docs1[0].ID != docs2[0].ID {
		t.Errorf("ID not deterministic: %q vs %q", docs1[0].ID, docs2[0].ID)
	}
}

// TestParseSearchScoreClamp verifies scores are clamped to [0.0, 1.0].
func TestParseSearchScoreClamp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rawScore float64
		wantMin  float64
		wantMax  float64
	}{
		{0.5, 0.0, 1.0},
		{1.5, 1.0, 1.0},  // clamped to 1.0
		{-0.1, 0.0, 0.0}, // clamped to 0.0
		{0.0, 0.0, 0.0},
		{1.0, 1.0, 1.0},
	}
	for _, tc := range tests {
		body := jsonWithScore(tc.rawScore)
		docs, _, err := exportedParseSearch(body, time.Now().UTC(), 1)
		if err != nil {
			t.Fatalf("score=%v: parseSearch: %v", tc.rawScore, err)
		}
		got := docs[0].Score
		if got < tc.wantMin || got > tc.wantMax {
			t.Errorf("rawScore=%v: Score=%v, want [%v, %v]", tc.rawScore, got, tc.wantMin, tc.wantMax)
		}
	}
}

// TestParseSearchScoreRawInMetadata verifies raw score before clamping is in metadata.
func TestParseSearchScoreRawInMetadata(t *testing.T) {
	t.Parallel()
	const rawScore = 1.5
	body := jsonWithScore(rawScore)
	docs, _, err := exportedParseSearch(body, time.Now().UTC(), 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	got, ok := docs[0].Metadata["score_raw"].(float64)
	if !ok {
		t.Fatalf("score_raw type = %T, want float64", docs[0].Metadata["score_raw"])
	}
	if got != rawScore {
		t.Errorf("score_raw = %v, want %v", got, rawScore)
	}
}

// TestParseSearchNextCursorOnlyOnLastDoc verifies next_cursor set on last doc only.
func TestParseSearchNextCursorOnlyOnLastDoc(t *testing.T) {
	t.Parallel()
	body := loadTestdata(t, "search_response_pagination.json")
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	if len(docs) < 2 {
		t.Skip("need at least 2 docs for this test")
	}
	// All docs except the last must not have next_cursor.
	for i, d := range docs[:len(docs)-1] {
		if _, ok := d.Metadata["next_cursor"]; ok {
			t.Errorf("docs[%d] has next_cursor, want only last doc to have it", i)
		}
	}
	// Last doc must have next_cursor.
	last := docs[len(docs)-1]
	if _, ok := last.Metadata["next_cursor"]; !ok {
		t.Error("last doc missing next_cursor")
	}
}

// TestParseSearchLangEmpty verifies Lang is always empty (SearXNG doesn't expose per-result lang).
func TestParseSearchLangEmpty(t *testing.T) {
	t.Parallel()
	body := loadTestdata(t, "search_response.json")
	now := time.Now().UTC()

	docs, _, err := exportedParseSearch(body, now, 1)
	if err != nil {
		t.Fatalf("parseSearch: %v", err)
	}
	for i, d := range docs {
		if d.Lang != "" {
			t.Errorf("docs[%d].Lang = %q, want empty", i, d.Lang)
		}
	}
}

// jsonWithScore returns a minimal JSON body with a single result at given score.
func jsonWithScore(score float64) []byte {
	type result struct {
		URL   string  `json:"url"`
		Title string  `json:"title"`
		Score float64 `json:"score"`
	}
	type resp struct {
		Query   string   `json:"query"`
		Results []result `json:"results"`
	}
	b, _ := json.Marshal(resp{
		Query: "test",
		Results: []result{
			{URL: "https://example.com", Title: "T", Score: score},
		},
	})
	return b
}
