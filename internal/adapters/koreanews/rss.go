// Package koreanews — RSS sub-source implementation.
// SPEC-ADP-009 REQ-ADP9-003 through REQ-ADP9-008: parallel RSS feed fetching
// via github.com/mmcdole/gofeed, NormalizedDoc mapping, per-feed timeouts.
package koreanews

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/elymas/universal-search/pkg/types"
	"github.com/mmcdole/gofeed"
	"golang.org/x/sync/errgroup"
)

// searchRSS fetches all configured RSS feeds concurrently, maps items to
// NormalizedDoc, and returns the merged (pre-dedup) slice plus any per-feed
// errors as a diagnostic slice (non-nil entries did not contribute docs).
//
// Per-feed error isolation: a single feed failure does NOT abort other feeds.
// All feed results are merged into one slice; per-feed errors are collected in
// errs (parallel to feeds) for caller diagnostics.
//
// @MX:WARN: [AUTO] errgroup with SetLimit — goroutine pool bounded by MaxParallelFeeds.
// @MX:REASON: Unbounded goroutine spawn on large feed lists would exhaust file descriptors.
// @MX:SPEC: SPEC-ADP-009
func searchRSS(
	ctx context.Context,
	adapterName string,
	opts Options,
	userAgent string,
	q types.Query,
) ([]types.NormalizedDoc, []error) {
	feeds := opts.RSSFeeds
	if len(feeds) == 0 {
		return nil, nil
	}

	// Guard: if ctx is already cancelled before launch, return immediately.
	// This avoids a deadlock where errgroup.SetLimit blocks indefinitely when
	// the parent ctx is already done (SPEC-FAN-001 §2.5 H18 pattern).
	select {
	case <-ctx.Done():
		return nil, nil
	default:
	}

	results := make([][]types.NormalizedDoc, len(feeds))
	errs := make([]error, len(feeds))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(opts.MaxParallelFeeds)

	for i, feedURL := range feeds {
		i, feedURL := i, feedURL // capture loop vars
		g.Go(func() error {
			docs, err := fetchFeed(gctx, adapterName, opts, feedURL, userAgent, q)
			results[i] = docs
			errs[i] = err
			return nil // never return error — per-feed isolation
		})
	}
	_ = g.Wait() // always nil since goroutines never return non-nil

	// Merge results into a single slice.
	var merged []types.NormalizedDoc
	for _, docs := range results {
		merged = append(merged, docs...)
	}

	return merged, errs
}

// fetchFeed fetches and parses a single RSS/Atom/JSON Feed URL.
// Returns NormalizedDoc slice and an error on failure.
// Per-feed timeout = min(opts.RSSPerFeedTimeout, time-until-parent-deadline).
func fetchFeed(
	ctx context.Context,
	adapterName string,
	opts Options,
	feedURL string,
	userAgent string,
	q types.Query,
) ([]types.NormalizedDoc, error) {
	// Calculate per-feed timeout.
	timeout := opts.RSSPerFeedTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}

	feedCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fp := gofeed.NewParser()
	fp.UserAgent = userAgent

	feed, err := fp.ParseURLWithContext(feedURL, feedCtx)
	if err != nil {
		return nil, fmt.Errorf("koreanews: fetch feed %q: %w", feedURL, err)
	}

	return feedItemsToDocs(adapterName, opts.NowFunc, feed, q), nil
}

// feedItemsToDocs maps parsed feed items to NormalizedDoc slice.
// Items are filtered against the query text (case-insensitive substring match
// in title or body). Empty query text means no filtering.
func feedItemsToDocs(
	adapterName string,
	nowFunc func() time.Time,
	feed *gofeed.Feed,
	q types.Query,
) []types.NormalizedDoc {
	now := nowFunc()
	queryLower := strings.ToLower(q.Text)

	docs := make([]types.NormalizedDoc, 0, len(feed.Items))
	for _, item := range feed.Items {
		if item == nil || item.Link == "" {
			continue
		}

		title := stripHTML(item.Title)
		body := stripHTML(itemBody(item))
		snippet := truncate(stripHTML(itemSnippet(item)), 200)

		// Filter: item must mention the query text in title or body (case-insensitive).
		if queryLower != "" {
			titleLower := strings.ToLower(title)
			bodyLower := strings.ToLower(body)
			if !strings.Contains(titleLower, queryLower) && !strings.Contains(bodyLower, queryLower) {
				continue
			}
		}

		published := itemPublished(item)
		lang := detectLang(title, body)

		doc := types.NormalizedDoc{
			ID:          fmt.Sprintf("rss-%s", item.Link),
			SourceID:    adapterName,
			URL:         item.Link,
			Title:       title,
			Body:        body,
			Snippet:     snippet,
			PublishedAt: published,
			RetrievedAt: now,
			Author:      itemAuthor(item),
			Score:       0.5,
			Lang:        lang,
			DocType:     types.DocTypeArticle,
			Metadata: map[string]any{
				"subsource": "rss",
				"feed_url":  feedLink(feed),
			},
		}
		doc.Hash = doc.CanonicalHash()
		docs = append(docs, doc)
	}
	return docs
}

// itemBody returns the richest body text available: Content > Description > "".
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

// itemPublished returns the item's published time or zero time.
func itemPublished(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return item.PublishedParsed.UTC()
	}
	if item.UpdatedParsed != nil {
		return item.UpdatedParsed.UTC()
	}
	return time.Time{}
}

// itemAuthor returns the item's author name or "".
func itemAuthor(item *gofeed.Item) string {
	if item.Author != nil && item.Author.Name != "" {
		return item.Author.Name
	}
	return ""
}

// feedLink returns the feed's channel-level link URL.
func feedLink(feed *gofeed.Feed) string {
	if feed != nil {
		return feed.Link
	}
	return ""
}

// detectLang uses the Korean heuristic; returns "ko" or "" (unknown).
func detectLang(title, body string) string {
	combined := title + " " + body
	return detectKorean(combined)
}
