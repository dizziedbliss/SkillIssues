-- A reserve of the GitHub budget for interactive work.
--
-- The crawler and the work a user is waiting on (tracked-issue checks now;
-- PR tracking and the issue-body fallback later) spend one GitHub budget:
-- GitHub counts points per credential. Until now the crawl could spend all of
-- it, down to ingest.rate_limit_floor, and the tracked-issue check queued
-- behind it on the same floor.
--
--   ingest.interactive_reserve_percent   share of the rate limit the crawler
--                                        lane never spends. Its floors are
--                                        now measured above the reserve, so
--                                        with a 12,500-point limit and 30%
--                                        the crawl stops at 3,750 + 100 and
--                                        discovery at 3,750 + 1,000; the
--                                        interactive lane stops at 100.
--
-- The two floors' descriptions change to say so.

-- +goose Up

INSERT INTO settings (key, value, value_type, min_value, max_value, description) VALUES
  ('ingest.interactive_reserve_percent', '30', 'int', 0, 90,
   'Share of the GitHub rate limit the crawler lane never spends, kept for interactive work such as tracked-issue checks. 0 lets the crawler spend everything down to its floor.')
ON CONFLICT (key) DO NOTHING;

UPDATE settings SET description = 'Stop issuing GraphQL calls when the remaining point budget falls below this. On the crawler lane it is measured above the interactive reserve.'
 WHERE key = 'ingest.rate_limit_floor';
UPDATE settings SET description = 'Discovery stops searching when the shared budget falls below this plus the interactive reserve; kept above ingest.rate_limit_floor so the issue crawl keeps going after discovery yields.'
 WHERE key = 'ingest.discovery.rate_limit_floor';

-- +goose Down

DELETE FROM settings WHERE key = 'ingest.interactive_reserve_percent';

UPDATE settings SET description = 'Stop issuing GraphQL calls when the remaining point budget falls below this.'
 WHERE key = 'ingest.rate_limit_floor';
UPDATE settings SET description = 'Discovery stops searching when the shared crawler budget falls below this; kept above ingest.rate_limit_floor so the issue crawl keeps going after discovery yields.'
 WHERE key = 'ingest.discovery.rate_limit_floor';
