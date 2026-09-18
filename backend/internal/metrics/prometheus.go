// Package metrics provides lightweight, Prometheus-compatible telemetry for SkillIssues Go services.
// It exposes standard counters, gauges, and histograms over HTTP at /metrics without heavy dependencies.
package metrics

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Registry holds all registered metrics.
type Registry struct {
	mu sync.RWMutex

	// Counters
	httpRequestsTotal     map[string]*int64
	githubGraphQLRequests map[string]*int64
	syncJobsTotal         map[string]*int64
	contributionVerifies  map[string]*int64

	// Gauges
	githubRateLimitRemaining int64
	workerQueueDepth         map[string]*int64
	dbConnectionsActive      int64

	// Histograms (bucketed)
	httpDurationSum   map[string]*float64
	httpDurationCount map[string]*int64
}

// Default is the singleton metrics registry.
var Default = NewRegistry()

// NewRegistry creates a new initialized metrics registry.
func NewRegistry() *Registry {
	return &Registry{
		httpRequestsTotal:     make(map[string]*int64),
		githubGraphQLRequests: make(map[string]*int64),
		syncJobsTotal:         make(map[string]*int64),
		contributionVerifies:  make(map[string]*int64),
		workerQueueDepth:      make(map[string]*int64),
		httpDurationSum:       make(map[string]*float64),
		httpDurationCount:     make(map[string]*int64),
	}
}

// RecordHTTPRequest records an HTTP request completion.
func (r *Registry) RecordHTTPRequest(method, path string, status int, duration time.Duration) {
	key := fmt.Sprintf(`method="%s",path="%s",status="%d"`, method, sanitizePath(path), status)

	r.mu.Lock()
	counter, ok := r.httpRequestsTotal[key]
	if !ok {
		var val int64
		counter = &val
		r.httpRequestsTotal[key] = counter
	}

	countPtr, ok := r.httpDurationCount[key]
	if !ok {
		var val int64
		countPtr = &val
		r.httpDurationCount[key] = countPtr
	}

	sumPtr, ok := r.httpDurationSum[key]
	if !ok {
		var val float64
		sumPtr = &val
		r.httpDurationSum[key] = sumPtr
	}
	r.mu.Unlock()

	atomic.AddInt64(counter, 1)
	atomic.AddInt64(countPtr, 1)

	r.mu.Lock()
	*sumPtr += duration.Seconds()
	r.mu.Unlock()
}

// RecordGitHubGraphQLRequest increments the count of GitHub GraphQL operations.
func (r *Registry) RecordGitHubGraphQLRequest(operation string, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	key := fmt.Sprintf(`operation="%s",status="%s"`, operation, status)

	r.mu.Lock()
	defer r.mu.Unlock()
	counter, ok := r.githubGraphQLRequests[key]
	if !ok {
		var val int64
		counter = &val
		r.githubGraphQLRequests[key] = counter
	}
	atomic.AddInt64(counter, 1)
}

// SetGitHubRateLimit updates the remaining GitHub GraphQL rate limit gauge.
func (r *Registry) SetGitHubRateLimit(remaining int) {
	atomic.StoreInt64(&r.githubRateLimitRemaining, int64(remaining))
}

// SetWorkerQueueDepth updates the worker pool queue depth gauge.
func (r *Registry) SetWorkerQueueDepth(poolName string, depth int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	gauge, ok := r.workerQueueDepth[poolName]
	if !ok {
		var val int64
		gauge = &val
		r.workerQueueDepth[poolName] = gauge
	}
	atomic.StoreInt64(gauge, int64(depth))
}

// RecordSyncJob records the outcome of a repository issue sync job.
func (r *Registry) RecordSyncJob(status string) {
	key := fmt.Sprintf(`status="%s"`, status)
	r.mu.Lock()
	defer r.mu.Unlock()
	counter, ok := r.syncJobsTotal[key]
	if !ok {
		var val int64
		counter = &val
		r.syncJobsTotal[key] = counter
	}
	atomic.AddInt64(counter, 1)
}

