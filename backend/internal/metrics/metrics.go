// Package metrics is the Prometheus telemetry for SkillIssues Go services.
//
// Every process exposes Go runtime and process metrics. Each binary then
// registers only the families it actually produces (RegisterAPI,
// RegisterIngest, RegisterGitHub, RegisterDBPool), so a gauge that one
// process never sets is not exported from it as a misleading zero.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry is the process-wide registry served at /metrics.
var Registry = prometheus.NewRegistry()

func init() {
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

// Handler serves the registry in Prometheus exposition format. A collector
// that fails during a scrape fails the scrape (HTTP 500) rather than
// silently returning partial data.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		ErrorHandling:     promhttp.HTTPErrorOnError,
		Registry:          Registry,
		EnableOpenMetrics: false,
	})
}

// ---------------------------------------------------------------------------
// HTTP (go-api)
// ---------------------------------------------------------------------------

var (
	HTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests handled, by matched route pattern and status code.",
	}, []string{"method", "route", "code"})

	HTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency by matched route pattern.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"method", "route"})

	HTTPResponseBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_response_bytes_total",
		Help: "Response body bytes written, by matched route pattern.",
	}, []string{"method", "route"})

	BudgetErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "github_budget_errors_total",
		Help: "Failures reading or writing the shared github_budget table, by operation and lane.",
	}, []string{"op", "lane"})

	CacheErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_errors_total",
		Help: "Failed Redis calls by operation (get, set, del, rate_limit). Cache misses are not errors. The API fails open on each.",
	}, []string{"op"})

	ReportsHiddenRefreshErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "reports_hidden_refresh_errors_total",
		Help: "Failed re-reads of which reported targets are hidden. Until one succeeds, the last read stands.",
	})

	CacheSkipped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_skipped_total",
		Help: "Redis calls not attempted because Redis failed recently, by operation. The API fails open on each without waiting; one call per api.cache.redis_retry_seconds probes for recovery.",
	}, []string{"op"})
)

// RegisterAPI registers the families go-api produces.
func RegisterAPI() {
	Registry.MustRegister(HTTPRequests, HTTPDuration, HTTPResponseBytes, BudgetErrors, CacheErrors, CacheSkipped, ReportsHiddenRefreshErrors)
}

// ---------------------------------------------------------------------------
// GitHub client (both processes)
// ---------------------------------------------------------------------------

var (
	GitHubRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "github_requests_total",
		Help: "Outbound GitHub API requests by API, operation and HTTP status code (\"error\" for transport failures).",
	}, []string{"api", "operation", "code"})

	GitHubDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "github_request_duration_seconds",
		Help:    "Outbound GitHub API request latency.",
		Buckets: []float64{.1, .25, .5, 1, 2, 4, 8, 15},
	}, []string{"api", "operation"})
)

// RegisterGitHub registers the outbound GitHub client families.
func RegisterGitHub() {
	Registry.MustRegister(GitHubRequests, GitHubDuration)
}

// ---------------------------------------------------------------------------
// Ingestion (ingestd)
// ---------------------------------------------------------------------------

