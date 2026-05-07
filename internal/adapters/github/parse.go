// Package github — NormalizedDoc parsing for all three search intents.
// REQ-ADP4-005: field mapping tables from §6.3.1/6.3.2/6.3.3.
package github

import (
	"fmt"
	"strconv"
	"time"

	gogithub "github.com/google/go-github/v73/github"

	"github.com/elymas/universal-search/pkg/types"
)

// snippetMaxRunes is the maximum rune length of the Snippet field.
const snippetMaxRunes = 280

// parseCodeResults converts a go-github CodeSearchResult into a []NormalizedDoc.
// code hits map to DocTypeRepo in v0.1 (Open Question §11.1 defers DocTypeCode).
func parseCodeResults(result *gogithub.CodeSearchResult, nextPage int, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	if result == nil {
		return nil, nil
	}
	docs := make([]types.NormalizedDoc, 0, len(result.CodeResults))
	for i, cr := range result.CodeResults {
		if cr == nil {
			continue
		}
		name := safeStr(cr.Name)
		path := safeStr(cr.Path)
		sha := safeStr(cr.SHA)
		htmlURL := safeStr(cr.HTMLURL)
		repoFull := ""
		if cr.Repository != nil {
			repoFull = safeStr(cr.Repository.FullName)
		}

		id := fmt.Sprintf("github:code:%s@%s:%s", repoFull, sha, path)
		title := name
		if title == "" {
			title = path
		}
		snippet := truncateRunes(path, snippetMaxRunes)

		meta := map[string]any{
			"kind": "code",
		}
		if repoFull != "" {
			meta["repo_full_name"] = repoFull
		}
		if path != "" {
			meta["path"] = path
		}
		if sha != "" {
			meta["sha"] = sha
		}

		doc := types.NormalizedDoc{
			ID:          id,
			SourceID:    "github",
			URL:         htmlURL,
			Title:       title,
			Body:        path,
			Snippet:     snippet,
			RetrievedAt: retrievedAt,
			Author:      "",
			Score:       0.5, // No GitHub-provided score in v73 CodeResult.
			Lang:        "",
			DocType:     types.DocTypeRepo,
			Metadata:    meta,
			Hash:        "",
		}

		// Set next_cursor on the last doc when a next page exists.
		if nextPage > 0 && i == len(result.CodeResults)-1 {
			doc.Metadata["next_cursor"] = strconv.Itoa(nextPage)
		}

		docs = append(docs, doc)
	}
	return docs, nil
}

// parseIssueResults converts a go-github IssuesSearchResult into []NormalizedDoc.
//
// @MX:ANCHOR: [AUTO] parseIssueResults — issue field-mapping integrity gate.
// @MX:REASON: Every issue/PR NormalizedDoc passes through this function;
// a mapping error here corrupts all issue/PR search results.
// @MX:SPEC: SPEC-ADP-004
func parseIssueResults(result *gogithub.IssuesSearchResult, nextPage int, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	if result == nil {
		return nil, nil
	}
	docs := make([]types.NormalizedDoc, 0, len(result.Issues))
	for i, issue := range result.Issues {
		if issue == nil {
			continue
		}

		id := fmt.Sprintf("github:issue:%d", safeInt64(issue.ID))
		url := safeStr(issue.HTMLURL)
		title := safeStr(issue.Title)
		body := safeStr(issue.Body)
		snippet := truncateRunes(body, snippetMaxRunes)
		if snippet == "" {
			snippet = title
		}

		author := ""
		if issue.User != nil {
			author = safeStr(issue.User.Login)
		}

		comments := safeInt(issue.Comments)
		isPR := issue.PullRequestLinks != nil
		kind := "issue"
		if isPR {
			kind = "pr"
		}

		// Extract repo full name from RepositoryURL.
		repoFullName := extractRepoFromURL(safeStr(issue.RepositoryURL))

		meta := map[string]any{
			"number":          safeInt(issue.Number),
			"state":           safeStr(issue.State),
			"is_pull_request": isPR,
			"comments":        comments,
			"kind":            kind,
			"repo_full_name":  repoFullName,
		}

		// Optional metadata.
		if len(issue.Labels) > 0 {
			labels := make([]string, 0, len(issue.Labels))
			for _, l := range issue.Labels {
				if l != nil {
					labels = append(labels, safeStr(l.Name))
				}
			}
			meta["labels"] = labels
		}
		if issue.UpdatedAt != nil && !issue.UpdatedAt.IsZero() {
			meta["updated_at"] = issue.UpdatedAt.Format(time.RFC3339)
		}
		if issue.Reactions != nil && issue.Reactions.TotalCount != nil {
			meta["reactions_total_count"] = *issue.Reactions.TotalCount
		}

		doc := types.NormalizedDoc{
			ID:          id,
			SourceID:    "github",
			URL:         url,
			Title:       title,
			Body:        body,
			Snippet:     snippet,
			PublishedAt: safeTime(issue.CreatedAt),
			RetrievedAt: retrievedAt,
			Author:      author,
			Score:       normalizeScore(comments * 10),
			Lang:        "",
			DocType:     types.DocTypeIssue,
			Metadata:    meta,
			Hash:        "",
		}

		if nextPage > 0 && i == len(result.Issues)-1 {
			doc.Metadata["next_cursor"] = strconv.Itoa(nextPage)
		}

		docs = append(docs, doc)
	}
	return docs, nil
}