// RecordContributionVerification records the result of a PR verification.
func (r *Registry) RecordContributionVerification(result string) {
	key := fmt.Sprintf(`result="%s"`, result)
	r.mu.Lock()
	defer r.mu.Unlock()
	counter, ok := r.contributionVerifies[key]
	if !ok {
		var val int64
		counter = &val
		r.contributionVerifies[key] = counter
	}
	atomic.AddInt64(counter, 1)
}

// SetDBActiveConnections sets the current number of active database pool connections.
func (r *Registry) SetDBActiveConnections(count int) {
	atomic.StoreInt64(&r.dbConnectionsActive, int64(count))
}

// Handler returns an http.Handler that renders Prometheus text format metrics.
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		var sb strings.Builder

		// http_requests_total
		sb.WriteString("# HELP http_requests_total Total number of HTTP requests processed\n")
		sb.WriteString("# TYPE http_requests_total counter\n")
		r.mu.RLock()
		for labels, count := range r.httpRequestsTotal {
			sb.WriteString(fmt.Sprintf("http_requests_total{%s} %d\n", labels, atomic.LoadInt64(count)))
		}

		// http_request_duration_seconds
		sb.WriteString("# HELP http_request_duration_seconds Duration of HTTP requests in seconds\n")
		sb.WriteString("# TYPE http_request_duration_seconds summary\n")
		for labels, count := range r.httpDurationCount {
			sum := *r.httpDurationSum[labels]
			c := atomic.LoadInt64(count)
			sb.WriteString(fmt.Sprintf("http_request_duration_seconds_sum{%s} %f\n", labels, sum))
			sb.WriteString(fmt.Sprintf("http_request_duration_seconds_count{%s} %d\n", labels, c))
		}

		// github_rate_limit_remaining
		sb.WriteString("# HELP github_rate_limit_remaining Remaining GitHub API rate limit budget\n")
		sb.WriteString("# TYPE github_rate_limit_remaining gauge\n")
		sb.WriteString(fmt.Sprintf("github_rate_limit_remaining %d\n", atomic.LoadInt64(&r.githubRateLimitRemaining)))

		// github_graphql_requests_total
		sb.WriteString("# HELP github_graphql_requests_total Total GitHub GraphQL requests made\n")
		sb.WriteString("# TYPE github_graphql_requests_total counter\n")
		for labels, count := range r.githubGraphQLRequests {
			sb.WriteString(fmt.Sprintf("github_graphql_requests_total{%s} %d\n", labels, atomic.LoadInt64(count)))
		}

		// sync_jobs_total
		sb.WriteString("# HELP sync_jobs_total Total repository issue sync jobs executed\n")
		sb.WriteString("# TYPE sync_jobs_total counter\n")
		for labels, count := range r.syncJobsTotal {
			sb.WriteString(fmt.Sprintf("sync_jobs_total{%s} %d\n", labels, atomic.LoadInt64(count)))
		}

		// worker_queue_depth
		sb.WriteString("# HELP worker_queue_depth Number of queued items in worker pools\n")
		sb.WriteString("# TYPE worker_queue_depth gauge\n")
		for pool, gauge := range r.workerQueueDepth {
			sb.WriteString(fmt.Sprintf("worker_queue_depth{pool=\"%s\"} %d\n", pool, atomic.LoadInt64(gauge)))
		}

		// contribution_verify_total
		sb.WriteString("# HELP contribution_verify_total Total PR verification attempts\n")
		sb.WriteString("# TYPE contribution_verify_total counter\n")
		for labels, count := range r.contributionVerifies {
			sb.WriteString(fmt.Sprintf("contribution_verify_total{%s} %d\n", labels, atomic.LoadInt64(count)))
		}

		// db_connections_active
		sb.WriteString("# HELP db_connections_active Number of active database connections in pool\n")
		sb.WriteString("# TYPE db_connections_active gauge\n")
		sb.WriteString(fmt.Sprintf("db_connections_active %d\n", atomic.LoadInt64(&r.dbConnectionsActive)))
		r.mu.RUnlock()

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sb.String()))
	})
}

// sanitizePath normalizes path parameters to prevent high cardinality metric labels.
func sanitizePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		// If part looks like a numeric ID, replace with :id
		if isNumeric(part) {
			parts[i] = ":id"
		}
	}
	return "/" + strings.Join(parts, "/")
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
