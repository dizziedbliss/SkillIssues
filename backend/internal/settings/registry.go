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
	// SearchSampleSize bounds a search's Title category (that many matches are
	// read off the index and ordered by stars, however many a common word has)
	// and how deep one category of a search pages.
	SearchSampleSize Key = "api.search.sample_size"
	// SearchPerRepoCap is the most issues one repository contributes to the
	// Repository or Language category of a search.
	SearchPerRepoCap Key = "api.search.per_repo_cap"

	SessionTTLHours Key = "api.session.ttl_hours"
	// SessionPruneIntervalSeconds is how often expired sessions and
	// abandoned OAuth handshakes are deleted.
	SessionPruneIntervalSeconds Key = "api.session.prune_interval_seconds"

	// NotificationsPageSize is how many notifications GET /notifications
	// returns, newest first.
	NotificationsPageSize Key = "api.notifications.page_size"

	CacheHotIssuesTTLSeconds      Key = "api.cache.hot_issues_ttl_seconds"
	CacheSearchTTLSeconds         Key = "api.cache.search_ttl_seconds"
	CacheRecommendationsTTLSecond Key = "api.cache.recommendations_ttl_seconds"

	// CachePublicMaxAgeSeconds and CachePublicStaleSeconds are the
	// Cache-Control a signed-out catalog or search response carries, so a
	// browser or an edge cache can answer a repeat visit without reaching the
	// API. A response to a signed-in caller is always private, no-store.
	CachePublicMaxAgeSeconds Key = "api.cache.public_max_age_seconds"
	// StatsMaxAgeSeconds is GET /stats's public cache lifetime: short, as the
	// landing page polls it for its live counts.
	StatsMaxAgeSeconds      Key = "api.stats.max_age_seconds"
	CachePublicStaleSeconds Key = "api.cache.public_stale_seconds"

	// CacheRedisTimeoutMs and CacheRedisRetrySeconds keep a Redis outage from
	// slowing requests down: each call gets a deadline, and after a failure
	// Redis is skipped (rate limiting and caching fail open) and probed once
	// per retry interval until it answers.
	CacheRedisTimeoutMs    Key = "api.cache.redis_timeout_ms"
	CacheRedisRetrySeconds Key = "api.cache.redis_retry_seconds"

	// HTTPMaxBodyBytes caps a request body on every API route except the
	// webhook; WebhookMaxBodyBytes caps the GitHub webhook, whose payloads are
	// larger than any client request.
	HTTPMaxBodyBytes    Key = "api.http.max_body_bytes"
	WebhookMaxBodyBytes Key = "api.webhook.max_body_bytes"

	// HTTPOutboundTimeoutSeconds is the deadline on every call the server makes
	// to GitHub (sign-in, installation token, the crawl's GraphQL) and to
	// ingestd. Nothing outbound may run without one.
	HTTPOutboundTimeoutSeconds Key = "http.outbound_timeout_seconds"

	// IssuesMaxLimit and IssuesMaxPage bound what a caller may ask of
	// GET /issues; RecommendationsMaxLimit bounds GET /recommendations. A
	// request over a cap is clamped, not refused. RecommendationsDefaultLimit
	// is the size without ?limit, the only one cached.
	IssuesMaxLimit              Key = "api.issues.max_limit"
	IssuesMaxPage               Key = "api.issues.max_page"
	RecommendationsMaxLimit     Key = "api.recommendations.max_limit"
	RecommendationsDefaultLimit Key = "api.recommendations.default_limit"
	// RecommendationsCandidatePool is how many open issues GET
	// /recommendations reads off the shuffled feed before scoring them.
	RecommendationsCandidatePool Key = "api.recommendations.candidate_pool"

	// ThemesMaxPerUser and ThemesMaxBytes bound what an account stores as
	// themes (internal/theme).
	ThemesMaxPerUser Key = "api.themes.max_per_user"
	ThemesMaxBytes   Key = "api.themes.max_bytes"

	// IssuesQueryTimeoutMs is the deadline on the queries behind one
	// GET /issues or GET /issues/{id}. It sits below the server's write
	// timeout, so a slow query is cancelled and answered with a 503 instead of
	// running on after the connection has been dropped.
	IssuesQueryTimeoutMs Key = "api.issues.query_timeout_ms"

	// IssuesRareLanguageThreshold is where browsing by a language (a filter,
	// or a signed-in caller's preferred languages) switches from walking the
	// catalog index to sorting that language's issues: below it, the issues
	// are few enough to sort and too sparse in the index to walk to.
	IssuesRareLanguageThreshold Key = "api.issues.rare_language_threshold"
	// IssuesLanguageCountTTLSeconds is how long that count is reused: it
	// costs more than the page it decides, and a language's size moves slowly.
	IssuesLanguageCountTTLSeconds Key = "api.issues.language_count_ttl_seconds"
	// IssuesRareProbeMs bounds counting a combination of filters to decide
	// how to read it; IssuesShuffleWalkMaxRepos bounds how many repositories
	// one page of the shuffled feed walks (issue/filters.go, shuffle.go).
	IssuesRareProbeMs         Key = "api.issues.rare_probe_ms"
	IssuesShuffleWalkMaxRepos Key = "api.issues.shuffle_walk_max_repos"
	IssuesShuffleWalkBudgetMs Key = "api.issues.shuffle_walk_budget_ms"

	// ProfileBioMaxChars, ProfileMaxLanguages and ProfileMaxInterests bound
	// what PUT /profile stores; past them it is refused, not trimmed.
	ProfileBioMaxChars  Key = "api.profile.bio_max_chars"
	ProfileMaxLanguages Key = "api.profile.max_languages"
	ProfileMaxInterests Key = "api.profile.max_interests"

	// ReportsHideAfter is how many people reporting one target take it out of
	// lists until reviewed; ReportsPerUserPerDay caps one person's reports;
	// ReportsHiddenRefreshSeconds is how often go-api re-reads what is hidden.
	ReportsHideAfter            Key = "api.reports.hide_after"
	ReportsPerUserPerDay        Key = "api.reports.per_user_per_day"
	ReportsHiddenRefreshSeconds Key = "api.reports.hidden_refresh_seconds"
	// ThemesPublicPageSize is a page of the community theme listing.
	ThemesPublicPageSize Key = "api.themes.public_page_size"

	// BodiesFallbackMissesPerHour caps how many issue bodies one caller may
	// have fetched from GitHub per hour through GET /issues/{id}/body. Cache
	// hits are free; a miss spends the server's interactive budget.
	BodiesFallbackMissesPerHour Key = "api.bodies.fallback_misses_per_hour"

	// IssuesFilterMaxValues is the most values one GET /issues filter
	// (labels, topics, …) may name; each is a condition the query evaluates.
	IssuesFilterMaxValues Key = "api.issues.filter_max_values"

	IngestGraphQLPageSize Key = "ingest.graphql_page_size"
	IngestRateLimitFloor  Key = "ingest.rate_limit_floor"
	// IngestInteractiveReservePercent is the share of the GitHub rate limit
	// the crawler lane never spends, kept for interactive work a user is
	// waiting on (tracked-issue checks). The crawler lane's floors —
	// ingest.rate_limit_floor and ingest.discovery.rate_limit_floor — are
	// measured above it; the interactive lane stops at ingest.rate_limit_floor.
	IngestInteractiveReservePercent Key = "ingest.interactive_reserve_percent"
	IngestMaxAttempts               Key = "ingest.max_attempts"
	IngestBackoffBaseSeconds        Key = "ingest.backoff_base_seconds"
	// IngestRoutineWorkers backs the Initial/Freshness pool and
	// IngestPriorityWorkers the Deep Diver pool (Phase 5) — same keys as
	// before that split existed, repurposed meaning, not a priority queue.
	IngestRoutineWorkers  Key = "ingest.routine_workers"
	IngestPriorityWorkers Key = "ingest.priority_workers"

	// IngestClaimIntervalSeconds is the coordinator's cycle: short enough
	// that claimed capacity doesn't sit idle, long enough not to hammer
	// Postgres with empty-result polls.
	IngestClaimIntervalSeconds Key = "ingest.claim_interval_seconds"
	// IngestLeaseSeconds bounds how long a claimed job may sit in a pool's
	// queue and run before ReclaimExpiredLeases may hand it to another
	// claim. Job writes are fenced on the lease owner, so a worker that
	// overruns it loses its write rather than corrupting the cursor.
	IngestLeaseSeconds Key = "ingest.lease_seconds"
	// IngestLeaseReclaimIntervalSeconds is how often expired leases (a
	// crashed or stalled worker) are returned to pending.
	IngestLeaseReclaimIntervalSeconds Key = "ingest.lease_reclaim_interval_seconds"
	// IngestRateLimitPauseSeconds is how long the crawler lane pauses after
	// GitHub reports a rate limit, and how long a job released for lack of
	// budget waits before it can be claimed again.
	IngestRateLimitPauseSeconds Key = "ingest.rate_limit_pause_seconds"

	IngestEligibilityMinStars         Key = "ingest.eligibility.min_stars"
	IngestEligibilityPushedWithinDays Key = "ingest.eligibility.pushed_within_days"

	// IngestFreshnessIntervalSeconds is how stale a repository's last
	// successful sync must be before its first issue page is re-fetched.
	// This is what makes the catalog cycle continuously instead of going
	// quiet once every repository has been seeded once.
	IngestFreshnessIntervalSeconds Key = "ingest.freshness_interval_seconds"
	// IngestEnqueueBatchSize bounds how many jobs one coordinator cycle
	// enqueues, for both new-repository seeding and freshness re-queues.
	IngestEnqueueBatchSize Key = "ingest.enqueue_batch_size"
	// IngestEligibilityRecheckSeconds is how often stored repository
	// metadata is re-tested against the current eligibility thresholds, so
	// changing a threshold takes effect without a re-import or a restart.
	IngestEligibilityRecheckSeconds Key = "ingest.eligibility.recheck_seconds"

	// IngestMetricsRefreshSeconds is how often ingestd re-reads queue,
	// repository and GitHub budget gauges from Postgres for /metrics. They are
	// GROUP BY scans over ingest_jobs, so they do not run on every claim cycle.
	IngestMetricsRefreshSeconds Key = "ingest.metrics_refresh_seconds"

	// IngestMaxPagesPerRepo caps how many pages the Deep Diver will fetch for
	// a single repository before the job is marked done instead of
	// requeued, regardless of GitHub's hasNextPage. 0 means unlimited. This
	// bounds how much of the 40GB disk one mega-repository's issue tracker
	// can consume.
	IngestMaxPagesPerRepo Key = "ingest.max_pages_per_repo"
	// IngestMaxDBSizeMB is the ceiling on pg_database_size() above which the
	// coordinator stops enqueuing and claiming new ingestion work entirely —
	// a global guardrail on top of the per-repository one above. 0 means
	// unlimited. Checked against Postgres itself rather than container free
	// space, since ingestd cannot see the postgres volume.
	IngestMaxDBSizeMB Key = "ingest.max_db_size_mb"

	// IngestClosedPageSize is how many issues closed since a repository's
	// last completed pass are fetched alongside each page of a freshness
	// pass. Kept small so the extra connection does not raise the request's
	// rate-limit cost.
	IngestClosedPageSize Key = "ingest.closed_page_size"
	// IngestTrackedIssueCheckSeconds is how stale an open issue that a user
	// saved or is working on may get before its state is re-checked on
	// GitHub, independent of the repository's freshness pass.
	IngestTrackedIssueCheckSeconds Key = "ingest.tracked_issue_check_seconds"
	// IngestTrackedIssueBatchSize is how many tracked issues one state-check
	// request covers (one GraphQL alias each).
	IngestTrackedIssueBatchSize Key = "ingest.tracked_issue_batch_size"

	// IngestClosedIssueRetentionDays is how long after closing on GitHub an
	// issue is kept before it is deleted. Issues anyone saved or worked on
	// are never deleted. 0 keeps every closed issue.
	IngestClosedIssueRetentionDays Key = "ingest.closed_issue_retention_days"
	// IngestClosedIssuePruneIntervalSeconds is how often that deletion runs.
	IngestClosedIssuePruneIntervalSeconds Key = "ingest.closed_issue_prune_interval_seconds"
	// IngestClosedIssuePruneBatchSize is how many issues one DELETE removes.
	IngestClosedIssuePruneBatchSize Key = "ingest.closed_issue_prune_batch_size"

	// IngestDiscoveryEnabled turns the Discovery Sweeper on. Off by default:
	// it spends the crawler's GitHub budget, so turning it on is a deliberate
	// live change rather than a side effect of a deploy.
	IngestDiscoveryEnabled Key = "ingest.discovery.enabled"
	// IngestDiscoveryWorkers backs the discovery pool (one search page each).
	IngestDiscoveryWorkers Key = "ingest.discovery_workers"
	// IngestDiscoveryWindowHours is the pushed-date span of each window a new
	// sweep starts with; IngestDiscoveryMinWindowSeconds is how far a window
	// with more than GitHub's 1,000-result search cap is halved before the
	// sweeper settles for the first 1,000 and marks it truncated.
	IngestDiscoveryWindowHours      Key = "ingest.discovery.window_hours"
	IngestDiscoveryMinWindowSeconds Key = "ingest.discovery.min_window_seconds"
	IngestDiscoveryPageSize         Key = "ingest.discovery.page_size"
	// IngestDiscoveryRateLimitFloor is discovery's own floor on the shared
	// crawler budget, above the interactive reserve. Higher than
	// ingest.rate_limit_floor, so discovery yields before the issue crawl does.
	IngestDiscoveryRateLimitFloor Key = "ingest.discovery.rate_limit_floor"
	// IngestDiscoveryIncrementalIntervalSeconds and
	// IngestDiscoveryFullIntervalSeconds schedule the two sweep kinds: an
	// incremental one over repositories pushed since the last completed
	// sweep, and a full one over the whole pushed-within range.
	IngestDiscoveryIncrementalIntervalSeconds Key = "ingest.discovery.incremental_interval_seconds"
	IngestDiscoveryFullIntervalSeconds        Key = "ingest.discovery.full_interval_seconds"

	// IngestArchive* drive the GH Archive reader (internal/ingest/archive.go):
	// hourly public event files applied to stored issues with no GitHub API
	// call. While it keeps up (lag under IngestArchiveMaxLagSeconds) the
	// freshness crawl runs every IngestArchiveRepairIntervalSeconds instead of
	// ingest.freshness_interval_seconds, and archive candidates replace the
	// incremental discovery sweep.
	IngestArchiveEnabled                Key = "ingest.archive.enabled"
	IngestArchiveWorkers                Key = "ingest.archive.workers"
	IngestArchivePollSeconds            Key = "ingest.archive.poll_seconds"
	IngestArchiveRetrySeconds           Key = "ingest.archive.retry_seconds"
	IngestArchiveMissingAfterHours      Key = "ingest.archive.missing_after_hours"
	IngestArchiveMaxBackfillHours       Key = "ingest.archive.max_backfill_hours"
	IngestArchiveDownloadTimeoutSeconds Key = "ingest.archive.download_timeout_seconds"
	IngestArchiveMaxFileMB              Key = "ingest.archive.max_file_mb"
	IngestArchiveApplyBatchSize         Key = "ingest.archive.apply_batch_size"
	IngestArchiveMaxLagSeconds          Key = "ingest.archive.max_lag_seconds"
	IngestArchiveRepairIntervalSeconds  Key = "ingest.archive.repair_interval_seconds"
	IngestArchiveCandidateBatchSize     Key = "ingest.archive.candidate_batch_size"
	IngestArchiveCandidateRecheckDays   Key = "ingest.archive.candidate_recheck_days"

	// IngestBodiesCacheMaxMB and IngestBodiesPruneIntervalSeconds bound the
	// issue body cache (issue_bodies): ingestd trims it back to this size,
	// least recently read first, never a saved issue's body.
	IngestBodiesCacheMaxMB           Key = "ingest.bodies.cache_max_mb"
	IngestBodiesPruneIntervalSeconds Key = "ingest.bodies.prune_interval_seconds"

	// IngestCatalog* schedule the catalog's periodic jobs (ingest/catalog.go):
	// redrawing the shuffled feed's repository order, and recomputing the
	// filter dropdowns' values (catalog_facets), keeping FacetsMaxValues per
	// kind.
	IngestCatalogShuffleIntervalSeconds Key = "ingest.catalog.shuffle_interval_seconds"
	IngestCatalogFacetsRefreshSeconds   Key = "ingest.catalog.facets_refresh_seconds"
	IngestCatalogFacetsMaxValues        Key = "ingest.catalog.facets_max_values"

	// Live catalog counts (ingest/stats.go, GET /stats): how often the
	// trigger-written deltas are folded in, how often the totals are recounted
	// exactly, and how many recent commits the activity list keeps.
	IngestStatsRollupSeconds  Key = "ingest.stats.rollup_seconds"
	IngestStatsRecountSeconds Key = "ingest.stats.recount_seconds"
	IngestStatsActivityRows   Key = "ingest.stats.activity_rows"

	// FlowEnabled turns the live visualizer's tap on in go-api and ingestd.
	// Off, nothing is buffered or published; flowd keeps serving an idle stream.
	FlowEnabled Key = "flow.enabled"
	// FlowBufferSize bounds the events a publisher holds while Redis catches
	// up, and the lines flowd holds for one slow viewer. Past it, events drop.
	FlowBufferSize Key = "flow.buffer_size"
	// FlowPublishTimeoutMs is the deadline for one PUBLISH of a batch.
	FlowPublishTimeoutMs Key = "flow.publish_timeout_ms"
	// FlowMaxLabelChars truncates a public label or payload (an issue title,
	// a repository name) before it leaves the process.
	FlowMaxLabelChars Key = "flow.max_label_chars"
	// FlowMaxEventsPerSecond caps the labelled events flowd relays; every
	// event still counts towards its link's density.
	FlowMaxEventsPerSecond Key = "flow.max_events_per_second"
	// FlowMaxViewers caps open /stream connections.
	FlowMaxViewers Key = "flow.max_viewers"
	// FlowDensityIntervalMs is the window flowd counts events per link over
	// before sending one density line per link.
	FlowDensityIntervalMs Key = "flow.density_interval_ms"
	// FlowHeartbeatSeconds is how often an idle stream gets a keep-alive.
	FlowHeartbeatSeconds Key = "flow.heartbeat_seconds"
	// FlowAmbientRefreshSeconds is how often flowd asks Prometheus for the
	// network rates of links nothing is instrumented on, and
	// FlowAmbientRateWindowSeconds the rate() window it asks over. The window
	// must span at least two cAdvisor scrapes.
	FlowAmbientRefreshSeconds    Key = "flow.ambient_refresh_seconds"
	FlowAmbientRateWindowSeconds Key = "flow.ambient_rate_window_seconds"
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

