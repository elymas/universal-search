// Package access — async cache write-through goroutine.
//
// REQ-CACHE-009: After Phase 3-5 success with CacheWriteThrough=true,
// spawn a goroutine to upsert the fetched content into the index.
// The goroutine is tracked via Fetcher.writeThroughWG for graceful shutdown.
package access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// cacheWriteThrough spawns an asynchronous goroutine that upserts the
// fetched content into the index via the IndexLookup port.
//
// The goroutine has its own derived context with a 30s timeout, independent
// of the caller's context. It is tracked by Fetcher.writeThroughWG so that
// Shutdown() can drain in-flight upserts.
//
// @MX:WARN: [AUTO] Async goroutine tracked by writeThroughWG.
// @MX:REASON: Removing goroutine tracking breaks the Shutdown drain contract
// (REQ-CACHE-015: Close must drain within 5s). The writeThroughWG.Add(1) /
// Done() pair MUST be balanced.
// @MX:SPEC: SPEC-CACHE-001
func (f *Fetcher) cacheWriteThrough(content *FetchedContent) {
	if !f.opts.CacheWriteThrough || f.opts.IndexLookup == nil || content == nil {
		return
	}

	f.writeThroughWG.Add(1)
	go func() {
		defer f.writeThroughWG.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		doc := buildNormalizedDoc(content)
		if err := f.opts.IndexLookup.Upsert(ctx, []types.NormalizedDoc{doc}); err != nil {
			logger := f.logger()
			if logger != nil {
				logger.LogAttrs(ctx, slog.LevelWarn, "access.cache_writethrough: upsert failed",
					slog.String("url", content.URL),
					slog.String("error", err.Error()),
				)
			}
		}
	}()
}

// buildNormalizedDoc converts a FetchedContent into a NormalizedDoc for upsert.
func buildNormalizedDoc(content *FetchedContent) types.NormalizedDoc {
	id := docID("access-cache", content.URL)
	return types.NormalizedDoc{
		ID:          id,
		SourceID:    "access-cache",
		URL:         content.URL,
		Body:        string(content.Body),
		DocType:     types.DocTypeOther,
		RetrievedAt: content.FetchedAt,
		Metadata:    map[string]any{"content_type": content.ContentType},
	}
}

// docID generates a stable, content-addressed document ID from sourceID and URL.
// Uses sha256 so upserts are idempotent across multiple fetch calls for the same URL.
func docID(sourceID, rawURL string) string {
	sum := sha256.Sum256([]byte(sourceID + "\x00" + rawURL))
	return "access-" + hex.EncodeToString(sum[:])[:16]
}
