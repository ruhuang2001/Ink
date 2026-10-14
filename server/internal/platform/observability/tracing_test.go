package observability

import (
	"io"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestTracingRequiresExplicitEndpoint(t *testing.T) {
	for _, test := range []struct {
		name, endpoint, tracesEndpoint string
		enabled                        bool
	}{
		{"default", "", "", false},
		{"whitespace", " ", "\t", false},
		{"general endpoint", "http://127.0.0.1:4318", "", true},
		{"trace endpoint", "", "http://127.0.0.1:4318/v1/traces", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", test.endpoint)
			t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", test.tracesEndpoint)
			if TracingEnabled() != test.enabled {
				t.Fatal("unexpected tracing activation")
			}
			if !test.enabled {
				previous := otel.GetTracerProvider()
				shutdown, err := ConfigureTracing(t.Context(), "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
				if err != nil {
					t.Fatal(err)
				}
				if otel.GetTracerProvider() != previous {
					t.Fatal("disabled tracing installed a provider")
				}
				if err := shutdown(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
