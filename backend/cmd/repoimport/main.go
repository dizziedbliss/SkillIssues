// cmd/repoimport is the offline importer for repository metadata (plan
// Phase 4, D18): the Kaggle "github-repository-metadata-with-5-stars"
// dataset, or any file matching its JSON shape. It streams the (multi-GB)
// JSON array rather than loading it into memory.
//
// It stores only records that pass the eligibility filter. Of the ~4.2M
// records in the dataset, ~288k pass at the current thresholds; storing the
// rest cost ~2.8GB of rows nothing is ever allowed to crawl. A full pass
// over the source file takes ~16 minutes and is idempotent, so loosening a
// threshold is a re-run — not a reason to warehouse the whole dataset.
//
// That means the importer is the only place eligibility can *loosen*.
// ingest.RecomputeEligibility still re-tests the rows that are stored, so
// tightening a threshold (or metadata going stale) takes effect live.
//
// Ineligible records are counted by reason and reported in aggregate at the
// end of the run; there is deliberately no row per rejected record, which
// would be ~3.9M ledger rows to record "below_star_floor". Only records
// that cannot be stored at all (no nameWithOwner) land in import_skipped.
//
// Does not touch issues, ingest_jobs, or anything else — this only ever
// populates repositories and import_skipped.
package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/config"
	"github.com/dizziedbliss/skillissues/internal/db"
	"github.com/dizziedbliss/skillissues/internal/ingest"
	"github.com/dizziedbliss/skillissues/internal/settings"
	"github.com/jackc/pgx/v5"
)

type languageEntry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type topicEntry struct {
	Name string `json:"name"`
}

// repoRecord mirrors one element of the Kaggle dataset's JSON array.
type repoRecord struct {
	NameWithOwner   string          `json:"nameWithOwner"`
	Stars           int             `json:"stars"`
	Forks           int             `json:"forks"`
	Watchers        int             `json:"watchers"`
	IsFork          bool            `json:"isFork"`
	IsArchived      bool            `json:"isArchived"`
	Languages       []languageEntry `json:"languages"`
	Topics          []topicEntry    `json:"topics"`
	Issues          int             `json:"issues"`
	Description     *string         `json:"description"`
	PrimaryLanguage *string         `json:"primaryLanguage"`
	CreatedAt       *time.Time      `json:"createdAt"`
	PushedAt        *time.Time      `json:"pushedAt"`
	License         *string         `json:"license"`
	ForkingAllowed  bool            `json:"forkingAllowed"`
}

