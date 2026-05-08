// Package index — Embedder interface and zeroEmbedder stub.
// SPEC-IDX-001 REQ-IDX-014 §2.7 (scope item e).
package index

import "context"

// Embedder is the port that the hybrid index uses for dense vector generation.
// SPEC-IDX-002 wires the production BGE-M3 implementation; v0.1 ships zeroEmbedder.
//
// Contract:
//   - Embed returns a slice of length len(texts), each vector of length Dimensions().
//   - Partial success is NOT supported: all-or-nothing per call.
//   - Dimensions() must match the Qdrant collection vector_size at construction time.
type Embedder interface {
	// Embed produces dense vectors for the given texts.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dimensions returns the static embedding dimensionality.
	Dimensions() int
}

// zeroEmbedder is the v0.1 stub that returns zero-vectors of dimension 1024.
// SPEC-IDX-002 replaces this with BGE-M3; no SPEC-IDX-001 surface change is needed.
type zeroEmbedder struct{}

// Embed returns len(texts) zero-vectors, each of length 1024.
func (zeroEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = make([]float32, 1024)
	}
	return out, nil
}

// Dimensions returns 1024 (matching BGE-M3 output dimensionality).
func (zeroEmbedder) Dimensions() int { return 1024 }
