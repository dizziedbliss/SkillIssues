package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

type operationKey struct{}

// WithOperation tags ctx so InstrumentedTransport can label the outbound
// request. Unlabelled requests are recorded as operation="unknown".
func WithOperation(ctx context.Context, op string) context.Context {
	return context.WithValue(ctx, operationKey{}, op)
}

// InstrumentedTransport records GitHub request counts and latency. It never
// inspects headers or bodies, so credentials cannot reach a label.
type InstrumentedTransport struct {
	Base http.RoundTripper
}

func (t InstrumentedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	op, ok := req.Context().Value(operationKey{}).(string)
	if !ok {
		op = "unknown"
	}
	api := "rest"
	if req.URL.Path == "/graphql" {
		api = "graphql"
	}

	start := time.Now()
	resp, err := base.RoundTrip(req)
	GitHubDuration.WithLabelValues(api, op).Observe(time.Since(start).Seconds())

	code := "error"
	if err == nil {
		code = strconv.Itoa(resp.StatusCode)
	}
	GitHubRequests.WithLabelValues(api, op, code).Inc()
	return resp, err
}
