// Package pg provides the PostgreSQL sub-client for the hybrid index layer.
// SPEC-IDX-001 REQ-IDX-008 (scope item h).
package pg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds construction parameters for the PostgreSQL client.
type Config struct {
	// ConnString is the pgx DSN (e.g., "postgresql://user:pass@localhost:5432/db").
	ConnString string
	// MaxConns is the connection pool maximum (default 6 = 2 × MaxParallel).
	MaxConns int32
	// MigrationsDir is the path to migration SQL files (default "deploy/postgres/migrations").
	MigrationsDir string
}

// DocRow is a single row in the docs table.
//
// @MX:NOTE: [AUTO] Two-key idempotency: ON CONFLICT (doc_id) DO UPDATE for content edits + UNIQUE (content_hash) for replay.
// @MX:SPEC: SPEC-IDX-001
type DocRow struct {
	DocID       string
	ContentHash string
	SourceID    string
	URL         string
	Title       string
	Body        string
	Snippet     string
	Lang        string
	DocType     string
	PublishedAt *time.Time
	RetrievedAt time.Time
	TeamID      *string // NULL in v0.1 per SPEC-IDX-004 reservation
	Payload     []byte  // JSONB
}

// Filters encodes PostgreSQL query filter conditions.
type Filters struct {
	SourceID string
	Lang     string
	TeamID   string
	Since    *time.Time
	Until    *time.Time
	Limit    int
}

// Client wraps a pgxpool.Pool with index-layer semantics.
type Client struct {
	pool *pgxpool.Pool
	cfg  Config
}

// NewClient creates a PostgreSQL connection pool from cfg.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.MaxConns == 0 {
		cfg.MaxConns = 6
	}
	if cfg.MigrationsDir == "" {
		cfg.MigrationsDir = "deploy/postgres/migrations"
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.ConnString)
	if err != nil {
		return nil, fmt.Errorf("pg: parse config: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxConns

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("pg: new pool: %w", err)
	}

	return &Client{pool: pool, cfg: cfg}, nil
}

// EnsureSchema applies migration files from MigrationsDir idempotently.
// Re-running against an existing schema is a no-op. Structural drift (missing
// expected columns) returns ErrSchemaBootstrapFailed.
func (c *Client) EnsureSchema(ctx context.Context) error {
	// Read migration files in lexicographic order.
	entries, err := os.ReadDir(c.cfg.MigrationsDir)
	if err != nil {
		return fmt.Errorf("pg: read migrations dir %q: %w", c.cfg.MigrationsDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := filepath.Join(c.cfg.MigrationsDir, entry.Name())
		sql, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("pg: read migration %q: %w", entry.Name(), readErr)
		}

		if _, execErr := c.pool.Exec(ctx, string(sql)); execErr != nil {
			return fmt.Errorf("pg: exec migration %q: %w", entry.Name(), execErr)
		}
	}

	// Drift check: verify expected columns exist.
	return c.verifySchema(ctx)
}

// verifySchema checks that the docs table has all expected columns.
func (c *Client) verifySchema(ctx context.Context) error {
	required := []string{
		"doc_id", "content_hash", "source_id", "url", "title", "body",
		"snippet", "lang", "doc_type", "published_at", "retrieved_at",
		"team_id", "payload", "created_at", "updated_at",
	}

	rows, err := c.pool.Query(ctx,
		`SELECT column_name FROM information_schema.columns WHERE table_name = 'docs'`)
	if err != nil {
		return fmt.Errorf("pg: verify schema: %w", err)
	}
	defer rows.Close()

	found := make(map[string]bool)
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return fmt.Errorf("pg: scan column name: %w", err)
		}
		found[col] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, col := range required {
		if !found[col] {
			return fmt.Errorf("pg: schema drift: missing column %q: %w", col, errSchemaDrift)
		}
	}
	return nil
}

// errSchemaDrift is the sentinel wrapped inside ErrSchemaBootstrapFailed.
var errSchemaDrift = errors.New("schema drift detected")

