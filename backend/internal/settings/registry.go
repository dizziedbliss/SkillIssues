package settings

import "time"

// Key identifies a live-reloadable operational setting. Values live in the
// `settings` table and can be changed on a running system.
//
// Only operational values belong here. Product constants — XP tiers, the level
// curve, badge thresholds, recommendation weights, difficulty label maps — stay
// in code so behaviour is always traceable to a commit and cannot be restated
// retroactively by an admin write. Secrets stay in the environment.
//
// "Live" means one of three things, and it is worth knowing which:
//
//   - Effective on next read: anything consulted per request or per operation —
//     rate limits, page sizes, cache TTLs, the search cap, retry counts.
//   - Effective next cycle: the sync interval, which is re-read each time the
//     scheduler loop comes around.
//   - Pushed on change: worker concurrency, which is handed to the pools via
//     Live.OnChange because nothing re-reads it on its own.
type Key string

const (
	RateLimitUnauthPerHour Key = "api.rate_limit.unauth_per_hour"
	RateLimitAuthPerHour   Key = "api.rate_limit.auth_per_hour"
	RateLimitWindowSeconds Key = "api.rate_limit.window_seconds"

	IssuesDefaultPageSize Key = "api.issues.default_page_size"
	IssuesSearchPageSize  Key = "api.issues.search_page_size"
	SearchPerCategoryCap  Key = "api.search.per_category_cap"

	SessionTTLHours Key = "api.session.ttl_hours"

	CacheHotIssuesTTLSeconds      Key = "api.cache.hot_issues_ttl_seconds"
	CacheSearchTTLSeconds         Key = "api.cache.search_ttl_seconds"
	CacheRecommendationsTTLSecond Key = "api.cache.recommendations_ttl_seconds"

	ContributionVerifyPerHour Key = "api.contribution.verify_per_hour"

	IngestSyncIntervalSeconds Key = "ingest.sync_interval_seconds"
	IngestGraphQLPageSize     Key = "ingest.graphql_page_size"
	IngestRateLimitFloor      Key = "ingest.rate_limit_floor"
	IngestMaxAttempts         Key = "ingest.max_attempts"
	IngestBackoffBaseSeconds  Key = "ingest.backoff_base_seconds"
	// IngestRoutineWorkers backs the Initial/Freshness pool and
	// IngestPriorityWorkers the Deep Diver pool (Phase 5) — same keys as
	// before that split existed, repurposed meaning, not a priority queue.
	IngestRoutineWorkers  Key = "ingest.routine_workers"
	IngestPriorityWorkers Key = "ingest.priority_workers"
	// IngestOnDemandWorkers is reserved for the on-demand issue-body read
	// path (Phase 6); not read by anything yet.
	IngestOnDemandWorkers Key = "ingest.ondemand_workers"

	IngestEligibilityMinStars         Key = "ingest.eligibility.min_stars"
	IngestEligibilityPushedWithinDays Key = "ingest.eligibility.pushed_within_days"
)

// Kind is the value type a key accepts.
type Kind string

const (
	KindInt    Kind = "int"
	KindBool   Kind = "bool"
	KindString Kind = "string"
)

// Spec declares a key's type, fallback and bounds. The bounds mirror the CHECK
// and trigger in migration 00002; the database is the authoritative boundary,
// and this copy exists so the API can reject early and describe itself.
type Spec struct {
	Kind        Kind
	Default     any
	Min, Max    float64
	Bounded     bool
	Description string
}

func intSpec(def, min, max int, desc string) Spec {
	return Spec{
		Kind: KindInt, Default: def,
		Min: float64(min), Max: float64(max), Bounded: true,
		Description: desc,
	}
}

