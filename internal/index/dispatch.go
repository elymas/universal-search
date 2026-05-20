// Package index — parallel fan-out helpers for Search and Upsert.
// SPEC-IDX-001 REQ-IDX-006 §2.4, REQ-IDX-009 (scope item i).
package index

import (
	"context"
	"time"

	"github.com/elymas/universal-search/internal/index/meili"
	"github.com/elymas/universal-search/internal/index/pg"
	"github.com/elymas/universal-search/internal/index/qdrant"
	"github.com/elymas/universal-search/pkg/types"
	"golang.org/x/sync/errgroup"
)

// deriveStoreCtx creates a per-store context bounded by both the per-store
// timeout and the remaining time to the parent context deadline (§2.4).
//
// @MX:NOTE: [AUTO] Magic constants (per-store defaults) documented in §6.6. The pre-cancel branch fires when remaining <= 0.
func (idx *Index) deriveStoreCtx(parent context.Context, store string) (context.Context, context.CancelFunc) {
	deadline := idx.opts.PerStoreTimeout[store]
	if pDeadline, ok := parent.Deadline(); ok {
		if remaining := time.Until(pDeadline); remaining < deadline {
			deadline = remaining
		}
	}
	if deadline <= 0 {
		ctx, cancel := context.WithCancel(parent)
		cancel() // immediately cancelled
		return ctx, cancel
	}
	return context.WithTimeout(parent, deadline)
}

// parallelSearch dispatches concurrent queries to all three stores.
// Workers return nil even on per-store error (soft-fail discipline per §2.6)
// so errgroup's first-error-cancel does NOT kill siblings.
//
// @MX:WARN: [AUTO] Outbound fan-out spawns 3 goroutines. Removing the per-goroutine defer cancel() sequence invalidates NFR-IDX-004 zero-leak guarantee.
// @MX:REASON: goroutine-lifecycle invariant — every ctx MUST be cancelled to avoid leak.
func (idx *Index) parallelSearch(
	parent context.Context,
	q IndexQuery,
	vector []float32,
) (rankLists map[string][]Ranked, errs map[string]error, latencies map[string]time.Duration) {
	rankLists = make(map[string][]Ranked, 3)
	errs = make(map[string]error, 3)
	latencies = make(map[string]time.Duration, 3)

	type storeResult struct {
		store    string
		list     []Ranked
		err      error
		duration time.Duration
	}

	results := make(chan storeResult, 3)

	eg, _ := errgroup.WithContext(parent)
	eg.SetLimit(idx.opts.MaxParallel)

	maxRes := q.MaxResults
	if maxRes <= 0 {
		maxRes = defaultMaxResults
	}

	// Qdrant goroutine.
	eg.Go(func() error {
		start := time.Now()
		storeCtx, cancel := idx.deriveStoreCtx(parent, "qdrant")
		defer cancel()

		var list []Ranked
		var err error

		filter := buildQdrantFilter(q)
		scored, searchErr := idx.qd.Search(storeCtx, vector, filter, uint64(maxRes))
		if searchErr != nil {
			err = searchErr
		} else {
			list = scoredPointsToRanked(scored)
		}
		results <- storeResult{store: "qdrant", list: list, err: err, duration: time.Since(start)}
		return nil // soft-fail: never propagate per-store error to errgroup
	})

	// Meilisearch goroutine.
	eg.Go(func() error {
		start := time.Now()
		storeCtx, cancel := idx.deriveStoreCtx(parent, "meili")
		defer cancel()

		var list []Ranked
		var err error

		opts := meili.SearchOptions{Limit: int64(maxRes), Filter: buildMeiliFilter(q)}
		docs, searchErr := idx.me.Search(storeCtx, idx.me.IndexName(), q.Text, opts)
		if searchErr != nil {
			err = searchErr
		} else {
			list = meiliDocsToRanked(docs)
		}
		results <- storeResult{store: "meili", list: list, err: err, duration: time.Since(start)}
		return nil
	})

	// PostgreSQL goroutine (filter-only per research §3.10).
	eg.Go(func() error {
		start := time.Now()
		storeCtx, cancel := idx.deriveStoreCtx(parent, "pg")
		defer cancel()

		var list []Ranked
		var err error

		filters := buildPGFilters(q, maxRes)
		rows, searchErr := idx.pg.Search(storeCtx, filters)
		if searchErr != nil {
			err = searchErr
		} else {
			list = pgRowsToRanked(rows)
		}
		results <- storeResult{store: "pg", list: list, err: err, duration: time.Since(start)}
		return nil
	})

	_ = eg.Wait()
	close(results)

	for r := range results {
		rankLists[r.store] = r.list
		errs[r.store] = r.err
		latencies[r.store] = r.duration
	}
	return rankLists, errs, latencies
}

