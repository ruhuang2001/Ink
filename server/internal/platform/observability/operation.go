package observability

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// Operation is configured once with a static service operation name.
type Operation struct {
	name     string
	results  *prometheus.CounterVec
	duration prometheus.Histogram
}

// Operation creates a trace-only observer when metrics are disabled.
func (m *Metrics) Operation(name string) *Operation {
	if m == nil {
		return &Operation{name: name}
	}
	labels := prometheus.Labels{"operation": name}
	o := &Operation{
		name:     name,
		results:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ink_operations_total", Help: "Individual operation outcomes, not scan outcomes.", ConstLabels: labels}, []string{"result"}),
		duration: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "ink_operation_duration_seconds", Help: "Individual operation duration.", ConstLabels: labels, Buckets: []float64{.01, .05, .1, .5, 1, 5, 15, 30, 60, 120}}),
	}
	m.registry.MustRegister(o.results, o.duration)
	for _, result := range []string{"success", "error", "canceled"} {
		o.results.WithLabelValues(result)
	}
	return o
}

// Start is nil-safe for services without monitoring. Call completion once.
func (o *Operation) Start(ctx context.Context) (context.Context, func(error)) {
	if o == nil {
		return ctx, func(error) {}
	}
	ctx, span := otel.Tracer("ink").Start(ctx, o.name)
	started := time.Now()
	return ctx, func(err error) {
		defer span.End()
		result := "success"
		if err != nil {
			result = "error"
			span.SetStatus(codes.Error, "operation failed")
			if onlyCancellation(err) {
				result = "canceled"
			}
		}
		if o.results != nil {
			o.results.WithLabelValues(result).Inc()
			o.duration.Observe(time.Since(started).Seconds())
		}
	}
}
