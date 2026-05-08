// Package index is the hybrid index layer for Universal Search.
// It orchestrates Qdrant (dense vectors via gRPC), Meilisearch (BM25 via HTTP),
// and PostgreSQL (relational metadata via pgx) with RRF fusion.
//
// SPEC-IDX-001 REQ-IDX-001 (scope items a, b).
package index

import (
	"context"
	"fmt"
	"time"

	"github.com/elymas/universal-search/internal/index/meili"
	"github.com/elymas/universal-search/internal/index/pg"
	"github.com/elymas/universal-search/internal/index/qdrant"
	"github.com/elymas/universal-search/internal/obs"
	"github.com/elymas/universal-search/pkg/types"
	oteltrace "go.opentelemetry.io/otel/trace"
	otelnooptrace "go.opentelemetry.io/otel/trace/noop"
)

// collectionName is the shared collection/index name across all three stores.
const collectionName = "usearch_docs"

// Index is the top-level orchestrator for the three-store hybrid index.
// It is immutable post-construction; all fields are safe for concurrent use.
//
// @MX:NOTE: [AUTO] Three-store hybrid: Qdrant (dense), Meili (BM25), PG (relational). Immutable post-New.
type Index struct {
	qd       *qdrant.Client
	me       *meili.Client
	pg       *pg.Client
	embedder Embedder
	obs      *obs.Obs
	opts     Options
}

// New constructs a hybrid Index from the given options.
//
// Validation:
//   - Returns ErrEmbedderRequired when opts.Embedder == nil.
//   - Returns ErrSchemaBootstrapFailed (wrapping store error) when
//     AutoEnsureSchema == true and any store rejects schema init.
//   - Zero-valued numeric/map fields are replaced with documented defaults.
//
// @MX:ANCHOR: [AUTO] Sole construction point for the hybrid index. fan_in >= 3 (CLI, tests, CACHE-001).
// @MX:REASON: constructor signature change propagates to all callers; AutoEnsureSchema wires all three stores.
// @MX:SPEC: SPEC-IDX-001
func New(ctx context.Context, opts Options) (*Index, error) {
	if opts.Embedder == nil {
		return nil, ErrEmbedderRequired
	}

	// Capture AutoEnsureSchema before applyDefaults (bool zero = false).
	autoSchema := opts.AutoEnsureSchema

	opts = applyDefaults(opts)

	// Construct Qdrant client.
	qd, err := qdrant.NewClient(opts.Qdrant)
	if err != nil {
		return nil, fmt.Errorf("index: qdrant: %w", err)
	}

	// Construct Meilisearch client.
	me, err := meili.NewClient(opts.Meili)
	if err != nil {
		_ = qd.Close()
		return nil, fmt.Errorf("index: meili: %w", err)
	}

	// Construct PostgreSQL client.
	pgc, err := pg.NewClient(ctx, opts.PG)
	if err != nil {
		_ = qd.Close()
		return nil, fmt.Errorf("index: pg: %w", err)
	}

	idx := &Index{
		qd:       qd,
		me:       me,
		pg:       pgc,
		embedder: opts.Embedder,
		obs:      opts.Obs,
		opts:     opts,
	}

	if autoSchema {
		if err := idx.ensureAllSchemas(ctx); err != nil {
			_ = idx.Close()
			return nil, fmt.Errorf("%w: %v", ErrSchemaBootstrapFailed, err)
		}
	}

	return idx, nil
}

// ensureAllSchemas bootstraps schema on all three stores.
func (idx *Index) ensureAllSchemas(ctx context.Context) error {
	// Qdrant collection.
	if err := idx.qd.EnsureCollection(ctx, collectionName, uint64(idx.embedder.Dimensions())); err != nil {
		return fmt.Errorf("qdrant: %w", err)
	}

	// Meilisearch index.
	settings := meili.IndexSettings{
		SearchableAttributes: []string{"title", "body", "snippet"},
		FilterableAttributes: []string{"source_id", "lang", "doc_type", "team_id", "published_at"},
		DistinctAttribute:    "doc_id",
	}
	if err := idx.me.EnsureIndex(ctx, collectionName, settings); err != nil {
		return fmt.Errorf("meili: %w", err)
	}

	// PostgreSQL schema.
	if err := idx.pg.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("pg: %w", err)
	}

	return nil
}

// tracer returns an OTel tracer or a no-op tracer when obs is nil.
func (idx *Index) tracer() oteltrace.Tracer {
	if idx.obs != nil && idx.obs.HasTracer() {
		return idx.obs.Tracer("index")
	}
	return otelnooptrace.NewTracerProvider().Tracer("index")
}

