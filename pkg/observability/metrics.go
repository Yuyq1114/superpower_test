package observability

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	RequestsTotal   *prometheus.CounterVec
	ErrorsTotal     *prometheus.CounterVec
	DurationSeconds *prometheus.HistogramVec
}

func NewRegistry() *prometheus.Registry { return prometheus.NewRegistry() }

func NewMetrics(registry prometheus.Registerer) *Metrics {
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "fitness_checkin_requests_total", Help: "Total requests handled."}, []string{"service", "method"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "fitness_checkin_request_duration_seconds", Help: "Request duration in seconds."}, []string{"service", "method"})
	errors := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "fitness_checkin_errors_total", Help: "Total errors returned."}, []string{"service", "method"})
	return &Metrics{
		RequestsTotal:   registerCounterVec(registry, requests),
		ErrorsTotal:     registerCounterVec(registry, errors),
		DurationSeconds: registerHistogramVec(registry, duration),
	}
}

func registerCounterVec(registry prometheus.Registerer, collector *prometheus.CounterVec) *prometheus.CounterVec {
	if err := registry.Register(collector); err != nil {
		if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := already.ExistingCollector.(*prometheus.CounterVec); ok {
				return existing
			}
		}
	}
	return collector
}

func registerHistogramVec(registry prometheus.Registerer, collector *prometheus.HistogramVec) *prometheus.HistogramVec {
	if err := registry.Register(collector); err != nil {
		if already, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := already.ExistingCollector.(*prometheus.HistogramVec); ok {
				return existing
			}
		}
	}
	return collector
}