// upsertPartial holds per-store upsert outcome.
type upsertPartial struct {
	store    string
	inserted int
	skipped  int
	err      error
	duration time.Duration
}

// parallelUpsert dispatches concurrent writes to all three stores for a single batch.
//
// @MX:WARN: [AUTO] Outbound fan-out spawns 3 goroutines per batch. Same goroutine-lifecycle invariant as parallelSearch.
// @MX:REASON: every per-store ctx MUST be cancelled; goroutine count is bounded by errgroup.SetLimit(3).
func (idx *Index) parallelUpsert(
	parent context.Context,
	docs []types.NormalizedDoc,
) []upsertPartial {
	results := make(chan upsertPartial, 3)

	eg, _ := errgroup.WithContext(parent)
	eg.SetLimit(idx.opts.MaxParallel)

	// Qdrant goroutine.
	eg.Go(func() error {
		start := time.Now()
		storeCtx, cancel := idx.deriveStoreCtx(parent, "qdrant")
		defer cancel()

		var err error
		points := docsToQdrantPoints(docs, idx.embedder)
		if upsertErr := idx.qd.Upsert(storeCtx, points); upsertErr != nil {
			err = upsertErr
		}
		results <- upsertPartial{store: "qdrant", inserted: len(docs), err: err, duration: time.Since(start)}
		return nil
	})

	// Meilisearch goroutine.
	eg.Go(func() error {
		start := time.Now()
		storeCtx, cancel := idx.deriveStoreCtx(parent, "meili")
		defer cancel()

		var err error
		mDocs := docsToMeiliDocs(docs)
		task, addErr := idx.me.AddDocuments(storeCtx, idx.me.IndexName(), mDocs)
		if addErr != nil {
			err = addErr
		}
		_ = task // fire-and-forget per D12; tests call WaitForTask separately
		results <- upsertPartial{store: "meili", inserted: len(docs), err: err, duration: time.Since(start)}
		return nil
	})

	// PostgreSQL goroutine.
	eg.Go(func() error {
		start := time.Now()
		storeCtx, cancel := idx.deriveStoreCtx(parent, "pg")
		defer cancel()

		rows := docsToPGRows(docs)
		inserted, skipped, err := idx.pg.Upsert(storeCtx, rows)
		results <- upsertPartial{store: "pg", inserted: inserted, skipped: skipped, err: err, duration: time.Since(start)}
		return nil
	})

	_ = eg.Wait()
	close(results)

	var partials []upsertPartial
	for r := range results {
		partials = append(partials, r)
	}
	return partials
}

// --- Filter builders ---

func buildQdrantFilter(q IndexQuery) *qdrant.Filter {
	if q.SourceID == "" && q.Lang == "" && q.TeamID == "" {
		return nil
	}
	return &qdrant.Filter{SourceID: q.SourceID, Lang: q.Lang, TeamID: q.TeamID}
}

func buildMeiliFilter(q IndexQuery) string {
	parts := []string{}
	if q.SourceID != "" {
		parts = append(parts, "source_id = \""+q.SourceID+"\"")
	}
	if q.Lang != "" {
		parts = append(parts, "lang = \""+q.Lang+"\"")
	}
	if q.TeamID != "" {
		parts = append(parts, "team_id = \""+q.TeamID+"\"")
	}
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " AND " + parts[i]
	}
	return result
}

func buildPGFilters(q IndexQuery, limit int) pg.Filters {
	f := pg.Filters{Limit: limit, SourceID: q.SourceID, Lang: q.Lang, TeamID: q.TeamID}
	if !q.Since.IsZero() {
		t := q.Since
		f.Since = &t
	}
	if !q.Until.IsZero() {
		t := q.Until
		f.Until = &t
	}
	return f
}

// --- Ranked converters ---

func scoredPointsToRanked(pts []qdrant.ScoredPoint) []Ranked {
	out := make([]Ranked, 0, len(pts))
	for _, p := range pts {
		doc := payloadToDoc(p.ID, p.Payload)
		out = append(out, Ranked{DocID: p.ID, Doc: doc})
	}
	return out
}

func meiliDocsToRanked(docs []meili.Document) []Ranked {
	out := make([]Ranked, 0, len(docs))
	for _, d := range docs {
		docID, _ := d["doc_id"].(string)
		doc := meiliDocToNormalizedDoc(d)
		out = append(out, Ranked{DocID: docID, Doc: doc})
	}
	return out
}

func pgRowsToRanked(rows []pg.DocRow) []Ranked {
	out := make([]Ranked, 0, len(rows))
	for _, r := range rows {
		doc := pgRowToNormalizedDoc(r)
		out = append(out, Ranked{DocID: r.DocID, Doc: doc})
	}
	return out
}

