// Package access — Phase 1: local index lookup.
//
// REQ-CACHE-002: Phase 1 calls IndexLookup.LookupByURL; on hit returns
// FetchedContent built from NormalizedDoc.Body; on miss returns ErrPhaseNotApplicable.
package access

import (
	"context"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// phase1Index looks up the URL in the local index (SPEC-IDX-001 port).
// Returns (content, nil) on cache hit, or (nil, ErrPhaseNotApplicable) on miss.
func phase1Index(ctx context.Context, idx IndexLookup, rawURL string) (*FetchedContent, error) {
	if idx == nil {
		return nil, ErrPhaseNotApplicable
	}

	doc, found, err := idx.LookupByURL(ctx, rawURL)
	if err != nil {
		// Infrastructure error → treat as miss, escalate.
		return nil, ErrPhaseMiss
	}
	if !found || doc == nil {
		return nil, ErrPhaseMiss
	}

	return docToFetchedContent(doc, rawURL), nil
}

// docToFetchedContent converts a NormalizedDoc from the index into a
// FetchedContent suitable for Phase 1 success.
func docToFetchedContent(doc *types.NormalizedDoc, rawURL string) *FetchedContent {
	url := doc.URL
	if url == "" {
		url = rawURL
	}
	return &FetchedContent{
		URL:         url,
		Body:        []byte(doc.Body),
		ContentType: "text/html; charset=utf-8",
		StatusCode:  0, // Phase 1 hit: no HTTP status
		FetchedAt:   time.Now().UTC(),
	}
}
