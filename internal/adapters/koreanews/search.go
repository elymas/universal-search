// Package koreanews — composite Search dispatcher.
// SPEC-ADP-009 REQ-ADP9-001: dispatches to enabled sub-sources, merges results,
// deduplicates, and returns sorted by Score desc.
package koreanews

import (
	"context"
	"errors"
	"sort"
	"strings"
	"unicode"

	"github.com/elymas/universal-search/pkg/types"
)

// Search executes the composite Korean-news search across all enabled sub-sources.
//
// Dispatch order:
//  1. Validate query (non-empty, non-whitespace).
//  2. Collect enabled sub-sources (RSS / KNC / Daum).
//  3. Run each sub-source sequentially (RSS uses internal parallelism per feed).
//  4. Merge all results, apply deduplication, sort by Score desc.
//
// Errors from stub sub-sources (Daum, KNC when unavailable) are silently
// dropped if at least one other sub-source succeeded. If ALL sub-sources fail,
// the first non-nil error is returned wrapped in *types.SourceError.
//
// @MX:ANCHOR: [AUTO] Composite Search entry; callers: types.Adapter interface,
// registry wrappedAdapter, fanout, integration tests
// @MX:REASON: fan_in >= 3; sole method that aggregates sub-source results
// @MX:SPEC: SPEC-ADP-009
func (a *Adapter) Search(ctx context.Context, q types.Query) ([]types.NormalizedDoc, error) {
	// Validate query.
	if isAllWhitespace(q.Text) {
		return nil, &types.SourceError{
			Adapter:  a.Name(),
			Category: types.CategoryPermanent,
			Cause:    ErrInvalidQuery,
		}
	}

	// Validate RSS config when RSS is enabled.
	if a.opts.RSSEnabled && len(a.opts.RSSFeeds) == 0 {
		return nil, &types.SourceError{
			Adapter:  a.Name(),
			Category: types.CategoryPermanent,
			Cause:    ErrEmptyRSSFeedList,
		}
	}

	var (
		allDocs []types.NormalizedDoc
		firstErr error
		anySuccess bool
	)

	// RSS sub-source.
	if a.opts.RSSEnabled {
		docs, feedErrs := searchRSS(ctx, a.Name(), a.opts, a.userAgent, q)
		allDocs = append(allDocs, docs...)
		// Per-feed errors are non-fatal if some feeds succeeded.
		if len(docs) > 0 {
			anySuccess = true
		} else {
			// Capture first non-nil feed error as candidate.
			for _, e := range feedErrs {
				if e != nil && firstErr == nil {
					firstErr = &types.SourceError{
						Adapter:  a.Name(),
						Category: types.CategoryTransient,
						Cause:    e,
					}
				}
			}
		}
	}

	// KNC sub-source.
	if a.opts.KNCEnabled {
		docs, err := searchKNC(ctx, a.Name(), a.opts, a.httpClient, a.userAgent, q)
		if err == nil {
			allDocs = append(allDocs, docs...)
			anySuccess = true
		} else if firstErr == nil {
			firstErr = err
		}
	}

	// Daum sub-source (always-stub; appends nothing, records first error if needed).
	if a.opts.DaumEnabled {
		_, err := searchDaum(ctx, a.Name())
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// If no sub-source produced docs and we have an error, return it.
	if !anySuccess && firstErr != nil {
		return nil, firstErr
	}

	// Check context cancellation.
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &types.SourceError{
				Adapter:  a.Name(),
				Category: types.CategoryTransient,
				Cause:    err,
			}
		}
		return nil, &types.SourceError{
			Adapter:  a.Name(),
			Category: types.CategoryUnavailable,
			Cause:    err,
		}
	}

	// Deduplicate and sort.
	deduped, _ := dedupDocs(allDocs)
	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].Score > deduped[j].Score
	})

	return deduped, nil
}

// isAllWhitespace returns true when s is empty or contains only Unicode whitespace.
func isAllWhitespace(s string) bool {
	if s == "" {
		return true
	}
	return strings.IndexFunc(s, func(r rune) bool {
		return !unicode.IsSpace(r)
	}) == -1
}
