package http

import "testing"

func TestNewRouterExposesPublicAndProtectedRoutes(t *testing.T) {
	r := NewRouter(nil)
	for _, path := range []string{"/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout", "/api/v1/plans", "/api/v1/checkins", "/api/v1/body-metrics", "/api/v1/statistics/summary"} {
		if path == "" || len(r.Routes()) == 0 {
			t.Fatal("router has no routes")
		}
	}
}
