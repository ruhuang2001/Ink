package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ruhuang/ink/server/internal/platform/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func observedTestServer() (*Server, *bytes.Buffer) {
	logs := new(bytes.Buffer)
	return &Server{metrics: observability.NewMetrics(), logger: slog.New(slog.NewJSONHandler(logs, nil))}, logs
}

func scrapeMetrics(t *testing.T, server *Server) string {
	t.Helper()
	w := httptest.NewRecorder()
	server.metrics.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("scrape: %d %s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

func TestHTTPObservationUsesTemplatesAndRedactsData(t *testing.T) {
	s, logs := observedTestServer()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /jobs/{jobID}", s.wrap(func(w http.ResponseWriter, r *http.Request, requestID string) {
		if r.Context().Value(requestIDKey) != requestID || w.Header().Get("X-Request-ID") != requestID {
			t.Fatal("request correlation differs")
		}
		writeError(w, requestID, http.StatusBadRequest, "invalid_request", "invalid")
	}))
	handler := s.observeHTTP(mux)
	ids := make(map[string]bool)
	for _, id := range []string{"private-job-1", "private-job-2"} {
		r := httptest.NewRequest(http.MethodPost, "/jobs/"+id+"?token=private-query", strings.NewReader("private-body"))
		r.Header.Set("Authorization", "Bearer private-auth")
		r.Header.Set("X-Request-ID", "private-untrusted-id")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		var response errorEnvelope
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		id := w.Header().Get("X-Request-ID")
		if id == "" || ids[id] || response.RequestID != id {
			t.Fatalf("invalid request ID %q", id)
		}
		ids[id] = true
	}
	scrape := scrapeMetrics(t, s)
	if !strings.Contains(scrape, `ink_http_requests_total{method="POST",route="/jobs/{jobID}",status="400"} 2`) {
		t.Fatal(scrape)
	}
	for _, secret := range []string{"private-job", "private-query", "private-body", "private-auth", "private-untrusted-id"} {
		if strings.Contains(logs.String()+scrape, secret) {
			t.Fatalf("leaked %q", secret)
		}
	}
	for id := range ids {
		if !strings.Contains(logs.String(), id) {
			t.Fatalf("missing log for %s", id)
		}
	}
}

func TestHTTPObservationUnknownMethodsAndPathsAreBounded(t *testing.T) {
	s, _ := observedTestServer()
	handler := s.Handler()
	for _, path := range []string{"/unknown-one", "/unknown-two", "/a/../secret-three", "//secret-four"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("ARBITRARY", path, nil))
	}
	scrape := scrapeMetrics(t, s)
	for _, secret := range []string{"unknown-one", "unknown-two", "secret-three", "secret-four", "ARBITRARY"} {
		if strings.Contains(scrape, secret) {
			t.Fatalf("unbounded label %q", secret)
		}
	}
}

func TestHTTPObservationStatusSizeAndPanic(t *testing.T) {
	for _, test := range []struct {
		name                 string
		handler              http.HandlerFunc
		status, size, panics int
		abort                bool
	}{
		{"implicit", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hello")) }, 200, 5, 0, false},
		{"empty", func(http.ResponseWriter, *http.Request) {}, 200, 0, 0, false},
		{"first final status", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204); w.WriteHeader(500) }, 204, 0, 0, false},
		{"panic", func(http.ResponseWriter, *http.Request) { panic("private-panic") }, 500, -1, 1, false},
		{"partial panic", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("part")); panic("private-panic") }, 200, 4, 1, true},
		{"flushed panic", func(w http.ResponseWriter, _ *http.Request) { w.(http.Flusher).Flush(); panic("private-panic") }, 200, 0, 1, true},
		{"intentional abort", func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }, 0, 0, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, logs := observedTestServer()
			w := httptest.NewRecorder()
			var caught any
			func() {
				defer func() { caught = recover() }()
				s.observeHTTP(test.handler).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
			}()
			if (caught != nil) != test.abort || (caught != nil && caught != http.ErrAbortHandler) {
				t.Fatalf("unexpected panic %v", caught)
			}
			var log struct {
				Status   int  `json:"status"`
				Size     int  `json:"response_size_bytes"`
				Panicked bool `json:"panicked"`
			}
			if err := json.Unmarshal(logs.Bytes(), &log); err != nil {
				t.Fatal(err)
			}
			if log.Status != test.status || (test.size >= 0 && log.Size != test.size) || log.Panicked != (test.panics == 1) {
				t.Fatalf("log: %+v", log)
			}
			scrape := scrapeMetrics(t, s)
			if !strings.Contains(scrape, "ink_http_in_flight_requests 0") {
				t.Fatal("in-flight gauge leaked")
			}
			if strings.Contains(logs.String()+w.Body.String()+scrape, "private-panic") {
				t.Fatal("panic detail leaked")
			}
		})
	}
}

