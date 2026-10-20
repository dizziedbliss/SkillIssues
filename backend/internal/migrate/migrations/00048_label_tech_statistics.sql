-- Statistics for filtering the catalog by label and tech together
-- (issue/filters.go), which timed out on production for the easiest and
-- newest sorts (labels=bug&tech=go: 503 after 10s).
--
-- issues_lower_labels_stats: the label filter is written on
-- catalog_lower_labels(labels), an expression the planner kept no statistics
-- for, so it assumed its default 0.5% selectivity. On a 2M-issue copy that
-- estimated 9,943 open issues labelled "bug" where there are 227,981. With so
-- few expected, it probed the (repository, labels) index once per Go
-- repository (19,661 probes, 7.2s) instead of walking the catalog index in
-- order (41ms once the estimate is right). Expression statistics keep the
-- expression's most common elements, as for a plain array column.
--
-- repositories.tech: "go" was estimated at 1,437 repositories for 19,674. A
-- larger sample keeps more of the generated tech tags in the most common
-- elements.
--
-- +goose Up

CREATE STATISTICS IF NOT EXISTS issues_lower_labels_stats ON (catalog_lower_labels(labels)) FROM issues;
ALTER TABLE repositories ALTER COLUMN tech SET STATISTICS 1000;
ANALYZE issues;
ANALYZE repositories;

-- +goose Down

ALTER TABLE repositories ALTER COLUMN tech SET STATISTICS -1;
DROP STATISTICS IF EXISTS issues_lower_labels_stats;
