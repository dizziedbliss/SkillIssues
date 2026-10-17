package flowd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dizziedbliss/skillissues/internal/flow"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

// Stream serves GET /stream: Server-Sent Events by default, one JSON object
// per line with ?format=ndjson (for curl -N). A browser is answered only from
// an allowed origin; a client that sends no Origin (curl) always is. Past
// flow.max_viewers, a new viewer gets 503.
func (r *Relay) Stream(allowedOrigins []string) http.Handler {
	allowed := map[string]bool{}
	for _, o := range allowedOrigins {
		allowed[strings.TrimRight(strings.ToLower(strings.TrimSpace(o)), "/")] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if origin := req.Header.Get("Origin"); origin != "" {
			if !allowed[strings.TrimRight(strings.ToLower(origin), "/")] {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		ndjson := req.URL.Query().Get("format") == "ndjson"
		v := r.join()
		if v == nil {
			w.Header().Set("Retry-After", "30")
			http.Error(w, "too many viewers", http.StatusServiceUnavailable)
			return
		}
		defer r.leave(v)

		if ndjson {
			w.Header().Set("Content-Type", "application/x-ndjson")
		} else {
			w.Header().Set("Content-Type", "text/event-stream")
		}
		w.Header().Set("Cache-Control", "no-cache, no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)

		r.serve(req.Context(), w, v, ndjson)
	})
}

// serve writes lines to one viewer until it goes away. A write that cannot
// finish within two heartbeats means a stalled client, which is dropped.
func (r *Relay) serve(ctx context.Context, w http.ResponseWriter, v *viewer, ndjson bool) {
	rc := http.NewResponseController(w)
	heartbeat := r.live().Seconds(settings.FlowHeartbeatSeconds)
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()

	write := func(b []byte) bool {
		if err := rc.SetWriteDeadline(time.Now().Add(2 * heartbeat)); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return false
		}
		if _, err := w.Write(b); err != nil {
			return false
		}
		return rc.Flush() == nil
	}

	// Open with a comment, so the client sees the stream is up before the
	// first event.
	if !write(frameComment(ndjson, r.now())) {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case l := <-v.lines:
			if !write(frame(ndjson, l)) {
				return
			}
		case <-ticker.C:
			if !write(frameComment(ndjson, r.now())) {
				return
			}
			if next := r.live().Seconds(settings.FlowHeartbeatSeconds); next != heartbeat {
				heartbeat = next
				ticker.Reset(heartbeat)
			}
		}
	}
}

func frame(ndjson bool, l line) []byte {
	if ndjson {
		return append(append(make([]byte, 0, len(l)+1), l...), '\n')
	}
	b := make([]byte, 0, len(l)+8)
	b = append(b, "data: "...)
	b = append(b, l...)
	return append(b, '\n', '\n')
}

// frameComment is the keep-alive: an SSE comment, which EventSource ignores,
// or a heartbeat line on ndjson, which has no comments.
func frameComment(ndjson bool, now time.Time) []byte {
	if !ndjson {
		return []byte(": heartbeat\n\n")
	}
	return fmt.Appendf(nil, `{"t":%q,"from":%q,"to":%q,"kind":%q,"label":"","ms":0,"n":0}`+"\n",
		now.UTC().Format(time.RFC3339Nano), flow.Flowd, flow.Users, flow.KindHeartbeat)
}

// RunDensity sends link densities every flow.density_interval_ms until ctx
// is done.
func (r *Relay) RunDensity(ctx context.Context) {
	for {
		window := r.live().Millis(settings.FlowDensityIntervalMs)
		t := time.NewTimer(window)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			r.flushDensity(window)
		}
	}
}