// Search executes a parallel query across all three stores and returns RRF-fused results.
//
// Soft-fail discipline (§2.6): a single store failure produces a partial result
// with PerStoreErrors[store] set. All-stores-fail returns (nil, ErrAllStoresFailed).
//
// @MX:ANCHOR: [AUTO] Sole retrieval entry point. fan_in >= 4 (CLI, MCP, CACHE-001, IDX-005).
// @MX:REASON: contract boundary; signature change ripples to CLI-001 + MCP-001 + CACHE-001 + IDX-005.
// @MX:SPEC: SPEC-IDX-001
func (idx *Index) Search(ctx context.Context, q IndexQuery) (*IndexResult, error) {
	tracer := idx.tracer()
	spanCtx, span := tracer.Start(ctx, "index.search",
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()

	start := time.Now()

	// Embed the query text (one-shot per call).
	var vector []float32
	if q.Text != "" {
		embeddings, err := idx.embedder.Embed(spanCtx, []string{q.Text})
		if err != nil {
			return nil, fmt.Errorf("index: embed: %w", err)
		}
		vector = embeddings[0]
	} else {
		vector = make([]float32, idx.embedder.Dimensions())
	}

	rankLists, perStoreErrs, perStoreLat := idx.parallelSearch(spanCtx, q, vector)

	// All-fail condition: all stores errored AND all rank lists are empty.
	allErrs := true
	allEmpty := true
	for _, e := range perStoreErrs {
		if e == nil {
			allErrs = false
		}
	}
	for _, list := range rankLists {
		if len(list) > 0 {
			allEmpty = false
		}
	}
	if allErrs && allEmpty && len(perStoreErrs) == 3 {
		return nil, ErrAllStoresFailed
	}

	fusionStart := time.Now()
	fused := fuseRRF(rankLists, idx.opts.RRFWeights, idx.opts.RRFConstantK)
	fusionLat := time.Since(fusionStart)

	// Clamp to MaxResults.
	maxRes := q.MaxResults
	if maxRes <= 0 {
		maxRes = defaultMaxResults
	}
	if len(fused) > maxRes {
		fused = fused[:maxRes]
	}

	docs := make([]types.NormalizedDoc, len(fused))
	for i, f := range fused {
		docs[i] = f.Doc
	}

	perStoreCounts := make(map[string]int, 3)
	for store, list := range rankLists {
		perStoreCounts[store] = len(list)
	}

	elapsed := time.Since(start)
	stats := SearchStats{
		StoreLatencies: perStoreLat,
		FusionLatency:  fusionLat,
		PerStoreCounts: perStoreCounts,
		FusedCount:     len(fused),
		ElapsedSeconds: elapsed.Seconds(),
	}

	// Retain only non-nil per-store errors.
	cleanErrs := make(map[string]error)
	for store, e := range perStoreErrs {
		if e != nil {
			cleanErrs[store] = e
		}
	}

	result := &IndexResult{Docs: docs, PerStoreErrors: cleanErrs, Stats: stats}
	emitSearch(idx.obs, span, result, cleanErrs, elapsed)
	return result, nil
}

// Upsert ingests a batch of documents into all three stores in parallel.
// Invalid documents (per NormalizedDoc.Validate) are skipped and counted.
// Soft-fail discipline: per-store errors are recorded; Upsert never returns error.
//
// @MX:ANCHOR: [AUTO] Sole ingestion entry point. fan_in >= 3 (CLI, bulk-ingest, IDX-005).
// @MX:REASON: contract boundary; signature change propagates to all consumers.
// @MX:SPEC: SPEC-IDX-001
func (idx *Index) Upsert(ctx context.Context, docs []types.NormalizedDoc) (*UpsertResult, error) {
	tracer := idx.tracer()
	spanCtx, span := tracer.Start(ctx, "index.upsert",
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()

	start := time.Now()

	result := &UpsertResult{
		PerStoreErrors: make(map[string]error),
		Stats: UpsertStats{
			DocCount:          len(docs),
			PerStoreLatencies: make(map[string]time.Duration, 3),
		},
	}

	if len(docs) == 0 {
		result.Stats.ElapsedSeconds = time.Since(start).Seconds()
		return result, nil
	}

	// Validate: separate valid from invalid docs.
	valid := make([]types.NormalizedDoc, 0, len(docs))
	var validationErrs []error
	for _, d := range docs {
		if err := d.Validate(); err != nil {
			validationErrs = append(validationErrs, err)
			result.Skipped++
			result.Stats.SkippedCount++
		} else {
			valid = append(valid, d)
		}
	}

	if len(validationErrs) > 0 {
		result.PerStoreErrors["validation"] = fmt.Errorf(
			"%d validation error(s): first=%v", len(validationErrs), validationErrs[0])
		if idx.obs != nil && idx.obs.Logger != nil {
			idx.obs.Logger.Warn("index.upsert: validation errors in batch",
				"batch_size", len(docs),
				"skipped_count", len(validationErrs),
			)
		}
	}

	if len(valid) == 0 {
		result.Stats.ElapsedSeconds = time.Since(start).Seconds()
		return result, nil
	}

	// Sequential batches; intra-batch stores are parallel.
	batchSize := idx.opts.BulkBatchSize
	for i := 0; i < len(valid); i += batchSize {
		end := i + batchSize
		if end > len(valid) {
			end = len(valid)
		}
		batch := valid[i:end]

		partials := idx.parallelUpsert(spanCtx, batch)
		for _, p := range partials {
			if p.err != nil {
				result.PerStoreErrors[p.store] = p.err
				result.Stats.ErrorCount++
			} else {
				result.Stats.SuccessCount++
				result.Inserted += p.inserted
			}
			result.Stats.PerStoreLatencies[p.store] += p.duration
		}
	}

	elapsed := time.Since(start)
	result.Stats.ElapsedSeconds = elapsed.Seconds()
	emitUpsert(idx.obs, span, result, elapsed)
	return result, nil
}

// Close orderly shuts down all three store clients.
// Returns the first non-nil error but attempts all three closes.
func (idx *Index) Close() error {
	var firstErr error
	idx.pg.Close() // pg client returns void

	if err := idx.qd.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := idx.me.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// MeiliClient returns the underlying Meilisearch client (for test helpers).
func (idx *Index) MeiliClient() *meili.Client { return idx.me }

// PGClient returns the underlying PostgreSQL client (for test helpers).
func (idx *Index) PGClient() *pg.Client { return idx.pg }