func boolSpec(def bool, desc string) Spec {
	return Spec{Kind: KindBool, Default: def, Description: desc}
}

// Pull request tracking and XP (internal/ingest/prs.go, internal/xp).
const (
	IngestPRsEnabled             Key = "ingest.prs.enabled"
	IngestPRsUsersPerCycle       Key = "ingest.prs.users_per_cycle"
	IngestPRsUsersPerQuery       Key = "ingest.prs.users_per_query"
	IngestPRsPageSize            Key = "ingest.prs.page_size"
	IngestPRsHotIntervalSeconds  Key = "ingest.prs.hot_interval_seconds"
	IngestPRsWarmIntervalSeconds Key = "ingest.prs.warm_interval_seconds"
	IngestPRsColdIntervalSeconds Key = "ingest.prs.cold_interval_seconds"
	IngestPRsHotWindowDays       Key = "ingest.prs.hot_window_days"
	IngestPRsRetryBaseSeconds    Key = "ingest.prs.retry_base_seconds"
	PRsRefreshMinSeconds         Key = "api.prs.refresh_min_seconds"
	PRsPageSize                  Key = "api.prs.page_size"
	ClaimsDays                   Key = "api.claims.days"
	LeaderboardsEnabled          Key = "api.leaderboards.enabled"
	LeaderboardsSize             Key = "api.leaderboards.size"
	ProfilesPublicPulls          Key = "api.profiles.public_pulls"
	ClaimsMaxActive              Key = "api.claims.max_active"
	ClaimsRefreshSeconds         Key = "api.claims.refresh_seconds"
	XPRecentLimit                Key = "api.xp.recent_limit"
	XPBaseBeginner               Key = "xp.base_beginner"
	XPBaseIntermediate           Key = "xp.base_intermediate"
	XPBaseAdvanced               Key = "xp.base_advanced"
	XPBaseUnclassified           Key = "xp.base_unclassified"
	XPBaseUnlinked               Key = "xp.base_unlinked"
	XPFirstMergeRepoBonus        Key = "xp.first_merge_repo_bonus"
	XPBacklogBonus               Key = "xp.backlog_bonus"
	XPBacklogDays                Key = "xp.backlog_days"
	XPCIBonus                    Key = "xp.ci_bonus"
	XPApprovalBonus              Key = "xp.approval_bonus"
	XPApprovalBonusNoCI          Key = "xp.approval_bonus_no_ci"
	XPCleanMergeBonus            Key = "xp.clean_merge_bonus"
	XPCleanMergeBonusNoCI        Key = "xp.clean_merge_bonus_no_ci"
	XPBackfillPercent            Key = "xp.backfill_percent"
	XPRepoDailyCap               Key = "xp.repo_daily_cap"
	XPOpened                     Key = "xp.opened"
	XPOpenedWeeklyCap            Key = "xp.opened_weekly_cap"
	XPLevelStep                  Key = "xp.level_step"
	XPQuestReward                Key = "xp.quest_reward"
	XPPathReward                 Key = "xp.path_reward"
	PathsBeginnerSteps           Key = "api.paths.beginner_steps"
	PathsIntermediateSteps       Key = "api.paths.intermediate_steps"
	PathsPickPages               Key = "api.paths.pick_pages"
)

