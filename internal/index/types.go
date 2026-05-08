// Package index — public types for the hybrid index layer.
// SPEC-IDX-001 REQ-IDX-001 (scope item c).
package index

import (
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// IndexQuery is the value type for retrieval requests.
// All filter fields are optional; zero values mean "no filter".
type IndexQuery struct {
	// Text is the free-text query. If empty, stores perform filter-only queries.
	Text string
	// Lang filters results to a specific BCP-47 language code. Empty = any.
	Lang string
	// DocTypes restricts results to the given document types. Nil = any.
	DocTypes []types.DocType
	// Since and Until filter by published_at range. Zero = unbounded.
	Since time.Time
	Until time.Time
	// SourceID restricts results to a single adapter source. Empty = any.
	SourceID string
	// TeamID is reserved for SPEC-IDX-004 multi-tenancy enforcement.
	// In v0.1: empty = all rows; non-empty = filter (excludes NULL rows, so empty result).
	TeamID string
	// MaxResults caps the fused result set. Default 50 when zero.
	MaxResults int
}

// IndexResult is the return value of Index.Search.
type IndexResult struct {
	// Docs is the fused, ranked slice of matching documents.
	Docs []types.NormalizedDoc
	// PerStoreErrors holds per-store errors (soft-fail discipline per §2.6).
	// A nil entry means the store succeeded. "validation" key is not used here.
	PerStoreErrors map[string]error
	// Stats carries latency and cardinality information.
	Stats SearchStats
}

// UpsertResult is the return value of Index.Upsert.
type UpsertResult struct {
	// Inserted is the number of new documents (not already present by doc_id).
	Inserted int
	// Skipped is the count of docs rejected by validation (REQ-IDX-004).
	Skipped int
	// PerStoreErrors holds per-store write errors; "validation" key collects
	// pre-store validation rejections as a slice encoded in the error message.
	PerStoreErrors map[string]error
	// Stats carries timing and doc-count information.
	Stats UpsertStats
}

// SearchStats carries per-call performance telemetry for Index.Search.
type SearchStats struct {
	// StoreLatencies maps store name → wall-clock duration of that store's query.
	StoreLatencies map[string]time.Duration
	// FusionLatency is the wall-clock time spent in fuseRRF.
	FusionLatency time.Duration
	// PerStoreCounts maps store name → number of results returned by that store.
	PerStoreCounts map[string]int
	// FusedCount is the total number of fused documents before MaxResults clamping.
	FusedCount int
	// ElapsedSeconds is the total wall-clock time for the Search call.
	ElapsedSeconds float64
}

// UpsertStats carries per-call performance telemetry for Index.Upsert.
type UpsertStats struct {
	// DocCount is the total number of documents that entered the pipeline
	// (valid + invalid; Skipped subtracted in UpsertResult.Skipped).
	DocCount int
	// SkippedCount is the number of docs rejected by validation.
	SkippedCount int
	// SuccessCount is the number of per-store write operations that succeeded.
	SuccessCount int
	// ErrorCount is the number of per-store write operations that failed.
	ErrorCount int
	// PerStoreLatencies maps store name → aggregate write latency across all batches.
	PerStoreLatencies map[string]time.Duration
	// ElapsedSeconds is the total wall-clock time for the Upsert call.
	ElapsedSeconds float64
}
