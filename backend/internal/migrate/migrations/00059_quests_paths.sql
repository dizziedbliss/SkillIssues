-- +goose Up
-- Learning paths: a role's five catalog issues, beginner to intermediate,
-- done one merge at a time. Quests need no table: they are ledger rows
-- (xp_events, source 'quest', key = the quest).

CREATE TABLE IF NOT EXISTS learning_paths (
  id           BIGSERIAL PRIMARY KEY,
  user_id      INTEGER     NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role         TEXT        NOT NULL,
  started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at  TIMESTAMPTZ,
  abandoned_at TIMESTAMPTZ
);
-- One path in progress per person.
CREATE UNIQUE INDEX IF NOT EXISTS learning_paths_one_open ON learning_paths (user_id) WHERE finished_at IS NULL AND abandoned_at IS NULL;

CREATE TABLE IF NOT EXISTS learning_path_steps (
  path_id   BIGINT  NOT NULL REFERENCES learning_paths(id) ON DELETE CASCADE,
  position  INTEGER NOT NULL,
  -- NULL once its issue is pruned from the catalog; a step done keeps done_at.
  issue_id  INTEGER REFERENCES issues(id) ON DELETE SET NULL,
  difficulty TEXT   NOT NULL,
  -- The merged pull request that closed it, as ingestd saw it.
  done_at   TIMESTAMPTZ,
  PRIMARY KEY (path_id, position)
);

-- XP the tracker pays when a pull request of the claimer's closes an issue
-- they claimed reads the claim.
GRANT SELECT ON issue_claims TO skillissues_ingest;
GRANT SELECT, INSERT, UPDATE ON learning_paths, learning_path_steps TO skillissues_api;
GRANT USAGE ON SEQUENCE learning_paths_id_seq TO skillissues_api;
GRANT SELECT, UPDATE ON learning_paths, learning_path_steps TO skillissues_ingest;

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('xp.quest_reward', '50', 'int', 0, 10000, 'XP for finishing a quest.'),
  ('xp.path_reward', '200', 'int', 0, 100000, 'XP for finishing a learning path.'),
  ('api.paths.beginner_steps', '3', 'int', 0, 10, 'Beginner issues a new learning path starts with.'),
  ('api.paths.intermediate_steps', '2', 'int', 0, 10, 'Intermediate issues a new learning path ends with.'),
  ('api.paths.pick_pages', '5', 'int', 1, 50, 'Shuffled-feed pages read to find a learning path''s issues of one difficulty.')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM settings WHERE key IN ('xp.quest_reward', 'xp.path_reward', 'api.paths.beginner_steps', 'api.paths.intermediate_steps', 'api.paths.pick_pages');
REVOKE SELECT ON issue_claims FROM skillissues_ingest;
DROP TABLE IF EXISTS learning_path_steps;
DROP TABLE IF EXISTS learning_paths;
