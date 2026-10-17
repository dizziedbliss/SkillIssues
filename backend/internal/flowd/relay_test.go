package flowd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/flow"
	"github.com/dizziedbliss/skillissues/internal/settings"
)

func testRelay(snap *settings.Snapshot) *Relay {
	r := New(func() *settings.Snapshot { return snap })
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return at }
	return r
}

func batch(t *testing.T, events ...flow.Event) []byte {
	t.Helper()
	b, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func hop(from, to flow.Node) flow.Event {
	return flow.Event{T: time.Now().UTC(), From: from, To: to, Kind: flow.KindSQL, Label: "SELECT issues", N: 1}
}

func drain(v *viewer) []flow.Event {
	var out []flow.Event
	for {
		select {
		case l := <-v.lines:
			var e flow.Event
			if err := json.Unmarshal(l, &e); err != nil {
				panic(err)
			}
			out = append(out, e)
		default:
			return out
		}
	}
}

func TestLabelBudgetAndDensity(t *testing.T) {
	r := testRelay(settings.Defaults().With(settings.FlowMaxEventsPerSecond, 3))
	v := r.join()
	events := make([]flow.Event, 10)
	for i := range events {
		events[i] = hop(flow.API, flow.Postgres)
	}
	r.Ingest(batch(t, events...))

	if got := drain(v); len(got) != 3 {
		t.Fatalf("relayed %d labelled events, want the budget of 3", len(got))
	}

	r.flushDensity(time.Second)
	byLink := map[string]flow.Event{}
	for _, e := range drain(v) {
		byLink[string(e.Kind)+":"+string(e.From)+"→"+string(e.To)] = e
	}
	if d := byLink["density:go-api→postgres"]; d.N != 10 || d.Ms != 1000 {
		t.Errorf("density = %+v, want all 10 events over 1000 ms", d)
	}
	if d := byLink["density:redis→flowd"]; d.N != 10 {
		t.Errorf("redis→flowd density = %+v, want 10", d)
	}
	if v := byLink["viewers:flowd→users"]; v.N != 1 {
		t.Errorf("viewers = %+v, want 1", v)
	}

	// The window resets.
	r.flushDensity(time.Second)
	for _, e := range drain(v) {
		if e.Kind == flow.KindDensity {
			t.Errorf("density line after an empty window: %+v", e)
		}
	}
}

func TestBudgetRefillsOverTime(t *testing.T) {
	r := testRelay(settings.Defaults().With(settings.FlowMaxEventsPerSecond, 2))
	at := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return at }
	v := r.join()
	r.Ingest(batch(t, hop(flow.API, flow.Redis), hop(flow.API, flow.Redis), hop(flow.API, flow.Redis)))
	if got := len(drain(v)); got != 2 {
		t.Fatalf("relayed %d, want 2", got)
	}
	at = at.Add(500 * time.Millisecond)
	r.Ingest(batch(t, hop(flow.API, flow.Redis), hop(flow.API, flow.Redis)))
	if got := len(drain(v)); got != 1 {
		t.Fatalf("after half a second relayed %d, want 1", got)
	}
}

func TestInvalidEventsAreDropped(t *testing.T) {
	r := testRelay(settings.Defaults())
	v := r.join()
	bad := []flow.Event{
		{T: time.Now(), From: "evil", To: flow.Postgres, Kind: flow.KindSQL},
		{T: time.Now(), From: flow.API, To: flow.API, Kind: flow.KindSQL},
		{T: time.Now(), From: flow.API, To: flow.Postgres, Kind: flow.KindDensity, N: 1e9},
		{From: flow.API, To: flow.Postgres, Kind: flow.KindSQL},
	}
	r.Ingest(batch(t, bad...))
	r.Ingest([]byte("not json"))
	if got := drain(v); len(got) != 0 {
		t.Errorf("relayed invalid events: %+v", got)
	}
	r.flushDensity(time.Second)
	for _, e := range drain(v) {
		if e.Kind == flow.KindDensity && e.From != flow.Redis {
			t.Errorf("invalid event counted towards density: %+v", e)
		}
	}
}

// flowd re-encodes events through flow.Event, so a field a publisher should
// never have sent does not reach a viewer.
func TestUnknownFieldsNeverReachViewers(t *testing.T) {
	r := testRelay(settings.Defaults())
	v := r.join()
	r.Ingest([]byte(`[{"t":"2026-10-18T12:00:00Z","from":"go-api","to":"postgres","kind":"sql","label":"SELECT issues","ms":1,"n":1,"ip":"203.0.113.9","user_id":42}]`))
	select {
	case l := <-v.lines:
		if strings.Contains(string(l), "203.0.113.9") || strings.Contains(string(l), "user_id") {
			t.Errorf("relayed line kept an unknown field: %s", l)
		}
	default:
		t.Fatal("nothing relayed")
	}
}

