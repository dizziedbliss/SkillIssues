package flow

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/metrics"
)

// Transport emits <self>→github or <self>→ingestd for each outbound call,
// labelled with the metrics.WithOperation name (or the HTTP method when a
// call has none) and its status. It never reads the URL path, headers or
// body. A call to any other host is not emitted.
//
// A call made under a request inherits its redaction: go-api's calls to
// GitHub are sign-in and the device-token check, which run under /auth/ and
// so are published as symbols.
type Transport struct {
	Base http.RoundTripper
}

func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	to := hostNode(req.URL.Hostname())
	if to == "" || !Active() {
		return base.RoundTrip(req)
	}

	start := time.Now()
	resp, err := base.RoundTrip(req)
	status := "error"
	if err == nil {
		status = strconv.Itoa(resp.StatusCode)
	}
	label := metrics.Operation(req.Context())
	if label == "" {
		label = req.Method
	}
	kind := KindGitHub
	if to != GitHub {
		kind = KindHTTP
	}
	Emit(req.Context(), Event{
		From: Self(), To: to, Kind: kind,
		Label: label, Payload: status,
		Ms: float64(time.Since(start).Microseconds()) / 1000,
	})
	return resp, err
}

func hostNode(host string) Node {
	host = strings.ToLower(host)
	switch {
	case host == "github.com" || host == "api.github.com" || strings.HasSuffix(host, ".github.com"):
		return GitHub
	case host == string(Ingestd):
		return Ingestd
	}
	return ""
}
