-- Migration 006 Down: Drop unique lower username index
DROP INDEX IF EXISTS idx_users_username_lower;

