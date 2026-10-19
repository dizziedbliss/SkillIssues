package issue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// Facet is one value a filter dropdown offers, and how many open issues have
// it.
type Facet struct {
	Value  string `json:"value"`
	Issues int    `json:"issues"`
}

// Facets are the filter dropdowns' values, the most common first. ingestd
// computes them (catalog_facets) every ingest.catalog.facets_refresh_seconds:
// counting over every open issue is too heavy for a request. RefreshedAt is
// nil until it first has.
type Facets struct {
	Labels      []Facet    `json:"labels"`
	Topics      []Facet    `json:"topics"`
	Tech        []Facet    `json:"tech"`
	Languages   []Facet    `json:"languages"`
	RefreshedAt *time.Time `json:"refreshed_at"`
}

// Facets reads the filter dropdowns' values.
func (s *Service) Facets(ctx context.Context) (Facets, error) {
	out := Facets{Labels: []Facet{}, Topics: []Facet{}, Tech: []Facet{}, Languages: []Facet{}}
	err := s.db.QueryRows(ctx,
		`SELECT kind, value, issues FROM catalog_facets ORDER BY kind, issues DESC, value`, nil,
		func(rows pgx.Rows) error {
			var kind string
			var f Facet
			if err := rows.Scan(&kind, &f.Value, &f.Issues); err != nil {
				return err
			}
			switch kind {
			case "label":
				out.Labels = append(out.Labels, f)
			case "topic":
				out.Topics = append(out.Topics, f)
			case "tech":
				out.Tech = append(out.Tech, f)
			case "language":
				out.Languages = append(out.Languages, f)
			}
			return nil
		})
	if err != nil {
		return Facets{}, fmt.Errorf("issue: read facets: %w", err)
	}
	var at time.Time
	err = s.db.QueryRow(ctx, `SELECT refreshed_at FROM catalog_refresh WHERE name = 'facets'`, nil,
		func(row pgx.Row) error { return row.Scan(&at) })
	switch {
	case err == nil:
		out.RefreshedAt = &at
	case !errors.Is(err, db.ErrNotFound):
		return Facets{}, fmt.Errorf("issue: read facets refresh time: %w", err)
	}
	return out, nil
}
