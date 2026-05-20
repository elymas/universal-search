// Package github — Search method and filter qualifier logic.
// REQ-ADP4-002: multi-intent routing via Query.Filters[kind].
// REQ-ADP4-007: filter qualifier append.
// REQ-ADP4-008: input validation.
package github

import (
	"context"
	"strconv"
	"strings"
	"time"
	"unicode"

	gogithub "github.com/google/go-github/v73/github"

	"github.com/elymas/universal-search/pkg/types"
)

const (
	defaultPerPage = 25
	maxPerPage     = 100
)

// validIntents is the set of recognised kind values.
var validIntents = map[string]struct{}{
	"code":   {},
	"issues": {},
	"repos":  {},
}

// Search implements types.Adapter.Search for the GitHub adapter.
//
// Routing:
//   - kind=code   → /search/code
//   - kind=issues → /search/issues
//   - kind=repos  → /search/repositories (default)
//
// @MX:ANCHOR: [AUTO] Search — sole entry point for all GitHub fanout calls.
// @MX:REASON: Contract boundary; signature change ripples to FAN-001 + IDX-001
// + SYN-001. fan_in ≥ 3 (registry wrappedAdapter, fanout, tests).
// @MX:SPEC: SPEC-ADP-004
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// --- Input validation (REQ-ADP4-008) ---
	if strings.TrimSpace(q.Text) == "" || isWhitespaceOnly(q.Text) {
		return nil, &types.SourceError{
			Adapter:  "github",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	// Validate cursor: must be empty or a positive integer.
	page := 1
	if q.Cursor != "" {
		p, err := strconv.Atoi(q.Cursor)
		if err != nil || p <= 0 {
			return nil, &types.SourceError{
				Adapter:  "github",
				Category: types.CategoryPermanent,
				Cause:    ErrInvalidCursor,
			}
		}
		page = p
	}

	// Determine intent from Filters.
	intent := "repos" // default
	for _, f := range q.Filters {
		if f.Key == "kind" {
			intent = f.Value
			break
		}
	}
	if _, ok := validIntents[intent]; !ok {
		return nil, &types.SourceError{
			Adapter:  "github",
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidIntent,
		}
	}

	// Per-page clamping.
	perPage := q.MaxResults
	if perPage <= 0 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}

	// Build the search query string with qualifiers.
	queryStr := appendQualifiers(q.Text, q.Filters, intent)

	opts := &gogithub.SearchOptions{
		ListOptions: gogithub.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
	}

	retrievedAt := time.Now().UTC()

	switch intent {
	case "code":
		return a.searchCode(ctx, queryStr, opts, retrievedAt)
	case "issues":
		return a.searchIssues(ctx, queryStr, opts, retrievedAt)
	default: // "repos"
		return a.searchRepos(ctx, queryStr, opts, retrievedAt)
	}
}

func (a *Adapter) searchCode(ctx context.Context, query string, opts *gogithub.SearchOptions, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	result, resp, err := a.ghClient.Search.Code(ctx, query, opts)
	if err != nil {
		if se := categorizeError(err); se != nil {
			return nil, se
		}
		return nil, err
	}
	nextPage := 0
	if resp != nil {
		nextPage = resp.NextPage
	}
	docs, pErr := parseCodeResults(result, nextPage, retrievedAt)
	if pErr != nil {
		return nil, &types.SourceError{
			Adapter:  "github",
			Category: types.CategoryPermanent,
			Cause:    pErr,
		}
	}
	return docs, nil
}

func (a *Adapter) searchIssues(ctx context.Context, query string, opts *gogithub.SearchOptions, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	result, resp, err := a.ghClient.Search.Issues(ctx, query, opts)
	if err != nil {
		if se := categorizeError(err); se != nil {
			return nil, se
		}
		return nil, err
	}
	nextPage := 0
	if resp != nil {
		nextPage = resp.NextPage
	}
	docs, pErr := parseIssueResults(result, nextPage, retrievedAt)
	if pErr != nil {
		return nil, &types.SourceError{
			Adapter:  "github",
			Category: types.CategoryPermanent,
			Cause:    pErr,
		}
	}
	return docs, nil
}

func (a *Adapter) searchRepos(ctx context.Context, query string, opts *gogithub.SearchOptions, retrievedAt time.Time) ([]types.NormalizedDoc, error) {
	result, resp, err := a.ghClient.Search.Repositories(ctx, query, opts)
	if err != nil {
		if se := categorizeError(err); se != nil {
			return nil, se
		}
		return nil, err
	}
	nextPage := 0
	if resp != nil {
		nextPage = resp.NextPage
	}
	docs, pErr := parseRepoResults(result, nextPage, retrievedAt)
	if pErr != nil {
		return nil, &types.SourceError{
			Adapter:  "github",
			Category: types.CategoryPermanent,
			Cause:    pErr,
		}
	}
	return docs, nil
}

// appendQualifiers translates applicable Query.Filters into GitHub search
// qualifier suffixes appended to the base query text.
//
// Filter translation table (§6.4):
//
//	since       → created:>=<RFC3339>  (code, issues, repos)
//	language    → language:<value>     (code, repos)
//	repo        → repo:<value>         (issues)
//	org         → org:<value>          (code, issues, repos)
//	user        → user:<value>         (code, issues, repos)
//	topic       → topic:<value>        (repos)
//	state       → state:<open|closed>  (issues)
//	is_pr       → is:pr                (issues)
//
// Filters with non-applicable intent, empty values, or malformed values are
// silently dropped. Unknown keys are silently ignored.
//
// @MX:NOTE: [AUTO] 8-key filter-qualifier mapping per §6.4. Future contributors
// adding a new qualifier key should update this function and the table in
// SPEC-ADP-004 §6.4.
// @MX:SPEC: SPEC-ADP-004
func appendQualifiers(base string, filters []types.Filter, intent string) string {
	var qualifiers []string

	for _, f := range filters {
		if f.Key == "kind" || f.Value == "" {
			continue
		}
		q := buildQualifier(f.Key, f.Value, intent)
		if q != "" {
			qualifiers = append(qualifiers, q)
		}
	}

	if len(qualifiers) == 0 {
		return base
	}
	return base + " " + strings.Join(qualifiers, " ")
}

// buildQualifier converts a single filter key/value into a qualifier string.
// Returns "" when the filter is not applicable for the given intent or is malformed.
func buildQualifier(key, value, intent string) string {
	switch key {
	case "since":
		// Applicable to all intents; value must parse as RFC 3339.
		_, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return ""
		}
		return "created:>=" + value

	case "language":
		// Applicable to code and repos.
		if intent != "code" && intent != "repos" {
			return ""
		}
		return "language:" + value

	case "repo":
		// Applicable to issues only.
		if intent != "issues" {
			return ""
		}
		return "repo:" + value

	case "org":
		// Applicable to all intents.
		return "org:" + value

	case "user":
		// Applicable to all intents.
		return "user:" + value

	case "topic":
		// Applicable to repos only.
		if intent != "repos" {
			return ""
		}
		return "topic:" + value

	case "state":
		// Applicable to issues only; value must be "open" or "closed".
		if intent != "issues" {
			return ""
		}
		if value != "open" && value != "closed" {
			return ""
		}
		return "state:" + value

	case "is_pr":
		// Applicable to issues only.
		if intent != "issues" {
			return ""
		}
		return "is:pr"
	}
	return ""
}

// isWhitespaceOnly returns true if all runes in s are Unicode whitespace.
func isWhitespaceOnly(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
