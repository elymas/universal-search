// Package github — parse function tests.
// Tests #26–35: REQ-ADP4-005 field mapping correctness.
package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestParseCodeResultsFieldMapping verifies NormalizedDoc field mapping for
// code search results against 5 fixtures.
func TestParseCodeResultsFieldMapping(t *testing.T) {
	t.Parallel()
	srv := newCodeStubServer(t, "search_code_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("adapter", "code", 5, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least one doc")
	}

	doc := docs[0]
	// Code hits map to DocTypeRepo in v0.1 (Open Question §11.1).
	if doc.DocType != types.DocTypeRepo {
		t.Errorf("DocType = %q, want %q", doc.DocType, types.DocTypeRepo)
	}
	if doc.SourceID != "github" {
		t.Errorf("SourceID = %q, want github", doc.SourceID)
	}
	if doc.URL == "" {
		t.Error("URL must not be empty")
	}
	if doc.Hash != "" {
		t.Errorf("Hash must be empty, got %q", doc.Hash)
	}
	if doc.RetrievedAt.IsZero() {
		t.Error("RetrievedAt must not be zero")
	}
	// kind metadata key must be "code".
	if k, ok := doc.Metadata["kind"]; !ok || k != "code" {
		t.Errorf("Metadata[kind] = %v, want code", k)
	}
}

// TestParseIssueResultsFieldMapping verifies NormalizedDoc field mapping for
// issue search results against 5 fixtures.
func TestParseIssueResultsFieldMapping(t *testing.T) {
	t.Parallel()
	srv := newIssueStubServer(t, "search_issues_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("goroutine", "issues", 5, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least one doc")
	}

	doc := docs[0]
	if doc.DocType != types.DocTypeIssue {
		t.Errorf("DocType = %q, want %q", doc.DocType, types.DocTypeIssue)
	}
	if doc.SourceID != "github" {
		t.Errorf("SourceID = %q, want github", doc.SourceID)
	}
	if doc.URL == "" {
		t.Error("URL must not be empty")
	}
	if doc.Hash != "" {
		t.Errorf("Hash must be empty, got %q", doc.Hash)
	}
	// kind must be "issue" or "pr".
	k, ok := doc.Metadata["kind"]
	if !ok {
		t.Error("Metadata missing kind key")
	}
	if k != "issue" && k != "pr" {
		t.Errorf("Metadata[kind] = %v, want issue or pr", k)
	}
	// number key must be present.
	if _, ok := doc.Metadata["number"]; !ok {
		t.Error("Metadata missing number key")
	}
	// state key must be present.
	if _, ok := doc.Metadata["state"]; !ok {
		t.Error("Metadata missing state key")
	}
}

// TestParseRepoResultsFieldMapping verifies NormalizedDoc field mapping for
// repository search results against 5 fixtures.
func TestParseRepoResultsFieldMapping(t *testing.T) {
	t.Parallel()
	srv := newRepoStubServer(t, "search_repos_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) < 5 {
		t.Fatalf("expected ≥5 docs, got %d", len(docs))
	}

	for i, doc := range docs[:5] {
		if doc.DocType != types.DocTypeRepo {
			t.Errorf("doc[%d]: DocType = %q, want %q", i, doc.DocType, types.DocTypeRepo)
		}
		if doc.SourceID != "github" {
			t.Errorf("doc[%d]: SourceID = %q, want github", i, doc.SourceID)
		}
		if doc.URL == "" {
			t.Errorf("doc[%d]: URL empty", i)
		}
		if doc.Hash != "" {
			t.Errorf("doc[%d]: Hash must be empty", i)
		}
		if doc.ID == "" {
			t.Errorf("doc[%d]: ID must not be empty", i)
		}
		// Required metadata keys for repos.
		for _, key := range []string{"full_name", "stars", "forks", "open_issues", "kind"} {
			if _, ok := doc.Metadata[key]; !ok {
				t.Errorf("doc[%d]: Metadata missing required key %q", i, key)
			}
		}
		if k := doc.Metadata["kind"]; k != "repo" {
			t.Errorf("doc[%d]: Metadata[kind] = %v, want repo", i, k)
		}
	}
}

// TestParseDeletedUserNilSafe verifies that a nil User on an issue doesn't panic.
func TestParseDeletedUserNilSafe(t *testing.T) {
	t.Parallel()
	srv := newIssueStubServer(t, "search_issues_nil_user.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("deleted user", "issues", 10, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least one doc")
	}
	if docs[0].Author != "" {
		t.Errorf("Author from nil user should be empty, got %q", docs[0].Author)
	}
}

// TestParseNoLanguageNilSafe verifies that a nil Language on a repo is handled.
func TestParseNoLanguageNilSafe(t *testing.T) {
	t.Parallel()
	srv := newRepoStubServer(t, "search_repos_no_language.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("config", "repos", 10, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least one doc")
	}
	// Metadata["language"] should be empty string, not panic.
	if lang, ok := docs[0].Metadata["language"]; ok {
		if s, ok := lang.(string); ok && s != "" {
			t.Errorf("Metadata[language] for nil language = %q, want empty", s)
		}
	}
}

