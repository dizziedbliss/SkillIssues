package ingest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// Facets group labels case-insensitively under their commonest spelling and
// count open issues only; a refreshed job is not due again until its interval
// has passed.
func TestRefreshFacetsAndSchedule(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	label := fmt.Sprintf("facet-test-%d", time.Now().UnixNano())

	var repoID int
	if err := pool.QueryRow(ctx, `INSERT INTO repositories (full_name, url, language) VALUES ($1, 'u', 'Go') RETURNING id`,
		[]any{"facet-test/" + label}, func(row pgx.Row) error { return row.Scan(&repoID) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Exec(context.Background(), `DELETE FROM repositories WHERE id = $1`, repoID); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	// Two lower-case, one capitalised, one closed: the facet is the
	// lower-case spelling with 3 open issues.
	upper := strings.ToUpper(label[:1]) + label[1:]
	for n, row := range []struct{ label, state string }{{label, "OPEN"}, {label, "OPEN"}, {upper, "OPEN"}, {label, "CLOSED"}} {
		if err := pool.Exec(ctx,
			`INSERT INTO issues (repository_id, number, title, url, difficulty, state, labels) VALUES ($1, $2, 't', 'u', 'UNCLASSIFIED', $3, ARRAY[$4])`,
			repoID, n+1, row.state, row.label); err != nil {
			t.Fatal(err)
		}
	}

	if err := RefreshFacets(ctx, pool, 100000); err != nil {
		t.Fatal(err)
	}
	var value string
	var issues int
	if err := pool.QueryRow(ctx, `SELECT value, issues FROM catalog_facets WHERE kind = 'label' AND lower(value) = $1`,
		[]any{label}, func(row pgx.Row) error { return row.Scan(&value, &issues) }); err != nil {
		t.Fatalf("label facet: %v", err)
	}
	if value != label || issues != 3 {
		t.Errorf("label facet = %q with %d issues, want %q with 3", value, issues, label)
	}

	if due, err := CatalogJobDue(ctx, pool, catalogFacets, time.Hour); err != nil || due {
		t.Errorf("facets due right after a refresh: %v, %v", due, err)
	}
	if due, err := CatalogJobDue(ctx, pool, catalogFacets, 0); err != nil || !due {
		t.Errorf("facets not due with a zero interval: %v, %v", due, err)
	}
}
