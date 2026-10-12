// cmd/repoimport is a one-shot offline importer for repository metadata
// (plan Phase 4, D18): the Kaggle "github-repository-metadata-with-5-stars"
// dataset, or any file matching its JSON shape. It streams the (multi-GB)
// JSON array rather than loading it into memory, filters by the same
// eligibility rule the live crawler uses (D1), upserts eligible repositories
// idempotently, and records everything else in a skip ledger so the
// decision stays auditable and a threshold change is one local re-pass.
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
	"github.com/dizziedbliss/skillissues/internal/settings"
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
	flag.Parse()

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

	imp := &importer{pool: pool, minStars: minStars, pushedWithinDays: pushedWithinDays}
	if err := imp.run(ctx, reader, *logEvery); err != nil {
		slog.Error("import failed", "error", err)
		os.Exit(1)
	}

	slog.Info("import complete",
		"processed", imp.processed,
		"imported", imp.imported,
		"skipped", imp.skipped,
		"errors", imp.errored,
	)
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

	processed, imported, skipped, errored int
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
				"imported", imp.imported,
				"skipped", imp.skipped,
				"errors", imp.errored,
			)
		}
	}

	return nil
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
		return fmt.Errorf("record has no nameWithOwner")
	}

	ok, reason := eligible(r, imp.minStars, imp.pushedWithinDays)
	if !ok {
		imp.skipped++
		return imp.pool.Exec(ctx,
			`INSERT INTO import_skipped (full_name, reason) VALUES ($1, $2)
			 ON CONFLICT (full_name) DO UPDATE SET reason = EXCLUDED.reason, skipped_at = now()`,
			r.NameWithOwner, reason,
		)
	}

	topics := make([]string, 0, len(r.Topics))
	for _, t := range r.Topics {
		topics = append(topics, t.Name)
	}
	languagesJSON, err := json.Marshal(r.Languages)
	if err != nil {
		return fmt.Errorf("marshal languages: %w", err)
	}

	var parentFullName *string
	if r.Parent != nil && r.Parent.NameWithOwner != "" {
		parentFullName = &r.Parent.NameWithOwner
	}

	url := "https://github.com/" + r.NameWithOwner

	imp.imported++
	if err := imp.pool.Exec(ctx,
		`INSERT INTO repositories (
			full_name, url, description, language, stars, license, forkable,
			last_activity_at, forks, watchers, is_archived, created_at, pushed_at,
			topics, languages, forking_allowed, parent_full_name, is_eligible, eligibility_reason
		 ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,true,'')
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
			forking_allowed = EXCLUDED.forking_allowed
		 WHERE repositories.stars IS DISTINCT FROM EXCLUDED.stars
			OR repositories.forks IS DISTINCT FROM EXCLUDED.forks
			OR repositories.watchers IS DISTINCT FROM EXCLUDED.watchers
			OR repositories.is_archived IS DISTINCT FROM EXCLUDED.is_archived
			OR repositories.pushed_at IS DISTINCT FROM EXCLUDED.pushed_at
			OR repositories.description IS DISTINCT FROM EXCLUDED.description`,
		r.NameWithOwner, url, r.Description, r.PrimaryLanguage, r.Stars, r.License, r.ForkingAllowed,
		r.PushedAt, r.Forks, r.Watchers, r.IsArchived, r.CreatedAt, r.PushedAt,
		topics, languagesJSON, r.ForkingAllowed, parentFullName,
	); err != nil {
		return fmt.Errorf("upsert repository: %w", err)
	}

	// A repo that was skipped in an earlier pass but is eligible now (e.g. it
	// crossed the star floor) should stop showing up in the skip ledger.
	return imp.pool.Exec(ctx, `DELETE FROM import_skipped WHERE full_name = $1`, r.NameWithOwner)
}
