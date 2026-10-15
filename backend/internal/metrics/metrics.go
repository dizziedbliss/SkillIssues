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

	ContributionVerifications = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "contribution_verify_total",
		Help: "Contribution verification attempts by result.",
	}, []string{"result"})

	BudgetErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "github_budget_errors_total",
		Help: "Failures reading or writing the shared github_budget table, by operation and lane.",
	}, []string{"op", "lane"})

	CacheErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_errors_total",
		Help: "Failed Redis calls by operation (get, set, del, rate_limit). Cache misses are not errors. The API fails open on each.",
	}, []string{"op"})
)

// RegisterAPI registers the families go-api produces.
func RegisterAPI() {
	Registry.MustRegister(HTTPRequests, HTTPDuration, HTTPResponseBytes, ContributionVerifications, BudgetErrors, CacheErrors)
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

	WorkerPool = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "worker_pool",
		Help: "Worker pool state by pool and field (concurrency, active, queue_length, queue_capacity).",
	}, []string{"pool", "field"})
)

// RegisterIngest registers the families ingestd produces.
func RegisterIngest() {
	Registry.MustRegister(
		IngestJobOutcomes, IngestIssuesUpserted, IngestIssuesClosed, IngestCycleErrors,
		IngestJobs, IngestStuckLeases, IngestLeasesReclaimed, IngestDBSizeBytes, IngestPaused, RepositoriesEligible,
		GitHubBudgetRemaining, GitHubBudgetLimit, GitHubBudgetResetSeconds, GitHubSecondaryPaused,
		WorkerPool, BudgetErrors,
	)
}
