package main

import (
	gatewayclients "github.com/example/fitness-checkin/services/api-gateway/internal/clients"
	gatewayhttp "github.com/example/fitness-checkin/services/api-gateway/internal/http"
	"net/http"
	"net/http/httptest"
	"testing"
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
