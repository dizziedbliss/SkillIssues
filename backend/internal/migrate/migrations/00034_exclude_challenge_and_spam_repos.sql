-- Three more repositories that are never crawled (see 00028 for why and how the
-- list is applied):
--
--   type-challenges/type-challenges  its issues are user-submitted TypeScript
--                                    challenges, not work a contributor can pick up
--   haizlin/fe-interview             an interview-question collection; the issues
--                                    are the questions
--   microsoft/JARVIS                 the tracker is flooded with spam issues
--
-- Takes effect now rather than at the next eligibility recheck, and the
-- coordinator's prune step then deletes the issues already stored for them
-- (never one a user saved).
--
-- +goose Up

INSERT INTO excluded_repositories (full_name, reason) VALUES
  ('type-challenges/type-challenges', 'issues are user-submitted challenges, not contributable work'),
  ('haizlin/fe-interview',            'issues are interview questions, not contributable work'),
  ('microsoft/JARVIS',                'issue tracker flooded with spam')
ON CONFLICT DO NOTHING;

UPDATE repositories
SET is_eligible = false, eligibility_reason = 'excluded'
WHERE lower(full_name) IN (
    lower('type-challenges/type-challenges'),
    lower('haizlin/fe-interview'),
    lower('microsoft/JARVIS'))
  AND eligibility_reason IS DISTINCT FROM 'not_found';

-- +goose Down

DELETE FROM excluded_repositories
WHERE lower(full_name) IN (
    lower('type-challenges/type-challenges'),
    lower('haizlin/fe-interview'),
    lower('microsoft/JARVIS'));
