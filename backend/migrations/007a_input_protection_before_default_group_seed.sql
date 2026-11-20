-- Migration 008 inserts the initial group before 258 introduces the historical
-- policy column. Give that seed the new-group default on a fresh installation.
SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '30s';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM groups) THEN
        ALTER TABLE groups ADD COLUMN IF NOT EXISTS gemini_input_counting_enabled BOOLEAN NOT NULL DEFAULT TRUE;
    END IF;
END $$;
