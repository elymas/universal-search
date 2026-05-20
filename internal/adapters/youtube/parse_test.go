package youtube

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/elymas/universal-search/pkg/types"
)

var fixedTime = time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return data
}

func TestParseSearchResponseFieldMapping(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "search_response.json")
	docs, _, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected docs, got none")
	}
	for i, doc := range docs {
		if doc.ID == "" {
			t.Errorf("docs[%d].ID empty", i)
		}
		if doc.SourceID != "youtube" {
			t.Errorf("docs[%d].SourceID = %q, want %q", i, doc.SourceID, "youtube")
		}
		if doc.URL == "" {
			t.Errorf("docs[%d].URL empty", i)
		}
		if doc.RetrievedAt != fixedTime {
			t.Errorf("docs[%d].RetrievedAt = %v, want %v", i, doc.RetrievedAt, fixedTime)
		}
		if doc.Hash != "" {
			t.Errorf("docs[%d].Hash = %q, want empty", i, doc.Hash)
		}
		if doc.DocType != types.DocTypeVideo {
			t.Errorf("docs[%d].DocType = %q, want %q", i, doc.DocType, types.DocTypeVideo)
		}
		if doc.Score < 0 || doc.Score > 1 {
			t.Errorf("docs[%d].Score = %v out of [0,1]", i, doc.Score)
		}
		// Required Metadata keys.
		for _, k := range []string{"channel_id", "channel_url", "duration_seconds", "view_count", "thumbnail_url", "available_transcript_langs"} {
			if _, ok := doc.Metadata[k]; !ok {
				t.Errorf("docs[%d].Metadata missing key %q", i, k)
			}
		}
	}
}

func TestParseSearchResponseHashEmpty(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "search_response.json")
	docs, _, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, doc := range docs {
		if doc.Hash != "" {
			t.Errorf("docs[%d].Hash = %q, want empty", i, doc.Hash)
		}
	}
}

func TestParseSearchResponseEmptyItems(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "search_response_empty.json")
	docs, cursor, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
	if cursor != "" {
		t.Errorf("expected empty cursor, got %q", cursor)
	}
}

func TestParseSearchResponseSelectsKoreanLang(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "search_response_korean.json")
	docs, _, err := parseSearchResponse(body, fixedTime, 0, "ko")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if docs[0].Lang != "ko" {
		t.Errorf("Lang = %q, want %q", docs[0].Lang, "ko")
	}
}

func TestParseSearchResponseIncludesTranscriptSnippet(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "search_response.json")
	docs, _, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// At least one doc should have a transcript_snippet (search_response.json has them).
	found := false
	for _, doc := range docs {
		if v, ok := doc.Metadata["transcript_snippet"]; ok {
			s, ok2 := v.(string)
			if !ok2 || s == "" {
				t.Error("transcript_snippet present but empty")
				continue
			}
			if utf8.RuneCountInString(s) > maxTranscriptRunes {
				t.Errorf("transcript_snippet rune count %d > %d", utf8.RuneCountInString(s), maxTranscriptRunes)
			}
			found = true
		}
	}
	if !found {
		t.Error("no doc has transcript_snippet in Metadata")
	}
}

func TestParseSearchResponseTruncatesOverlongTranscript(t *testing.T) {
	t.Parallel()
	// Build a synthetic sidecar response with a 1000-rune transcript_snippet.
	longSnippet := strings.Repeat("a", 1000)
	item := ytItem{
		ID:                       "trunctest",
		URL:                      "https://www.youtube.com/watch?v=trunctest",
		Title:                    "Truncation Test",
		ViewCount:                ptr64(100),
		AvailableTranscriptLangs: []string{"en"},
		TranscriptSnippet:        longSnippet,
		TranscriptLang:           "en",
	}
	resp := ytSearchResponse{Items: []ytItem{item}, HasMore: false}
	body, _ := json.Marshal(resp)

	docs, _, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	snippet, ok := docs[0].Metadata["transcript_snippet"].(string)
	if !ok {
		t.Fatal("transcript_snippet not in Metadata")
	}
	if utf8.RuneCountInString(snippet) > maxTranscriptRunes+1 { // +1 for ellipsis rune
		t.Errorf("transcript_snippet runes = %d, want ≤ %d+1", utf8.RuneCountInString(snippet), maxTranscriptRunes)
	}
}

func TestParseSearchResponsePaginationCursor(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "search_response_pagination.json")
	docs, cursor, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cursor == "" {
		t.Fatal("expected non-empty cursor for has_more=true")
	}
	// The last doc must have next_cursor.
	last := docs[len(docs)-1]
	nc, ok := last.Metadata["next_cursor"]
	if !ok {
		t.Fatal("last doc missing next_cursor in Metadata")
	}
	if nc != cursor {
		t.Errorf("last.Metadata[next_cursor] = %v, want %q", nc, cursor)
	}
	// Earlier docs must NOT have next_cursor.
	for i, doc := range docs[:len(docs)-1] {
		if _, ok := doc.Metadata["next_cursor"]; ok {
			t.Errorf("docs[%d] has next_cursor, expected only on last doc", i)
		}
	}
}

