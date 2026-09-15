-- Migration 002: Add missing author column to issues table
-- CRITICAL BUG FIX: ingestion/app.py upserts an `author` field that was
-- never defined in the schema, causing silent runtime errors.

ALTER TABLE issues ADD COLUMN IF NOT EXISTS author TEXT;

COMMENT ON COLUMN issues.author IS
  'GitHub login of the issue author. Nullable: GitHub accounts may be deleted or unavailable.';
