// Package access — IndexLookup port interface and noop stub.
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
// SPEC-IDX-001's *Index satisfies this interface. v0.1 ships noopIndexLookup
// for tests and nil-safe fast-paths in the cascade.
type IndexLookup interface {
	// LookupByURL returns a cached document for the given URL, or (nil, false, nil)
	// on a cache miss. Returns a non-nil error only on infrastructure failures.
	LookupByURL(ctx context.Context, url string) (*types.NormalizedDoc, bool, error)
	// Upsert stores or updates the given documents in the index.
	Upsert(ctx context.Context, docs []types.NormalizedDoc) error
}

// noopIndexLookup is a zero-cost stub that always returns a cache miss.
// Used when Options.IndexLookup is nil (Phase 1 skipped).
type noopIndexLookup struct{}

func (noopIndexLookup) LookupByURL(_ context.Context, _ string) (*types.NormalizedDoc, bool, error) {
	return nil, false, nil
}

func (noopIndexLookup) Upsert(_ context.Context, _ []types.NormalizedDoc) error {
	return nil
}