// Registry is the complete set of live-reloadable keys. Adding one here without
// a matching migration row means it falls back to Default and is reported as
// drift at startup.
var Registry = map[Key]Spec{
	IngestPRsEnabled:             boolSpec(true, "Read signed-up people's public pull requests to catalog repositories and pay XP for merges (interactive lane)."),
	IngestPRsUsersPerCycle:       intSpec(50, 1, 1000, "People whose pull requests are read per coordinator cycle."),
	IngestPRsUsersPerQuery:       intSpec(10, 1, 50, "People read in one GraphQL request (one alias each)."),
	IngestPRsPageSize:            intSpec(20, 1, 100, "Pull requests per person per request, newest-updated first."),
	IngestPRsHotIntervalSeconds:  intSpec(900, 60, 86400, "How often someone with an open pull request updated within ingest.prs.hot_window_days is read."),
	IngestPRsWarmIntervalSeconds: intSpec(3600, 60, 604800, "How often someone with an older open pull request is read."),
	IngestPRsColdIntervalSeconds: intSpec(86400, 300, 2592000, "How often someone with no open pull request is read."),
	IngestPRsHotWindowDays:       intSpec(7, 1, 90, "An open pull request updated this recently makes its author hot."),
	IngestPRsRetryBaseSeconds:    intSpec(300, 30, 86400, "First wait after a failed read; doubles per attempt, up to ingest.prs.cold_interval_seconds."),
	LeaderboardsEnabled:          boolSpec(false, "Serve GET /leaderboards. People who keep their profile private are never listed."),
	LeaderboardsSize:             intSpec(50, 1, 500, "People one leaderboard lists."),
	ProfilesPublicPulls:          intSpec(20, 0, 200, "Merged pull requests a public profile lists."),
	ClaimsDays:                   intSpec(7, 1, 60, "Days a claim lasts without an open pull request of the claimer's that closes the issue."),
	ClaimsMaxActive:              intSpec(2, 1, 20, "Claims one person may hold at once."),
	ClaimsRefreshSeconds:         intSpec(30, 5, 3600, "How often go-api re-reads which issues are claimed, for lists."),
	PRsPageSize:                  intSpec(50, 1, 500, "Tracked pull requests GET /me/pulls returns."),
	XPRecentLimit:                intSpec(20, 1, 200, "Latest XP events GET /me/xp returns."),
	PRsRefreshMinSeconds:         intSpec(300, 0, 86400, "Shortest gap between two \"check my pull requests now\" requests from one person."),
	XPBaseBeginner:               intSpec(75, 0, 10000, "XP for a merge that closes a beginner catalog issue."),
	XPBaseIntermediate:           intSpec(140, 0, 10000, "XP for a merge that closes an intermediate catalog issue."),
	XPBaseAdvanced:               intSpec(250, 0, 10000, "XP for a merge that closes an advanced catalog issue."),
	XPBaseUnclassified:           intSpec(100, 0, 10000, "XP for a merge that closes a catalog issue of unknown difficulty."),
	XPBaseUnlinked:               intSpec(50, 0, 10000, "XP for a merge to a catalog repository that closes no catalog issue."),
	XPFirstMergeRepoBonus:        intSpec(15, 0, 10000, "Extra XP for someone's first merge in a repository."),
	XPBacklogBonus:               intSpec(50, 0, 10000, "Extra XP for closing a catalog issue that had been open xp.backlog_days or more when it was fixed."),
	XPBacklogDays:                intSpec(180, 1, 3650, "Age at which an issue counts as neglected backlog."),
	XPCIBonus:                    intSpec(15, 0, 10000, "Extra XP when the merged pull request's checks passed (repositories with checks)."),
	XPApprovalBonus:              intSpec(10, 0, 10000, "Extra XP when a maintainer approved it, in a repository with checks."),
	XPApprovalBonusNoCI:          intSpec(20, 0, 10000, "Extra XP when a maintainer approved it, in a repository without checks."),
	XPCleanMergeBonus:            intSpec(5, 0, 10000, "Extra XP when it merged with no changes requested, in a repository with checks."),
	XPCleanMergeBonusNoCI:        intSpec(10, 0, 10000, "Extra XP when it merged with no changes requested, in a repository without checks."),
	XPBackfillPercent:            intSpec(50, 0, 100, "Share of the XP paid for merges from before someone signed up."),
	XPRepoDailyCap:               intSpec(400, 0, 100000, "Most merge XP one person earns in one repository in one day (UTC); 0 = no cap."),
	XPOpened:                     intSpec(5, 0, 1000, "XP for opening a pull request that closes a catalog issue, once per issue; reversed if it closes unmerged."),
	XPOpenedWeeklyCap:            intSpec(3, 0, 100, "Most opened-pull-request rewards per person per week (UTC ISO week)."),
	XPQuestReward:                intSpec(50, 0, 10000, "XP for finishing a quest."),
	XPPathReward:                 intSpec(200, 0, 100000, "XP for finishing a learning path."),
	PathsBeginnerSteps:           intSpec(3, 0, 10, "Beginner issues a new learning path starts with."),
	PathsPickPages:               intSpec(5, 1, 50, "Shuffled-feed pages read to find a learning path's issues of one difficulty."),
	PathsIntermediateSteps:       intSpec(2, 0, 10, "Intermediate issues a new learning path ends with."),
	XPLevelStep:                  intSpec(100, 1, 100000, "Level L starts at xp.level_step x L(L-1)/2 XP: 100, 300, 600, 1000 ..."),

	RateLimitUnauthPerHour: intSpec(180, 1, 100000, "Requests per window allowed for unauthenticated callers."),
	RateLimitAuthPerHour:   intSpec(1000, 1, 1000000, "Requests per window allowed for authenticated callers."),
	RateLimitWindowSeconds: intSpec(3600, 60, 86400, "Length of the rate-limit window."),

	IssuesDefaultPageSize: intSpec(20, 1, 100, "Issues returned when the catalog is browsed without a search term."),
	IssuesSearchPageSize:  intSpec(50, 1, 200, "Issues returned for a search query."),
	SearchPerCategoryCap:  intSpec(25, 1, 100, "Maximum results per match category, so one category cannot flood the rest."),
	SearchPerRepoCap:      intSpec(3, 1, 100, "Most issues one repository contributes to a search's Repository or Language category, so one large repository cannot fill it."),
	SearchSampleSize:      intSpec(2000, 100, 100000, "Title matches a search reads before ordering them by stars, and the deepest one category of a search pages; bounds the work for a common word."),
	NotificationsPageSize: intSpec(50, 1, 200, "Notifications returned by GET /notifications."),

	SessionTTLHours:             intSpec(168, 1, 8760, "Session lifetime. Applies to newly issued sessions only."),
	SessionPruneIntervalSeconds: intSpec(3600, 60, 86400, "How often expired sessions and abandoned OAuth handshakes are deleted."),

	CacheHotIssuesTTLSeconds:      intSpec(120, 0, 3600, "Cache lifetime for the default unauthenticated catalog page. 0 disables."),
	CacheSearchTTLSeconds:         intSpec(60, 0, 3600, "Cache lifetime for unauthenticated search results. 0 disables."),
	StatsMaxAgeSeconds:            intSpec(10, 0, 3600, "Public cache lifetime of GET /stats, at the edge and in Redis."),
	CacheRecommendationsTTLSecond: intSpec(300, 0, 3600, "Cache lifetime for per-user recommendations. 0 disables."),
	CachePublicMaxAgeSeconds:      intSpec(60, 0, 3600, "Cache-Control max-age on catalog and search responses to signed-out callers. 0 sends no-cache."),
	CachePublicStaleSeconds:       intSpec(300, 0, 86400, "Cache-Control stale-while-revalidate on catalog and search responses to signed-out callers."),
	CacheRedisTimeoutMs:           intSpec(250, 10, 5000, "Deadline for one Redis call, connecting included. A call that misses it fails open."),
	CacheRedisRetrySeconds:        intSpec(5, 1, 300, "After a failed Redis call, how long Redis is skipped before one call probes whether it has recovered."),

	HTTPMaxBodyBytes:    intSpec(1048576, 1024, 104857600, "Largest request body accepted on any API route except the webhook; larger bodies get 413."),
	WebhookMaxBodyBytes: intSpec(26214400, 1024, 104857600, "Largest GitHub webhook body accepted (GitHub's own cap is 25 MiB); larger bodies get 413."),

	HTTPOutboundTimeoutSeconds: intSpec(15, 1, 120, "Deadline for each outbound call to GitHub or ingestd; a call that exceeds it fails."),

	IssuesMaxLimit:                intSpec(100, 1, 500, "Largest ?limit honoured by GET /issues; a larger request is clamped."),
	IssuesMaxPage:                 intSpec(200, 1, 10000, "Deepest ?page honoured by GET /issues; a deeper request is clamped. Deep offsets are expensive."),
	RecommendationsMaxLimit:       intSpec(20, 1, 100, "Largest ?limit honoured by GET /recommendations; a larger request is clamped."),
	RecommendationsDefaultLimit:   intSpec(3, 1, 100, "Recommendations GET /recommendations returns without ?limit (never above api.recommendations.max_limit). Only this size is cached."),
	ThemesMaxPerUser:              intSpec(20, 1, 200, "Most themes one account may keep."),
	ThemesMaxBytes:                intSpec(65536, 1024, 524288, "Largest theme, TOML and stylesheet together, in bytes."),
	RecommendationsCandidatePool:  intSpec(200, 10, 1000, "Open issues GET /recommendations reads off the shuffled feed, matching the caller's languages and level, before scoring them."),
	IssuesLanguageCountTTLSeconds: intSpec(600, 0, 86400, "How long a language's counted open issues are reused when deciding how to browse it. 0 counts on every request."),
	IssuesRareProbeMs:             intSpec(500, 0, 10000, "How long counting a combination of filters may take when deciding how to browse it; one that takes longer is read in index order. 0 does not bound it."),
	IssuesShuffleWalkBudgetMs:     intSpec(2000, 0, 9000, "How long one page of the shuffled feed may look for matches before it is returned short, with a cursor to continue from. 0 does not bound it."),
	ProfileBioMaxChars:            intSpec(280, 0, 2000, "The longest bio a profile may hold, in characters."),
	ProfileMaxLanguages:           intSpec(12, 1, 100, "The most preferred languages a profile may list."),
	ProfileMaxInterests:           intSpec(12, 1, 100, "The most interests a profile may list."),
	ReportsHideAfter:              intSpec(3, 1, 100, "Different people reporting the same issue, repository or theme before it leaves lists until reviewed."),
	ReportsPerUserPerDay:          intSpec(20, 1, 1000, "Reports one person may make in 24 hours."),
	ReportsHiddenRefreshSeconds:   intSpec(60, 5, 3600, "How often go-api re-reads which reported targets are hidden."),
	ThemesPublicPageSize:          intSpec(24, 1, 100, "Public themes per page of the community listing."),
	IssuesShuffleWalkMaxRepos:     intSpec(3000, 0, 1000000, "Repositories one page of the shuffled feed walks before reading the rest of the page from the matching issues. 0 walks without a bound."),
	IssuesRareLanguageThreshold:   intSpec(20000, 0, 1000000, "A language with fewer open issues than this is read repository-first and sorted when browsing, instead of walking the catalog index. 0 always walks the index."),
	IssuesQueryTimeoutMs:          intSpec(10000, 100, 25000, "Deadline for the queries behind one GET /issues or GET /issues/{id}. A query that misses it is cancelled and the caller gets 503."),

	IngestGraphQLPageSize:           intSpec(50, 1, 100, "Issues requested per GraphQL page. GitHub caps this at 100."),
	IngestRateLimitFloor:            intSpec(100, 0, 5000, "Stop issuing GraphQL calls when the remaining point budget falls below this. On the crawler lane it is measured above the interactive reserve."),
	IngestInteractiveReservePercent: intSpec(30, 0, 90, "Share of the GitHub rate limit the crawler lane never spends, kept for interactive work such as tracked-issue checks. 0 lets the crawler spend everything down to its floor."),
	IngestMaxAttempts:               intSpec(3, 1, 10, "Attempts per repository before a sync is recorded as failed."),
	IngestBackoffBaseSeconds:        intSpec(1, 1, 60, "Base for exponential retry backoff between sync attempts."),
	// D20: ~4 total in-flight by default, sized for the 2 vCPU target
	// deployment rather than a 20-vCPU dev box.
	IngestRoutineWorkers:  intSpec(2, 1, 64, "Concurrent Initial/Freshness workers (one page per eligible repository)."),
	IngestPriorityWorkers: intSpec(1, 1, 64, "Concurrent Deep Diver workers (subsequent pagination)."),

	IngestClaimIntervalSeconds:        intSpec(5, 1, 300, "How often the ingest coordinator runs a cycle: enqueue, reclaim, claim and submit."),
	IngestLeaseSeconds:                intSpec(300, 60, 3600, "How long a claimed ingest job may wait and run before its lease can be reclaimed. Must exceed the slowest page fetch plus queue wait."),
	IngestLeaseReclaimIntervalSeconds: intSpec(60, 5, 3600, "How often expired ingest job leases are returned to pending."),
	IngestRateLimitPauseSeconds:       intSpec(60, 5, 3600, "How long the crawler lane pauses after a GitHub rate limit, and how long a job released for lack of budget waits."),

	IngestEligibilityMinStars:         intSpec(25, 0, 100000, "Minimum stars for a repository to be eligible for ingestion. Abuse control, not ranking."),
	IngestEligibilityPushedWithinDays: intSpec(730, 1, 3650, "A repository must have been pushed to within this many days to be eligible for ingestion."),

	IngestFreshnessIntervalSeconds:  intSpec(86400, 300, 2592000, "How old a repository's last successful sync must be before its first issue page is re-fetched."),
	IngestEnqueueBatchSize:          intSpec(500, 1, 50000, "Jobs enqueued per coordinator cycle, for both new-repository seeding and freshness re-queues."),
	IngestEligibilityRecheckSeconds: intSpec(3600, 60, 86400, "How often stored repository metadata is re-tested against the current eligibility thresholds."),
	IngestMetricsRefreshSeconds:     intSpec(30, 5, 3600, "How often ingestd refreshes its queue, repository and GitHub budget gauges from Postgres for /metrics."),

	// D6 disk guardrails. Defaults are sized for the 40GB VPS target: at the
	// measured ~0.79 KB/row, 100 pages per repo (at up to 100 issues/page)
	// bounds any single repository to ~10,000 issues, and 30000 MB leaves
	// ~10GB of the 40GB disk for WAL, vacuum headroom, images and the OS —
	// both 0 (unlimited) remain valid, explicit opt-outs.
	IngestMaxPagesPerRepo: intSpec(100, 0, 100000, "Pages the Deep Diver will fetch for one repository before stopping, regardless of hasNextPage. 0 disables the cap."),
	IngestMaxDBSizeMB:     intSpec(30000, 0, 1000000, "pg_database_size() ceiling above which the coordinator stops enqueuing and claiming new ingestion work. 0 disables the cap."),

	IngestClosedPageSize:           intSpec(40, 1, 100, "Issues closed since a repository's last completed pass, fetched alongside each page of a freshness pass."),
	IngestTrackedIssueCheckSeconds: intSpec(3600, 60, 86400, "How stale an open issue a user saved or is working on may get before its state is re-checked on GitHub."),
	IngestTrackedIssueBatchSize:    intSpec(50, 1, 100, "Tracked issues checked per GitHub request."),

	IngestClosedIssueRetentionDays:        intSpec(7, 0, 3650, "Days after closing on GitHub before an issue nobody saved or worked on is deleted. 0 keeps every closed issue."),
	IngestClosedIssuePruneIntervalSeconds: intSpec(3600, 60, 86400, "How often closed issues past their retention are deleted."),
	IngestClosedIssuePruneBatchSize:       intSpec(5000, 100, 100000, "Closed issues deleted per statement."),

	IngestDiscoveryEnabled:                    boolSpec(false, "Run the Discovery Sweeper: GitHub repository Search over pushed-date windows for repositories the dataset does not contain."),
	IngestDiscoveryWorkers:                    intSpec(1, 1, 16, "Concurrent Discovery Sweeper workers (one search page each)."),
	IngestDiscoveryWindowHours:                intSpec(24, 1, 720, "Pushed-date span of each search window a new sweep starts with; denser windows are halved."),
	IngestDiscoveryMinWindowSeconds:           intSpec(60, 1, 86400, "Smallest window a dense one is halved down to. Past it, only GitHub's first 1,000 results are read and the window is marked truncated."),
	IngestDiscoveryPageSize:                   intSpec(100, 1, 100, "Repositories requested per search page. GitHub caps this at 100."),
	IngestDiscoveryRateLimitFloor:             intSpec(1000, 0, 12500, "Discovery stops searching when the shared budget falls below this plus the interactive reserve; kept above ingest.rate_limit_floor so the issue crawl keeps going after discovery yields."),
	IngestDiscoveryIncrementalIntervalSeconds: intSpec(86400, 3600, 2592000, "How often an incremental sweep covers repositories pushed since the last completed sweep."),
	IngestArchiveEnabled:                      boolSpec(false, "Read GH Archive's hourly public event files and apply their issue events to stored repositories."),
	IngestArchiveWorkers:                      intSpec(1, 1, 8, "Archive hours processed at once."),
	IngestArchivePollSeconds:                  intSpec(300, 30, 3600, "How often newly ended hours are queued for the archive reader."),
	IngestArchiveRetrySeconds:                 intSpec(600, 60, 86400, "Wait before retrying an archive hour whose file is not published yet, or whose processing failed."),
	IngestArchiveMissingAfterHours:            intSpec(24, 1, 720, "An archive hour whose file is still not published this many hours after it ended is marked missing."),
	IngestArchiveMaxBackfillHours:             intSpec(168, 1, 2160, "On first start, the archive reader goes back to the oldest completed crawl pass, but no further than this."),
	IngestArchiveDownloadTimeoutSeconds:       intSpec(300, 10, 3600, "Deadline for downloading and reading one archive hour."),
	IngestArchiveMaxFileMB:                    intSpec(512, 1, 4096, "Largest compressed archive file accepted; a larger one fails its hour."),
	IngestArchiveApplyBatchSize:               intSpec(500, 1, 10000, "Issues from the archive written per transaction."),
	IngestArchiveMaxLagSeconds:                intSpec(21600, 3600, 604800, "If the archive falls further behind than this, the freshness crawl returns to ingest.freshness_interval_seconds and the incremental discovery sweep resumes."),
	IngestArchiveRepairIntervalSeconds:        intSpec(2592000, 86400, 31536000, "While the archive keeps up, the freshness crawl re-checks each repository this rarely, to catch what events miss (deleted or transferred issues)."),
	IngestArchiveCandidateBatchSize:           intSpec(50, 1, 100, "Repositories seen in the archive but not stored, looked up on GitHub per call."),
	IngestCatalogShuffleIntervalSeconds:       intSpec(86400, 3600, 2592000, "How often the shuffled feed's repository order is re-randomised."),
	IngestCatalogFacetsRefreshSeconds:         intSpec(86400, 3600, 2592000, "How often the filter dropdowns' values and counts (catalog_facets) are recomputed."),
	IngestCatalogFacetsMaxValues:              intSpec(300, 10, 5000, "Values kept per filter (labels, topics, tech, languages), the most common first."),
	IngestStatsRollupSeconds:                  intSpec(5, 1, 3600, "How often ingestd folds the issue-change deltas into the live catalog counts behind GET /stats."),
	IngestStatsRecountSeconds:                 intSpec(3600, 60, 86400, "How often ingestd recounts open issues and eligible repositories exactly, correcting any drift in the live counts."),
	IngestStatsActivityRows:                   intSpec(50, 0, 1000, "How many recent crawl and archive commits GET /stats lists as activity."),
	IssuesFilterMaxValues:                     intSpec(20, 1, 100, "Most values one GET /issues filter may name; more is a 400."),
	IngestBodiesCacheMaxMB:                    intSpec(512, 0, 100000, "Size the issue body cache is trimmed back to, least recently read first; bodies of saved issues are always kept. 0 keeps only those."),
	IngestBodiesPruneIntervalSeconds:          intSpec(3600, 60, 86400, "How often the issue body cache is trimmed."),
	BodiesFallbackMissesPerHour:               intSpec(30, 0, 10000, "Issue bodies one caller may have fetched from GitHub per hour through GET /issues/{id}/body; cached bodies do not count. 0 serves the cache only."),
	IngestArchiveCandidateRecheckDays:         intSpec(30, 1, 3650, "A looked-up candidate that was not stored is looked up again after this many days, if it is still active."),
	IngestDiscoveryFullIntervalSeconds:        intSpec(2592000, 86400, 31536000, "How often a full sweep covers the whole pushed-within range, catching repositories that crossed the star floor without a new push."),

	FlowEnabled:                  boolSpec(true, "Publish live flow events from go-api and ingestd for the public visualizer (flowd). Signed-in traffic is published as symbols only."),
	FlowBufferSize:               intSpec(1024, 64, 65536, "Flow events a publisher holds while Redis catches up, and lines flowd holds for one slow viewer; past it, events are dropped and counted."),
	FlowPublishTimeoutMs:         intSpec(250, 10, 5000, "Deadline for publishing one batch of flow events to Redis."),
	FlowMaxLabelChars:            intSpec(80, 8, 500, "Longest public label or payload (an issue title, a repository name) a flow event carries; longer text is truncated."),
	FlowMaxEventsPerSecond:       intSpec(40, 1, 1000, "Labelled flow events flowd relays per second; the rest count only towards their link's density."),
	FlowMaxViewers:               intSpec(200, 1, 5000, "Concurrent /stream viewers flowd serves; past it, new viewers get 503."),
	FlowDensityIntervalMs:        intSpec(1000, 100, 60000, "Window flowd counts events per link over before sending each link's density."),
	FlowHeartbeatSeconds:         intSpec(15, 1, 300, "How often flowd sends a keep-alive on an idle stream."),
	FlowAmbientRefreshSeconds:    intSpec(5, 1, 300, "How often flowd reads cAdvisor network rates from Prometheus for the links nothing is instrumented on."),
	FlowAmbientRateWindowSeconds: intSpec(120, 30, 3600, "rate() window for those network rates; must span at least two cAdvisor scrapes."),
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
func (s *Snapshot) Millis(k Key) time.Duration {
	return time.Duration(s.Int(k)) * time.Millisecond
}

func (s *Snapshot) Hours(k Key) time.Duration {
	return time.Duration(s.Int(k)) * time.Hour
}

// With returns a copy of s with k set to v, for tests that need a value other
// than the default. The receiver is unchanged.
func (s *Snapshot) With(k Key, v any) *Snapshot {
	vals := s.Raw()
	vals[k] = v
	return newSnapshot(vals)
}

// Raw exposes the resolved values for the admin endpoint.
func (s *Snapshot) Raw() map[Key]any {
	out := make(map[Key]any, len(s.values))
	for k, v := range s.values {
		out[k] = v
	}
	return out
}
