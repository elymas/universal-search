// Package history_test validates the async writer.
package history_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/usearch/history"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAsyncWriterWriteAndDrain verifies async write + drain.
func TestAsyncWriterWriteAndDrain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	writer := history.NewAsyncWriter(backend, 10, nil)
	writer.Write(newTestEntry("async-1"))
	writer.Write(newTestEntry("async-2"))
	writer.Close(2 * time.Second)

	entries, err := backend.List(0)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

// TestAsyncWriterDropOnFullBuffer verifies entries are dropped when buffer is full.
func TestAsyncWriterDropOnFullBuffer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	var buf bytes.Buffer
	logger := history.NewWarnLogger(&buf)

	// Buffer size 1 -- will drop after first write fills it.
	writer := history.NewAsyncWriter(backend, 1, logger)
	for i := 0; i < 5; i++ {
		writer.Write(newTestEntry("drop-"+string(rune('0'+i))))
	}
	writer.Close(2 * time.Second)

	// Some entries should have been written, some dropped.
	entries, err := backend.List(0)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(entries), 5)

	// Warning about buffer full should appear.
	warnOutput := buf.String()
	assert.True(t, strings.Contains(warnOutput, "buffer full"), "expected buffer full warning, got: %s", warnOutput)
}

// TestAsyncWriterNilIsNoop verifies nil writer is safe.
func TestAsyncWriterNilIsNoop(t *testing.T) {
	history.WriteAsync(nil, newTestEntry("safe"))
	history.DrainAndClose(nil, time.Second)
	// No panic = pass.
}

// TestAsyncWriterCloseTimeout verifies Close respects the drain timeout.
func TestAsyncWriterCloseTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	writer := history.NewAsyncWriter(backend, 100, nil)
	writer.Write(newTestEntry("timeout-test"))

	start := time.Now()
	writer.Close(50 * time.Millisecond)
	elapsed := time.Since(start)

	// Should complete within reasonable time (not hang).
	assert.Less(t, elapsed, 2*time.Second, "Close should not hang")
}

// TestAsyncWriterConcurrentWrites verifies concurrent writes are safe.
func TestAsyncWriterConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	backend, err := history.NewJSONLBackend(path, 1000, 90)
	require.NoError(t, err)

	writer := history.NewAsyncWriter(backend, 100, nil)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			defer func() { done <- struct{}{} }()
			writer.Write(newTestEntry("concurrent-"+string(rune('0'+n))))
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
	writer.Close(2 * time.Second)

	entries, err := backend.List(0)
	require.NoError(t, err)
	assert.Len(t, entries, 10)
}
