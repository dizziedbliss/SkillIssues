// Package flow is the tap behind the live data-flow visualizer.
//
// go-api and ingestd describe each hop data takes — an HTTP request, a query,
// a GitHub call, a crawled page — as an Event and hand it to Emit. Emit is the
// only place an event is redacted, so no caller can forget to: an event whose
// context is tied to a signed-in user (MarkUser, UserLinked) leaves the
// process as random symbols. The Publisher batches events to Redis
// (PUBLISH flow:events), fire-and-forget; flowd relays them to viewers.
//
// An Event has no field for an IP, a token, a request id, a user id or a SQL
// value, so none can be published by mistake. Labels are built from route
// patterns, SQL verbs and table names, GitHub operation names, and public
// GitHub data (repository names, issue titles).
package flow

import "time"

// Channel is the Redis pub/sub channel events are published on.
const Channel = "flow:events"

// Node is one body in the visualizer. The ids are the compose service names,
// plus the two outside the host: users and GitHub.
type Node string

const (
	Users            Node = "users"
	GitHub           Node = "github"
	Caddy            Node = "caddy"
	Frontend         Node = "frontend"
	API              Node = "go-api"
	Ingestd          Node = "ingestd"
	Flowd            Node = "flowd"
	Postgres         Node = "postgres"
	Redis            Node = "redis"
	Prometheus       Node = "prometheus"
	Grafana          Node = "grafana"
	PostgresExporter Node = "postgres-exporter"
	RedisExporter    Node = "redis-exporter"
	NodeExporter     Node = "node-exporter"
	Cadvisor         Node = "cadvisor"
)

var nodes = map[Node]bool{
	Users: true, GitHub: true, Caddy: true, Frontend: true, API: true, Ingestd: true,
	Flowd: true, Postgres: true, Redis: true, Prometheus: true, Grafana: true,
	PostgresExporter: true, RedisExporter: true, NodeExporter: true, Cadvisor: true,
}

// Valid reports whether n is one of the known nodes.
func (n Node) Valid() bool { return nodes[n] }

// Kind says what a line describes.
type Kind string

const (
	KindHTTP      Kind = "http"      // a request through caddy to go-api
	KindSQL       Kind = "sql"       // one statement: verb and table only
	KindRedis     Kind = "redis"     // a cache or rate-limit call
	KindGitHub    Kind = "github"    // an outbound GitHub call
	KindCrawl     Kind = "crawl"     // a crawled page of issues, after it committed
	KindDiscovery Kind = "discovery" // a searched page of repositories, after it committed

	// Written by flowd only.
	KindDensity   Kind = "density"   // events on a link over the last window
	KindAmbient   Kind = "ambient"   // bytes/s on a link nothing is instrumented on
	KindViewers   Kind = "viewers"   // open /stream connections
	KindHeartbeat Kind = "heartbeat" // keep-alive on an ndjson stream
)

// Event is one line of the public stream.
type Event struct {
	T       time.Time `json:"t"`
	From    Node      `json:"from"`
	To      Node      `json:"to"`
	Kind    Kind      `json:"kind"`
	Label   string    `json:"label"`
	Payload string    `json:"payload,omitempty"`
	// Ms is the hop's duration in milliseconds, where it has one. For a
	// density line it is the window the count covers.
	Ms float64 `json:"ms"`
	// N is 1 for a hop. A density line carries the count of events on the
	// link in the window, an ambient line the link's bytes per second, a
	// viewers line the number of viewers.
	N int64 `json:"n"`
	// Redacted marks an event tied to a signed-in user: Label and Payload
	// are random symbols, unrelated to the original text and its length.
	Redacted bool `json:"redacted,omitempty"`
}
