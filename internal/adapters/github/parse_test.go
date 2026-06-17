// Package github — parse function tests.
// Tests #26–35: REQ-ADP4-005 field mapping correctness.
package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	gogithub "github.com/google/go-github/v73/github"

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

// --- REQ-ADP4a-002 / 003: commit parse tests ---

// strPtr returns a *string for test struct construction.
func strPtr(s string) *string { return &s }

// TestParseCommitResultsFieldMapping verifies NormalizedDoc field mapping for
// commit search results against the 3-item fixture (REQ-ADP4a-002 / AC-004).
func TestParseCommitResultsFieldMapping(t *testing.T) {
	t.Parallel()
	srv := newCommitStubServer(t, "search_commits_response.json", 0)
	defer srv.Close()
	a := newTestAdapter(t, srv.URL)

	docs, err := a.Search(context.Background(), testQuery("fix bug", "commit", 5, ""))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs, got %d", len(docs))
	}

	// Doc 0: full commit with author/committer/html_url.
	d0 := docs[0]
	const sha0 = "abc123def4567890abcdef1234567890abcdef12"
	const repo0 = "elymas/universal-search"
	wantID0 := "github:commit:" + repo0 + "@" + sha0
	if d0.ID != wantID0 {
		t.Errorf("doc0 ID = %q, want %q", d0.ID, wantID0)
	}
	if d0.SourceID != "github" {
		t.Errorf("doc0 SourceID = %q, want github", d0.SourceID)
	}
	if d0.URL != "https://github.com/elymas/universal-search/commit/"+sha0 {
		t.Errorf("doc0 URL = %q, want html_url", d0.URL)
	}
	// Title = first line of message, <=80 runes.
	if d0.Title != "fix: handle nil pointer in search dispatch" {
		t.Errorf("doc0 Title = %q, want subject line", d0.Title)
	}
	if len([]rune(d0.Title)) > 80 {
		t.Errorf("doc0 Title rune count = %d, want <= 80", len([]rune(d0.Title)))
	}
	// Body = full message.
	if !strings.Contains(d0.Body, "Reviewed-by: bob") {
		t.Errorf("doc0 Body = %q, want full multi-line message", d0.Body)
	}
	if d0.Body != "fix: handle nil pointer in search dispatch\n\nAdds a nil-guard for the Commit pointer so malformed commits do not panic.\n\nReviewed-by: bob." {
		t.Errorf("doc0 Body not the unmodified full message: %q", d0.Body)
	}
	// Snippet = truncateRunes(message, 280).
	if d0.Snippet != truncateRunes(d0.Body, snippetMaxRunes) {
		t.Errorf("doc0 Snippet mismatch: got %q", d0.Snippet)
	}
	wantPublished0 := time.Date(2026, 5, 1, 10, 15, 30, 0, time.UTC)
	if !d0.PublishedAt.Equal(wantPublished0) {
		t.Errorf("doc0 PublishedAt = %v, want %v", d0.PublishedAt, wantPublished0)
	}
	if d0.Author != "Alice Lee" {
		t.Errorf("doc0 Author = %q, want Alice Lee", d0.Author)
	}
	if d0.Score != 0.5 {
		t.Errorf("doc0 Score = %v, want 0.5 (neutral)", d0.Score)
	}
	if d0.DocType != types.DocTypeRepo {
		t.Errorf("doc0 DocType = %q, want DocTypeRepo", d0.DocType)
	}
	if d0.Lang != "" {
		t.Errorf("doc0 Lang = %q, want empty", d0.Lang)
	}
	if d0.Hash != "" {
		t.Errorf("doc0 Hash = %q, want empty", d0.Hash)
	}
	// Metadata REQUIRED keys.
	if d0.Metadata["sha"] != sha0 {
		t.Errorf("doc0 meta[sha] = %v, want %q", d0.Metadata["sha"], sha0)
	}
	if d0.Metadata["repo_full_name"] != repo0 {
		t.Errorf("doc0 meta[repo_full_name] = %v, want %q", d0.Metadata["repo_full_name"], repo0)
	}
	if d0.Metadata["message_subject"] != "fix: handle nil pointer in search dispatch" {
		t.Errorf("doc0 meta[message_subject] = %v, want subject", d0.Metadata["message_subject"])
	}
	if d0.Metadata["kind"] != "commit" {
		t.Errorf("doc0 meta[kind] = %v, want commit", d0.Metadata["kind"])
	}
	// Metadata OPTIONAL keys.
	if d0.Metadata["author_name"] != "Alice Lee" {
		t.Errorf("doc0 meta[author_name] = %v, want Alice Lee", d0.Metadata["author_name"])
	}
	if d0.Metadata["author_email"] != "alice@example.com" {
		t.Errorf("doc0 meta[author_email] = %v", d0.Metadata["author_email"])
	}
	if d0.Metadata["committer_name"] != "Alice Lee" {
		t.Errorf("doc0 meta[committer_name] = %v", d0.Metadata["committer_name"])
	}
	if d0.Metadata["authored_date"] != "2026-05-01T10:15:30Z" {
		t.Errorf("doc0 meta[authored_date] = %v", d0.Metadata["authored_date"])
	}
	// Every doc must pass Validate (non-empty URL guaranteed).
	if err := d0.Validate(); err != nil {
		t.Errorf("doc0 Validate: %v", err)
	}

	// Doc 1: null author/committer but html_url present.
	d1 := docs[1]
	const sha1 = "def456abc7890123abcdef4567890abcdef45678"
	const repo1 = "golang/go"
	if d1.ID != "github:commit:"+repo1+"@"+sha1 {
		t.Errorf("doc1 ID = %q", d1.ID)
	}
	if d1.URL != "https://github.com/golang/go/commit/"+sha1 {
		t.Errorf("doc1 URL = %q, want html_url (present)", d1.URL)
	}
	if d1.Author != "Bob Kim" {
		t.Errorf("doc1 Author = %q, want Bob Kim (from commit metadata)", d1.Author)
	}
	if err := d1.Validate(); err != nil {
		t.Errorf("doc1 Validate: %v", err)
	}

	// Doc 2: null author/committer AND null html_url -> URL synthesized.
	d2 := docs[2]
	const sha2 = "999002ffffffffffffffffffffffffffffffff"
	const repo2 = "kubernetes/kubernetes"
	if d2.URL != "https://github.com/"+repo2+"/commit/"+sha2 {
		t.Errorf("doc2 URL = %q, want synthesized permalink", d2.URL)
	}
	if err := d2.Validate(); err != nil {
		t.Errorf("doc2 Validate: %v", err)
	}
}

