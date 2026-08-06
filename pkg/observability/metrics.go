package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	RequestsTotal *prometheus.CounterVec
	ErrorsTotal   *prometheus.CounterVec
}

func NewRegistry() *prometheus.Registry { return prometheus.NewRegistry() }

func NewMetrics(registry prometheus.Registerer) *Metrics {
	metrics := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "fitness_checkin_requests_total", Help: "Total requests handled."}, []string{"service", "method"}),
		ErrorsTotal:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "fitness_checkin_errors_total", Help: "Total errors returned."}, []string{"service", "method"}),
	}
	registerCollector(registry, metrics.RequestsTotal)
	registerCollector(registry, metrics.ErrorsTotal)
	return metrics
}

func registerCollector(registry prometheus.Registerer, collector prometheus.Collector) {
	if err := registry.Register(collector); err != nil {
		if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
			_ = already.ExistingCollector
		}
	}
}
