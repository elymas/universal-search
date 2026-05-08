// Package index — deterministic doc_id generator.
// SPEC-IDX-001 REQ-IDX-014 (scope item k).
package index

import (
	"crypto/sha256"
	"encoding/hex"
)

// docID returns a 16-character lowercase hex identifier for a document.
// Formula: hex(sha256(sourceID + "\x00" + url))[:16]
//
// Properties:
//   - Pure: no I/O, no time, no randomness.
//   - Deterministic: same input → byte-equal output across goroutines, processes, replays.
//   - 16 hex chars = 64 bits of entropy (birthday collision at ~10^7 docs is ~10^-7).
//   - The NUL separator prevents prefix collision:
//     docID("redd", "ithttps://x") != docID("reddit", "https://x").
//
// @MX:ANCHOR: [AUTO] Every ingested doc gets its identifier here. fan_in >= 5 (Upsert, qdrant, meili, pg, tests).
// @MX:REASON: doc_id determinism is the load-bearing invariant for cross-store idempotency; changing the formula invalidates existing data.
// @MX:SPEC: SPEC-IDX-001
func docID(sourceID, url string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(sourceID))
	_, _ = h.Write([]byte("\x00"))
	_, _ = h.Write([]byte(url))
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:8]) // 8 bytes = 16 hex chars
}
