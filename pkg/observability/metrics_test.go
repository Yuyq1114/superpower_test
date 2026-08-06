package observability

import (
	"github.com/prometheus/client_golang/prometheus/testutil"
	"testing"
)

func TestNewMetricsRegistersPrometheusCollectorsWithoutDuplicatePanic(t *testing.T) {
	registry := NewRegistry()
	metrics := NewMetrics(registry)
	if metrics.RequestsTotal == nil || metrics.ErrorsTotal == nil {
		t.Fatal("metrics collectors must be initialized")
	}
	if _, err := registry.Gather(); err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
}

func TestNewMetricsReusesExistingCollectors(t *testing.T) {
	registry := NewRegistry()
	first := NewMetrics(registry)
	second := NewMetrics(registry)
	second.RequestsTotal.WithLabelValues("gateway", "Get").Inc()
	if got := testutil.ToFloat64(first.RequestsTotal.WithLabelValues("gateway", "Get")); got != 1 {
		t.Fatalf("shared counter = %v, want 1", got)
	}
	if got := testutil.CollectAndCount(registry); got != 1 {
		t.Fatalf("metric families = %v, want 1", got)
	}
}
