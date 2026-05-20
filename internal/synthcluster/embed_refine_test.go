// Package synthcluster_test — RED phase tests for embedding refinement.
// SPEC-SYN-003 REQ-SYN3-003: hybrid mode cosine refinement + fallback.
package synthcluster_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elymas/universal-search/internal/embedder"
	"github.com/elymas/universal-search/internal/synthcluster"
	"github.com/elymas/universal-search/pkg/types"
)

// mockEmbedder implements synthcluster.Embedder for test control.
type mockEmbedder struct {
	callCount int
	vectors   [][]float64
	err       error
	delay     time.Duration
}

func (m *mockEmbedder) Embed(ctx context.Context, req embedder.Request) (embedder.Response, error) {
	m.callCount++
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return embedder.Response{}, ctx.Err()
		}
	}
	if m.err != nil {
		return embedder.Response{}, m.err
	}
	return embedder.Response{Dense: m.vectors}, nil
}

// cosineSim computes cosine similarity between two equal-length vectors.
func cosineSim(a, b []float64) float64 {
	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	denom := sqrtFloat(magA) * sqrtFloat(magB)
	return dot / denom
}

func sqrtFloat(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for range 20 {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

// vectorsWithCosine builds two unit-normalised vectors with the given cosine similarity.
// Simplified: returns [1,0,...] and [cos, sin, 0,...] for 2D.
func vectorsWithCosine(cosine float64) ([]float64, []float64) {
	sin := sqrtFloat(1 - cosine*cosine)
	return []float64{1.0, 0.0}, []float64{cosine, sin}
}

// makeHybridDocs creates near-identical docs (SimHash distance ~0) for hybrid tests.
func makeHybridDocs(ids ...string) []types.NormalizedDoc {
	docs := make([]types.NormalizedDoc, len(ids))
	for i, id := range ids {
		docs[i] = types.NormalizedDoc{
			ID:          id,
			SourceID:    "test",
			URL:         "https://example.com/" + id,
			Title:       "Hybrid cluster test article news event",
			Body:        "Hybrid mode test body content identical near duplicate.",
			RetrievedAt: time.Now(),
			Score:       float64(len(ids)-i) * 0.1,
		}
	}
	return docs
}

// TestHybridCallsEmbedderOnce: with N candidate docs, Embed is called once.
// REQ-SYN3-003.
func TestHybridCallsEmbedderOnce(t *testing.T) {
	t.Parallel()
	docs := makeHybridDocs("h1", "h2", "h3", "h4")

	// All pairs same cosine > threshold to confirm cluster.
	v1, v2 := vectorsWithCosine(0.96)
	vectors := [][]float64{v1, v2, v1, v2}

	mock := &mockEmbedder{vectors: vectors}
	opts := synthcluster.Options{
		Mode:             synthcluster.ModeHybrid,
		HammingThreshold: 10,
		CosineThreshold:  0.92,
		Embedder:         mock,
	}

	_, _, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster returned error: %v", err)
	}
	if mock.callCount != 1 {
		t.Errorf("Embed called %d times, want exactly 1", mock.callCount)
	}
}

// TestCosineAboveThresholdConfirmsPair: cosine > 0.92 → pair confirmed.
// REQ-SYN3-003.
func TestCosineAboveThresholdConfirmsPair(t *testing.T) {
	t.Parallel()
	docs := makeHybridDocs("ab1", "ab2")
	v1, v2 := vectorsWithCosine(0.95) // above 0.92
	mock := &mockEmbedder{vectors: [][]float64{v1, v2}}

	opts := synthcluster.Options{
		Mode:             synthcluster.ModeHybrid,
		HammingThreshold: 10,
		CosineThreshold:  0.92,
		Embedder:         mock,
	}
	reps, _, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster returned error: %v", err)
	}
	if len(reps) != 1 {
		t.Errorf("cosine=0.95 (above threshold): expected 1 representative, got %d", len(reps))
	}
}

// TestCosineBelowThresholdDemotesPair: cosine < 0.92 → pair demoted (separate clusters).
// REQ-SYN3-003.
func TestCosineBelowThresholdDemotesPair(t *testing.T) {
	t.Parallel()
	docs := makeHybridDocs("cd1", "cd2")
	v1, v2 := vectorsWithCosine(0.80) // below 0.92

	mock := &mockEmbedder{vectors: [][]float64{v1, v2}}
	opts := synthcluster.Options{
		Mode:             synthcluster.ModeHybrid,
		HammingThreshold: 10,
		CosineThreshold:  0.92,
		Embedder:         mock,
	}
	reps, _, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster returned error: %v", err)
	}
	if len(reps) != 2 {
		t.Errorf("cosine=0.80 (below threshold): expected 2 representatives (demoted), got %d", len(reps))
	}
}