// parseRepoResults converts a go-github RepositoriesSearchResult into []NormalizedDoc.
//
// @MX:ANCHOR: [AUTO] parseRepoResults — repo field-mapping integrity gate.
// @MX:REASON: Every default-routed (kind=repos) GitHub search passes through
// this transform. A mapping error here corrupts all repo NormalizedDocs.
// @MX:SPEC: SPEC-ADP-004
func parseRepoResults(result *gogithub.RepositoriesSearchResult, nextPage int, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	if result == nil {
		return nil, nil
	}
	docs := make([]types.NormalizedDoc, 0, len(result.Repositories))
	for i, repo := range result.Repositories {
		if repo == nil {
			continue
		}

		id := fmt.Sprintf("github:repo:%d", safeInt64(repo.ID))
		url := safeStr(repo.HTMLURL)
		fullName := safeStr(repo.FullName)
		desc := safeStr(repo.Description)

		snippet := truncateRunes(desc, snippetMaxRunes)
		if snippet == "" {
			snippet = fullName
		}

		author := ""
		if repo.Owner != nil {
			author = safeStr(repo.Owner.Login)
		}

		language := ""
		if repo.Language != nil {
			language = *repo.Language
		}

		stars := safeInt(repo.StargazersCount)
		forks := safeInt(repo.ForksCount)
		openIssues := safeInt(repo.OpenIssuesCount)

		meta := map[string]any{
			"full_name":   fullName,
			"language":    language,
			"stars":       stars,
			"forks":       forks,
			"open_issues": openIssues,
			"kind":        "repo",
		}

		// Optional metadata.
		if len(repo.Topics) > 0 {
			meta["topics"] = repo.Topics
		}
		if repo.DefaultBranch != nil {
			meta["default_branch"] = *repo.DefaultBranch
		}
		if repo.PushedAt != nil && !repo.PushedAt.IsZero() {
			meta["pushed_at"] = repo.PushedAt.Format(time.RFC3339)
		}
		if repo.Size != nil {
			meta["size_kb"] = *repo.Size
		}

		doc := types.NormalizedDoc{
			ID:          id,
			SourceID:    "github",
			URL:         url,
			Title:       fullName,
			Body:        desc,
			Snippet:     snippet,
			PublishedAt: safeTime(repo.CreatedAt),
			RetrievedAt: retrievedAt,
			Author:      author,
			Score:       normalizeScore(stars),
			Lang:        "",
			DocType:     types.DocTypeRepo,
			Metadata:    meta,
			Hash:        "",
		}

		if nextPage > 0 && i == len(result.Repositories)-1 {
			doc.Metadata["next_cursor"] = strconv.Itoa(nextPage)
		}

		docs = append(docs, doc)
	}
	return docs, nil
}

// --- nil-safe helpers ---

func safeStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func safeInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func safeInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func safeTime(p *gogithub.Timestamp) time.Time {
	if p == nil {
		return time.Time{}
	}
	return p.UTC()
}

// truncateRunes truncates s to at most maxRunes runes.
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

// extractRepoFromURL extracts "owner/repo" from a GitHub API repository URL.
// e.g., "https://api.github.com/repos/golang/go" → "golang/go".
func extractRepoFromURL(apiURL string) string {
	const prefix = "/repos/"
	idx := -1
	for i := 0; i <= len(apiURL)-len(prefix); i++ {
		if apiURL[i:i+len(prefix)] == prefix {
			idx = i
			break
		}
	}
	if idx < 0 {
		return apiURL
	}
	return apiURL[idx+len(prefix):]
}
