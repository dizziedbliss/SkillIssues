-- Real submission time (D2d, Phase 2): `created_at` is when the contribution
-- was started, not when the PR was opened. Backfill existing PR_OPEN/PR_MERGED
-- rows from `updated_at`, the best available approximation for history that
-- predates this column.
--
-- +goose Up

ALTER TABLE contributions ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ;

UPDATE contributions
SET submitted_at = updated_at
WHERE submitted_at IS NULL AND state IN ('PR_OPEN', 'PR_CLOSED', 'PR_MERGED', 'VERIFIED');

-- +goose Down

ALTER TABLE contributions DROP COLUMN IF EXISTS submitted_at;
