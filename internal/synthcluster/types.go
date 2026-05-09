// Package synthcluster provides pre-synthesis near-duplicate clustering.
//
// SPEC-SYN-003: SimHash-first, optionally embedding-refined clustering stage.
// Reduces synthesizer input redundancy while preserving every input doc_id
// under the SPEC-SYN-002 traceability contract.
//
// Public surface:
//
//	Cluster(ctx, docs, opts) ([]NormalizedDoc, Stats, error)
//
// Insertion point: cmd/usearch/query.go between fanout result and synth.Synthesize.
package synthcluster

import (
	"context"

	"github.com/elymas/universal-search/internal/embedder"
	"github.com/elymas/universal-search/pkg/types"
)

// Mode controls the clustering algorithm for a Cluster call.
//
// @MX:NOTE: [AUTO] DEDUPCLUSTER_MODE env var maps to these constants.
// Default is ModeSimhashOnly (no embedding RTT in query path). ModeHybrid
// is opt-in. ModeOff is a pass-through for debugging / emergency rollback.
type Mode string

const (
	// ModeSimhashOnly runs SimHash + Hamming filter only. Default.
	ModeSimhashOnly Mode = "simhash_only"
	// ModeHybrid runs SimHash then (when embedder is reachable) cosine refinement.
	ModeHybrid Mode = "hybrid"
	// ModeOff bypasses clustering entirely — pass-through.
	ModeOff Mode = "off"
)

// Embedder is the interface consumed by synthcluster for embedding refinement.
// Only internal/embedder.Client satisfies this interface in production;
// tests use a mock implementation.
//
// @MX:ANCHOR: [AUTO] Embedder interface for hybrid mode refinement; callers: Cluster, tests
// @MX:REASON: fan_in >= 3; decouples synthcluster from embedder.Client concrete type
type Embedder interface {
	Embed(ctx context.Context, req embedder.Request) (embedder.Response, error)
}

// Options configures a single Cluster call.
// All fields are safe to inject from tests for threshold control.
//
// @MX:NOTE: [AUTO] Env-var defaults: DEDUPCLUSTER_MODE, DEDUPCLUSTER_HAMMING_THRESHOLD,
// DEDUPCLUSTER_COSINE_THRESHOLD, DEDUPCLUSTER_EMBEDDING_TIMEOUT_MS.
// Options.Embedder nil means hybrid mode will fall back (treated as unreachable).
type Options struct {
	// Mode selects the clustering algorithm. Default: ModeSimhashOnly.
	Mode Mode

	// HammingThreshold is the maximum Hamming distance for SimHash candidate pairs.
	// Default 4, range [0,64]. Lower = stricter dedup.
	//
	// @MX:NOTE: [AUTO] Algorithm choice: threshold=4 matches Manku et al. (WWW 2007)
	// recommendation for near-duplicate detection at web scale. Korean char-3-shingle
	// SimHash may require tuning; SPEC-EVAL-003 will provide empirical floor.
	HammingThreshold int

	// CosineThreshold is the minimum cosine similarity for hybrid-mode pair confirmation.
	// Default 0.92, range [0.0, 1.0]. BGE-M3 cross-lingual guidance (BAAI 2024 §5).
	//
	// @MX:NOTE: [AUTO] 0.92 derived from BGE-M3 paper cross-lingual cosine threshold
	// guidance. Demotes pairs scoring below this after SimHash prefilter.
	CosineThreshold float64

	// EmbeddingTimeoutMs is the per-call timeout for the embedder in hybrid mode.
	// Default 1500 ms (SPEC-SYN-003 §2.1.c). 0 means use 1500 ms.
	EmbeddingTimeoutMs int

	// Embedder is the embedding client used in ModeHybrid. May be nil (fallback).
	Embedder Embedder

	// RequestID is propagated to log records and embedder requests.
	RequestID string
}

// Stats contains per-call accounting returned by Cluster.
type Stats struct {
	// InputDocs is the number of docs passed to Cluster.
	InputDocs int
	// OutputDocs is the number of representative docs returned.
	OutputDocs int
	// ClustersFormed is the number of multi-doc clusters (size >= 2) detected.
	ClustersFormed int
	// DocsCollapsed is the total number of docs absorbed into clusters (not representatives).
	DocsCollapsed int
	// EmbeddingFallback is true when hybrid mode fell back to simhash-only clustering.
	EmbeddingFallback bool
	// Mode is the effective mode used for this call.
	Mode Mode
}

// clusterMeta is the versioned schema written to Metadata["spec_syn003_cluster"]
// on each representative doc of a multi-doc cluster.
//
// Schema version 1 (SPEC-SYN-003 §2.1.e).
type clusterMeta struct {
	SchemaVersion int      `json:"schema_version"`
	Members       []string `json:"members"`
	SimHash       string   `json:"simhash"`
	DedupMode     string   `json:"dedup_mode"`
	CosineMin     float64  `json:"cosine_min,omitempty"`
	ClusterSize   int      `json:"cluster_size"`
}

// toMap converts clusterMeta to the map[string]any stored in NormalizedDoc.Metadata.
func (cm clusterMeta) toMap() map[string]any {
	m := map[string]any{
		"schema_version": cm.SchemaVersion,
		"members":        cm.Members,
		"simhash":        cm.SimHash,
		"dedup_mode":     cm.DedupMode,
		"cluster_size":   cm.ClusterSize,
	}
	if cm.CosineMin != 0 {
		m["cosine_min"] = cm.CosineMin
	}
	return m
}

// docWithHash pairs a NormalizedDoc with its computed SimHash digest.
type docWithHash struct {
	doc  types.NormalizedDoc
	hash uint64
	idx  int // original input index
}