var (
	IngestJobOutcomes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_job_outcomes_total",
		Help: "Processed ingest jobs by job type and outcome.",
	}, []string{"job_type", "outcome"})

	IngestIssuesUpserted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ingest_issues_upserted_total",
		Help: "Issues written by the crawler.",
	})

	IngestIssuesClosed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_issues_closed_total",
		Help: "Issues ingestd marked CLOSED, by how it found out: crawl (a freshness pass) or tracked (the saved/in-progress state check).",
	}, []string{"source"})

	IngestIssuesPruned = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ingest_issues_pruned_total",
		Help: "Closed issues deleted after ingest.closed_issue_retention_days (never saved or worked-on ones).",
	})

	IngestCycleErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "ingest_cycle_errors_total",
		Help: "Coordinator cycle step failures, by step.",
	}, []string{"step"})

	IngestJobs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ingest_jobs",
		Help: "Rows in ingest_jobs by job type and status, as of the last refresh.",
	}, []string{"job_type", "status"})

	IngestStuckLeases = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ingest_stuck_leases",
		Help: "Running jobs whose lease has expired and not yet been reclaimed.",
	})

	IngestLeasesReclaimed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ingest_leases_reclaimed_total",
		Help: "Running jobs returned to pending because their lease expired.",
	})

	IngestDBSizeBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ingest_db_size_bytes",
		Help: "pg_database_size() as seen by the disk guardrail.",
	})

	IngestPaused = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ingest_paused",
		Help: "1 while the disk guardrail (ingest.max_db_size_mb) is pausing new work.",
	})

	RepositoriesEligible = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "repositories",
		Help: "Stored repositories by eligibility and whether they have ever been synced.",
	}, []string{"eligible", "synced"})

	GitHubBudgetRemaining = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_rate_limit_remaining",
		Help: "Remaining GitHub GraphQL points for the server installation, by lane (from github_budget).",
	}, []string{"lane"})

	GitHubBudgetLimit = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_rate_limit_limit",
		Help: "GitHub GraphQL point limit for the server installation, by lane.",
	}, []string{"lane"})

	GitHubBudgetResetSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_rate_limit_reset_timestamp_seconds",
		Help: "Unix time at which the lane's GitHub budget resets.",
	}, []string{"lane"})

	GitHubSecondaryPaused = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "github_secondary_limit_paused",
		Help: "1 while a GitHub secondary rate limit is pausing the lane.",
	}, []string{"lane"})

	DiscoveryWindowOutcomes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "discovery_window_outcomes_total",
		Help: "Processed Discovery Sweeper window pages by outcome (page, split, budget_deferred, rate_limited, fetch_error, lease_lost, persist_error, …).",
	}, []string{"outcome"})

	DiscoveryRepositories = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "discovery_repositories_total",
		Help: "Repositories the Discovery Sweeper saw, by what it did with them: new, updated, renamed, ineligible, excluded, conflict.",
	}, []string{"outcome"})

	DiscoveryWindowsTruncated = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "discovery_windows_truncated_total",
		Help: "Search windows at ingest.discovery.min_window_seconds that still matched more than GitHub's 1,000-result cap; only the first 1,000 were read.",
	})

	DiscoveryWindows = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "discovery_windows",
		Help: "Rows in discovery_windows by status, as of the last refresh.",
	}, []string{"status"})

	DiscoveryLastSweepCompleted = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "discovery_last_sweep_completed_timestamp_seconds",
		Help: "Unix time the latest successful sweep of each kind completed; 0 = never.",
	}, []string{"kind"})

	ArchiveHourOutcomes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "archive_hour_outcomes_total",
		Help: "GH Archive hours processed, by outcome (done, not_published, missing, fetch_error, parse_error, persist_error, lease_lost).",
	}, []string{"outcome"})

	ArchiveIssueEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "archive_issue_events_total",
		Help: "Issues the GH Archive reader took from an hour, by what it did with them: applied, stale (not newer than stored), stale_or_not_stored (an update-only write — a closed issue, or the disk guard on — that wrote nothing: not newer, or never stored), ineligible_repo, candidate (repository not stored).",
	}, []string{"result"})

	ArchiveHours = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "archive_hours",
		Help: "Rows in archive_hours by status, as of the last refresh.",
	}, []string{"status"})

	ArchiveLagSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "archive_lag_seconds",
		Help: "How far behind now the GH Archive reader's complete coverage ends: every hour before it is done or missing.",
	})

	ArchiveFallback = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "archive_fallback",
		Help: "1 while the archive reader is enabled but further behind than ingest.archive.max_lag_seconds, so the daily freshness crawl and the incremental discovery sweep run instead.",
	})

	ArchiveCandidates = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "archive_candidates_total",
		Help: "Repositories seen in the archive but not stored, looked up on GitHub, by outcome: new, updated, renamed, ineligible, excluded, conflict, not_found.",
	}, []string{"outcome"})

	BodyFetches = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "body_fetches_total",
		Help: "Issue bodies ingestd was asked to fetch for the body fallback, by outcome: fetched, budget, no-credential, gone, not-stored.",
	}, []string{"outcome"})

	RepositoryIdentityConflicts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "repository_identity_conflicts_total",
		Help: "Repository rows whose GitHub id or name collided with another row: duplicate (an id-less twin marked duplicate), rename_blocked (a rename onto a name another repository's row holds), id_mismatch (a name now resolving to a different repository).",
	}, []string{"kind"})

	WorkerPool = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "worker_pool",
		Help: "Worker pool state by pool and field (concurrency, active, queue_length, queue_capacity).",
	}, []string{"pool", "field"})
)

// ---------------------------------------------------------------------------
// Flow tap (go-api, ingestd) and relay (flowd)
// ---------------------------------------------------------------------------

var (
	FlowEventsPublished = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flow_events_published_total",
		Help: "Flow events this process published to Redis for the live visualizer.",
	})

	FlowEventsDropped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "flow_events_dropped_total",
		Help: "Flow events discarded, by reason: buffer_full, encode and redis (publisher), no_subscriber (published with no flowd listening), decode, invalid and slow_viewer (flowd).",
	}, []string{"reason"})

	FlowEventsRelayed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flow_events_relayed_total",
		Help: "Labelled flow events flowd sent to viewers (within flow.max_events_per_second); the rest count only towards link density.",
	})

	FlowViewers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "flow_viewers",
		Help: "Open /stream connections on flowd.",
	})

	FlowAmbientErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "flow_ambient_errors_total",
		Help: "Failed Prometheus queries for the visualizer's ambient (uninstrumented) link rates.",
	})
)

// RegisterFlowPublisher registers the families a process that taps flow
// events produces (go-api, ingestd).
func RegisterFlowPublisher() {
	Registry.MustRegister(FlowEventsPublished, FlowEventsDropped)
}

// RegisterFlowd registers the families flowd produces.
func RegisterFlowd() {
	Registry.MustRegister(FlowEventsDropped, FlowEventsRelayed, FlowViewers, FlowAmbientErrors)
}

// RegisterIngest registers the families ingestd produces.
func RegisterIngest() {
	Registry.MustRegister(
		IngestJobOutcomes, IngestIssuesUpserted, IngestIssuesClosed, IngestIssuesPruned, IngestCycleErrors,
		IngestJobs, IngestStuckLeases, IngestLeasesReclaimed, IngestDBSizeBytes, IngestPaused, RepositoriesEligible,
		GitHubBudgetRemaining, GitHubBudgetLimit, GitHubBudgetResetSeconds, GitHubSecondaryPaused,
		WorkerPool, BudgetErrors,
		DiscoveryWindowOutcomes, DiscoveryRepositories, DiscoveryWindowsTruncated, DiscoveryWindows,
		DiscoveryLastSweepCompleted, RepositoryIdentityConflicts,
		ArchiveHourOutcomes, ArchiveIssueEvents, ArchiveHours, ArchiveLagSeconds, ArchiveFallback, ArchiveCandidates,
		BodyFetches,
	)
}
