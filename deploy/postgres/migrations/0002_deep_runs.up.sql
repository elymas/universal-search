-- SPEC-DEEP-003 Phase D: deep_runs table for /deep tree exploration run summaries.
-- Idempotent: IF NOT EXISTS prevents errors on re-run.

CREATE TABLE IF NOT EXISTS deep_runs (
    run_id        VARCHAR(64)  PRIMARY KEY,
    query         TEXT         NOT NULL,
    breadth       INTEGER      NOT NULL DEFAULT 4,
    depth         INTEGER      NOT NULL DEFAULT 3,
    total_nodes   INTEGER      NOT NULL DEFAULT 0,
    total_tokens  BIGINT       NOT NULL DEFAULT 0,
    total_cost_usd NUMERIC(10,4) NOT NULL DEFAULT 0,
    status        VARCHAR(32)  NOT NULL DEFAULT 'pending',
    started_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_deep_runs_status      ON deep_runs (status);
CREATE INDEX IF NOT EXISTS idx_deep_runs_started_at   ON deep_runs (started_at);
