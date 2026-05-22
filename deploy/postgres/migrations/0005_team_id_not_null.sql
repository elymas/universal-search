-- SPEC-IDX-004 REQ-IDX4-010: team_id NOT NULL with default + composite indexes.
-- Single-transaction: backfill NULL → 'default' + ALTER SET NOT NULL + composite indexes.
-- Idempotent: uses COALESCE pattern and IF NOT EXISTS.

BEGIN;

-- Step 1: Backfill NULL team_id rows with 'default'.
UPDATE docs SET team_id = 'default' WHERE team_id IS NULL;

-- Step 2: Alter team_id to NOT NULL DEFAULT 'default'.
-- Only apply if not already NOT NULL.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'docs' AND column_name = 'team_id' AND is_nullable = 'YES'
    ) THEN
        ALTER TABLE docs ALTER COLUMN team_id SET NOT NULL;
        ALTER TABLE docs ALTER COLUMN team_id SET DEFAULT 'default';
    END IF;
END $$;

-- Step 3: Composite indexes for efficient team-scoped queries.
CREATE INDEX IF NOT EXISTS idx_docs_team_id_source_id
    ON docs (team_id, source_id);

CREATE INDEX IF NOT EXISTS idx_docs_team_published
    ON docs (team_id, published_at DESC);

COMMIT;
