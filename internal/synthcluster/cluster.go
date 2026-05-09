// Package synthcluster — core Cluster function.
//
// SPEC-SYN-003: pre-synthesis near-duplicate clustering.
// Insertion point: cmd/usearch/query.go between fanout result and synth.Synthesize.
package synthcluster

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/elymas/universal-search/internal/embedder"
	"github.com/elymas/universal-search/pkg/types"
)

const (
	defaultHammingThreshold = 4
	defaultCosineThreshold  = 0.92
	defaultEmbedTimeoutMs   = 1500
)

// Cluster groups near-duplicate NormalizedDocs into clusters and returns one
// representative per cluster, with non-representative doc_ids persisted in
// Metadata["spec_syn003_cluster"]["members"] for downstream auditability.
//
// Guarantees (SPEC-SYN-003 NFR-SYN3-002):
//   - Every input doc_id appears exactly once across union(rep.ID, members(any rep)).
//   - No representative lists itself as a member.
//   - Running Cluster on the output produces clusters of size 1 (idempotence).
//
// @MX:ANCHOR: [AUTO] Primary public API; callers: cmd/usearch/query.go, tests, benchmarks
// @MX:REASON: fan_in >= 3; all clustering calls route through this function
func Cluster(ctx context.Context, docs []types.NormalizedDoc, opts Options) ([]types.NormalizedDoc, Stats, error) {
	// Apply defaults.
	if opts.HammingThreshold == 0 {
		opts.HammingThreshold = defaultHammingThreshold
	}
	if opts.CosineThreshold == 0 {
		opts.CosineThreshold = defaultCosineThreshold
	}
	if opts.EmbeddingTimeoutMs == 0 {
		opts.EmbeddingTimeoutMs = defaultEmbedTimeoutMs
	}
	if opts.Mode == "" {
		opts.Mode = ModeSimhashOnly
	}

	stats := Stats{InputDocs: len(docs), Mode: opts.Mode}

	// Pass-through: mode=off returns input unchanged with no mutations.
	// REQ-SYN3-005.
	if opts.Mode == ModeOff {
		stats.OutputDocs = len(docs)
		return docs, stats, nil
	}

	if len(docs) == 0 {
		return nil, stats, nil
	}

	// Step 1: compute SimHash for all valid docs.
	hashed := make([]docWithHash, 0, len(docs))
	for i, d := range docs {
		text := d.Title + "\n" + d.Body
		h := SimHash64(text)
		hashed = append(hashed, docWithHash{doc: d, hash: h, idx: i})
	}

	// Step 2: O(N²) candidate-pair detection via Hamming distance.
	// Acceptable at fanout volumes (N ≤ 200) per SPEC-SYN-003 research §7.
	n := len(hashed)
	uf := newUnionFind(n)

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			dist := HammingDistance(hashed[i].hash, hashed[j].hash)
			if dist <= opts.HammingThreshold {
				uf.union(i, j)
			}
		}
	}

	// Step 3: optional embedding cosine refinement (hybrid mode).
	embeddingFallback := false
	if opts.Mode == ModeHybrid {
		var err error
		embeddingFallback, err = refineWithEmbedding(ctx, opts, hashed, uf)
		if err != nil {
			// Context cancellation propagates only when not a fallback.
			if !embeddingFallback {
				return nil, stats, err
			}
		}
		stats.EmbeddingFallback = embeddingFallback
	}

	// Step 4: assemble clusters from Union-Find groups.
	groups := uf.groups(n)

	reps := make([]types.NormalizedDoc, 0, len(groups))
	for _, members := range groups {
		rep, collapsed := assembleCluster(hashed, members, opts.Mode, embeddingFallback)
		if len(collapsed) > 0 {
			stats.ClustersFormed++
			stats.DocsCollapsed += len(collapsed)
		}
		reps = append(reps, rep)
	}

	// Sort representatives for deterministic output (by ID).
	sort.Slice(reps, func(i, j int) bool {
		return reps[i].ID < reps[j].ID
	})

	stats.OutputDocs = len(reps)
	return reps, stats, nil
}

