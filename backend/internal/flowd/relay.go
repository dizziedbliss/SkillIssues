// Package flowd relays the flow events go-api and ingestd publish to the
// viewers of the public live visualizer.
//
// Every event counts towards its link's density, sent once per
// flow.density_interval_ms. At most flow.max_events_per_second of them are
// also relayed with their label, so a busy link reads as "×312/s" and a
// handful of particles rather than an unreadable log. flowd holds no
// credentials beyond reading its own settings, and re-encodes every event
// through flow.Event, so a field a publisher should not have sent never
// reaches a viewer.
package flowd

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/dizziedbliss/skillissues/internal/flow"
	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// hopKinds are the kinds a publisher may send. The rest are flowd's own.
var hopKinds = map[flow.Kind]bool{
	flow.KindHTTP: true, flow.KindSQL: true, flow.KindRedis: true,
	flow.KindGitHub: true, flow.KindCrawl: true, flow.KindDiscovery: true,
}

type link struct{ from, to flow.Node }

// line is one encoded event, framed per viewer at write time.
type line []byte

type viewer struct{ lines chan line }

// Relay fans events out to viewers.
type Relay struct {
	live func() *settings.Snapshot
	now  func() time.Time

	mu       sync.Mutex
	viewers  map[*viewer]struct{}
	counts   map[link]int64
	received int64
	tokens   float64
	refilled time.Time
}

// New returns a relay reading its limits from live.
func New(live func() *settings.Snapshot) *Relay {
	return &Relay{
		live:    live,
		now:     time.Now,
		viewers: map[*viewer]struct{}{},
		counts:  map[link]int64{},
	}
}

// Ingest handles one published batch: a JSON array of events.
func (r *Relay) Ingest(msg []byte) {
	var batch []flow.Event
	if err := json.Unmarshal(msg, &batch); err != nil {
		metrics.FlowEventsDropped.WithLabelValues("decode").Inc()
		slog.Warn("flowd: undecodable batch", "error", err, "bytes", len(msg))
		return
	}
	rate := float64(r.live().Int(settings.FlowMaxEventsPerSecond))

	r.mu.Lock()
	defer r.mu.Unlock()
	r.received += int64(len(batch))
	for _, e := range batch {
		if !hopKinds[e.Kind] || !e.From.Valid() || !e.To.Valid() || e.From == e.To || e.T.IsZero() {
			metrics.FlowEventsDropped.WithLabelValues("invalid").Inc()
			continue
		}
		r.counts[link{e.From, e.To}]++
		if r.take(rate) && len(r.viewers) > 0 {
			r.broadcastLocked(e)
			metrics.FlowEventsRelayed.Inc()
		}
	}
}

// take spends one token from a bucket holding at most one second's worth,
// refilled at rate per second.
func (r *Relay) take(rate float64) bool {
	now := r.now()
	if r.refilled.IsZero() {
		r.tokens = rate
	} else {
		r.tokens += now.Sub(r.refilled).Seconds() * rate
	}
	r.refilled = now
	r.tokens = min(r.tokens, rate)
	if r.tokens < 1 {
		return false
	}
	r.tokens--
	return true
}

// flushDensity sends, for the window just ended, one density line per link
// that carried events, the events received from Redis, and the viewer count.
func (r *Relay) flushDensity(window time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	counts := r.counts
	r.counts = make(map[link]int64, len(counts))
	received := r.received
	r.received = 0

	t := r.now().UTC()
	ms := float64(window.Milliseconds())
	for l, n := range counts {
		r.broadcastLocked(flow.Event{T: t, From: l.from, To: l.to, Kind: flow.KindDensity, N: n, Ms: ms})
	}
	if received > 0 {
		r.broadcastLocked(flow.Event{T: t, From: flow.Redis, To: flow.Flowd, Kind: flow.KindDensity, N: received, Ms: ms})
	}
	r.broadcastLocked(flow.Event{T: t, From: flow.Flowd, To: flow.Users, Kind: flow.KindViewers, N: int64(len(r.viewers))})
}

// sendAmbient sends one ambient line per link with a known rate.
func (r *Relay) sendAmbient(rates map[link]float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.now().UTC()
	for l, rate := range rates {
		r.broadcastLocked(flow.Event{T: t, From: l.from, To: l.to, Kind: flow.KindAmbient, N: int64(rate + 0.5)})
	}
}

func (r *Relay) broadcastLocked(e flow.Event) {
	if len(r.viewers) == 0 {
		return
	}
	b, err := json.Marshal(e)
	if err != nil {
		metrics.FlowEventsDropped.WithLabelValues("encode").Inc()
		slog.Error("flowd: encode event", "error", err)
		return
	}
	for v := range r.viewers {
		select {
		case v.lines <- b:
		default:
			metrics.FlowEventsDropped.WithLabelValues("slow_viewer").Inc()
		}
	}
}

// join adds a viewer, or returns nil when flow.max_viewers are connected.
func (r *Relay) join() *viewer {
	snap := r.live()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.viewers) >= snap.Int(settings.FlowMaxViewers) {
		return nil
	}
	v := &viewer{lines: make(chan line, snap.Int(settings.FlowBufferSize))}
	r.viewers[v] = struct{}{}
	metrics.FlowViewers.Set(float64(len(r.viewers)))
	return v
}

func (r *Relay) leave(v *viewer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.viewers, v)
	metrics.FlowViewers.Set(float64(len(r.viewers)))
}
