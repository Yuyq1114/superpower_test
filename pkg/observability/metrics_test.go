package observability

import "testing"

func TestNewMetricsRegistersPrometheusCollectorsWithoutDuplicatePanic(t *testing.T) {
	registry := NewRegistry()
	metrics := NewMetrics(registry)
	if metrics.RequestsTotal == nil || metrics.ErrorsTotal == nil {
		t.Fatal("metrics collectors must be initialized")
	}
	if _, err := registry.Gather(); err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	if NewMetrics(registry) == nil {
		t.Fatal("registering metrics twice should remain usable")
	}
}
