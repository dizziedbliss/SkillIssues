ALTER TABLE developer_profiles
  DROP CONSTRAINT IF EXISTS chk_experience_level,
  DROP CONSTRAINT IF EXISTS chk_target_level;

ALTER TABLE issues
  DROP CONSTRAINT IF EXISTS chk_difficulty,
  DROP CONSTRAINT IF EXISTS chk_state;

ALTER TABLE contributions
  DROP CONSTRAINT IF EXISTS chk_contribution_state;

ALTER TABLE sync_state
  DROP CONSTRAINT IF EXISTS chk_priority,
  DROP CONSTRAINT IF EXISTS chk_sync_status;