// TestEmbedderUnreachableFallsBackToSimhash: ErrSidecarUnreachable triggers fallback.
// REQ-SYN3-003.
func TestEmbedderUnreachableFallsBackToSimhash(t *testing.T) {
	t.Parallel()
	docs := makeHybridDocs("ur1", "ur2")
	mock := &mockEmbedder{err: embedder.ErrSidecarUnreachable}

	opts := synthcluster.Options{
		Mode:             synthcluster.ModeHybrid,
		HammingThreshold: 10,
		CosineThreshold:  0.92,
		Embedder:         mock,
	}
	reps, stats, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster returned error on fallback: %v", err)
	}
	// Fallback to simhash: identical content → 1 cluster.
	if len(reps) != 1 {
		t.Errorf("fallback: expected 1 representative, got %d", len(reps))
	}
	if !stats.EmbeddingFallback {
		t.Error("Stats.EmbeddingFallback must be true on unreachable embedder")
	}
}

// TestEmbedderTimeoutFallsBackToSimhash: embedder timeout triggers fallback.
// REQ-SYN3-003.
func TestEmbedderTimeoutFallsBackToSimhash(t *testing.T) {
	t.Parallel()
	docs := makeHybridDocs("to1", "to2")
	// Use context cancellation to simulate timeout (EmbeddingTimeoutMs very small).
	mock := &mockEmbedder{err: embedder.ErrTimeout}

	opts := synthcluster.Options{
		Mode:                synthcluster.ModeHybrid,
		HammingThreshold:    10,
		CosineThreshold:     0.92,
		EmbeddingTimeoutMs:  1, // 1ms → will expire immediately
		Embedder:            mock,
	}
	reps, stats, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster returned error on timeout fallback: %v", err)
	}
	if len(reps) != 1 {
		t.Errorf("timeout fallback: expected 1 rep, got %d", len(reps))
	}
	if !stats.EmbeddingFallback {
		t.Error("Stats.EmbeddingFallback must be true on timeout")
	}
}

// TestEmbeddingFallbackCounterIncrementsOncePerCall: even 5 clusters, fallback == 1.
// REQ-SYN3-003.
func TestEmbeddingFallbackCounterIncrementsOncePerCall(t *testing.T) {
	t.Parallel()
	docs := makeHybridDocs("ef1", "ef2", "ef3", "ef4")
	mock := &mockEmbedder{err: embedder.ErrSidecarUnreachable}

	opts := synthcluster.Options{
		Mode:             synthcluster.ModeHybrid,
		HammingThreshold: 10,
		CosineThreshold:  0.92,
		Embedder:         mock,
	}
	_, stats, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster returned error: %v", err)
	}
	if !stats.EmbeddingFallback {
		t.Error("Stats.EmbeddingFallback must be true")
	}
	if mock.callCount != 1 {
		t.Errorf("Embed called %d times, want 1 (only attempted once before fallback)", mock.callCount)
	}
}

// TestModeOffNoEmbedderCall: embedder must not be called in mode=off. REQ-SYN3-005.
func TestModeOffNoEmbedderCall(t *testing.T) {
	t.Parallel()
	docs := makeHybridDocs("no1", "no2")
	mock := &mockEmbedder{}

	opts := synthcluster.Options{
		Mode:     synthcluster.ModeOff,
		Embedder: mock,
	}
	_, _, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster returned error: %v", err)
	}
	if mock.callCount != 0 {
		t.Errorf("mode=off: Embed called %d times, want 0", mock.callCount)
	}
}

// TestEmbedderModelLoadFailedFallback: ErrModelLoadFailed also triggers fallback.
func TestEmbedderModelLoadFailedFallback(t *testing.T) {
	t.Parallel()
	docs := makeHybridDocs("ml1", "ml2")
	mock := &mockEmbedder{err: embedder.ErrModelLoadFailed}

	opts := synthcluster.Options{
		Mode:             synthcluster.ModeHybrid,
		HammingThreshold: 10,
		CosineThreshold:  0.92,
		Embedder:         mock,
	}
	reps, stats, err := synthcluster.Cluster(context.Background(), docs, opts)
	if err != nil {
		t.Fatalf("Cluster returned error on model-load fallback: %v", err)
	}
	// Fallback: simhash-only → identical content → 1 cluster.
	if len(reps) != 1 {
		t.Errorf("model-load fallback: expected 1 rep, got %d", len(reps))
	}
	if !stats.EmbeddingFallback {
		t.Error("Stats.EmbeddingFallback must be true on ErrModelLoadFailed")
	}
}

// TestContextCancelledFallback: context cancellation during embed → fallback.
func TestContextCancelledFallback(t *testing.T) {
	t.Parallel()
	docs := makeHybridDocs("cc1", "cc2")
	mock := &mockEmbedder{err: errors.New("context canceled")}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	opts := synthcluster.Options{
		Mode:             synthcluster.ModeHybrid,
		HammingThreshold: 10,
		CosineThreshold:  0.92,
		Embedder:         mock,
	}
	// With cancelled context in hybrid mode, should fall back gracefully.
	_, _, err := synthcluster.Cluster(ctx, docs, opts)
	// Either nil (fallback) or context error is acceptable.
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}
