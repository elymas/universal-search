-- SPEC-IDX-005: answer_cache table for team-shared answer reuse.
-- REQ-IDX5-006: Durable storage for cached synthesis responses.
-- REQ-IDX5-007: team_id isolation key for cross-tenant protection.

CREATE TABLE IF NOT EXISTS answer_cache (
    doc_id          TEXT        NOT NULL,
    team_id         TEXT        NOT NULL,
    query_hash      TEXT        NOT NULL,
    query_text      TEXT        NOT NULL DEFAULT '',
    category        TEXT        NOT NULL DEFAULT 'unknown',
    response_json   TEXT        NOT NULL DEFAULT '',
    similarity      DOUBLE PRECISION NOT NULL DEFAULT 0,
    ttl_seconds     INTEGER     NOT NULL DEFAULT 7200,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_served_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    hit_count       INTEGER     NOT NULL DEFAULT 0,
    force_stale     BOOLEAN     NOT NULL DEFAULT FALSE,
    PRIMARY KEY (doc_id)
);

-- Index for team-scoped lookups ordered by creation time.
CREATE INDEX IF NOT EXISTS idx_answer_cache_team_created
    ON answer_cache (team_id, created_at DESC);

-- Index for category-based queries within a team.
CREATE INDEX IF NOT EXISTS idx_answer_cache_team_category
    ON answer_cache (team_id, category);

-- Index for force_stale eviction scans.
CREATE INDEX IF NOT EXISTS idx_answer_cache_force_stale
    ON answer_cache (force_stale) WHERE force_stale = TRUE;
