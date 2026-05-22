package idx5

import (
	"log/slog"
	"sync"
)

// WriteFunc is the signature for the async write callback.
// In production, this writes to Qdrant + PG answer_cache.
type WriteFunc func(docID string) error

// Writeback handles fire-and-forget async cache writes.
// REQ-IDX5-006: on fanout MISS path, async write to Qdrant + PG.
//
// @MX:WARN: [AUTO] Spawns goroutines for async writes. Panic recovery is mandatory.
// @MX:REASON: Response latency must not be impacted by write failures.
// @MX:SPEC: SPEC-IDX-005
type Writeback struct {
	write WriteFunc
	wg    sync.WaitGroup
}

// NewWriteback creates a new Writeback with the given write function.
func NewWriteback(write WriteFunc) *Writeback {
	return &Writeback{write: write}
}

// FireAndForget enqueues an async write. Does not block the caller.
// REQ-IDX5-006: fire-and-forget goroutine with panic recovery.
func (wb *Writeback) FireAndForget(docID, teamID, queryText, category, responseJSON string, similarity float64) {
	wb.wg.Add(1)
	go func() {
		defer wb.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("idx5: writeback panic recovered",
					"doc_id", docID,
					"team_id", teamID,
					"panic", r,
				)
			}
		}()
		if err := wb.write(docID); err != nil {
			slog.Error("idx5: writeback failed",
				"doc_id", docID,
				"team_id", teamID,
				"error", err,
			)
		}
	}()
}

// Wait blocks until all pending writes complete. Used for testing.
func (wb *Writeback) Wait() {
	wb.wg.Wait()
}
