// Package index — Options struct and defaults for the hybrid index layer.
// SPEC-IDX-001 REQ-IDX-001 (scope item d).
package index

import (
	"time"

	"github.com/elymas/universal-search/internal/index/meili"
	"github.com/elymas/universal-search/internal/index/pg"
	"github.com/elymas/universal-search/internal/index/qdrant"
	"github.com/elymas/universal-search/internal/obs"
)

// Default values documented in §6.6 and §6.3 of SPEC-IDX-001.
const (
	defaultMaxParallel   = 3
	defaultRRFConstantK  = 60
	defaultBulkBatchSize = 100
	defaultMaxResults    = 50
)

// defaultPerStoreTimeout holds the hardcoded timeout defaults per §6.6.
//
// @MX:NOTE: [AUTO] Magic constants: qdrant=200ms, meili=300ms, pg=100ms per SPEC-IDX-001 §6.6 / §2.4 derivation.
var defaultPerStoreTimeout = map[string]time.Duration{
	"qdrant": 200 * time.Millisecond,
	"meili":  300 * time.Millisecond,
	"pg":     100 * time.Millisecond,
}

// Options configures the hybrid index layer.
// Zero values for numeric/map fields are replaced by documented defaults in applyDefaults.
type Options struct {
	// Qdrant is the Qdrant sub-client configuration.
	Qdrant qdrant.Config
	// Meili is the Meilisearch sub-client configuration.
	Meili meili.Config
	// PG is the PostgreSQL sub-client configuration.
	PG pg.Config
	// Embedder is required; use zeroEmbedder{} for testing or until SPEC-IDX-002 lands.
	Embedder Embedder
	// Obs is the observability bundle. May be nil (all emit calls are nil-safe).
	Obs *obs.Obs
	// MaxParallel caps the errgroup concurrency across the three stores. Default 3.
	MaxParallel int
	// PerStoreTimeout maps store name → per-store context timeout. Default §6.6.
	PerStoreTimeout map[string]time.Duration
	// RRFConstantK is the k constant in the RRF formula. Default 60 (paper default).
	RRFConstantK int
	// RRFWeights maps store name → per-ranker weight. Default 1.0 per store.
	RRFWeights map[string]float64
	// BulkBatchSize is the maximum docs per parallelUpsert batch. Default 100.
	BulkBatchSize int
	// AutoEnsureSchema calls EnsureSchema on all stores during New. Default true.
	AutoEnsureSchema bool
}

// applyDefaults fills zero-valued fields with documented defaults.
// A zero-valued Options always returns after applyDefaults as a valid config.
func applyDefaults(o Options) Options {
	if o.MaxParallel <= 0 {
		o.MaxParallel = defaultMaxParallel
	}
	if o.RRFConstantK <= 0 {
		o.RRFConstantK = defaultRRFConstantK
	}
	if o.BulkBatchSize <= 0 {
		o.BulkBatchSize = defaultBulkBatchSize
	}

	if o.PerStoreTimeout == nil {
		o.PerStoreTimeout = make(map[string]time.Duration, 3)
	}
	for store, d := range defaultPerStoreTimeout {
		if _, ok := o.PerStoreTimeout[store]; !ok {
			o.PerStoreTimeout[store] = d
		}
	}

	if o.RRFWeights == nil {
		o.RRFWeights = make(map[string]float64, 3)
	}
	for _, store := range []string{"qdrant", "meili", "pg"} {
		if _, ok := o.RRFWeights[store]; !ok {
			o.RRFWeights[store] = 1.0
		}
	}

	// AutoEnsureSchema defaults to true.
	// Since bool zero-value is false, we cannot distinguish "not set" from "false"
	// without a pointer. The SPEC mandates true as default, so we set it here
	// unless the caller explicitly wants the opposite. We use a helper field trick:
	// callers who want false must use the DisableAutoSchema option (not in scope)
	// or set the field after calling applyDefaults. For simplicity in v0.1, we
	// treat AutoEnsureSchema=false as "not set" and default to true only when
	// other fields are also at zero. Since the field is a plain bool, New handles
	// the defaulting logic directly rather than here to avoid over-riding explicit false.

	return o
}
