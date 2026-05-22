// Package history_test validates the history backends.
// SPEC-CLI-002 REQ-CLI2-010/011: JSONL backend + async writer + FIFO eviction.
package history_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/usearch/history"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestEntry creates a minimal valid history entry for testing.
func newTestEntry(id string) history.Entry {
	return history.Entry{
		ID:            id,
		Timestamp:     time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
		Command:       "query",
		Prompt:        "test prompt " + id,
		Category:      "general",
		Adapters:      []string{"reddit", "hackernews"},
		Summary:       "test summary",
		Citations:     2,
		ExitCode:      0,
		LatencyMs:     150,
		CostUSD:       0.003,
		RequestID:     "req-" + id,
		SchemaVersion: 1,
	}
}

// --- JSONL Backend ---

// TestJSONLWriteAndList verifies basic write + list round-trip.
func TestJSONLWriteAndList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	entry := newTestEntry("abc123")
	require.NoError(t, backend.Write(entry))

	entries, err := backend.List(0)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "abc123", entries[0].ID)
	assert.Equal(t, "query", entries[0].Command)
	assert.Equal(t, "test prompt abc123", entries[0].Prompt)
	assert.Equal(t, 1, entries[0].SchemaVersion)
}

// TestJSONLListWithLimit verifies limit is respected.
func TestJSONLListWithLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	baseTime := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		e := newTestEntry("entry-" + string(rune('0'+i)))
		e.Timestamp = baseTime.Add(time.Duration(i) * time.Minute)
		require.NoError(t, backend.Write(e))
	}

	entries, err := backend.List(3)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	// Most recent first.
	assert.Equal(t, "entry-4", entries[0].ID)
}

// TestJSONLGetByID verifies Get retrieves a specific entry.
func TestJSONLGetByID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	require.NoError(t, backend.Write(newTestEntry("find-me")))
	require.NoError(t, backend.Write(newTestEntry("other")))

	entry, err := backend.Get("find-me")
	require.NoError(t, err)
	assert.Equal(t, "find-me", entry.ID)
	assert.Equal(t, "test prompt find-me", entry.Prompt)
}

// TestJSONLGetUnknownIDReturnsError verifies Get returns error for missing ID.
func TestJSONLGetUnknownIDReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	_, err = backend.Get("nonexistent")
	assert.Error(t, err)
}

// TestJSONLSearchSubstring verifies substring search on prompt field.
func TestJSONLSearchSubstring(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	e1 := newTestEntry("a")
	e1.Prompt = "golang generics overview"
	require.NoError(t, backend.Write(e1))

	e2 := newTestEntry("b")
	e2.Prompt = "rust async patterns"
	require.NoError(t, backend.Write(e2))

	e3 := newTestEntry("c")
	e3.Prompt = "golang generics tutorial"
	require.NoError(t, backend.Write(e3))

	results, err := backend.Search("golang generics")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

// TestJSONLClearRemovesAll verifies Clear with zero since removes everything.
func TestJSONLClearRemovesAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	require.NoError(t, backend.Write(newTestEntry("a")))
	require.NoError(t, backend.Write(newTestEntry("b")))

	require.NoError(t, backend.Clear(time.Time{}))

	entries, err := backend.List(0)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// TestJSONLClearSince verifies Clear with since removes only old entries.
func TestJSONLClearSince(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	// Old entry.
	old := newTestEntry("old")
	old.Timestamp = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, backend.Write(old))

	// Recent entry.
	recent := newTestEntry("recent")
	recent.Timestamp = time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	require.NoError(t, backend.Write(recent))

	cutoff := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, backend.Clear(cutoff))

	entries, err := backend.List(0)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "recent", entries[0].ID)
}

// TestJSONLFIFOEviction verifies max_entries FIFO eviction.
func TestJSONLFIFOEviction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 3, 90)
	require.NoError(t, err)

	baseTime := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		e := newTestEntry("e" + string(rune('0'+i)))
		e.Timestamp = baseTime.Add(time.Duration(i) * time.Minute)
		require.NoError(t, backend.Write(e))
	}

	entries, err := backend.List(0)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	// Oldest (e0, e1) evicted; newest 3 remain.
	assert.Equal(t, "e4", entries[0].ID)
	assert.Equal(t, "e3", entries[1].ID)
	assert.Equal(t, "e2", entries[2].ID)
}

// TestJSONLRetentionPurgesOldEntries verifies retention_days purge on write.
func TestJSONLRetentionPurgesOldEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	// 1-day retention.
	backend, err := history.NewJSONLBackend(path, 1000, 1)
	require.NoError(t, err)

	// Write an old entry.
	old := newTestEntry("old")
	old.Timestamp = time.Now().Add(-48 * time.Hour)
	require.NoError(t, backend.Write(old))

	// Write a new entry -- triggers retention purge.
	recent := newTestEntry("recent")
	recent.Timestamp = time.Now()
	require.NoError(t, backend.Write(recent))

	entries, err := backend.List(0)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "recent", entries[0].ID)
}

// TestJSONLCreateDir verifies backend creates parent directory.
func TestJSONLCreateDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	require.NoError(t, backend.Write(newTestEntry("test")))

	_, err = os.Stat(path)
	assert.NoError(t, err)
}

// TestJSONLListReverseChronological verifies entries are most-recent-first.
func TestJSONLListReverseChronological(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	e1 := newTestEntry("first")
	e1.Timestamp = time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	require.NoError(t, backend.Write(e1))

	e2 := newTestEntry("second")
	e2.Timestamp = time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	require.NoError(t, backend.Write(e2))

	e3 := newTestEntry("third")
	e3.Timestamp = time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)
	require.NoError(t, backend.Write(e3))

	entries, err := backend.List(0)
	require.NoError(t, err)
	assert.Equal(t, "third", entries[0].ID)
	assert.Equal(t, "second", entries[1].ID)
	assert.Equal(t, "first", entries[2].ID)
}
