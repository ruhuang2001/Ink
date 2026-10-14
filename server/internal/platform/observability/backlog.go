package observability

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type BacklogSample struct {
	Queue  string
	Count  int64
	Oldest time.Time
}

type BacklogSource interface {
	SampleBacklog(context.Context, time.Time) ([]BacklogSample, error)
}

// StartBacklog samples outside the scrape path. Failed samples retain the last
// values and success timestamp so stale zeros cannot masquerade as health.
func (m *Metrics) StartBacklog(ctx context.Context, source BacklogSource) <-chan struct{} {
	items := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "ink_backlog_items", Help: "Database-wide queue population; use max, not sum across replicas."}, []string{"queue"})
	oldest := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "ink_backlog_oldest_due_timestamp_seconds", Help: "Oldest due time (creation time for informational populations), zero if empty."}, []string{"queue"})
	lastSuccess := prometheus.NewGauge(prometheus.GaugeOpts{Name: "ink_backlog_last_success_timestamp_seconds", Help: "Last successful complete backlog sample; zero before first success."})
	errors := prometheus.NewCounter(prometheus.CounterOpts{Name: "ink_backlog_sample_errors_total", Help: "Failed or timed-out backlog samples."})
	m.registry.MustRegister(items, oldest, lastSuccess, errors)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			sampleCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			samples, err := source.SampleBacklog(sampleCtx, time.Now())
			cancel()
			if err != nil {
				errors.Inc()
			} else {
				for _, sample := range samples {
					items.WithLabelValues(sample.Queue).Set(float64(sample.Count))
					stamp := float64(0)
					if !sample.Oldest.IsZero() {
						stamp = float64(sample.Oldest.UnixNano()) / 1e9
					}
					oldest.WithLabelValues(sample.Queue).Set(stamp)
				}
				lastSuccess.SetToCurrentTime()
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return done
}
