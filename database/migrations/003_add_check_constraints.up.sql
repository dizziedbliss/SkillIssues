-- Migration 003: Add CHECK constraints for enum columns
-- Enforces valid values at the database level.

ALTER TABLE developer_profiles
  ADD CONSTRAINT chk_experience_level
    CHECK (experience_level IN ('BEGINNER', 'INTERMEDIATE', 'ADVANCED')),
  ADD CONSTRAINT chk_target_level
    CHECK (target_level IN ('BEGINNER', 'INTERMEDIATE', 'ADVANCED'));

ALTER TABLE issues
  ADD CONSTRAINT chk_difficulty
    CHECK (difficulty IN ('BEGINNER', 'INTERMEDIATE', 'ADVANCED')),
  ADD CONSTRAINT chk_state
    CHECK (state IN ('OPEN', 'CLOSED'));

ALTER TABLE contributions
  ADD CONSTRAINT chk_contribution_state
    CHECK (state IN ('STARTED', 'PR_OPEN', 'PR_CLOSED', 'PR_MERGED', 'VERIFIED'));

ALTER TABLE sync_state
  ADD CONSTRAINT chk_priority
    CHECK (priority IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
  ADD CONSTRAINT chk_sync_status
    CHECK (sync_status IN ('PENDING', 'RUNNING', 'DONE', 'FAILED'));
