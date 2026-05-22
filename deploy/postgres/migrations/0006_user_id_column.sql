-- SPEC-IDX-004 REQ-IDX4-010: Add user_id column + partial index.
-- Idempotent: ADD COLUMN IF NOT EXISTS + CREATE INDEX IF NOT EXISTS.

-- Add user_id column (NULL — only set for user_private visibility).
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'docs' AND column_name = 'user_id'
    ) THEN
        ALTER TABLE docs ADD COLUMN user_id TEXT NULL;
    END IF;
END $$;

-- Partial index for efficient user-private lookups.
CREATE INDEX IF NOT EXISTS idx_docs_team_user
    ON docs (team_id, user_id) WHERE user_id IS NOT NULL;
