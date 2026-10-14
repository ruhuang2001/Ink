package observability

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// RegisterPool exports local pool statistics only; scrapes never query the DB
// or acquire a connection. Register once for the process's primary pool.
func (m *Metrics) RegisterPool(pool *pgxpool.Pool) {
	for _, metric := range []struct {
		name  string
		help  string
		value func() float64
	}{
		{"connections", "Current total pool connections.", func() float64 { return float64(pool.Stat().TotalConns()) }},
		{"max_connections", "Maximum pool connections.", func() float64 { return float64(pool.Stat().MaxConns()) }},
		{"acquired_connections", "Connections currently acquired.", func() float64 { return float64(pool.Stat().AcquiredConns()) }},
		{"idle_connections", "Idle pool connections.", func() float64 { return float64(pool.Stat().IdleConns()) }},
		{"constructing_connections", "Connections being constructed.", func() float64 { return float64(pool.Stat().ConstructingConns()) }},
	} {
		m.registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "ink_db_pool_" + metric.name, Help: metric.help,
		}, metric.value))
	}
	for _, metric := range []struct {
		name  string
		help  string
		value func() float64
	}{
		{"acquires_total", "Successful connection acquisitions.", func() float64 { return float64(pool.Stat().AcquireCount()) }},
		{"acquire_duration_seconds_total", "Cumulative duration of successful acquisitions, including waits.", func() float64 { return pool.Stat().AcquireDuration().Seconds() }},
		{"empty_acquires_total", "Acquisitions that waited for a connection.", func() float64 { return float64(pool.Stat().EmptyAcquireCount()) }},
		{"canceled_acquires_total", "Acquisitions canceled by context.", func() float64 { return float64(pool.Stat().CanceledAcquireCount()) }},
	} {
		m.registry.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name: "ink_db_pool_" + metric.name, Help: metric.help,
		}, metric.value))
	}
}
