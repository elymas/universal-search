// Package history provides the async writer for non-blocking history writes.
//
// SPEC-CLI-002 REQ-CLI2-010: History write SHALL be asynchronous and SHALL NOT
// block the CLI's exit. Write errors SHALL be logged to stderr at WARN level.
package history

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// AsyncWriter wraps a Backend with a buffered channel for non-blocking writes.
// Close drains pending entries with a configurable grace period.
type AsyncWriter struct {
	backend Backend
	ch      chan Entry
	done    chan struct{}
	wg      sync.WaitGroup
	warnLog *log.Logger
}

// NewAsyncWriter creates an async writer with the given buffer size.
// warnLog receives write error warnings; defaults to stderr if nil.
func NewAsyncWriter(backend Backend, bufferSize int, warnLog *log.Logger) *AsyncWriter {
	if warnLog == nil {
		warnLog = log.New(os.Stderr, "usearch: ", log.LstdFlags)
	}
	w := &AsyncWriter{
		backend: backend,
		ch:      make(chan Entry, bufferSize),
		done:    make(chan struct{}),
		warnLog: warnLog,
	}
	w.wg.Add(1)
	go w.process()
	return w
}

// Write queues an entry for async write. Never blocks (drops on full buffer).
func (w *AsyncWriter) Write(entry Entry) {
	select {
	case w.ch <- entry:
	default:
		w.warnLog.Printf("history: buffer full, dropping entry %s", entry.ID)
	}
}

// Close drains pending entries and stops the writer.
// Blocks for up to drainTimeout, then abandons remaining entries.
func (w *AsyncWriter) Close(drainTimeout time.Duration) {
	close(w.done)

	// Drain with timeout.
	deadline := time.After(drainTimeout)
	drained := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(drained)
	}()

	select {
	case <-drained:
		return
	case <-deadline:
		w.warnLog.Printf("history: drain timed out after %s, some entries may be lost", drainTimeout)
	}
}

// process reads from the channel and writes to the backend.
func (w *AsyncWriter) process() {
	defer w.wg.Done()
	for {
		select {
		case entry, ok := <-w.ch:
			if !ok {
				return
			}
			if err := w.backend.Write(entry); err != nil {
				w.warnLog.Printf("history: write error for entry %s: %v", entry.ID, err)
			}
		case <-w.done:
			// Drain remaining entries synchronously.
			for {
				select {
				case entry, ok := <-w.ch:
					if !ok {
						return
					}
					if err := w.backend.Write(entry); err != nil {
						w.warnLog.Printf("history: write error for entry %s: %v", entry.ID, err)
					}
				default:
					return
				}
			}
		}
	}
}

// WriteAsync is a convenience function for fire-and-forget history writes.
// If writer is nil, the write is silently skipped.
// @MX:ANCHOR: [AUTO] Async history write entry point.
// @MX:REASON: Called from query.go and future deep.go after successful invocations.
func WriteAsync(w *AsyncWriter, entry Entry) {
	if w == nil {
		return
	}
	w.Write(entry)
}

// DrainAndClose drains pending writes and closes the async writer.
// If writer is nil, this is a no-op.
func DrainAndClose(w *AsyncWriter, timeout time.Duration) {
	if w == nil {
		return
	}
	w.Close(timeout)
}

// NewWarnLogger creates a logger that writes to the given writer.
func NewWarnLogger(w io.Writer) *log.Logger {
	if w == nil {
		w = os.Stderr
	}
	return log.New(w, "usearch: ", 0)
}

// FormatWarnLog formats a warning message for stderr output.
func FormatWarnLog(format string, args ...interface{}) string {
	return "usearch: " + fmt.Sprintf(format, args...)
}
