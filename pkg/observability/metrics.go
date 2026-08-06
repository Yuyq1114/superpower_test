package observability

import "expvar"

func RegisterMetrics() *expvar.Map {
	metrics := expvar.NewMap("fitness_checkin")
	metrics.Set("requests_total", new(expvar.Int))
	metrics.Set("errors_total", new(expvar.Int))
	return metrics
}
