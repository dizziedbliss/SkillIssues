package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/flow"
)

// tapChain mirrors the public chain's order around the tap: Logger records the
// pattern the mux matched, FlowTap emits it, and the mux sits beneath.
func tapChain(t *testing.T) (http.Handler, func() []byte) {
	t.Helper()
	var published []byte
	p := flow.NewPublisherFunc(flow.API, func(_ context.Context, msg []byte) (int64, error) {
		published = append(published, msg...)
		return 1, nil
	}, func() flow.Limits {
		return flow.Limits{Enabled: true, BufferSize: 64, PublishTimeout: time.Second, MaxLabelChars: 80}
	})
	flow.Install(p)
	t.Cleanup(func() { flow.Install(nil) })

	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"login":"octocat","id":583231}`))
	}
	mux.HandleFunc("GET /saved/{id}", ok)
	mux.HandleFunc("GET /issues/{id}", ok)
	mux.HandleFunc("GET /auth/github/callback", ok)
	h := Logger(FlowTap(flow.Users)(RecordRoute(mux)))
	return h, func() []byte {
		p.Flush(context.Background())
		out := published
		published = nil
		return out
	}
}

func decodeHops(t *testing.T, raw []byte) []flow.Event {
	t.Helper()
	var hops []flow.Event
	if err := json.Unmarshal(raw, &hops); err != nil {
		t.Fatalf("published bytes are not one JSON batch: %v\n%s", err, raw)
	}
	return hops
}

// withoutTimes is the published batch with every event's timestamp taken
// out: a leak check looks for an id like 777 in what an event says, and a
// nanosecond timestamp ("…08.727774171Z") can contain it by chance.
func withoutTimes(t *testing.T, raw []byte) string {
	t.Helper()
	var events []map[string]any
	if err := json.Unmarshal(raw, &events); err != nil {
		t.Fatalf("published bytes are not one JSON batch: %v\n%s", err, raw)
	}
	for _, e := range events {
		delete(e, "t")
	}
	out, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestFlowTapRedactsASignedInRequest(t *testing.T) {
	h, flush := tapChain(t)
	req := httptest.NewRequest(http.MethodGet, "/saved/48151623?q=private", nil)
	req.RemoteAddr = "203.0.113.9:5555"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	req.AddCookie(&http.Cookie{Name: "skillissues_session", Value: "sess-secret-value"})
	h.ServeHTTP(httptest.NewRecorder(), req)

	raw := flush()
	for _, leak := range []string{"48151623", "private", "/saved", "203.0.113", "198.51.100", "sess-secret-value", "octocat", "583231"} {
		if strings.Contains(withoutTimes(t, raw), leak) {
			t.Errorf("published flow events contain %q:\n%s", leak, raw)
		}
	}
	hops := decodeHops(t, raw)
	if len(hops) != 2 {
		t.Fatalf("got %d hops, want users→caddy and caddy→go-api", len(hops))
	}
	for _, e := range hops {
		if !e.Redacted {
			t.Errorf("signed-in hop not redacted: %+v", e)
		}
	}
}

func TestFlowTapRedactsSignIn(t *testing.T) {
	h, flush := tapChain(t)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=c0de&state=s7ate", nil))
	raw := flush()
	if strings.Contains(string(raw), "c0de") || strings.Contains(string(raw), "/auth") {
		t.Errorf("sign-in leaked into the stream:\n%s", raw)
	}
	for _, e := range decodeHops(t, raw) {
		if !e.Redacted {
			t.Errorf("sign-in hop not redacted: %+v", e)
		}
	}
}

func TestFlowTapPublishesAnAnonymousReadByPattern(t *testing.T) {
	h, flush := tapChain(t)
	req := httptest.NewRequest(http.MethodGet, "/issues/777?search=foo", nil)
	req.RemoteAddr = "203.0.113.9:5555"
	h.ServeHTTP(httptest.NewRecorder(), req)

	raw := flush()
	for _, leak := range []string{"777", "foo", "203.0.113"} {
		if strings.Contains(withoutTimes(t, raw), leak) {
			t.Errorf("published flow events contain %q:\n%s", leak, raw)
		}
	}
	hops := decodeHops(t, raw)
	if len(hops) != 2 {
		t.Fatalf("got %d hops, want 2", len(hops))
	}
	if hops[0].From != flow.Users || hops[0].To != flow.Caddy || hops[1].From != flow.Caddy || hops[1].To != flow.API {
		t.Errorf("hops = %+v, want users→caddy→go-api", hops)
	}
	for _, e := range hops {
		if e.Redacted || e.Label != "GET /issues/{id}" || !strings.HasPrefix(e.Payload, "200 · ") {
			t.Errorf("anonymous hop = %+v, want the route pattern and status in clear", e)
		}
	}
}

func TestFlowTapNamesARateLimitedRequest(t *testing.T) {
	_, flush := tapChain(t)
	limited := Logger(FlowTap(flow.Users)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})))
	limited.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/issues/1", nil))
	for _, e := range decodeHops(t, flush()) {
		if e.Label != "rate limited" || !strings.HasPrefix(e.Payload, "429") {
			t.Errorf("rate-limited hop = %+v", e)
		}
	}
}
