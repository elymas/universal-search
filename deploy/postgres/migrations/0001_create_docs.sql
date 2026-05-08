-- SPEC-IDX-001 REQ-IDX-008: Create docs table with two-key idempotency schema.
-- Idempotent: IF NOT EXISTS prevents errors on re-run.

CREATE TABLE IF NOT EXISTS docs (
    doc_id       TEXT        PRIMARY KEY,
    content_hash TEXT        NOT NULL UNIQUE,
    source_id    TEXT        NOT NULL,
    url          TEXT        NOT NULL,
    title        TEXT,
    body         TEXT,
    snippet      TEXT,
    lang         TEXT,
    doc_type     TEXT,
    published_at TIMESTAMPTZ,
    retrieved_at TIMESTAMPTZ NOT NULL,
    team_id      TEXT        NULL,  -- reserved for SPEC-IDX-004 multi-tenancy enforcement
    payload      JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- B-tree indexes for filter-only search (REQ-IDX-008).
CREATE INDEX IF NOT EXISTS idx_docs_source_id    ON docs (source_id);
CREATE INDEX IF NOT EXISTS idx_docs_published_at ON docs (published_at);
CREATE INDEX IF NOT EXISTS idx_docs_team_id      ON docs (team_id);
