package httpapi

import (
	"context"
	"crypto/rand"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/ruhuang/ink/server/internal/platform/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// WithObservability configures optional process metrics and the readiness probe.
// A nil metrics registry keeps request logging and readiness without metrics.
// Apply options before calling Handler or accepting requests.
func WithObservability(metrics *observability.Metrics, ready func(context.Context) error) func(*Server) {
	return func(s *Server) {
		s.metrics = metrics
		s.ready = ready
	}
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.ready == nil || s.ready(r.Context()) != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) observeHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := "req_" + rand.Text()
		r = r.WithContext(WithRequestID(r.Context(), requestID))
		parent := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		traceCtx, span := otel.Tracer("ink").Start(parent, "http request", trace.WithSpanKind(trace.SpanKindServer))
		r = r.WithContext(traceCtx)
		w.Header().Set("X-Request-ID", requestID)
		status := 0
		var size int64
		wrapped := httpsnoop.Wrap(w, httpsnoop.Hooks{
			WriteHeader: func(next httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
				return func(code int) {
					next(code)
					if status == 0 && (code >= 200 || code == http.StatusSwitchingProtocols) {
						status = code
					}
				}
			},
			Write: func(next httpsnoop.WriteFunc) httpsnoop.WriteFunc {
				return func(body []byte) (int, error) {
					if status == 0 {
						status = http.StatusOK
					}
					n, err := next(body)
					size += int64(n)
					return n, err
				}
			},
			ReadFrom: func(next httpsnoop.ReadFromFunc) httpsnoop.ReadFromFunc {
				return func(reader io.Reader) (int64, error) {
					if status == 0 {
						status = http.StatusOK
					}
					n, err := next(reader)
					size += n
					return n, err
				}
			},
			Flush: func(next httpsnoop.FlushFunc) httpsnoop.FlushFunc {
				return func() {
					if status == 0 {
						status = http.StatusOK
					}
					next()
				}
			},
		})
		if s.metrics != nil {
			s.metrics.RequestStarted()
		}
		defer func() {
			failure := recover()
			panicked := failure != nil && failure != http.ErrAbortHandler
			aborted := failure != nil
			if panicked && status == 0 {
				// Panic values may contain credentials or user content. Never log them.
				wrapped.Header().Del("Content-Length")
				writeError(wrapped, requestID, http.StatusInternalServerError, "internal_error", "服务暂时不可用。")
				aborted = false
			}
			if status == 0 && !aborted {
				status = http.StatusOK
			}
			// ServeMux fills Pattern after routing. Never fall back to the raw URL.
			route := r.Pattern
			if _, path, ok := strings.Cut(route, " "); ok {
				route = path
			}
			if route == "" {
				route = "unmatched"
			}
			method := metricMethod(r.Method)
			elapsed := time.Since(started)
			span.SetName(method + " " + route)
			span.SetAttributes(attribute.String("http.request.method", method), attribute.String("http.route", route), attribute.Int("http.response.status_code", status))
			if status >= 500 || panicked || aborted {
				span.SetStatus(codes.Error, "request failed")
			}
			span.End()
			if s.metrics != nil {
				s.metrics.RequestCompleted(method, route, status, elapsed, size, panicked)
			}
			level := slog.LevelInfo
			if status >= 500 || panicked || aborted {
				level = slog.LevelError
			}
			fields := []any{
				"request_id", requestID, "method", method, "route", route,
				"status", status, "duration_ms", float64(elapsed) / float64(time.Millisecond),
				"response_size_bytes", size, "panicked", panicked, "aborted", aborted,
			}
			if span.SpanContext().IsValid() {
				fields = append(fields, "trace_id", span.SpanContext().TraceID().String(), "span_id", span.SpanContext().SpanID().String())
			}
			s.logger.Log(r.Context(), level, "http request completed", fields...)
			if aborted {
				// Headers/body already sent cannot be replaced by an honest 500.
				// Let net/http close/reset the response without leaking a panic value.
				panic(http.ErrAbortHandler)
			}
		}()
		next.ServeHTTP(wrapped, r)
	})
}

func metricMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		return method
	default:
		return "OTHER"
	}
}
