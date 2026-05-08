// Package index — unit tests for Embedder interface and zeroEmbedder stub.
package index

import (
	"context"
	"testing"
)

func TestZeroEmbedder_Dimensions(t *testing.T) {
	t.Parallel()
	e := zeroEmbedder{}
	if e.Dimensions() != 1024 {
		t.Fatalf("zeroEmbedder.Dimensions() = %d, want 1024", e.Dimensions())
	}
}

func TestZeroEmbedder_Embed_LenMatches(t *testing.T) {
	t.Parallel()
	e := zeroEmbedder{}
	texts := []string{"hello", "world", "foo"}
	vecs, err := e.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("Embed returned error: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Fatalf("Embed returned %d vectors, want %d", len(vecs), len(texts))
	}
}

func TestZeroEmbedder_Embed_AllZero(t *testing.T) {
	t.Parallel()
	e := zeroEmbedder{}
	vecs, err := e.Embed(context.Background(), []string{"any"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	for _, f := range vecs[0] {
		if f != 0 {
			t.Fatalf("zeroEmbedder returned non-zero value %v", f)
		}
	}
}

func TestZeroEmbedder_Embed_DimensionLength(t *testing.T) {
	t.Parallel()
	e := zeroEmbedder{}
	vecs, _ := e.Embed(context.Background(), []string{"x"})
	if len(vecs[0]) != e.Dimensions() {
		t.Fatalf("vector length %d != Dimensions() %d", len(vecs[0]), e.Dimensions())
	}
}

func TestZeroEmbedder_Embed_Empty(t *testing.T) {
	t.Parallel()
	e := zeroEmbedder{}
	vecs, err := e.Embed(context.Background(), []string{})
	if err != nil {
		t.Fatalf("Embed empty returned error: %v", err)
	}
	if len(vecs) != 0 {
		t.Fatalf("expected empty result, got %d vectors", len(vecs))
	}
}

func TestZeroEmbedder_ImplementsInterface(t *testing.T) {
	t.Parallel()
	// Compile-time assertion that zeroEmbedder satisfies Embedder.
	var _ Embedder = zeroEmbedder{}
}