// Upsert inserts or updates a batch of DocRows idempotently.
// Uses ON CONFLICT (doc_id) DO UPDATE semantics.
// Returns (inserted, skipped, error). "skipped" counts rows where content_hash
// matched (identical content replay, no actual data change).
//
// @MX:NOTE: [AUTO] Two-key idempotency: ON CONFLICT (doc_id) DO UPDATE for content edits.
// @MX:SPEC: SPEC-IDX-001
func (c *Client) Upsert(ctx context.Context, docs []DocRow) (inserted, skipped int, err error) {
	if len(docs) == 0 {
		return 0, 0, nil
	}

	batch := &pgx.Batch{}
	for _, d := range docs {
		batch.Queue(`
INSERT INTO docs
  (doc_id, content_hash, source_id, url, title, body, snippet, lang, doc_type,
   published_at, retrieved_at, team_id, payload)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT (doc_id) DO UPDATE
  SET title        = EXCLUDED.title,
      body         = EXCLUDED.body,
      snippet      = EXCLUDED.snippet,
      content_hash = EXCLUDED.content_hash,
      retrieved_at = EXCLUDED.retrieved_at,
      updated_at   = NOW()
  WHERE docs.content_hash IS DISTINCT FROM EXCLUDED.content_hash
RETURNING doc_id`,
			d.DocID, d.ContentHash, d.SourceID, d.URL, d.Title, d.Body, d.Snippet,
			d.Lang, d.DocType, d.PublishedAt, d.RetrievedAt, d.TeamID, d.Payload,
		)
	}

	br := c.pool.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()

	for range docs {
		rows, qErr := br.Query()
		if qErr != nil {
			return inserted, skipped, fmt.Errorf("pg: upsert batch: %w", qErr)
		}
		if rows.Next() {
			inserted++
		} else {
			skipped++
		}
		rows.Close()
		if rows.Err() != nil {
			return inserted, skipped, fmt.Errorf("pg: upsert row error: %w", rows.Err())
		}
	}
	return inserted, skipped, nil
}

// Search queries the docs table with the given filters (filter-only, no full-text).
func (c *Client) Search(ctx context.Context, filters Filters) ([]DocRow, error) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}

	args := []any{}
	conds := []string{"1=1"}
	n := 1

	if filters.SourceID != "" {
		conds = append(conds, fmt.Sprintf("source_id = $%d", n))
		args = append(args, filters.SourceID)
		n++
	}
	if filters.Lang != "" {
		conds = append(conds, fmt.Sprintf("lang = $%d", n))
		args = append(args, filters.Lang)
		n++
	}
	if filters.TeamID != "" {
		conds = append(conds, fmt.Sprintf("team_id = $%d", n))
		args = append(args, filters.TeamID)
		n++
	}
	if filters.Since != nil {
		conds = append(conds, fmt.Sprintf("published_at >= $%d", n))
		args = append(args, *filters.Since)
		n++
	}
	if filters.Until != nil {
		conds = append(conds, fmt.Sprintf("published_at <= $%d", n))
		args = append(args, *filters.Until)
	}

	query := fmt.Sprintf(`
SELECT doc_id, content_hash, source_id, url, title, body, snippet, lang,
       doc_type, published_at, retrieved_at, team_id, payload
FROM docs
WHERE %s
ORDER BY retrieved_at DESC
LIMIT %d`, strings.Join(conds, " AND "), limit)

	rows, err := c.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("pg: search: %w", err)
	}
	defer rows.Close()

	var result []DocRow
	for rows.Next() {
		var r DocRow
		if scanErr := rows.Scan(
			&r.DocID, &r.ContentHash, &r.SourceID, &r.URL, &r.Title, &r.Body,
			&r.Snippet, &r.Lang, &r.DocType, &r.PublishedAt, &r.RetrievedAt,
			&r.TeamID, &r.Payload,
		); scanErr != nil {
			return nil, fmt.Errorf("pg: scan row: %w", scanErr)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// Close closes the connection pool.
func (c *Client) Close() {
	c.pool.Close()
}

// Pool returns the underlying pgxpool for test helpers.
func (c *Client) Pool() *pgxpool.Pool {
	return c.pool
}
