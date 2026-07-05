package telemetry

import (
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/sdk/resource"
)

func TestNewMeterProviderExposesRuntimeMetrics(t *testing.T) {
	_, handler, err := NewMeterProvider(resource.Empty())
	if err != nil {
		t.Fatalf("NewMeterProvider failed: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, metric := range []string{
		"go_goroutines",
		"process_cpu_seconds_total",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("expected metric %q in /metrics output", metric)
		}
	}
}
