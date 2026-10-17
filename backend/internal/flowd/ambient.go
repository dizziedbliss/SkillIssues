package flowd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dizziedbliss/skillissues/internal/flow"
	"github.com/dizziedbliss/skillissues/internal/httpx"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// serviceLabel is the label cAdvisor puts the compose service name in
// (cadvisor --whitelisted_container_labels=com.docker.compose.service).
const serviceLabel = "container_label_com_docker_compose_service"

// ambientLink is a link nothing in the code sees, drawn from one container's
// network rate. Each of these containers talks on that one link (or nearly
// so), which is what makes its own byte rate a fair stand-in for the link's.
type ambientLink struct {
	from, to  flow.Node
	container flow.Node
	transmit  bool // the container's transmit rate; otherwise its receive rate
}

var ambientLinks = []ambientLink{
	{flow.Frontend, flow.Caddy, flow.Frontend, true},
	{flow.Prometheus, flow.Grafana, flow.Grafana, false},
	{flow.PostgresExporter, flow.Prometheus, flow.PostgresExporter, true},
	{flow.Postgres, flow.PostgresExporter, flow.PostgresExporter, false},
	{flow.RedisExporter, flow.Prometheus, flow.RedisExporter, true},
	{flow.Redis, flow.RedisExporter, flow.RedisExporter, false},
	{flow.Cadvisor, flow.Prometheus, flow.Cadvisor, true},
}

// Ambient reads per-container network rates from Prometheus.
type Ambient struct {
	baseURL string
	http    httpx.Doer
	live    func() *settings.Snapshot
	failing atomic.Bool
}

// NewAmbient returns a reader for the Prometheus at baseURL.
func NewAmbient(baseURL string, client httpx.Doer, live func() *settings.Snapshot) *Ambient {
	return &Ambient{baseURL: strings.TrimRight(baseURL, "/"), http: client, live: live}
}

// Run sends ambient rates to r every flow.ambient_refresh_seconds until ctx
// is done. A failed query is counted and logged when failures start and stop;
// the links keep their last rate on the viewer's side until the next success.
func (a *Ambient) Run(ctx context.Context, r *Relay) {
	for {
		snap := a.live()
		if rates, err := a.rates(ctx, snap.Int(settings.FlowAmbientRateWindowSeconds)); err != nil {
			if ctx.Err() != nil {
				return
			}
			metrics.FlowAmbientErrors.Inc()
			if a.failing.CompareAndSwap(false, true) {
				slog.Warn("flowd: ambient rates unavailable from prometheus", "error", err)
			}
		} else {
			if a.failing.CompareAndSwap(true, false) {
				slog.Info("flowd: ambient rates available again")
			}
			r.sendAmbient(rates)
		}

		t := time.NewTimer(snap.Seconds(settings.FlowAmbientRefreshSeconds))
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

func (a *Ambient) rates(ctx context.Context, windowSeconds int) (map[link]float64, error) {
	tx, err := a.query(ctx, "transmit", windowSeconds)
	if err != nil {
		return nil, err
	}
	rx, err := a.query(ctx, "receive", windowSeconds)
	if err != nil {
		return nil, err
	}
	out := map[link]float64{}
	for _, l := range ambientLinks {
		src := rx
		if l.transmit {
			src = tx
		}
		if v, ok := src[string(l.container)]; ok {
			out[link{l.from, l.to}] = v
		}
	}
	return out, nil
}

// query returns bytes per second by compose service for one direction.
func (a *Ambient) query(ctx context.Context, direction string, windowSeconds int) (map[string]float64, error) {
	promql := fmt.Sprintf(`sum by (%s) (rate(container_network_%s_bytes_total{%s!=""}[%ds]))`,
		serviceLabel, direction, serviceLabel, windowSeconds)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.baseURL+"/api/v1/query?query="+url.QueryEscape(promql), nil)
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("query %s rates: %w", direction, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query %s rates: HTTP %d", direction, resp.StatusCode)
	}
	var body struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  [2]any            `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode %s rates: %w", direction, err)
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("query %s rates: status %q", direction, body.Status)
	}
	out := make(map[string]float64, len(body.Data.Result))
	for _, r := range body.Data.Result {
		s, ok := r.Value[1].(string)
		if !ok {
			return nil, fmt.Errorf("query %s rates: value is %T, not a string", direction, r.Value[1])
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("query %s rates: %w", direction, err)
		}
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			// A container with too few samples in the window; not a rate yet.
			continue
		}
		out[r.Metric[serviceLabel]] = v
	}
	return out, nil
}
