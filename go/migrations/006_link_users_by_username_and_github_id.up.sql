-- Migration 006: Add unique lower username index to prevent duplicate accounts between PAT and OAuth
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_lower ON users (lower(username));

