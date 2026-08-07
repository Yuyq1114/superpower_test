package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/fitness-checkin/pkg/observability"
	gatewayclients "github.com/example/fitness-checkin/services/api-gateway/internal/clients"
	gatewayhttp "github.com/example/fitness-checkin/services/api-gateway/internal/http"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestProductionDependenciesWireReadiness(t *testing.T) {
	cs := &gatewayclients.Clients{}
	d := productionDependencies(cs, "secret", nil)
	if d.Ready == nil {
		t.Fatal("production readiness is not wired")
	}
	r := gatewayhttp.NewRouter(d)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status=%d", w.Code)
	}
}

func TestGatewayMetricsCountNormalAuthFailureAndNotFound(t *testing.T) {
	reg := observability.NewRegistry()
	metrics := observability.NewMetrics(reg)
	r := gatewayhttp.NewRouterWithMetrics(&gatewayhttp.Dependencies{JWTSecret: "secret"}, metrics)
	r.GET("/metrics", func(c *gin.Context) { promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(c.Writer, c.Request) })
	for _, tc := range []struct {
		method, path string
		want         int
	}{{http.MethodGet, "/healthz", http.StatusOK}, {http.MethodPost, "/api/v1/plans", http.StatusUnauthorized}, {http.MethodGet, "/does-not-exist/user-input", http.StatusNotFound}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Fatalf("%s %s status=%d, want %d", tc.method, tc.path, w.Code, tc.want)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	for _, want := range []string{`fitness_checkin_requests_total{method="GET /healthz 200",service="api-gateway"} 1`, `fitness_checkin_requests_total{method="POST /api/v1/plans 401",service="api-gateway"} 1`, `fitness_checkin_errors_total{method="POST /api/v1/plans 401",service="api-gateway"} 1`, `fitness_checkin_requests_total{method="GET __unmatched__ 404",service="api-gateway"} 1`, `fitness_checkin_request_duration_seconds_count{method="GET /healthz 200",service="api-gateway"} 1`} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "does-not-exist") {
		t.Fatal("metrics must not contain unmatched raw paths")
	}
}
