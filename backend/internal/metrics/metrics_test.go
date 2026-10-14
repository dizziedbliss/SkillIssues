package metrics

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestInstrumentedTransportCountsByOperationAndCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()

	client := &http.Client{Transport: InstrumentedTransport{}}
	ctx := WithOperation(context.Background(), "TestOp")

	c := GitHubRequests.WithLabelValues("graphql", "TestOp", "418")
	before := testutil.ToFloat64(c)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/graphql", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := testutil.ToFloat64(c) - before; got != 1 {
		t.Errorf("graphql 418 count = %v, want 1", got)
	}

	failed := GitHubRequests.WithLabelValues("rest", "TestOp", "error")
	before = testutil.ToFloat64(failed)
	broken := &http.Client{Transport: InstrumentedTransport{Base: failingTransport{}}}
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/x/y", nil)
	if _, err := broken.Do(req); err == nil {
		t.Fatal("expected a transport error")
	}
	if got := testutil.ToFloat64(failed) - before; got != 1 {
		t.Errorf("transport error count = %v, want 1", got)
	}
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func TestHandlerExposesHistogramBuckets(t *testing.T) {
	RegisterAPI()
	HTTPDuration.WithLabelValues("GET", "GET /health").Observe(0.03)

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body, _ := io.ReadAll(rec.Body)
	out := string(body)

	for _, want := range []string{
		`http_request_duration_seconds_bucket{method="GET",route="GET /health",le="0.05"} 1`,
		`http_request_duration_seconds_bucket{method="GET",route="GET /health",le="0.025"} 0`,
		`http_request_duration_seconds_count{method="GET",route="GET /health"} 1`,
		"go_goroutines",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("exposition missing %q", want)
		}
	}
	if strings.Contains(out, "ingest_paused") {
		t.Error("API registry must not export ingestd-only gauges")
	}
}
