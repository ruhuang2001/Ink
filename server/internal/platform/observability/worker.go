package observability

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// WorkerMetrics measures scan progress, which does not imply task success.
type WorkerMetrics struct {
	scans       *prometheus.CounterVec
	duration    prometheus.Histogram
	inProgress  prometheus.Gauge
	lastStarted prometheus.Gauge
	lastEnded   prometheus.Gauge
	lastSuccess prometheus.Gauge
}

// Worker registers one enabled worker. Names must be static application names,
// never identifiers from jobs or users. Call once per worker before starting it.
func (m *Metrics) Worker(name string, interval time.Duration) *WorkerMetrics {
	labels := prometheus.Labels{"worker": name}
	gauge := func(name, help string) prometheus.Gauge {
		g := prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: help, ConstLabels: labels})
		m.registry.MustRegister(g)
		return g
	}
	w := &WorkerMetrics{
		scans: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ink_worker_scans_total", Help: "Completed scans by result, not individual task outcomes.", ConstLabels: labels,
		}, []string{"result"}),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "ink_worker_scan_duration_seconds", Help: "Completed scan duration including idle scans.", ConstLabels: labels,
			Buckets: []float64{.01, .1, 1, 5, 15, 30, 60, 120, 300, 600, 1800},
		}),
		inProgress:  gauge("ink_worker_scan_in_progress", "Scans currently executing."),
		lastStarted: gauge("ink_worker_last_scan_started_timestamp_seconds", "Last scan start, or zero before first scan."),
		lastEnded:   gauge("ink_worker_last_scan_completed_timestamp_seconds", "Last scan completion of any result, or zero."),
		lastSuccess: gauge("ink_worker_last_successful_scan_timestamp_seconds", "Last scan returning no error, including idle scans, or zero."),
	}
	gauge("ink_worker_poll_interval_seconds", "Configured worker polling interval in seconds.").Set(interval.Seconds())
	m.registry.MustRegister(w.scans, w.duration)
	for _, result := range []string{"success", "error", "canceled"} {
		w.scans.WithLabelValues(result)
	}
	return w
}

// StartScan returns a completion callback that must be called exactly once.
func (w *WorkerMetrics) StartScan() func(error) {
	started := time.Now()
	w.lastStarted.SetToCurrentTime()
	w.inProgress.Inc()
	return func(err error) {
		result := "error"
		if err == nil {
			result = "success"
			w.lastSuccess.SetToCurrentTime()
		} else if onlyCancellation(err) {
			result = "canceled"
		}
		w.scans.WithLabelValues(result).Inc()
		w.duration.Observe(time.Since(started).Seconds())
		w.lastEnded.SetToCurrentTime()
		w.inProgress.Dec()
	}
}

// A batch error joined with shutdown cancellation is still an error. Deadlines
// are errors too: timing out is not evidence of healthy progress.
func onlyCancellation(err error) bool {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !onlyCancellation(cause) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return onlyCancellation(wrapped.Unwrap())
	}
	return errors.Is(err, context.Canceled)
}
