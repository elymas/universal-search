-- SPEC-AUTH-003 Phase A: audit_events table for immutable audit trail.
-- Idempotent: IF NOT EXISTS prevents errors on re-run.
-- REQ-AUTH3-001: append-only, monthly partitioning, role separation.

-- Role separation: audit_writer can only SELECT/INSERT.
-- audit_admin can also DROP PARTITION.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'audit_writer') THEN
        CREATE ROLE audit_writer NOLOGIN;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'audit_admin') THEN
        CREATE ROLE audit_admin NOLOGIN;
    END IF;
END
$$;

-- Partitioned audit_events table (PARTITION BY RANGE ts).
CREATE TABLE IF NOT EXISTS audit_events (
    id          BIGSERIAL,
    event_type  TEXT        NOT NULL,
    decision    TEXT        NOT NULL DEFAULT 'none',
    user_id     TEXT        NOT NULL DEFAULT 'anonymous',
    tenant_id   TEXT        NOT NULL DEFAULT 'default',
    team_id     TEXT,
    request_id  TEXT,
    source      TEXT        NOT NULL DEFAULT 'go',
    resource    TEXT,
    action      TEXT,
    ip          TEXT,
    payload     JSONB       NOT NULL DEFAULT '{}',
    ts          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    prev_hash   TEXT,
    this_hash   TEXT,
    PRIMARY KEY (id, ts)
) PARTITION BY RANGE (ts);

-- Grants: audit_writer = SELECT + INSERT only.
GRANT SELECT, INSERT ON audit_events TO audit_writer;
GRANT USAGE, SELECT ON SEQUENCE audit_events_id_seq TO audit_writer;

-- Grants: audit_admin = all (for DROP PARTITION).
GRANT ALL ON audit_events TO audit_admin;
GRANT ALL ON SEQUENCE audit_events_id_seq TO audit_admin;

-- Append-only triggers: block UPDATE and DELETE.
-- NFR-AUTH3-001: application connection UPDATE/DELETE/TRUNCATE blocked.
CREATE OR REPLACE FUNCTION audit_events_no_update()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only: UPDATE is not permitted';
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_events_no_update_trigger ON audit_events;
CREATE TRIGGER audit_events_no_update_trigger
    BEFORE UPDATE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_no_update();

CREATE OR REPLACE FUNCTION audit_events_no_delete()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only: DELETE is not permitted';
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_events_no_delete_trigger ON audit_events;
CREATE TRIGGER audit_events_no_delete_trigger
    BEFORE DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_no_delete();

-- Indexes for common query patterns.
CREATE INDEX IF NOT EXISTS idx_audit_events_event_type ON audit_events(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_events_user_id ON audit_events(user_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_ts ON audit_events(tenant_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_request_id ON audit_events(request_id) WHERE request_id IS NOT NULL;

-- Helper function to create monthly partitions.
-- Call: SELECT create_audit_partition('2026-06-01');
CREATE OR REPLACE FUNCTION create_audit_partition(partition_start TIMESTAMPTZ)
RETURNS VOID AS $$
DECLARE
    partition_end TIMESTAMPTZ;
    partition_name TEXT;
    start_str TEXT;
    end_str TEXT;
BEGIN
    partition_end := partition_start + INTERVAL '1 month';
    partition_name := 'audit_events_y' || to_char(partition_start, 'YYYY') || 'm' || to_char(partition_start, 'MM');
    start_str := to_char(partition_start, 'YYYY-MM-DD');
    end_str := to_char(partition_end, 'YYYY-MM-DD');

    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF audit_events FOR VALUES FROM (%L) TO (%L)',
        partition_name, start_str, end_str
    );

    -- Grant same permissions on partition.
    EXECUTE format('GRANT SELECT, INSERT ON %I TO audit_writer', partition_name);
    EXECUTE format('GRANT ALL ON %I TO audit_admin', partition_name);
END;
$$ LANGUAGE plpgsql;

-- Create initial partitions for current and next month.
-- These will be extended at startup via EnsureCurrentPartition.
SELECT create_audit_partition(date_trunc('month', CURRENT_DATE));
SELECT create_audit_partition(date_trunc('month', CURRENT_DATE) + INTERVAL '1 month');

-- audit_partitions metadata table for tracking archived_at.
CREATE TABLE IF NOT EXISTS audit_partitions (
    partition_name TEXT PRIMARY KEY,
    range_start    TIMESTAMPTZ NOT NULL,
    range_end      TIMESTAMPTZ NOT NULL,
    archived_at    TIMESTAMPTZ,
    row_count      BIGINT      DEFAULT 0
);
