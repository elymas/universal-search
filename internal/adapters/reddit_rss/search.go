// Package redditrss — Search implementation for the Reddit RSS adapter.
// SPEC-ADP-001b REQ-ADP1B-005,007..016.
//
// Key design: explicit http.Request + client.Do before gofeed.Parse so that
// non-2xx status codes are classified precisely (429, 403, 4xx, 5xx) — gofeed's
// ParseURLWithContext hides the HTTP status in a generic error (plan.md §Tech Notes).
package redditrss

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/elymas/universal-search/pkg/types"
	"github.com/mmcdole/gofeed"
)

// Search executes a Reddit search RSS query and returns NormalizedDoc results.
//
// URL format: <BaseURL>?q=<encoded_text>&sort=relevance
// Empty/whitespace query text returns CategoryPermanent without a network call (REQ-ADP1B-016).
// Context cancellation is honoured at every blocking point (REQ-ADP1B-010).
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// REQ-ADP1B-016: empty/whitespace query → permanent error, no network call.
	if strings.TrimSpace(q.Text) == "" {
		return nil, &types.SourceError{
			Adapter:  a.Name(),
			Category: types.CategoryPermanent,
			Cause:    errors.New("empty or whitespace query text"),
		}
	}

	// REQ-ADP1B-010: check if context is already done before any I/O.
	select {
	case <-ctx.Done():
		return nil, ctxError(a.Name(), ctx)
	default:
	}

	// Build the search URL.
	searchURL := buildSearchURL(a.opts.BaseURL, q.Text)

	// Apply per-request timeout = min(opts.Timeout, remaining-ctx-deadline).
	reqCtx, cancel := requestContext(ctx, a.opts.Timeout)
	defer cancel()

	// Explicit HTTP request (not gofeed.ParseURLWithContext) to preserve
	// status-code fidelity for 429 / 403 / 4xx / 5xx mapping.
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  a.Name(),
			Category: types.CategoryPermanent,
			Cause:    err,
		}
	}
	req.Header.Set("User-Agent", a.userAgent)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		// Classify context errors separately from network errors.
		return nil, classifyHTTPErr(a.Name(), err, ctx)
	}
	defer func() { _ = resp.Body.Close() }()

	// Non-2xx: classify by status code.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		retryAfter := resp.Header.Get("Retry-After")
		cause := fmt.Errorf("HTTP %d", resp.StatusCode)
		return nil, categorizeStatus(resp.StatusCode, retryAfter, cause)
	}

	// 2xx: parse RSS body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &types.SourceError{
			Adapter:  a.Name(),
			Category: types.CategoryTransient,
			Cause:    fmt.Errorf("read body: %w", err),
		}
	}

	fp := gofeed.NewParser()
	feed, err := fp.ParseString(string(body))
	if err != nil {
		// REQ-ADP1B-015: parse failure on 2xx → CategoryTransient.
		return nil, &types.SourceError{
			Adapter:  a.Name(),
			Category: types.CategoryTransient,
			Cause:    fmt.Errorf("parse feed: %w", err),
		}
	}

	return feedItemsToDocs(a.Name(), a.opts.NowFunc, feed), nil
}

// buildSearchURL constructs the RSS search URL from the base and query text.
// REQ-ADP1B-005: emits q=<encoded>&sort=relevance only (no t= param in v0.1).
func buildSearchURL(base, text string) string {
	params := url.Values{}
	params.Set("q", text)
	params.Set("sort", "relevance")
	return base + "?" + params.Encode()
}

// requestContext creates a child context with the shorter of: opts.Timeout and
// remaining time until the parent's deadline. Mirrors koreanews fetchFeed logic.
func requestContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	return context.WithTimeout(ctx, timeout)
}

// classifyHTTPErr converts an http.Client.Do error into a *types.SourceError.
// Context deadline → CategoryTransient; other cancel/network → CategoryUnavailable.
func classifyHTTPErr(adapterName string, err error, ctx context.Context) *types.SourceError {
	// Check context state after the call.
	ctxErr := ctx.Err()
	if errors.Is(err, context.DeadlineExceeded) || ctxErr == context.DeadlineExceeded {
		return &types.SourceError{
			Adapter:  adapterName,
			Category: types.CategoryTransient,
			Cause:    err,
		}
	}
	return &types.SourceError{
		Adapter:  adapterName,
		Category: types.CategoryUnavailable,
		Cause:    err,
	}
}

// ctxError returns the appropriate *types.SourceError for a done context.
// DeadlineExceeded → CategoryTransient; other cancellation → CategoryUnavailable.
func ctxError(adapterName string, ctx context.Context) *types.SourceError {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &types.SourceError{
			Adapter:  adapterName,
			Category: types.CategoryTransient,
			Cause:    ctx.Err(),
		}
	}
	return &types.SourceError{
		Adapter:  adapterName,
		Category: types.CategoryUnavailable,
		Cause:    ctx.Err(),
	}
}

// feedItemsToDocs maps parsed gofeed items to NormalizedDoc.
// REQ-ADP1B-007,008,009: skip items with empty link; DocTypePost; Score=0.5.
// Unlike koreanews, reddit-rss does NOT filter by query text — the server
// already filtered results. No lang detection; no metadata extensions.
func feedItemsToDocs(adapterName string, nowFunc func() time.Time, feed *gofeed.Feed) []types.NormalizedDoc {
	now := nowFunc()
	docs := make([]types.NormalizedDoc, 0, len(feed.Items))

	for _, item := range feed.Items {
		// REQ-ADP1B-009: skip items with no link.
		if item == nil || item.Link == "" {
			continue
		}

		title := stripHTML(item.Title)
		body := stripHTML(itemBody(item))
		snippet := truncate(stripHTML(itemSnippet(item)), 200)

		doc := types.NormalizedDoc{
			ID:          fmt.Sprintf("rss-%s", item.Link),
			SourceID:    adapterName,
			URL:         item.Link,
			Title:       title,
			Body:        body,
			Snippet:     snippet,
			PublishedAt: itemPublished(item),
			RetrievedAt: now,
			Author:      itemAuthor(item),
			Score:       0.5, // REQ-ADP1B-008: neutral constant
			DocType:     types.DocTypePost,
		}
		doc.Hash = doc.CanonicalHash()
		docs = append(docs, doc)
	}
	return docs
}

// itemBody returns the richest body text: Content > Description > "".
func itemBody(item *gofeed.Item) string {
	if item.Content != "" {
		return item.Content
	}
	return item.Description
}

// itemSnippet returns a short excerpt: Description > Content > "".
func itemSnippet(item *gofeed.Item) string {
	if item.Description != "" {
		return item.Description
	}
	return item.Content
}

// itemPublished returns the item's UTC published time.
// Falls back to UpdatedParsed, then zero time (REQ-ADP1B-007, EC2).
func itemPublished(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return item.PublishedParsed.UTC()
	}
	if item.UpdatedParsed != nil {
		return item.UpdatedParsed.UTC()
	}
	return time.Time{}
}

// itemAuthor returns the item's author name or "" (REQ-ADP1B-007).
func itemAuthor(item *gofeed.Item) string {
	if item.Author != nil && item.Author.Name != "" {
		return item.Author.Name
	}
	return ""
}
