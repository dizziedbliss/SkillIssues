// cmd/repoimport is the offline importer for repository metadata (plan
// Phase 4, D18): the Kaggle "github-repository-metadata-with-5-stars"
// dataset, or any file matching its JSON shape. It streams the (multi-GB)
// JSON array rather than loading it into memory.
//
// It stores *every* record it can read, eligible or not, with is_eligible
// and eligibility_reason computed from the current thresholds. That is
// deliberate: eligibility is a live setting, so it has to be re-decidable
// from stored metadata (ingest.RecomputeEligibility) rather than only at
// import time. Dropping ineligible records on the floor meant lowering the
// star floor required a fresh pass over 3 GB of JSON before the crawler
// could see the repositories it had just become willing to crawl.
//
// Only records that cannot be stored at all (no nameWithOwner) land in the
// import_skipped ledger now.
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

type parentRef struct {
	NameWithOwner string `json:"nameWithOwner"`
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
	Parent          *parentRef      `json:"parent"`
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

	imp := &importer{
		pool:             pool,
		minStars:         minStars,
		pushedWithinDays: pushedWithinDays,
		batchSize:        *batchSize,
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

	// Writes are batched because this now stores every record in the
	// dataset, not the eligible minority: one round trip per record turns a
	// multi-million-row pass into hours of pure network latency.
	batchSize int
	batch     *pgx.Batch

	processed, stored, eligible, ineligible, unusable, errored int
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

// eligible applies D1's rule. The star floor is abuse control (anyone can
// open a fake issue on a trivially-created repo to burn crawl budget), not a
// quality ranking — there is deliberately no scoring here.
func eligible(r repoRecord, minStars, pushedWithinDays int) (bool, string) {
	var reasons []string
	if r.IsFork {
		reasons = append(reasons, "is_fork")
	}
	if r.IsArchived {
		reasons = append(reasons, "archived")
	}
	if r.Issues <= 0 {
		reasons = append(reasons, "no_issues")
	}
	if r.PrimaryLanguage == nil || *r.PrimaryLanguage == "" {
		reasons = append(reasons, "no_primary_language")
	}
	if r.PushedAt == nil || time.Since(*r.PushedAt) > time.Duration(pushedWithinDays)*24*time.Hour {
		reasons = append(reasons, "stale")
	}
	if r.Stars < minStars {
		reasons = append(reasons, "below_star_floor")
	}
	if len(reasons) > 0 {
		return false, strings.Join(reasons, ";")
	}
	return true, ""
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
	if ok {
		imp.eligible++
	} else {
		imp.ineligible++
	}

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

	var parentFullName *string
	if r.Parent != nil && r.Parent.NameWithOwner != "" {
		parentFullName = ingest.SanitizeOptional(&r.Parent.NameWithOwner)
	}

	url := "https://github.com/" + fullName

	// is_eligible/eligibility_reason are recomputed on every pass, except
	// where the crawler has already learned from GitHub that the repository
	// does not exist. That is a fact about the repository, not a threshold
	// decision, so a metadata refresh must not resurrect it.
	return imp.queue(ctx,
		`INSERT INTO repositories (
			full_name, url, description, language, stars, license, forkable,
			last_activity_at, forks, watchers, is_archived, created_at, pushed_at,
			topics, languages, forking_allowed, parent_full_name,
			is_fork, issue_count, is_eligible, eligibility_reason
		 ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
		 ON CONFLICT (lower(full_name)) DO UPDATE SET
			stars = EXCLUDED.stars,
			forks = EXCLUDED.forks,
			watchers = EXCLUDED.watchers,
			is_archived = EXCLUDED.is_archived,
			last_activity_at = EXCLUDED.last_activity_at,
			pushed_at = EXCLUDED.pushed_at,
			description = EXCLUDED.description,
			language = EXCLUDED.language,
			license = EXCLUDED.license,
			topics = EXCLUDED.topics,
			languages = EXCLUDED.languages,
			forking_allowed = EXCLUDED.forking_allowed,
			is_fork = EXCLUDED.is_fork,
			issue_count = EXCLUDED.issue_count,
			is_eligible = CASE
			  WHEN repositories.eligibility_reason = 'not_found' THEN false
			  ELSE EXCLUDED.is_eligible END,
			eligibility_reason = CASE
			  WHEN repositories.eligibility_reason = 'not_found' THEN 'not_found'
			  ELSE EXCLUDED.eligibility_reason END
		 WHERE repositories.stars IS DISTINCT FROM EXCLUDED.stars
			OR repositories.forks IS DISTINCT FROM EXCLUDED.forks
			OR repositories.watchers IS DISTINCT FROM EXCLUDED.watchers
			OR repositories.is_archived IS DISTINCT FROM EXCLUDED.is_archived
			OR repositories.pushed_at IS DISTINCT FROM EXCLUDED.pushed_at
			OR repositories.description IS DISTINCT FROM EXCLUDED.description
			OR repositories.is_fork IS DISTINCT FROM EXCLUDED.is_fork
			OR repositories.issue_count IS DISTINCT FROM EXCLUDED.issue_count
			OR repositories.is_eligible IS DISTINCT FROM EXCLUDED.is_eligible
			OR repositories.eligibility_reason IS DISTINCT FROM EXCLUDED.eligibility_reason`,
		fullName, url, description, primaryLanguage, r.Stars, license, r.ForkingAllowed,
		r.PushedAt, r.Forks, r.Watchers, r.IsArchived, r.CreatedAt, r.PushedAt,
		topics, languagesJSON, r.ForkingAllowed, parentFullName,
		r.IsFork, r.Issues, ok, reason,
	)
}
