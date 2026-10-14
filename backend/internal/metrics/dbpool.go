package metrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type dbPoolCollector struct {
	pool *pgxpool.Pool

	acquired, idle, constructing, total, max *prometheus.Desc
	acquires, emptyAcquires, canceled        *prometheus.Desc
	acquireSeconds                           *prometheus.Desc
}

// RegisterDBPool exports pgxpool statistics, read at scrape time.
func RegisterDBPool(pool *pgxpool.Pool) {
	d := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc("db_pool_"+name, help, nil, nil)
	}
	Registry.MustRegister(&dbPoolCollector{
		pool:           pool,
		acquired:       d("acquired_conns", "Connections currently checked out."),
		idle:           d("idle_conns", "Idle connections in the pool."),
		constructing:   d("constructing_conns", "Connections being established."),
		total:          d("total_conns", "Total connections in the pool."),
		max:            d("max_conns", "Configured pool size."),
		acquires:       d("acquires_total", "Successful connection acquires."),
		emptyAcquires:  d("empty_acquires_total", "Acquires that had to wait because no idle connection was available."),
		canceled:       d("canceled_acquires_total", "Acquires canceled by their context."),
		acquireSeconds: d("acquire_seconds_total", "Total time spent waiting to acquire connections."),
	})
}

func (c *dbPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{c.acquired, c.idle, c.constructing, c.total, c.max, c.acquires, c.emptyAcquires, c.canceled, c.acquireSeconds} {
		ch <- d
	}
}

func (c *dbPoolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.pool.Stat()
	g := func(d *prometheus.Desc, v float64) { ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v) }
	k := func(d *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v)
	}
	g(c.acquired, float64(s.AcquiredConns()))
	g(c.idle, float64(s.IdleConns()))
	g(c.constructing, float64(s.ConstructingConns()))
	g(c.total, float64(s.TotalConns()))
	g(c.max, float64(s.MaxConns()))
	k(c.acquires, float64(s.AcquireCount()))
	k(c.emptyAcquires, float64(s.EmptyAcquireCount()))
	k(c.canceled, float64(s.CanceledAcquireCount()))
	k(c.acquireSeconds, s.AcquireDuration().Seconds())
}