func TestPublicHealthEndpointsAndMetricsIsolation(t *testing.T) {
	s, _ := observedTestServer()
	for _, ready := range []bool{true, false} {
		s.ready = func(context.Context) error {
			if !ready {
				return errors.New("private-db-error")
			}
			return nil
		}
		handler := s.Handler()
		for path, status := range map[string]int{"/healthz": 200, "/readyz": 200, "/metrics": 404} {
			if path == "/readyz" && !ready {
				status = 503
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != status || strings.Contains(w.Body.String(), "private-db-error") {
				t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
			}
		}
	}
	s.ready = nil
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != 503 {
		t.Fatal("unconfigured readiness must fail closed")
	}
}

func TestMetricsRegistryIsolation(t *testing.T) {
	first, _ := observedTestServer()
	second, _ := observedTestServer()
	first.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if strings.Contains(scrapeMetrics(t, second), `route="/healthz"`) {
		t.Fatal("metrics leaked between servers")
	}
}

func TestHTTPObservationWithoutMetricsPreservesContract(t *testing.T) {
	s := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{})
	if s.metrics != nil {
		t.Fatal("default server must not create a metrics registry")
	}
	logs := new(bytes.Buffer)
	s.logger = slog.New(slog.NewJSONHandler(logs, nil))
	WithObservability(nil, func(context.Context) error { return nil })(s)
	for path, status := range map[string]int{"/healthz": 200, "/readyz": 200, "/metrics": 404} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		id := w.Header().Get("X-Request-ID")
		if w.Code != status || id == "" || !strings.Contains(logs.String(), id) {
			t.Fatalf("%s: status %d, request ID %q, logs %s", path, w.Code, id, logs)
		}
	}
	logs.Reset()
	w := httptest.NewRecorder()
	s.observeHTTP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("private-panic")
	})).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/private-id?token=private-query", nil))
	var response errorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != 500 || response.RequestID != w.Header().Get("X-Request-ID") || !strings.Contains(logs.String(), `"panicked":true`) {
		t.Fatalf("panic handling without metrics: %d %s %s", w.Code, w.Body, logs)
	}
	for _, secret := range []string{"private-panic", "private-id", "private-query"} {
		if strings.Contains(logs.String()+w.Body.String(), secret) {
			t.Fatalf("leaked %q without metrics", secret)
		}
	}
}

func TestTracingCorrelatesOperationAndSanitizesFailure(t *testing.T) {
	for _, metricsEnabled := range []bool{false, true} {
		name := "tracing only"
		if metricsEnabled {
			name = "tracing and metrics"
		}
		t.Run(name, func(t *testing.T) {
			testTracingCorrelatesOperationAndSanitizesFailure(t, metricsEnabled)
		})
	}
}

func testTracingCorrelatesOperationAndSanitizesFailure(t *testing.T, metricsEnabled bool) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	s, logs := observedTestServer()
	if !metricsEnabled {
		s.metrics = nil
	}
	operation := s.metrics.Operation("png_render")
	mux := http.NewServeMux()
	mux.HandleFunc("POST /preview/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, finish := operation.Start(r.Context())
		finish(errors.New("secret-upstream-error"))
		w.WriteHeader(500)
	})
	s.observeHTTP(mux).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/preview/private-id?secret=query", strings.NewReader("secret-body")))
	spans := exporter.GetSpans()
	if len(spans) != 2 || spans[0].Name != "png_render" || spans[1].Name != "POST /preview/{id}" || spans[0].Parent.SpanID() != spans[1].SpanContext.SpanID() || spans[0].SpanContext.TraceID() != spans[1].SpanContext.TraceID() {
		t.Fatalf("broken request/operation trace: %+v", spans)
	}
	for _, span := range spans {
		if span.Status.Code != codes.Error {
			t.Fatalf("failure not retained by tail sampler: %+v", span)
		}
		encoded, err := json.Marshal(span)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"secret", "private-id"} {
			if bytes.Contains(encoded, []byte(secret)) {
				t.Fatalf("sensitive trace data: %s", encoded)
			}
		}
	}
	if !strings.Contains(logs.String(), spans[1].SpanContext.TraceID().String()) {
		t.Fatalf("request log missing real trace ID: %s", logs)
	}
}