// TestParsePaginationCursor verifies that when NextPage > 0, the last doc
// carries Metadata["next_cursor"] set to the page number string.
func TestParsePaginationCursor(t *testing.T) {
	t.Parallel()
	// nextPage=2 means there is a second page.
	srv := newRepoStubServer(t, "search_repos_pagination.json", 2)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("pagination", "repos", 10, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least one doc")
	}
	last := docs[len(docs)-1]
	cursor, ok := last.Metadata["next_cursor"]
	if !ok {
		t.Fatal("last doc missing next_cursor in Metadata")
	}
	if cursor != "2" {
		t.Errorf("next_cursor = %v, want 2", cursor)
	}
	// Other docs must NOT have next_cursor.
	if len(docs) > 1 {
		for i, d := range docs[:len(docs)-1] {
			if _, ok := d.Metadata["next_cursor"]; ok {
				t.Errorf("doc[%d] should not have next_cursor", i)
			}
		}
	}
}

// TestParseNoCursorOnLastPage verifies that when NextPage == 0, no doc
// carries the next_cursor key.
func TestParseNoCursorOnLastPage(t *testing.T) {
	t.Parallel()
	// nextPage=0 means this is the last page.
	srv := newRepoStubServer(t, "search_repos_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("golang", "repos", 25, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for i, d := range docs {
		if _, ok := d.Metadata["next_cursor"]; ok {
			t.Errorf("doc[%d] should not have next_cursor on last page", i)
		}
	}
}

// TestParseHashEmpty verifies that every parsed doc has Hash == "".
func TestParseHashEmpty(t *testing.T) {
	t.Parallel()
	srv := newRepoStubServer(t, "search_repos_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("golang", "repos", 25, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for i, d := range docs {
		if d.Hash != "" {
			t.Errorf("doc[%d]: Hash = %q, want empty", i, d.Hash)
		}
	}
}

// TestParseMetadataKeysPerIntent verifies the required metadata key set is
// present for each intent type.
func TestParseMetadataKeysPerIntent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		intent  string
		fixture string
		newSrv  func(testing.TB, string, int) *httptest.Server
		keys    []string
	}{
		{
			intent:  "repos",
			fixture: "search_repos_25.json",
			newSrv:  newRepoStubServer,
			keys:    []string{"full_name", "stars", "forks", "open_issues", "kind"},
		},
		{
			intent:  "issues",
			fixture: "search_issues_25.json",
			newSrv:  newIssueStubServer,
			keys:    []string{"number", "state", "is_pull_request", "comments", "kind"},
		},
		{
			intent:  "code",
			fixture: "search_code_25.json",
			newSrv:  newCodeStubServer,
			keys:    []string{"kind"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.intent, func(t *testing.T) {
			t.Parallel()
			srv := tc.newSrv(t, tc.fixture, 0)
			defer srv.Close()
			a := newTestAdapter(t, srv.URL)

			docs, err := a.Search(context.Background(), testQuery("test", tc.intent, 5, ""))
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(docs) == 0 {
				t.Fatal("expected at least one doc")
			}
			doc := docs[0]
			for _, key := range tc.keys {
				if _, ok := doc.Metadata[key]; !ok {
					t.Errorf("Metadata missing required key %q for intent %q", key, tc.intent)
				}
			}
		})
	}
}

// TestParseMalformedJSON verifies that a truncated/malformed JSON response
// returns a SourceError with Category=CategoryPermanent.
func TestParseMalformedJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := os.ReadFile("testdata/search_malformed.json")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	_, err := a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	var se *types.SourceError
	if !isSourceError(err, &se) {
		t.Fatalf("expected *types.SourceError, got %T: %v", err, err)
	}
	if se.Category != types.CategoryPermanent {
		t.Errorf("Category = %v, want CategoryPermanent", se.Category)
	}
}

// TestParseIsPullRequestField verifies is_pull_request metadata field.
func TestParseIsPullRequestField(t *testing.T) {
	t.Parallel()
	srv := newIssueStubServer(t, "search_issues_pr.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("fix", "issues", 10, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) < 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}

	// First doc is a PR (has pull_request field).
	pr := docs[0]
	if v, ok := pr.Metadata["is_pull_request"]; !ok {
		t.Error("PR doc missing is_pull_request key")
	} else if v != true {
		t.Errorf("PR doc is_pull_request = %v, want true", v)
	}
	if pr.Metadata["kind"] != "pr" {
		t.Errorf("PR doc kind = %v, want pr", pr.Metadata["kind"])
	}

	// Second doc is an issue.
	issue := docs[1]
	if v, ok := issue.Metadata["is_pull_request"]; !ok {
		t.Error("issue doc missing is_pull_request key")
	} else if v != false {
		t.Errorf("issue doc is_pull_request = %v, want false", v)
	}
	if issue.Metadata["kind"] != "issue" {
		t.Errorf("issue doc kind = %v, want issue", issue.Metadata["kind"])
	}
}

// TestParseRepoOwnerNilSafe verifies that a nil Owner on a repo doesn't panic.
func TestParseRepoOwnerNilSafe(t *testing.T) {
	t.Parallel()
	srv := newRepoStubServer(t, "search_repos_nil_owner.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("orphan", "repos", 5, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least one doc")
	}
	if docs[0].Author != "" {
		t.Errorf("Author from nil owner should be empty, got %q", docs[0].Author)
	}
}

// TestParseRetrievedAtSetToNow verifies RetrievedAt is a recent timestamp.
func TestParseRetrievedAtSetToNow(t *testing.T) {
	t.Parallel()
	srv := newRepoStubServer(t, "search_repos_25.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	before := time.Now().Add(-time.Second)
	docs, err := a.Search(context.Background(), testQuery("golang", "repos", 5, ""))
	after := time.Now().Add(time.Second)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for i, d := range docs {
		if d.RetrievedAt.Before(before) || d.RetrievedAt.After(after) {
			t.Errorf("doc[%d]: RetrievedAt = %v, want between %v and %v",
				i, d.RetrievedAt, before, after)
		}
	}
}
