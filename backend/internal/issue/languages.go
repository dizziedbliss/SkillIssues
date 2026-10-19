package issue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/jackc/pgx/v5"
)

// languageCounts caches each language's number of open issues, counted only up
// to the rare-language threshold: BrowseIssues needs to know whether a
// language is rare, not how large a common one is, and stopping there keeps
// the count cheap however common it is. Counting still costs 50–100ms against
// a few milliseconds for the page itself, and a language's size barely moves
// from one minute to the next, so a count is reused for its TTL.
//
// Only languages with issues are cached. A language with none costs one index
// probe to count again, and a caller who invents languages cannot grow the
// cache: it is bounded by the languages repositories actually have.
type languageCounts struct {
	mu      sync.Mutex
	entries map[string]languageCount
}

type languageCount struct {
	n         int // open issues, capped at below
	below     int // the threshold n was counted against
	countedAt time.Time
}

// counts returns the open issues of each language (lower-cased), each capped
// at below. A ttl of 0 counts every time.
func (lc *languageCounts) counts(ctx context.Context, pool *db.Pool, langs []string, below int, ttl time.Duration) (map[string]int, error) {
	out := make(map[string]int, len(langs))
	var missing []string
	now := time.Now()

	lc.mu.Lock()
	for _, l := range langs {
		e, ok := lc.entries[l]
		if ok && e.below == below && now.Sub(e.countedAt) < ttl {
			out[l] = e.n
		} else {
			missing = append(missing, l)
		}
	}
	lc.mu.Unlock()
	if len(missing) == 0 {
		return out, nil
	}

	// One capped count per language. There is no ORDER BY, so nothing makes
	// the catalog index attractive: each reads repository-first through
	// repositories_lower_language_idx and stops at the cap.
	counted := make(map[string]int, len(missing))
	err := pool.QueryRows(ctx,
		`SELECT l, (SELECT count(*) FROM (
		              SELECT 1 FROM issues i JOIN repositories r ON r.id = i.repository_id
		              WHERE i.state = 'OPEN' AND lower(r.language) = l
		              LIMIT $2) t)
		 FROM unnest($1::text[]) AS l`,
		[]any{pgx.QueryExecModeExec, missing, below},
		func(rows pgx.Rows) error {
			var l string
			var n int
			if err := rows.Scan(&l, &n); err != nil {
				return err
			}
			counted[l] = n
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("issue: count language issues: %w", err)
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.entries == nil {
		lc.entries = make(map[string]languageCount)
	}
	for l, n := range counted {
		out[l] = n
		if n > 0 && ttl > 0 {
			lc.entries[l] = languageCount{n: n, below: below, countedAt: now}
		} else {
			delete(lc.entries, l)
		}
	}
	return out, nil
}
