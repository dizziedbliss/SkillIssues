package flow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dizziedbliss/skillissues/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func testLimits() Limits {
	return Limits{Enabled: true, BufferSize: 64, PublishTimeout: time.Second, MaxLabelChars: 80}
}

// capture installs a publisher that records every batch it is asked to
// publish, and returns a function that flushes and decodes them.
func capture(t *testing.T, lim func() Limits) (*Publisher, func() []Event) {
	t.Helper()
	var got []Event
	p := NewPublisherFunc(API, func(_ context.Context, msg []byte) (int64, error) {
		var batch []Event
		if err := json.Unmarshal(msg, &batch); err != nil {
			t.Fatalf("published batch is not a JSON array of events: %v", err)
		}
		got = append(got, batch...)
		return 1, nil
	}, lim)
	Install(p)
	t.Cleanup(func() { Install(nil) })
	return p, func() []Event {
		p.Flush(context.Background())
		out := got
		got = nil
		return out
	}
}

func onlyGlyphs(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune(string(glyphAlphabet), r) {
			return false
		}
	}
	return true
}

func TestEmitPublishesPublicTextAsIs(t *testing.T) {
	_, flush := capture(t, testLimits)
	Emit(context.Background(), Event{From: GitHub, To: Ingestd, Kind: KindCrawl, Label: "golang/go +50", Payload: "spec: clarify"})
	got := flush()
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	e := got[0]
	if e.Label != "golang/go +50" || e.Payload != "spec: clarify" || e.Redacted {
		t.Errorf("anonymous event changed: %+v", e)
	}
	if e.T.IsZero() || e.N != 1 {
		t.Errorf("Emit did not fill t and n: %+v", e)
	}
}

func TestEmitRedactsUserEventsInFull(t *testing.T) {
	_, flush := capture(t, testLimits)
	ctx := MarkUser(WithScope(context.Background()))
	for range 20 {
		Emit(ctx, Event{From: Users, To: Caddy, Kind: KindHTTP, Label: "GET /saved", Payload: "200 · 1.2 KB"})
	}
	got := flush()
	if len(got) != 20 {
		t.Fatalf("got %d events, want 20", len(got))
	}
	labels := map[string]bool{}
	for _, e := range got {
		if !e.Redacted || !onlyGlyphs(e.Label) || !onlyGlyphs(e.Payload) {
			t.Fatalf("user event not redacted to symbols: %+v", e)
		}
		labels[e.Label] = true
	}
	// Fresh symbols per event, so two events cannot be matched up.
	if len(labels) < 15 {
		t.Errorf("identical input gave only %d distinct redactions out of 20", len(labels))
	}
}

func TestMarkUserReachesEventsEmittedAboveIt(t *testing.T) {
	_, flush := capture(t, testLimits)
	outer := WithScope(context.Background())
	// A middleware deeper in the chain resolves the session on a derived
	// context; the tap above emits on the outer one after the request returns.
	inner := context.WithValue(outer, struct{}{}, 1)
	MarkUser(inner)
	Emit(outer, Event{From: Users, To: Caddy, Kind: KindHTTP, Label: "GET /me"})
	if got := flush(); len(got) != 1 || !got[0].Redacted {
		t.Fatalf("event emitted above MarkUser was not redacted: %+v", got)
	}
}

func TestUserLinkedWithoutScope(t *testing.T) {
	_, flush := capture(t, testLimits)
	Emit(UserLinked(context.Background()), Event{From: Ingestd, To: GitHub, Kind: KindGitHub, Label: "FetchIssueStates"})
	if got := flush(); len(got) != 1 || !got[0].Redacted {
		t.Fatalf("UserLinked event not redacted: %+v", got)
	}
}

func TestEmitDisabledQueuesNothing(t *testing.T) {
	_, flush := capture(t, func() Limits { l := testLimits(); l.Enabled = false; return l })
	Emit(context.Background(), Event{From: API, To: Postgres, Kind: KindSQL, Label: "SELECT issues"})
	if Active() {
		t.Error("Active() with flow.enabled off")
	}
	if got := flush(); len(got) != 0 {
		t.Errorf("disabled tap published %d events", len(got))
	}
}

func TestEmitWithoutPublisherIsANoOp(t *testing.T) {
	Install(nil)
	Emit(context.Background(), Event{From: API, To: Postgres, Kind: KindSQL, Label: "SELECT issues"})
	if Active() || Self() != "" {
		t.Error("no publisher installed, yet the tap reports active")
	}
}

func TestFullBufferDropsAndCounts(t *testing.T) {
	_, flush := capture(t, func() Limits { l := testLimits(); l.BufferSize = 2; return l })
	before := testutil.ToFloat64(metrics.FlowEventsDropped.WithLabelValues("buffer_full"))
	for range 5 {
		Emit(context.Background(), Event{From: API, To: Postgres, Kind: KindSQL, Label: "SELECT issues"})
	}
	if got := flush(); len(got) != 2 {
		t.Errorf("published %d events, want the 2 the buffer holds", len(got))
	}
	if d := testutil.ToFloat64(metrics.FlowEventsDropped.WithLabelValues("buffer_full")) - before; d != 3 {
		t.Errorf("buffer_full drops = %v, want 3", d)
	}
}

