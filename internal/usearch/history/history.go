// Package history provides query/deep invocation persistence for the usearch CLI.
//
// SPEC-CLI-002 REQ-CLI2-010: History backend with async write, FIFO eviction,
// and time-based retention. Default backend is JSONL; SQLite behind build tag.
package history

import "time"

// Entry represents a single history record for a query or deep invocation.
// REQ-CLI2-010: schema_version 1, all fields.
type Entry struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	Command       string    `json:"command"`       // "query" or "deep"
	Prompt        string    `json:"prompt"`
	Category      string    `json:"category"`      // intent classification
	Adapters      []string  `json:"adapters"`
	Summary       string    `json:"summary"`
	Citations     int        `json:"citations"`
	ExitCode      int       `json:"exit_code"`
	LatencyMs     int64     `json:"latency_ms"`
	CostUSD       float64   `json:"cost_usd"`
	RequestID     string    `json:"request_id"`
	SchemaVersion int       `json:"schema_version"`
}

// Backend is the interface for history persistence.
type Backend interface {
	// Write appends an entry. Must be safe for concurrent use.
	Write(entry Entry) error

	// List returns recent entries in reverse chronological order.
	// limit caps the number returned; 0 means unlimited.
	List(limit int) ([]Entry, error)

	// Get retrieves a single entry by ID.
	Get(id string) (*Entry, error)

	// Search returns entries matching the query string.
	Search(query string) ([]Entry, error)

	// Clear removes all entries. If since is non-zero, only entries
	// older than since are removed.
	Clear(since time.Time) error
}