func TestSlowViewerDropsInsteadOfBlocking(t *testing.T) {
	r := testRelay(settings.Defaults().With(settings.FlowBufferSize, 64).With(settings.FlowMaxEventsPerSecond, 1000))
	v := r.join()
	events := make([]flow.Event, 200)
	for i := range events {
		events[i] = hop(flow.Ingestd, flow.Postgres)
	}
	done := make(chan struct{})
	go func() { r.Ingest(batch(t, events...)); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Ingest blocked on a viewer that is not reading")
	}
	if got := len(drain(v)); got != 64 {
		t.Errorf("viewer buffered %d lines, want its buffer of 64", got)
	}
}

func TestStream(t *testing.T) {
	r := testRelay(settings.Defaults().With(settings.FlowMaxViewers, 1))
	srv := httptest.NewServer(r.Stream([]string{"https://flow.example.test"}))
	defer srv.Close()

	t.Run("disallowed origin", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/stream", nil)
		req.Header.Set("Origin", "https://evil.example")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want 403", resp.StatusCode)
		}
	})

	t.Run("sse, then the viewer cap", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/stream", nil)
		req.Header.Set("Origin", "https://flow.example.test")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" ||
			resp.Header.Get("Access-Control-Allow-Origin") != "https://flow.example.test" {
			t.Fatalf("status %d, headers %v", resp.StatusCode, resp.Header)
		}
		rd := bufio.NewReader(resp.Body)
		if l, _ := rd.ReadString('\n'); l != ": heartbeat\n" {
			t.Fatalf("first line = %q, want the opening comment", l)
		}
		_, _ = rd.ReadString('\n')

		// A second viewer is over the cap of 1.
		over, err := http.Get(srv.URL + "/stream")
		if err != nil {
			t.Fatal(err)
		}
		over.Body.Close()
		if over.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("viewer over the cap got %d, want 503", over.StatusCode)
		}

		r.Ingest(batch(t, hop(flow.API, flow.Postgres)))
		l, err := rd.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		var e flow.Event
		if !strings.HasPrefix(l, "data: ") || json.Unmarshal([]byte(strings.TrimPrefix(l, "data: ")), &e) != nil || e.Label != "SELECT issues" {
			t.Errorf("event line = %q", l)
		}
	})

	t.Run("ndjson", func(t *testing.T) {
		// The previous viewer has gone; wait for its slot to free.
		deadline := time.Now().Add(2 * time.Second)
		for {
			r.mu.Lock()
			n := len(r.viewers)
			r.mu.Unlock()
			if n == 0 || time.Now().After(deadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/stream?format=ndjson", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		rd := bufio.NewReader(resp.Body)
		l, _ := rd.ReadString('\n')
		var hb flow.Event
		if err := json.Unmarshal([]byte(l), &hb); err != nil || hb.Kind != flow.KindHeartbeat {
			t.Fatalf("first ndjson line = %q (%v), want a heartbeat object", l, err)
		}
		r.Ingest(batch(t, hop(flow.Ingestd, flow.Postgres)))
		l, _ = rd.ReadString('\n')
		var e flow.Event
		if err := json.Unmarshal([]byte(l), &e); err != nil || e.From != flow.Ingestd {
			t.Errorf("ndjson event line = %q (%v)", l, err)
		}
	})
}

func TestAmbientRates(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		q := req.URL.Query().Get("query")
		if !strings.Contains(q, "[120s]") {
			t.Errorf("query %q does not use the configured window", q)
		}
		v := "100"
		if strings.Contains(q, "receive") {
			v = "40"
		}
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"%[1]s":"frontend"},"value":[1,"%[2]s"]},
			{"metric":{"%[1]s":"grafana"},"value":[1,"%[2]s"]},
			{"metric":{"%[1]s":"cadvisor"},"value":[1,"NaN"]}]}}`, serviceLabel, v)
	}))
	defer prom.Close()

	a := NewAmbient(prom.URL, prom.Client(), func() *settings.Snapshot { return settings.Defaults() })
	rates, err := a.rates(context.Background(), 120)
	if err != nil {
		t.Fatal(err)
	}
	if got := rates[link{flow.Frontend, flow.Caddy}]; got != 100 {
		t.Errorf("frontend→caddy = %v, want frontend's transmit rate 100", got)
	}
	if got := rates[link{flow.Prometheus, flow.Grafana}]; got != 40 {
		t.Errorf("prometheus→grafana = %v, want grafana's receive rate 40", got)
	}
	if _, ok := rates[link{flow.Cadvisor, flow.Prometheus}]; ok {
		t.Error("a NaN rate was reported")
	}
}