// TestParseCommitNilSafe verifies nil-safety: Commit:nil (but SHA+repo
// present) yields a non-panicking doc with synthesized URL; a CommitResult
// lacking BOTH SHA and repo is skipped (REQ-ADP4a-002 / AC-006, AC-007b).
func TestParseCommitNilSafe(t *testing.T) {
	t.Parallel()
	retrievedAt := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)

	// Build a CommitResult with Commit == nil but SHA + Repository present.
	sha := "cafef00d"
	repoFullName := "foo/bar"
	result := &gogithub.CommitsSearchResult{
		Commits: []*gogithub.CommitResult{
			{
				SHA:    strPtr(sha),
				Commit: nil, // nil Commit pointer
				Repository: &gogithub.Repository{
					FullName: strPtr(repoFullName),
				},
				HTMLURL: nil,
			},
			// Commit lacking BOTH SHA and repo -> must be skipped.
			{
				SHA:        nil,
				Commit:     nil,
				Repository: nil,
				HTMLURL:    nil,
			},
		},
	}

	docs, err := parseCommitResults(result, 0, retrievedAt)
	if err != nil {
		t.Fatalf("parseCommitResults: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc (skip-path removes 1), got %d", len(docs))
	}
	doc := docs[0]
	if doc.Title != "" {
		t.Errorf("Title = %q, want empty (Commit:nil)", doc.Title)
	}
	if doc.Body != "" {
		t.Errorf("Body = %q, want empty (Commit:nil)", doc.Body)
	}
	if doc.Snippet != "" {
		t.Errorf("Snippet = %q, want empty (Commit:nil)", doc.Snippet)
	}
	if !doc.PublishedAt.IsZero() {
		t.Errorf("PublishedAt = %v, want zero time", doc.PublishedAt)
	}
	wantURL := "https://github.com/" + repoFullName + "/commit/" + sha
	if doc.URL != wantURL {
		t.Errorf("URL = %q, want synthesized %q", doc.URL, wantURL)
	}
	if doc.ID != "github:commit:"+repoFullName+"@"+sha {
		t.Errorf("ID = %q, want composite", doc.ID)
	}
	if err := doc.Validate(); err != nil {
		t.Errorf("Validate (synthesized URL): %v", err)
	}
}

