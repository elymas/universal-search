// Package access — unit tests for async cache write-through goroutine.
//
// REQ-CACHE-009: cacheWriteThrough spawns async upsert tracked by writeThroughWG.
package access

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// countingIndexLookup counts upsert calls.
type countingIndexLookup struct {
	upsertCalled atomic.Int32
}

func (c *countingIndexLookup) LookupByURL(_ context.Context, _ string) (*types.NormalizedDoc, bool, error) {
	return nil, false, nil
}

func (c *countingIndexLookup) Upsert(_ context.Context, _ []types.NormalizedDoc) error {
	c.upsertCalled.Add(1)
	return nil
}

func TestCacheWriteThrough_Disabled_NoUpsert(t *testing.T) {
	t.Parallel()
	lookup := &countingIndexLookup{}
	f, err := New(Options{
		AllowPrivateNetworks: true,
		CacheWriteThrough:    false,
		IndexLookup:          lookup,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer f.Close()

	content := &FetchedContent{URL: "http://example.com", Body: []byte("ok"), FetchedAt: time.Now().UTC()}
	f.cacheWriteThrough(content)
	f.writeThroughWG.Wait()

	if lookup.upsertCalled.Load() != 0 {
		t.Errorf("Upsert called %d times, want 0 when CacheWriteThrough=false",
			lookup.upsertCalled.Load())
	}
}

func TestCacheWriteThrough_Enabled_CallsUpsert(t *testing.T) {
	t.Parallel()
	lookup := &countingIndexLookup{}
	f, err := New(Options{
		AllowPrivateNetworks: true,
		CacheWriteThrough:    true,
		IndexLookup:          lookup,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer f.Close()

	content := &FetchedContent{URL: "http://example.com/page", Body: []byte("<html>ok</html>"), FetchedAt: time.Now().UTC()}
	f.cacheWriteThrough(content)
	f.writeThroughWG.Wait() // wait for goroutine

	if lookup.upsertCalled.Load() != 1 {
		t.Errorf("Upsert called %d times, want 1", lookup.upsertCalled.Load())
	}
}

func TestCacheWriteThrough_NilContent_NoUpsert(t *testing.T) {
	t.Parallel()
	lookup := &countingIndexLookup{}
	f, err := New(Options{
		AllowPrivateNetworks: true,
		CacheWriteThrough:    true,
		IndexLookup:          lookup,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer f.Close()

	f.cacheWriteThrough(nil) // nil content
	f.writeThroughWG.Wait()

	if lookup.upsertCalled.Load() != 0 {
		t.Errorf("Upsert called %d times with nil content, want 0", lookup.upsertCalled.Load())
	}
}

func TestCacheWriteThrough_NilIndex_NoUpsert(t *testing.T) {
	t.Parallel()
	f, err := New(Options{
		AllowPrivateNetworks: true,
		CacheWriteThrough:    true,
		IndexLookup:          nil, // no index
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer f.Close()

	content := &FetchedContent{URL: "http://example.com", Body: []byte("ok"), FetchedAt: time.Now().UTC()}
	f.cacheWriteThrough(content)
	f.writeThroughWG.Wait()
	// No panic, no upsert.
}

func TestDocID_Stable(t *testing.T) {
	t.Parallel()
	id1 := docID("access-cache", "http://example.com/page")
	id2 := docID("access-cache", "http://example.com/page")
	if id1 != id2 {
		t.Errorf("docID must be stable: %q != %q", id1, id2)
	}
}

func TestDocID_Unique_DifferentURLs(t *testing.T) {
	t.Parallel()
	id1 := docID("access-cache", "http://example.com/a")
	id2 := docID("access-cache", "http://example.com/b")
	if id1 == id2 {
		t.Error("docID must be unique for different URLs")
	}
}

func TestDocID_HasPrefix(t *testing.T) {
	t.Parallel()
	id := docID("access-cache", "http://example.com/")
	if len(id) == 0 {
		t.Error("docID must not be empty")
	}
}

func TestBuildNormalizedDoc_Fields(t *testing.T) {
	t.Parallel()
	content := &FetchedContent{
		URL:         "http://example.com/page",
		Body:        []byte("<html>test</html>"),
		ContentType: "text/html",
		FetchedAt:   time.Now().UTC(),
	}
	doc := buildNormalizedDoc(content)
	if doc.URL != content.URL {
		t.Errorf("doc.URL = %q, want %q", doc.URL, content.URL)
	}
	if doc.Body != string(content.Body) {
		t.Errorf("doc.Body = %q, want %q", doc.Body, string(content.Body))
	}
	if doc.SourceID != "access-cache" {
		t.Errorf("doc.SourceID = %q, want access-cache", doc.SourceID)
	}
	if doc.DocType != types.DocTypeOther {
		t.Errorf("doc.DocType = %q, want other", doc.DocType)
	}
}
