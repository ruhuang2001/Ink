// Package observability owns process-local operational state and metrics.
package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics uses a private registry so independent servers and tests never share counters.
type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	size     *prometheus.HistogramVec
	inFlight prometheus.Gauge
	panics   prometheus.Counter
}

func NewMetrics() *Metrics {
	labels := []string{"method", "route", "status"}
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ink_http_requests_total", Help: "Completed HTTP requests, including aborted requests (status 0 before headers).",
		}, labels),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ink_http_request_duration_seconds", Help: "HTTP handler duration in seconds.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2, 3, 5, 10, 30, 60, 120},
		}, labels),
		size: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ink_http_response_size_bytes", Help: "HTTP response body bytes successfully written.",
			Buckets: prometheus.ExponentialBuckets(256, 4, 9),
		}, labels),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ink_http_in_flight_requests", Help: "HTTP requests currently executing.",
		}),
		panics: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ink_http_panics_total", Help: "HTTP handler panics, excluding intentional http.ErrAbortHandler aborts.",
		}),
	}
	m.registry.MustRegister(m.requests, m.duration, m.size, m.inFlight, m.panics,
		collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

func (m *Metrics) RequestStarted() { m.inFlight.Inc() }

func (m *Metrics) RequestCompleted(method, route string, status int, elapsed time.Duration, size int64, panicked bool) {
	m.inFlight.Dec()
	labels := []string{method, route, strconv.Itoa(status)}
	m.requests.WithLabelValues(labels...).Inc()
	m.duration.WithLabelValues(labels...).Observe(elapsed.Seconds())
	m.size.WithLabelValues(labels...).Observe(float64(size))
	if panicked {
		m.panics.Inc()
	}
}

// Handler serves only metrics; attach it to a separate, private listener.
func (m *Metrics) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		MaxRequestsInFlight: 2,
		Timeout:             5 * time.Second,
	}))
	return mux
}