// TestParseCommitPaginationCursor verifies that when NextPage > 0 the LAST
// returned doc carries next_cursor and earlier docs do not (REQ-ADP4a-003 / AC-008).
func TestParseCommitPaginationCursor(t *testing.T) {
	t.Parallel()
	result := &gogithub.CommitsSearchResult{
		Commits: []*gogithub.CommitResult{
			{SHA: strPtr("s1"), Repository: &gogithub.Repository{FullName: strPtr("a/b")}, HTMLURL: strPtr("https://github.com/a/b/commit/s1")},
			{SHA: strPtr("s2"), Repository: &gogithub.Repository{FullName: strPtr("a/b")}, HTMLURL: strPtr("https://github.com/a/b/commit/s2")},
			{SHA: strPtr("s3"), Repository: &gogithub.Repository{FullName: strPtr("a/b")}, HTMLURL: strPtr("https://github.com/a/b/commit/s3")},
		},
	}
	docs, err := parseCommitResults(result, 2, time.Now().UTC())
	if err != nil {
		t.Fatalf("parseCommitResults: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs, got %d", len(docs))
	}
	// Earlier docs must NOT have next_cursor.
	for i := 0; i < 2; i++ {
		if _, ok := docs[i].Metadata["next_cursor"]; ok {
			t.Errorf("doc[%d] unexpectedly has next_cursor", i)
		}
	}
	// Last doc MUST carry next_cursor == "2".
	if got := docs[2].Metadata["next_cursor"]; got != "2" {
		t.Errorf("last doc next_cursor = %v, want \"2\"", got)
	}
}

// TestParseCommitNoCursorOnLastPage verifies that when NextPage == 0 no doc
// carries next_cursor (REQ-ADP4a-003 / AC-009).
func TestParseCommitNoCursorOnLastPage(t *testing.T) {
	t.Parallel()
	result := &gogithub.CommitsSearchResult{
		Commits: []*gogithub.CommitResult{
			{SHA: strPtr("s1"), Repository: &gogithub.Repository{FullName: strPtr("a/b")}, HTMLURL: strPtr("https://github.com/a/b/commit/s1")},
			{SHA: strPtr("s2"), Repository: &gogithub.Repository{FullName: strPtr("a/b")}, HTMLURL: strPtr("https://github.com/a/b/commit/s2")},
		},
	}
	docs, err := parseCommitResults(result, 0, time.Now().UTC())
	if err != nil {
		t.Fatalf("parseCommitResults: %v", err)
	}
	for i, d := range docs {
		if _, ok := d.Metadata["next_cursor"]; ok {
			t.Errorf("doc[%d] unexpectedly has next_cursor on last page", i)
		}
	}
}

// TestParseCommitEmpty verifies EC-002: nil/empty Commits -> (nil/empty, nil).
func TestParseCommitEmpty(t *testing.T) {
	t.Parallel()
	docs, err := parseCommitResults(nil, 0, time.Now().UTC())
	if err != nil {
		t.Fatalf("nil result: %v", err)
	}
	if docs != nil {
		t.Errorf("nil result -> docs = %v, want nil", docs)
	}
	empty := &gogithub.CommitsSearchResult{}
	docs2, err := parseCommitResults(empty, 0, time.Now().UTC())
	if err != nil {
		t.Fatalf("empty result: %v", err)
	}
	if len(docs2) != 0 {
		t.Errorf("empty result -> %d docs, want 0", len(docs2))
	}
}

// newCommitStubServer creates a stub server serving commit search results.
// nextPage > 0 adds a Link header indicating a subsequent page.
func newCommitStubServer(tb testing.TB, fixture string, nextPage int) *httptest.Server {
	tb.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if nextPage > 0 {
			linkURL := fmt.Sprintf("%s%s?q=%s&page=%d&per_page=%s",
				"http://"+r.Host, r.URL.Path,
				r.URL.Query().Get("q"),
				nextPage,
				r.URL.Query().Get("per_page"),
			)
			w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, linkURL))
		}
		writeJSONFile(w, "testdata/"+fixture)
	}))
}
