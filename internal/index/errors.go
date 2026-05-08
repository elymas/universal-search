// Package index — sentinel errors for the hybrid index layer.
// SPEC-IDX-001 REQ-IDX-001 (scope item m).
package index

import "errors"

// ErrAllStoresFailed is returned by Search when all three stores (Qdrant,
// Meilisearch, PostgreSQL) failed or timed out and produced no rank list.
// Upsert NEVER returns this sentinel; it always returns a non-nil *UpsertResult.
var ErrAllStoresFailed = errors.New("index: all three stores failed")

// ErrSchemaBootstrapFailed is returned by New when AutoEnsureSchema is true
// and any of the three stores rejects schema initialisation. The underlying
// store error is wrapped so callers can use errors.Is / errors.As.
var ErrSchemaBootstrapFailed = errors.New("index: schema bootstrap failed")

// ErrEmbedderRequired is returned by New when opts.Embedder is nil.
// Callers must inject either zeroEmbedder{} or a real implementation.
var ErrEmbedderRequired = errors.New("index: embedder is nil")