func TestPublishFailuresAreCounted(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		subs   int64
		err    error
	}{
		{"redis down", "redis", 0, errors.New("connection refused")},
		{"flowd not listening", "no_subscriber", 0, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPublisherFunc(API, func(context.Context, []byte) (int64, error) { return tc.subs, tc.err }, testLimits)
			Install(p)
			t.Cleanup(func() { Install(nil) })
			before := testutil.ToFloat64(metrics.FlowEventsDropped.WithLabelValues(tc.reason))
			Emit(context.Background(), Event{From: API, To: Redis, Kind: KindRedis, Label: "GET cache"})
			Emit(context.Background(), Event{From: API, To: Redis, Kind: KindRedis, Label: "GET cache"})
			p.Flush(context.Background())
			if d := testutil.ToFloat64(metrics.FlowEventsDropped.WithLabelValues(tc.reason)) - before; d != 2 {
				t.Errorf("%s drops = %v, want 2", tc.reason, d)
			}
		})
	}
}

// An Event has nowhere to put an address, a token, a request id or a user id.
func TestEventHasNoIdentifyingFields(t *testing.T) {
	want := []string{"t", "from", "to", "kind", "label", "payload", "ms", "n", "redacted"}
	var got []string
	typ := reflect.TypeOf(Event{})
	for i := range typ.NumField() {
		got = append(got, strings.Split(typ.Field(i).Tag.Get("json"), ",")[0])
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Event fields = %v, want exactly %v; a new field must not identify anyone", got, want)
	}
}

func TestClean(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"plain", 80, "plain"},
		{"line\nbreak\r\n\ttab", 80, "line break tab"},
		{"  padded  ", 80, "padded"},
		{"\x1b[31mred\x1b[0m", 80, "[31mred [0m"},
		{"abcdefghij", 5, "abcde…"},
		{"ab cd ef", 3, "ab …"},
		{"日本語のタイトル", 3, "日本語…"},
	}
	for _, tc := range cases {
		if got := clean(tc.in, tc.max); got != tc.want {
			t.Errorf("clean(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}

func TestSQLLabel(t *testing.T) {
	cases := map[string]string{
		`SELECT id, title FROM issues WHERE id = $1`:                                                  "SELECT issues",
		`  select * from repositories r join issues i on i.repository_id = r.id`:                      "SELECT repositories",
		`INSERT INTO issues (repository_id, number) VALUES ($1, $2)`:                                  "INSERT issues",
		`INSERT INTO issues (a) VALUES ($1) ON CONFLICT (repository_id, number) DO UPDATE SET a = $1`: "UPSERT issues",
		`UPDATE ingest_jobs SET status = 'running' WHERE id = $1`:                                     "UPDATE ingest_jobs",
		`DELETE FROM auth_sessions WHERE token = $1`:                                                  "DELETE auth_sessions",
		`-- comment FROM secrets
		 SELECT 1 FROM settings`: "SELECT settings",
		`/* FROM nope */ SELECT key FROM settings`:                              "SELECT settings",
		`SELECT 'FROM users' AS x FROM issues`:                                  "SELECT issues",
		`WITH due AS (SELECT id FROM ingest_jobs) UPDATE ingest_jobs SET x = 1`: "WITH ingest_jobs",
		`SELECT pg_database_size(current_database())`:                           "SELECT",
		`SELECT n FROM (SELECT count(*) AS n FROM issues) sub`:                  "SELECT issues",
		`SELECT x FROM (SELECT 1) sub`:                                          "SELECT",
		`begin`:                                                                 "BEGIN",
		`commit`:                                                                "COMMIT",
		`LISTEN settings_changed`:                                               "LISTEN",
		`VACUUM issues`:                                                         "query",
		``:                                                                      "query",
	}
	for sql, want := range cases {
		if got := sqlLabel(sql); got != want {
			t.Errorf("sqlLabel(%q) = %q, want %q", sql, got, want)
		}
	}
}

func TestTransportEmitsKnownHostsOnly(t *testing.T) {
	_, flush := capture(t, testLimits)
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: http.NoBody, Request: r}, nil
	})
	tr := Transport{Base: base}
	for _, raw := range []string{"https://api.github.com/graphql", "http://ingestd:8002/sync/issues", "http://example.com/x"} {
		u, _ := url.Parse(raw)
		req := httptest.NewRequest(http.MethodPost, raw, nil)
		req.URL = u
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	got := flush()
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2 (github and ingestd, not example.com): %+v", len(got), got)
	}
	if got[0].To != GitHub || got[0].Kind != KindGitHub || got[0].Payload != "200" || got[0].Label != "POST" {
		t.Errorf("github hop = %+v", got[0])
	}
	if got[1].To != Ingestd || got[1].Kind != KindHTTP {
		t.Errorf("ingestd hop = %+v", got[1])
	}
	for _, e := range got {
		if strings.Contains(e.Label+e.Payload, "graphql") || strings.Contains(e.Label+e.Payload, "sync") {
			t.Errorf("hop leaked a URL path: %+v", e)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
