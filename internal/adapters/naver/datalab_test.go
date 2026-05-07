package naver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/elymas/universal-search/pkg/types"
)

// TestSearchDataLab_HappyPath verifies DataLab POST returns one doc per keyword group.
// REQ-ADP8-013.
func TestSearchDataLab_HappyPath(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/datalab_response.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	var gotMethod, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	requestBody := `{"startDate":"2026-04-01","endDate":"2026-04-30","timeUnit":"date","keywordGroups":[{"groupName":"키워드그룹A","keywords":["golang","go언어"]},{"groupName":"키워드그룹B","keywords":["python","파이썬"]},{"groupName":"키워드그룹C","keywords":["kubernetes","k8s"]}]}`

	docs, err := a.Search(context.Background(), types.Query{
		Text: requestBody,
		Filters: []types.Filter{
			{Key: filterKeyVertical, Value: verticalDataLab},
		},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// Should return 3 docs — one per keyword group.
	if len(docs) != 3 {
		t.Fatalf("len(docs) = %d, want 3 (one per keyword group)", len(docs))
	}

	// Verify POST method was used.
	if gotMethod != http.MethodPost {
		t.Errorf("HTTP method = %q, want POST", gotMethod)
	}

	// Verify Content-Type header.
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}

	// Validate all docs.
	for i, doc := range docs {
		docCopy := doc
		if err := docCopy.Validate(); err != nil {
			t.Errorf("docs[%d].Validate() error = %v", i, err)
		}
		// Each doc should have metadata with keywords.
		if docCopy.Metadata == nil {
			t.Errorf("docs[%d].Metadata is nil", i)
			continue
		}
		if _, ok := docCopy.Metadata["keywords"]; !ok {
			t.Errorf("docs[%d].Metadata missing 'keywords'", i)
		}
		if _, ok := docCopy.Metadata["start_date"]; !ok {
			t.Errorf("docs[%d].Metadata missing 'start_date'", i)
		}
	}
}

// TestSearchDataLab_HTTP401 verifies DataLab 401 maps to CategoryPermanent.
func TestSearchDataLab_HTTP401(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), types.Query{
		Text: `{"startDate":"2026-04-01"}`,
		Filters: []types.Filter{
			{Key: filterKeyVertical, Value: verticalDataLab},
		},
	})
	if err == nil {
		t.Fatal("Search() error = nil, want error for 401")
	}
	se, ok := err.(*types.SourceError)
	if !ok {
		t.Fatalf("error type = %T, want *types.SourceError", err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want CategoryPermanent", se.Category)
	}
}

// TestParseDatalabResponse verifies direct parsing of DataLab fixture.
func TestParseDatalabResponse(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("testdata/datalab_response.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	docs, parseErr := parseDatalabResponse(body, fixedTime)
	if parseErr != nil {
		t.Fatalf("parseDatalabResponse() error = %v", parseErr)
	}
	if len(docs) != 3 {
		t.Fatalf("len(docs) = %d, want 3", len(docs))
	}

	// Each doc should have data_count = 30 (30 daily data points).
	for i, doc := range docs {
		if doc.Metadata == nil {
			t.Errorf("docs[%d].Metadata is nil", i)
			continue
		}
		count, ok := doc.Metadata["data_count"].(int)
		if !ok {
			t.Errorf("docs[%d].Metadata['data_count'] not int: %T", i, doc.Metadata["data_count"])
			continue
		}
		if count != 30 {
			t.Errorf("docs[%d].data_count = %d, want 30", i, count)
		}
	}
}

// TestParseDatalabResponse_MalformedJSON verifies malformed JSON returns a SourceError.
func TestParseDatalabResponse_MalformedJSON(t *testing.T) {
	t.Parallel()
	docs, err := parseDatalabResponse([]byte(`{broken json`), fixedTime)
	if err == nil {
		t.Fatal("parseDatalabResponse() error = nil, want error for malformed JSON")
	}
	if docs != nil {
		t.Error("parseDatalabResponse() returned non-nil docs on error")
	}
}