func main() {
	filePath := flag.String("file", envOr("IMPORT_FILE", "data/repo_metadata.json"), "path to the metadata JSON file (or a .zip containing one)")
	logEvery := flag.Int("log-every", 50000, "log progress every N records processed")
	batchSize := flag.Int("batch-size", 500, "statements per database round trip")
	flag.Parse()
	if *batchSize < 1 {
		*batchSize = 1
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	pool, err := db.New(ctx, db.DefaultConfig(cfg.DatabaseURL))
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	live := settings.NewLive(ctx, pool.Raw(), cfg.DatabaseURL)
	minStars := live.Current().Int(settings.IngestEligibilityMinStars)
	pushedWithinDays := live.Current().Int(settings.IngestEligibilityPushedWithinDays)
	slog.Info("eligibility thresholds", "min_stars", minStars, "pushed_within_days", pushedWithinDays)

	reader, closeFn, err := openRecordsFile(*filePath)
	if err != nil {
		slog.Error("failed to open input file", "error", err, "path", *filePath)
		os.Exit(1)
	}
	defer closeFn()

	excluded, err := loadExcluded(ctx, pool)
	if err != nil {
		slog.Error("failed to load excluded repositories", "error", err)
		os.Exit(1)
	}
	slog.Info("excluded repositories", "count", len(excluded))

	imp := &importer{
		pool:             pool,
		minStars:         minStars,
		pushedWithinDays: pushedWithinDays,
		excluded:         excluded,
		batchSize:        *batchSize,
		reasons:          map[string]int{},
	}
	if err := imp.run(ctx, reader, *logEvery); err != nil {
		slog.Error("import failed", "error", err)
		os.Exit(1)
	}

	slog.Info("import complete",
		"processed", imp.processed,
		"stored", imp.stored,
		"eligible", imp.eligible,
		"ineligible", imp.ineligible,
		"unusable", imp.unusable,
		"errors", imp.errored,
	)
	slog.Info("ineligible breakdown", imp.topReasons(10)...)
	if imp.errored > 0 {
		// A partially-applied import is not a success: the caller needs a
		// non-zero exit to notice, and the logged counts say how much
		// landed.
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// openRecordsFile opens path directly, or — if it's a .zip — opens the
// first entry inside it. The caller must call the returned close function.
func openRecordsFile(path string) (io.Reader, func(), error) {
	if !strings.HasSuffix(strings.ToLower(path), ".zip") {
		f, err := os.Open(path)
		if err != nil {
			return nil, nil, err
		}
		return f, func() { f.Close() }, nil
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, err
	}
	if len(zr.File) == 0 {
		zr.Close()
		return nil, nil, fmt.Errorf("repoimport: zip archive %s has no entries", path)
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		zr.Close()
		return nil, nil, err
	}
	return rc, func() { rc.Close(); zr.Close() }, nil
}

type importer struct {
	pool             *db.Pool
	minStars         int
	pushedWithinDays int

	// excluded holds the lower-cased full names in excluded_repositories. A
	// listed repository is never stored, whatever eligible() says. It is
	// checked here rather than in eligible(), which stays a pure function of
	// the record; ingest.RecomputeEligibility applies the same list to rows
	// that are already stored.
	excluded map[string]struct{}

	// Writes are batched because this now stores every record in the
	// dataset, not the eligible minority: one round trip per record turns a
	// multi-million-row pass into hours of pure network latency.
	batchSize int
	batch     *pgx.Batch

	// reasons counts ineligible records by their reason string, so a run
	// still explains itself without a row per rejected record.
	reasons map[string]int

	processed, stored, eligible, ineligible, unusable, errored int
}

// topReasons returns the n most common ineligibility reasons as an
// slog-friendly key/value list.
func (imp *importer) topReasons(n int) []any {
	keys := make([]string, 0, len(imp.reasons))
	for k := range imp.reasons {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return imp.reasons[keys[i]] > imp.reasons[keys[j]] })
	if len(keys) > n {
		keys = keys[:n]
	}
	out := make([]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, k, imp.reasons[k])
	}
	return out
}

// queue adds one statement to the pending batch, flushing when it is full.
func (imp *importer) queue(ctx context.Context, sql string, args ...any) error {
	if imp.batch == nil {
		imp.batch = &pgx.Batch{}
	}
	imp.batch.Queue(sql, args...)
	if imp.batch.Len() >= imp.batchSize {
		return imp.flush(ctx)
	}
	return nil
}

// flush sends the pending batch. If the batch fails, it is replayed one
// statement at a time so the failure is attributed to the record that
// caused it instead of losing the whole batch to a single bad row.
func (imp *importer) flush(ctx context.Context) error {
	if imp.batch == nil || imp.batch.Len() == 0 {
		return nil
	}
	batch := imp.batch
	imp.batch = nil

	results := imp.pool.Raw().SendBatch(ctx, batch)
	err := results.Close()
	if err == nil {
		imp.stored += batch.Len()
		return nil
	}

	slog.Warn("batch failed, replaying individually to isolate the bad record",
		"batch_size", batch.Len(), "error", err)
	for _, q := range batch.QueuedQueries {
		if execErr := imp.pool.Exec(ctx, q.SQL, q.Arguments...); execErr != nil {
			imp.errored++
			slog.Warn("record failed", "error", execErr, "args", q.Arguments[:1])
			continue
		}
		imp.stored++
	}
	return nil
}

// run streams the top-level JSON array, record by record, so a multi-GB
// file never needs to fit in memory at once.
func (imp *importer) run(ctx context.Context, r io.Reader, logEvery int) error {
	dec := json.NewDecoder(r)

	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("repoimport: read opening token: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return fmt.Errorf("repoimport: expected a top-level JSON array, got %v", tok)
	}

	for dec.More() {
		var rec repoRecord
		if err := dec.Decode(&rec); err != nil {
			return fmt.Errorf("repoimport: decode record #%d: %w", imp.processed, err)
		}
		imp.processed++

		if err := imp.handle(ctx, rec); err != nil {
			imp.errored++
			slog.Warn("record failed", "repo", rec.NameWithOwner, "error", err)
		}

		if logEvery > 0 && imp.processed%logEvery == 0 {
			slog.Info("progress",
				"processed", imp.processed,
				"stored", imp.stored,
				"eligible", imp.eligible,
				"ineligible", imp.ineligible,
				"unusable", imp.unusable,
				"errors", imp.errored,
			)
		}
	}

	return imp.flush(ctx)
}

// loadExcluded reads excluded_repositories as a set of lower-cased names.
func loadExcluded(ctx context.Context, pool *db.Pool) (map[string]struct{}, error) {
	set := map[string]struct{}{}
	err := pool.QueryRows(ctx, `SELECT lower(full_name) FROM excluded_repositories`, nil,
		func(rows pgx.Rows) error {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			set[name] = struct{}{}
			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("load excluded repositories: %w", err)
	}
	return set, nil
}

// eligible applies ingest.Eligible, the rule the Discovery Sweeper and
// ingest.RecomputeEligibility share.
func eligible(r repoRecord, minStars, pushedWithinDays int) (bool, string) {
	return ingest.Eligible(ingest.RepoFacts{
		IsFork:          r.IsFork,
		IsArchived:      r.IsArchived,
		IssueCount:      r.Issues,
		PrimaryLanguage: r.PrimaryLanguage,
		PushedAt:        r.PushedAt,
		Stars:           r.Stars,
	}, minStars, pushedWithinDays)
}

func (imp *importer) handle(ctx context.Context, r repoRecord) error {
	if r.NameWithOwner == "" {
		// Nothing identifies this record, so there is no row to store and
		// nothing to re-decide later. The ledger is now only for this.
		imp.unusable++
		return imp.queue(ctx,
			`INSERT INTO import_skipped (full_name, reason) VALUES ($1, $2)
			 ON CONFLICT (full_name) DO UPDATE SET reason = EXCLUDED.reason, skipped_at = now()`,
			fmt.Sprintf("<record #%d>", imp.processed), "no_name_with_owner",
		)
	}

	ok, reason := eligible(r, imp.minStars, imp.pushedWithinDays)
	if _, isExcluded := imp.excluded[strings.ToLower(r.NameWithOwner)]; isExcluded {
		ok, reason = false, "excluded"
	}
	if !ok {
		// Not stored. The dataset is ~4.2M records and only ~288k pass the
		// filter; keeping the rest would cost ~2.8GB for rows nothing is
		// ever allowed to crawl. A full re-pass over the source file is 16
		// minutes and idempotent, so loosening a threshold is a re-run, not
		// a reason to warehouse the whole dataset.
		//
		// Per-record skip rows are not written either, for the same reason —
		// 3.9M ledger rows to record "below_star_floor". The aggregate
		// breakdown is logged at the end of the run instead.
		imp.ineligible++
		imp.reasons[reason]++
		return nil
	}
	imp.eligible++

	// NUL bytes do occur in this dataset's descriptions and topic names, and
	// Postgres's text type rejects them outright — one NUL would otherwise
	// fail its whole batch. Same rule, same helper as the issue path.
	topics := make([]string, 0, len(r.Topics))
	for _, t := range r.Topics {
		topics = append(topics, t.Name)
	}
	topics = ingest.SanitizeSlice(topics)
	description := ingest.SanitizeOptional(r.Description)
	primaryLanguage := ingest.SanitizeOptional(r.PrimaryLanguage)
	license := ingest.SanitizeOptional(r.License)
	fullName := ingest.SanitizeText(r.NameWithOwner)
	for i := range r.Languages {
		// jsonb rejects \u0000 too, so this has to happen before the marshal.
		r.Languages[i].Name = ingest.SanitizeText(r.Languages[i].Name)
	}
	languagesJSON, err := json.Marshal(r.Languages)
	if err != nil {
		return fmt.Errorf("marshal languages: %w", err)
	}

	url := "https://github.com/" + fullName

	// is_eligible/eligibility_reason are recomputed on every pass, except
	// where the crawler has already learned from GitHub that the repository
	// does not exist, or that it is a duplicate of a row holding its GitHub
	// id. Those are facts about the repository, not threshold decisions, so
	// a metadata refresh must not resurrect them.
	return imp.queue(ctx,
		`INSERT INTO repositories (
			full_name, url, description, language, stars, license,
			forks, watchers, is_archived, created_at, pushed_at,
			topics, languages, forking_allowed,
			issue_count, is_eligible, eligibility_reason
		 ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		 ON CONFLICT (lower(full_name)) DO UPDATE SET
			stars = EXCLUDED.stars,
			forks = EXCLUDED.forks,
			watchers = EXCLUDED.watchers,
			is_archived = EXCLUDED.is_archived,
			pushed_at = EXCLUDED.pushed_at,
			description = EXCLUDED.description,
			language = EXCLUDED.language,
			license = EXCLUDED.license,
			topics = EXCLUDED.topics,
			languages = EXCLUDED.languages,
			forking_allowed = EXCLUDED.forking_allowed,
			issue_count = EXCLUDED.issue_count,
			is_eligible = CASE
			  WHEN repositories.eligibility_reason IN ('not_found', 'duplicate') THEN false
			  ELSE EXCLUDED.is_eligible END,
			eligibility_reason = CASE
			  WHEN repositories.eligibility_reason IN ('not_found', 'duplicate') THEN repositories.eligibility_reason
			  ELSE EXCLUDED.eligibility_reason END
		 WHERE repositories.stars IS DISTINCT FROM EXCLUDED.stars
			OR repositories.forks IS DISTINCT FROM EXCLUDED.forks
			OR repositories.watchers IS DISTINCT FROM EXCLUDED.watchers
			OR repositories.is_archived IS DISTINCT FROM EXCLUDED.is_archived
			OR repositories.pushed_at IS DISTINCT FROM EXCLUDED.pushed_at
			OR repositories.description IS DISTINCT FROM EXCLUDED.description
			OR repositories.issue_count IS DISTINCT FROM EXCLUDED.issue_count
			OR repositories.is_eligible IS DISTINCT FROM EXCLUDED.is_eligible
			OR repositories.eligibility_reason IS DISTINCT FROM EXCLUDED.eligibility_reason`,
		fullName, url, description, primaryLanguage, r.Stars, license,
		r.Forks, r.Watchers, r.IsArchived, r.CreatedAt, r.PushedAt,
		topics, languagesJSON, r.ForkingAllowed,
		r.Issues, ok, reason,
	)
}