// --- Document converters ---

func docsToQdrantPoints(docs []types.NormalizedDoc, embedder Embedder) []qdrant.Point {
	texts := make([]string, len(docs))
	for i, d := range docs {
		texts[i] = d.Title + " " + d.Body
	}

	vectors, err := embedder.Embed(context.Background(), texts)
	if err != nil || len(vectors) != len(docs) {
		// Fall back to zero vectors on embed error.
		vectors = make([][]float32, len(docs))
		for i := range vectors {
			vectors[i] = make([]float32, embedder.Dimensions())
		}
	}

	points := make([]qdrant.Point, 0, len(docs))
	for i, d := range docs {
		id := docID(d.SourceID, d.URL)
		var teamID any
		if d.Metadata != nil {
			teamID = d.Metadata["team_id"]
		}
		payload := map[string]any{
			"source_id":    d.SourceID,
			"url":          d.URL,
			"title":        d.Title,
			"lang":         d.Lang,
			"doc_type":     string(d.DocType),
			"retrieved_at": d.RetrievedAt.Unix(),
			"content_hash": d.CanonicalHash(),
			"team_id":      teamID,
		}
		if !d.PublishedAt.IsZero() {
			payload["published_at"] = d.PublishedAt.Unix()
		}
		points = append(points, qdrant.Point{ID: id, Vector: vectors[i], Payload: payload})
	}
	return points
}

func docsToMeiliDocs(docs []types.NormalizedDoc) []meili.Document {
	out := make([]meili.Document, 0, len(docs))
	for _, d := range docs {
		id := docID(d.SourceID, d.URL)
		var publishedAt any
		if !d.PublishedAt.IsZero() {
			publishedAt = d.PublishedAt.Unix()
		}
		out = append(out, meili.Document{
			"doc_id":       id,
			"source_id":    d.SourceID,
			"url":          d.URL,
			"title":        d.Title,
			"body":         d.Body,
			"snippet":      d.Snippet,
			"lang":         d.Lang,
			"doc_type":     string(d.DocType),
			"published_at": publishedAt,
			"team_id":      nil, // v0.1: always null per SPEC-IDX-004 reservation
			"content_hash": d.CanonicalHash(),
		})
	}
	return out
}

func docsToPGRows(docs []types.NormalizedDoc) []pg.DocRow {
	rows := make([]pg.DocRow, 0, len(docs))
	for _, d := range docs {
		id := docID(d.SourceID, d.URL)
		hash := d.CanonicalHash()
		docType := string(d.DocType)
		row := pg.DocRow{
			DocID:       id,
			ContentHash: hash,
			SourceID:    d.SourceID,
			URL:         d.URL,
			Title:       d.Title,
			Body:        d.Body,
			Snippet:     d.Snippet,
			Lang:        d.Lang,
			DocType:     docType,
			RetrievedAt: d.RetrievedAt,
			TeamID:      nil, // v0.1: NULL per SPEC-IDX-004 reservation
			Payload:     []byte("{}"),
		}
		if !d.PublishedAt.IsZero() {
			t := d.PublishedAt
			row.PublishedAt = &t
		}
		rows = append(rows, row)
	}
	return rows
}

func payloadToDoc(id string, payload map[string]any) types.NormalizedDoc {
	getString := func(key string) string {
		if v, ok := payload[key].(string); ok {
			return v
		}
		return ""
	}
	return types.NormalizedDoc{
		ID:       id,
		SourceID: getString("source_id"),
		URL:      getString("url"),
		Title:    getString("title"),
		Lang:     getString("lang"),
		DocType:  types.DocType(getString("doc_type")),
	}
}

func meiliDocToNormalizedDoc(d meili.Document) types.NormalizedDoc {
	getString := func(key string) string {
		if v, ok := d[key].(string); ok {
			return v
		}
		return ""
	}
	return types.NormalizedDoc{
		ID:       getString("doc_id"),
		SourceID: getString("source_id"),
		URL:      getString("url"),
		Title:    getString("title"),
		Body:     getString("body"),
		Snippet:  getString("snippet"),
		Lang:     getString("lang"),
		DocType:  types.DocType(getString("doc_type")),
	}
}

func pgRowToNormalizedDoc(r pg.DocRow) types.NormalizedDoc {
	doc := types.NormalizedDoc{
		ID:          r.DocID,
		SourceID:    r.SourceID,
		URL:         r.URL,
		Title:       r.Title,
		Body:        r.Body,
		Snippet:     r.Snippet,
		Lang:        r.Lang,
		DocType:     types.DocType(r.DocType),
		RetrievedAt: r.RetrievedAt,
	}
	if r.PublishedAt != nil {
		doc.PublishedAt = *r.PublishedAt
	}
	return doc
}
