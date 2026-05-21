// Package access — IndexLookup port interface.
//
// SPEC-CACHE-001 §2.1(e): IndexLookup interface decouples the access
// package from SPEC-IDX-001's concrete *index.Index type.
package access

import (
	"context"

	"github.com/elymas/universal-search/pkg/types"
)

// IndexLookup is the port that Phase 1 uses to look up cached content by URL
// and that the write-through goroutine uses to upsert freshly-fetched content.
//
// SPEC-IDX-001's *Index satisfies this interface.
type IndexLookup interface {
	// LookupByURL returns a cached document for the given URL, or (nil, false, nil)
	// on a cache miss. Returns a non-nil error only on infrastructure failures.
	LookupByURL(ctx context.Context, url string) (*types.NormalizedDoc, bool, error)
	// Upsert stores or updates the given documents in the index.
	Upsert(ctx context.Context, docs []types.NormalizedDoc) error
}