// Registry is the complete set of live-reloadable keys. Adding one here without
// a matching migration row means it falls back to Default and is reported as
// drift at startup.
var Registry = map[Key]Spec{
	RateLimitUnauthPerHour: intSpec(180, 1, 100000, "Requests per window allowed for unauthenticated callers."),
	RateLimitAuthPerHour:   intSpec(1000, 1, 1000000, "Requests per window allowed for authenticated callers."),
	RateLimitWindowSeconds: intSpec(3600, 60, 86400, "Length of the rate-limit window."),

	IssuesDefaultPageSize: intSpec(20, 1, 100, "Issues returned when the catalog is browsed without a search term."),
	IssuesSearchPageSize:  intSpec(50, 1, 200, "Issues returned for a search query."),
	SearchPerCategoryCap:  intSpec(25, 1, 100, "Maximum results per match category, so one category cannot flood the rest."),

	SessionTTLHours: intSpec(168, 1, 8760, "Session lifetime. Applies to newly issued sessions only."),

	CacheHotIssuesTTLSeconds:      intSpec(120, 0, 3600, "Cache lifetime for the default unauthenticated catalog page. 0 disables."),
	CacheSearchTTLSeconds:         intSpec(60, 0, 3600, "Cache lifetime for unauthenticated search results. 0 disables."),
	CacheRecommendationsTTLSecond: intSpec(300, 0, 3600, "Cache lifetime for per-user recommendations. 0 disables."),

	ContributionVerifyPerHour: intSpec(5, 1, 1000, "Maximum PR verification attempts per user per contribution per hour."),

	IngestSyncIntervalSeconds: intSpec(900, 30, 86400, "Delay between routine sync sweeps."),
	IngestGraphQLPageSize:     intSpec(50, 1, 100, "Issues requested per GraphQL page. GitHub caps this at 100."),
	IngestRateLimitFloor:      intSpec(100, 0, 5000, "Stop issuing GraphQL calls when the remaining point budget falls below this."),
	IngestMaxAttempts:         intSpec(3, 1, 10, "Attempts per repository before a sync is recorded as failed."),
	IngestBackoffBaseSeconds:  intSpec(1, 1, 60, "Base for exponential retry backoff between sync attempts."),
	// D20: ~4 total in-flight by default, sized for the 2 vCPU target
	// deployment rather than a 20-vCPU dev box.
	IngestRoutineWorkers:  intSpec(2, 1, 64, "Concurrent Initial/Freshness workers (one page per eligible repository)."),
	IngestPriorityWorkers: intSpec(1, 1, 64, "Concurrent Deep Diver workers (subsequent pagination)."),
	IngestOnDemandWorkers: intSpec(1, 1, 64, "Concurrent on-demand workers. Reserved for the on-demand issue-body read path; not wired up yet."),

	IngestEligibilityMinStars:         intSpec(25, 0, 100000, "Minimum stars for a repository to be eligible for ingestion. Abuse control, not ranking."),
	IngestEligibilityPushedWithinDays: intSpec(730, 1, 3650, "A repository must have been pushed to within this many days to be eligible for ingestion."),
}

// Snapshot is an immutable view of the settings. Readers hold one for the
// duration of an operation so a mid-flight reload cannot change values
// underneath them.
type Snapshot struct {
	values map[Key]any
}

func newSnapshot(values map[Key]any) *Snapshot {
	if values == nil {
		values = map[Key]any{}
	}
	return &Snapshot{values: values}
}

// Defaults returns a snapshot of registry defaults, for callers that have no
// database handle — tests, and services constructed without a live provider.
func Defaults() *Snapshot { return defaultSnapshot() }

// defaultSnapshot is what a process runs on before the first load succeeds, and
// what it keeps if a reload produces an unusable result.
func defaultSnapshot() *Snapshot {
	vals := make(map[Key]any, len(Registry))
	for k, spec := range Registry {
		vals[k] = spec.Default
	}
	return newSnapshot(vals)
}

// Int returns the value for k, falling back to the registry default if the key
// is absent or holds the wrong type.
func (s *Snapshot) Int(k Key) int {
	if v, ok := s.values[k]; ok {
		if n, ok := v.(int); ok {
			return n
		}
	}
	if spec, ok := Registry[k]; ok {
		if n, ok := spec.Default.(int); ok {
			return n
		}
	}
	return 0
}

func (s *Snapshot) Bool(k Key) bool {
	if v, ok := s.values[k]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	if spec, ok := Registry[k]; ok {
		if b, ok := spec.Default.(bool); ok {
			return b
		}
	}
	return false
}

func (s *Snapshot) String(k Key) string {
	if v, ok := s.values[k]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	if spec, ok := Registry[k]; ok {
		if str, ok := spec.Default.(string); ok {
			return str
		}
	}
	return ""
}

// Seconds reads an integer key as a duration in seconds.
func (s *Snapshot) Seconds(k Key) time.Duration {
	return time.Duration(s.Int(k)) * time.Second
}

// Hours reads an integer key as a duration in hours.
func (s *Snapshot) Hours(k Key) time.Duration {
	return time.Duration(s.Int(k)) * time.Hour
}

// Raw exposes the resolved values for the admin endpoint.
func (s *Snapshot) Raw() map[Key]any {
	out := make(map[Key]any, len(s.values))
	for k, v := range s.values {
		out[k] = v
	}
	return out
}