// refineWithEmbedding performs cosine similarity refinement in hybrid mode.
// It batches all candidate-pair-participant docs, calls Embed once, then
// demotes pairs below the cosine threshold.
//
// Returns (fallback bool, err error). If fallback is true, caller uses
// the SimHash-only result (uf is NOT modified in the fallback path).
func refineWithEmbedding(ctx context.Context, opts Options, hashed []docWithHash, uf *unionFind) (bool, error) {
	if opts.Embedder == nil {
		// No embedder configured → treat as sidecar unreachable.
		slog.WarnContext(ctx, "synthcluster: no embedder configured in hybrid mode, falling back to simhash-only",
			slog.String("request_id", opts.RequestID),
			slog.String("dedup_mode", string(opts.Mode)),
			slog.String("embedding_error", "embedder is nil"),
			slog.String("fallback_to", "simhash_only"),
		)
		return true, nil
	}

	// Identify candidate-pair participants (docs in multi-member groups).
	n := len(hashed)
	groups := uf.groups(n)
	var participantIdx []int
	for _, members := range groups {
		if len(members) >= 2 {
			participantIdx = append(participantIdx, members...)
		}
	}

	if len(participantIdx) == 0 {
		// No candidate pairs at all; nothing to refine.
		return false, nil
	}

	// Sort for determinism.
	sort.Ints(participantIdx)

	texts := make([]string, len(participantIdx))
	for i, idx := range participantIdx {
		d := hashed[idx].doc
		texts[i] = d.Title + "\n" + d.Body
	}

	// Create sub-context with embedding timeout.
	embedCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.EmbeddingTimeoutMs)*time.Millisecond)
	defer cancel()

	req := embedder.Request{
		RequestID:   opts.RequestID,
		Texts:       texts,
		ReturnDense: true,
	}

	resp, err := opts.Embedder.Embed(embedCtx, req)
	if err != nil {
		// Any embedder error (unreachable, timeout, model-load) → fallback.
		slog.WarnContext(ctx, "synthcluster: embedding fallback to simhash-only",
			slog.String("request_id", opts.RequestID),
			slog.String("dedup_mode", string(opts.Mode)),
			slog.String("embedding_error", err.Error()),
			slog.String("fallback_to", "simhash_only"),
		)
		return true, nil
	}

	// Map participant index → dense vector.
	vecByIdx := make(map[int][]float64, len(participantIdx))
	for i, idx := range participantIdx {
		if i < len(resp.Dense) {
			vecByIdx[idx] = resp.Dense[i]
		}
	}

	// Demote candidate pairs that fail the cosine threshold.
	// We rebuild a fresh Union-Find that only includes confirmed pairs.
	fresh := newUnionFind(n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			// Only check pairs that were candidates under SimHash.
			if !uf.connected(i, j) {
				continue
			}
			vi, okI := vecByIdx[i]
			vj, okJ := vecByIdx[j]
			if !okI || !okJ {
				// Not in candidate set (shouldn't happen); treat as not a pair.
				continue
			}
			if cosineSimilarity(vi, vj) >= opts.CosineThreshold {
				fresh.union(i, j)
			}
			// else: demote — pair not added to fresh UF.
		}
	}

	// Replace uf contents with refined result.
	*uf = *fresh
	return false, nil
}

// cosineSimilarity returns the cosine similarity of two equal-length vectors.
// Returns 0 if either vector has zero magnitude.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (sqrt64(magA) * sqrt64(magB))
}

// sqrt64 is a pure-Go square root for float64, avoiding math.Sqrt import churn.
func sqrt64(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	// 20 Newton-Raphson iterations — sufficient precision for cosine comparison.
	for range 20 {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

// assembleCluster selects a representative from a group of doc indices and
// annotates it with cluster metadata. Returns the representative and the
// list of collapsed member IDs.
func assembleCluster(hashed []docWithHash, memberIdxs []int, mode Mode, fallback bool) (types.NormalizedDoc, []string) {
	if len(memberIdxs) == 1 {
		// Size-1 cluster: return as-is, no metadata annotation.
		return hashed[memberIdxs[0]].doc, nil
	}

	// Sort members by input order for determinism during representative selection.
	sort.Ints(memberIdxs)

	// Select representative: highest Score → latest PublishedAt → longest Body → lexicographic ID.
	bestIdx := memberIdxs[0]
	for _, idx := range memberIdxs[1:] {
		if betterRep(hashed[idx].doc, hashed[bestIdx].doc) {
			bestIdx = idx
		}
	}

	rep := hashed[bestIdx].doc

	// Collect member IDs (all non-representative docs).
	var memberIDs []string
	for _, idx := range memberIdxs {
		if idx != bestIdx {
			memberIDs = append(memberIDs, hashed[idx].doc.ID)
		}
	}
	sort.Strings(memberIDs) // deterministic order

	// Determine effective dedup_mode string for metadata.
	dedupMode := string(mode)
	if fallback {
		dedupMode = "simhash_only"
	}

	// Annotate representative with cluster metadata.
	cm := clusterMeta{
		SchemaVersion: 1,
		Members:       memberIDs,
		SimHash:       SimHashHex(hashed[bestIdx].hash),
		DedupMode:     dedupMode,
		ClusterSize:   len(memberIdxs),
	}

	if rep.Metadata == nil {
		rep.Metadata = make(map[string]any)
	}
	rep.Metadata["spec_syn003_cluster"] = cm.toMap()

	return rep, memberIDs
}

// betterRep returns true if candidate should replace current as cluster representative.
// Tiebreaker order: Score → PublishedAt (later = better) → len(Body) → ID (lexicographic).
// REQ-SYN3-004: deterministic representative selection.
func betterRep(candidate, current types.NormalizedDoc) bool {
	if candidate.Score != current.Score {
		return candidate.Score > current.Score
	}
	if !candidate.PublishedAt.Equal(current.PublishedAt) {
		return candidate.PublishedAt.After(current.PublishedAt)
	}
	if len(candidate.Body) != len(current.Body) {
		return len(candidate.Body) > len(current.Body)
	}
	// Final tiebreaker: lexicographic ID (smaller ID wins → deterministic).
	// REQ-SYN3-004: when all else ties, input-order-first ≈ lexicographic-first.
	return candidate.ID < current.ID
}

