-- D17: issue bodies are never ingested or stored unbounded. Nothing in the
-- Go codebase has read issues.body since the on-demand read path
-- (internal/issue/body.go, issue_body_cache from migration 00003) shipped
-- in this same phase — verified by grep before writing this migration.
--
-- Deliberately held back until here rather than run with 00003: this way
-- 00003/00004 and the code deploy that stops reading the column stay
-- independently reversible. This step is not: the down migration restores
-- an empty column, not the original text, which is only recoverable from
-- GitHub itself via the on-demand path.
--
-- +goose Up

ALTER TABLE issues DROP COLUMN IF EXISTS body;

-- +goose Down

ALTER TABLE issues ADD COLUMN IF NOT EXISTS body TEXT DEFAULT '';
