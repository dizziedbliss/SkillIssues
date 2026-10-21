-- How many people an issue is assigned to on GitHub, so the catalog can leave
-- out issues someone is already working on (?unassigned=true, the web app's
-- default). Written by the crawl (GraphQL assignees.totalCount) and the GH
-- Archive reader (the issue's assignees in an event). NULL means it hasn't
-- been read since this column arrived, and counts as unassigned: unknown is
-- not a negative. It fills in as repositories are crawled again.

-- +goose Up

ALTER TABLE issues ADD COLUMN IF NOT EXISTS assignees SMALLINT;

-- +goose Down

ALTER TABLE issues DROP COLUMN IF EXISTS assignees;