func TestParseSearchResponseNoCursorOnLastPage(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "search_response_empty.json")
	docs, cursor, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cursor != "" {
		t.Errorf("expected empty cursor on last page, got %q", cursor)
	}
	for i, doc := range docs {
		if _, ok := doc.Metadata["next_cursor"]; ok {
			t.Errorf("docs[%d] has next_cursor on last page", i)
		}
	}
}

func TestParseSearchResponseNoTranscript(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "search_response_no_transcript.json")
	docs, _, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	// available_transcript_langs should be present and empty.
	langs, ok := docs[0].Metadata["available_transcript_langs"]
	if !ok {
		t.Fatal("available_transcript_langs missing")
	}
	ls, ok := langs.([]string)
	if !ok || len(ls) != 0 {
		t.Errorf("available_transcript_langs = %v, want empty []string", langs)
	}
}

func TestParseSearchResponseLivestreamNullViewCount(t *testing.T) {
	t.Parallel()
	// Build a synthetic response with null view_count.
	rawJSON := `{"items":[{"id":"live001","url":"https://www.youtube.com/watch?v=live001","title":"Livestream","available_transcript_langs":[]}],"has_more":false}`
	docs, _, err := parseSearchResponse([]byte(rawJSON), fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	// view_count=null → treated as 0 → Score=0.5.
	const tol = 0.001
	if math.Abs(docs[0].Score-0.5) > tol {
		t.Errorf("Score = %v, want 0.5 for null view_count", docs[0].Score)
	}
}

func TestParseSearchResponseMetadataKeys(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "search_response.json")
	docs, _, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	required := []string{"channel_id", "channel_url", "duration_seconds", "view_count", "thumbnail_url", "available_transcript_langs"}
	for i, doc := range docs {
		for _, k := range required {
			if _, ok := doc.Metadata[k]; !ok {
				t.Errorf("docs[%d].Metadata missing required key %q", i, k)
			}
		}
	}
}

func TestParseSearchResponseSkipsItemsWithError(t *testing.T) {
	t.Parallel()
	// Build a response with 2 good items and 1 error item.
	errMsg := "extraction failed"
	resp := ytSearchResponse{
		Items: []ytItem{
			{ID: "good1", URL: "https://www.youtube.com/watch?v=good1", Title: "Good 1", ViewCount: ptr64(100), AvailableTranscriptLangs: []string{}},
			{ID: "bad1", URL: "https://www.youtube.com/watch?v=bad1", Title: "Bad 1", Error: &errMsg, AvailableTranscriptLangs: []string{}},
			{ID: "good2", URL: "https://www.youtube.com/watch?v=good2", Title: "Good 2", ViewCount: ptr64(200), AvailableTranscriptLangs: []string{}},
		},
		HasMore: false,
	}
	body, _ := json.Marshal(resp)
	docs, _, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 docs (skipped error item), got %d", len(docs))
	}
	for _, doc := range docs {
		if doc.ID == "bad1" {
			t.Error("bad1 should have been skipped")
		}
	}
}

func TestParseSearchResponseMalformedJSON(t *testing.T) {
	t.Parallel()
	body := loadFixture(t, "search_response_malformed.json")
	_, _, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	se, ok := err.(*types.SourceError)
	if !ok {
		t.Fatalf("error type = %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %q, want %q", se.Category, types.CategoryPermanent)
	}
}

func TestParseSearchResponseTopLevelSidecarError(t *testing.T) {
	t.Parallel()
	rawJSON := `{"error":{"category":"unavailable","message":"yt-dlp signed-in challenge"}}`
	_, _, err := parseSearchResponse([]byte(rawJSON), fixedTime, 0, "en")
	if err == nil {
		t.Fatal("expected error for sidecar error envelope, got nil")
	}
	se, ok := err.(*types.SourceError)
	if !ok {
		t.Fatalf("error type = %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryUnavailable {
		t.Errorf("Category = %q, want unavailable", se.Category)
	}
}

func TestParseSearchResponseViewCountScore(t *testing.T) {
	t.Parallel()
	vc := int64(10_000)
	item := ytItem{
		ID:                       "scoretest",
		URL:                      "https://www.youtube.com/watch?v=scoretest",
		Title:                    "Score Test",
		ViewCount:                &vc,
		AvailableTranscriptLangs: []string{},
	}
	resp := ytSearchResponse{Items: []ytItem{item}, HasMore: false}
	body, _ := json.Marshal(resp)
	docs, _, err := parseSearchResponse(body, fixedTime, 0, "en")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := normalizeViewScore(vc)
	const tol = 0.001
	if math.Abs(docs[0].Score-want) > tol {
		t.Errorf("Score = %v, want %v", docs[0].Score, want)
	}
}

// ptr64 is a helper that returns a pointer to the given int64.
func ptr64(v int64) *int64 { return &v }
