// Package history provides the JSONL backend for query history persistence.
//
// SPEC-CLI-002 REQ-CLI2-010: JSONL append-only writer with FIFO eviction
// and time-based retention. One JSON object per line.
package history

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// JSONLBackend implements Backend using a JSONL file.
// Safe for concurrent use via mutex.
type JSONLBackend struct {
	mu            sync.Mutex
	path          string
	maxEntries    int
	retentionDays int
}

// NewJSONLBackend creates a new JSONL backend at the given path.
// maxEntries controls FIFO eviction (0 = unlimited).
// retentionDays controls time-based purge (0 = no purge).
func NewJSONLBackend(path string, maxEntries, retentionDays int) (*JSONLBackend, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("history: create dir: %w", err)
	}
	return &JSONLBackend{
		path:          path,
		maxEntries:    maxEntries,
		retentionDays: retentionDays,
	}, nil
}

// Write appends an entry to the JSONL file.
// Applies FIFO eviction and retention purge after write.
func (b *JSONLBackend) Write(entry Entry) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Append entry.
	f, err := os.OpenFile(b.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("history: open: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("history: marshal: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("history: write: %w", err)
	}

	// Apply eviction + retention.
	return b.evictLocked()
}

// List returns entries in reverse chronological order.
func (b *JSONLBackend) List(limit int) ([]Entry, error) {
	entries, err := b.readAll()
	if err != nil {
		return nil, err
	}

	// Sort reverse chronological.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

// Get retrieves a single entry by ID.
func (b *JSONLBackend) Get(id string) (*Entry, error) {
	entries, err := b.readAll()
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i], nil
		}
	}
	return nil, fmt.Errorf("history: entry %q not found", id)
}

// Search returns entries whose prompt contains the query string.
func (b *JSONLBackend) Search(query string) ([]Entry, error) {
	entries, err := b.readAll()
	if err != nil {
		return nil, err
	}

	var results []Entry
	lowerQuery := strings.ToLower(query)
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Prompt), lowerQuery) {
			results = append(results, e)
		}
	}

	// Sort reverse chronological.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})
	return results, nil
}

// Clear removes entries. If since is non-zero, only entries older than since are removed.
func (b *JSONLBackend) Clear(since time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if since.IsZero() {
		// Remove all.
		return os.Remove(b.path)
	}

	// Keep only entries newer than since.
	entries, err := b.readAllLocked()
	if err != nil {
		return err
	}

	var kept []Entry
	for _, e := range entries {
		if !e.Timestamp.Before(since) {
			kept = append(kept, e)
		}
	}

	return b.overwriteLocked(kept)
}

// readAll reads all entries from the file.
func (b *JSONLBackend) readAll() ([]Entry, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.readAllLocked()
}

// readAllLocked reads all entries. Caller must hold b.mu.
func (b *JSONLBackend) readAllLocked() ([]Entry, error) {
	f, err := os.Open(b.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("history: read: %w", err)
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("history: scan: %w", err)
	}
	return entries, nil
}

// evictLocked applies FIFO eviction and retention purge. Caller must hold b.mu.
func (b *JSONLBackend) evictLocked() error {
	entries, err := b.readAllLocked()
	if err != nil {
		return err
	}

	now := time.Now()
	changed := false

	// Retention purge.
	if b.retentionDays > 0 {
		cutoff := now.AddDate(0, 0, -b.retentionDays)
		var kept []Entry
		for _, e := range entries {
			if e.Timestamp.After(cutoff) {
				kept = append(kept, e)
			} else {
				changed = true
			}
		}
		entries = kept
	}

	// FIFO eviction.
	if b.maxEntries > 0 && len(entries) > b.maxEntries {
		// Sort chronological, keep newest.
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Timestamp.After(entries[j].Timestamp)
		})
		entries = entries[:b.maxEntries]
		changed = true
	}

	if changed {
		return b.overwriteLocked(entries)
	}
	return nil
}

// overwriteLocked writes all entries to the file. Caller must hold b.mu.
func (b *JSONLBackend) overwriteLocked(entries []Entry) error {
	f, err := os.Create(b.path)
	if err != nil {
		return fmt.Errorf("history: create: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, e := range entries {
		data, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("history: marshal: %w", err)
		}
		if _, err := w.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("history: write: %w", err)
		}
	}
	return w.Flush()
}
