package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/dizziedbliss/skillissues/internal/metrics"
)

func TestLoggerLabelsByRoutePatternThroughClonedRequests(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /issues/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Stand-in for OptionalAuth/CORS: middleware between Logger and the mux
	// that clones the request, so Logger never sees r.Pattern directly.
	cloning := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey("x"), 1)))
		})
	}
	h := Logger(cloning(RecordRoute(mux)))

	matched := metrics.HTTPRequests.WithLabelValues("GET", "GET /issues/{id}", "200")
	unmatched := metrics.HTTPRequests.WithLabelValues("GET", "unmatched", "404")
	beforeMatched, beforeUnmatched := testutil.ToFloat64(matched), testutil.ToFloat64(unmatched)

	for _, path := range []string{"/issues/1", "/issues/987654", "/wp-admin/setup.php"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	if got := testutil.ToFloat64(matched) - beforeMatched; got != 2 {
		t.Errorf("matched route count = %v, want 2", got)
	}
	if got := testutil.ToFloat64(unmatched) - beforeUnmatched; got != 1 {
		t.Errorf("unmatched route count = %v, want 1 (raw paths must not become labels)", got)
	}
}
