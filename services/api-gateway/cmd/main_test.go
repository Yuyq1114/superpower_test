package main

import (
	"net/http"
	"net/http/httptest"
	"os"
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
	d := productionDependencies(cs, "secret", nil, gatewayhttp.RefreshCookieConfig{})
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

func TestLoadRequiresAllowedOriginsAndTrimsList(t *testing.T) {
	for _, key := range []string{"JWT_SECRET", "ALLOWED_ORIGINS", "REFRESH_COOKIE_SECURE"} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
	t.Setenv("JWT_SECRET", "secret")

	if _, err := load(); err == nil {
		t.Fatal("expected error when ALLOWED_ORIGINS is missing")
	}

	t.Setenv("ALLOWED_ORIGINS", " http://localhost:5173 , http://127.0.0.1:5173 ")
	cfg, err := load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("AllowedOrigins = %#v, want 2 trimmed entries", cfg.AllowedOrigins)
	}
	for _, origin := range []string{"http://localhost:5173", "http://127.0.0.1:5173"} {
		if _, ok := cfg.AllowedOrigins[origin]; !ok {
			t.Fatalf("AllowedOrigins missing trimmed origin %q: %#v", origin, cfg.AllowedOrigins)
		}
	}
	if cfg.CookieSecure {
		t.Fatal("CookieSecure should default to false")
	}
}

func TestGatewayMetricsCountNormalAuthFailureMethodNotAllowedAndNotFound(t *testing.T) {
	reg := observability.NewRegistry()
	metrics := observability.NewMetrics(reg)
	r := gatewayhttp.NewRouterWithMetrics(&gatewayhttp.Dependencies{JWTSecret: "secret"}, metrics)
	r.GET("/metrics", func(c *gin.Context) { promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(c.Writer, c.Request) })
	for _, tc := range []struct {
		method, path string
		want         int
	}{{http.MethodGet, "/healthz", http.StatusOK}, {http.MethodPost, "/api/v1/plans", http.StatusUnauthorized}, {http.MethodPost, "/healthz", http.StatusMethodNotAllowed}, {http.MethodGet, "/does-not-exist/user-input", http.StatusNotFound}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Fatalf("%s %s status=%d, want %d", tc.method, tc.path, w.Code, tc.want)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	for _, want := range []string{`fitness_checkin_requests_total{method="GET /healthz 200",service="api-gateway"} 1`, `fitness_checkin_requests_total{method="POST /api/v1/plans 401",service="api-gateway"} 1`, `fitness_checkin_errors_total{method="POST /api/v1/plans 401",service="api-gateway"} 1`, `fitness_checkin_requests_total{method="POST __method_not_allowed__ 405",service="api-gateway"} 1`, `fitness_checkin_errors_total{method="POST __method_not_allowed__ 405",service="api-gateway"} 1`, `fitness_checkin_requests_total{method="GET __unmatched__ 404",service="api-gateway"} 1`, `fitness_checkin_request_duration_seconds_count{method="GET /healthz 200",service="api-gateway"} 1`} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "does-not-exist") {
		t.Fatal("metrics must not contain unmatched raw paths")
	}
}
