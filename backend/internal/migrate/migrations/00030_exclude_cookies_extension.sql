-- Add OhMyGuus/I-Still-Dont-Care-About-Cookies to the repositories that are
-- never crawled (see 00028 for why and how the list is applied). Its issue
-- tracker is a stream of per-site breakage reports for a browser extension,
-- not work a contributor can pick up.
--
-- Takes effect now rather than at the next eligibility recheck, and the
-- coordinator's prune step then deletes the issues already stored for it
-- (never one a user saved).
--
-- +goose Up

INSERT INTO excluded_repositories (full_name, reason) VALUES
  ('OhMyGuus/I-Still-Dont-Care-About-Cookies',
   'issue tracker is per-site breakage reports for a browser extension, not contributable work')
ON CONFLICT DO NOTHING;

UPDATE repositories
SET is_eligible = false, eligibility_reason = 'excluded'
WHERE lower(full_name) = lower('OhMyGuus/I-Still-Dont-Care-About-Cookies')
  AND eligibility_reason IS DISTINCT FROM 'not_found';

-- +goose Down

DELETE FROM excluded_repositories
WHERE lower(full_name) = lower('OhMyGuus/I-Still-Dont-Care-About-Cookies');
