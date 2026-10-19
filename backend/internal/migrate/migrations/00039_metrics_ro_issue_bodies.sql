-- Let the dashboards see the issue body cache without seeing a body.
--
-- issue_bodies (00038) holds attacker-controlled Markdown. metrics_ro gets
-- only the bookkeeping columns, so Grafana can show how many bodies are
-- cached, how many the archive has refreshed and how recently they were read
-- — never what one says.

-- +goose Up

GRANT SELECT (issue_id, source, fetched_at, last_read_at) ON issue_bodies TO metrics_ro;

-- +goose Down

REVOKE SELECT (issue_id, source, fetched_at, last_read_at) ON issue_bodies FROM metrics_ro;
