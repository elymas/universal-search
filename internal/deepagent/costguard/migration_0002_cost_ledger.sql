-- SPEC-DEEP-004 Phase A: cost_ledger table for /deep quota and cost tracking.
-- Idempotent: IF NOT EXISTS prevents errors on re-run.
-- REQ-DEEP4-006: every Go-side llm.Client call writes one row.

CREATE TABLE IF NOT EXISTS cost_ledger (
    id               BIGSERIAL     PRIMARY KEY,
    user_id          TEXT          NOT NULL DEFAULT 'anonymous',
    tenant_id        TEXT          NOT NULL DEFAULT 'default',
    request_id       TEXT          NOT NULL,
    deep_run_id      TEXT,
    model            TEXT          NOT NULL,
    prompt_tokens    INT           NOT NULL DEFAULT 0,
    completion_tokens INT          NOT NULL DEFAULT 0,
    usd_cost         NUMERIC(10,6) NOT NULL DEFAULT 0,
    cache_hit        BOOLEAN       NOT NULL DEFAULT FALSE,
    intent_category  TEXT,
    outcome          TEXT          NOT NULL,
    ts               TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cost_ledger_user_ts ON cost_ledger(user_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_cost_ledger_tenant_ts ON cost_ledger(tenant_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_cost_ledger_deep_run ON cost_ledger(deep_run_id) WHERE deep_run_id IS NOT NULL;
