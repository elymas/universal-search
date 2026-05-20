// Package access — unit tests for Fetcher lifecycle (New, Close, Shutdown).
//
// REQ-CACHE-015: Shutdown drains in-flight goroutines.
package access

import (
	"testing"
	"time"
)

func TestNew_NoPlaywright_Succeeds(t *testing.T) {
	t.Parallel()
	f, err := New(Options{PlaywrightEnabled: false, AllowPrivateNetworks: true})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestClose_Idempotent(t *testing.T) {
	t.Parallel()
	f, err := New(Options{AllowPrivateNetworks: true})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	// Close twice must not panic.
	_ = f.Close()
	_ = f.Close()
}

func TestShutdown_Idempotent(t *testing.T) {
	t.Parallel()
	f, err := New(Options{AllowPrivateNetworks: true})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	// Shutdown twice must not panic.
	_ = f.Shutdown(t.Context())
	_ = f.Shutdown(t.Context())
}

func TestShutdown_DrainsWriteThrough(t *testing.T) {
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

	// Trigger async write-through.
	content := &FetchedContent{URL: "http://example.com", Body: []byte("x"), FetchedAt: time.Now().UTC()}
	f.cacheWriteThrough(content)

	// Shutdown should drain the goroutine.
	if err := f.Shutdown(t.Context()); err != nil {
		t.Errorf("Shutdown() error: %v", err)
	}

	if lookup.upsertCalled.Load() != 1 {
		t.Errorf("Upsert called %d times after Shutdown, want 1", lookup.upsertCalled.Load())
	}
}

func TestFetcher_Logger_NilObs(t *testing.T) {
	t.Parallel()
	// When obs is noopObs, logger() returns nil.
	f, err := New(Options{AllowPrivateNetworks: true})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer f.Close()

	// logger() is called inside cacheWriteThrough error path — just verify no panic.
	logger := f.logger()
	if logger != nil {
		t.Error("logger() with noopObs must return nil")
	}
}
