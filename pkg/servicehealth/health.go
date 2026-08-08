package servicehealth

import (
	"context"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"sync/atomic"
)

type Check func(context.Context) error
type Server struct {
	healthv1.UnimplementedHealthServer
	serving atomic.Bool
	checks  []Check
}

func New(checks ...Check) *Server   { return &Server{checks: checks} }
func (s *Server) SetServing(v bool) { s.serving.Store(v) }
func (s *Server) Serving(ctx context.Context) bool {
	if !s.serving.Load() {
		return false
	}
	for _, check := range s.checks {
		if check(ctx) != nil {
			return false
		}
	}
	return true
}
func (s *Server) Check(ctx context.Context, _ *healthv1.HealthCheckRequest) (*healthv1.HealthCheckResponse, error) {
	status := healthv1.HealthCheckResponse_NOT_SERVING
	if s.Serving(ctx) {
		status = healthv1.HealthCheckResponse_SERVING
	}
	return &healthv1.HealthCheckResponse{Status: status}, nil
}
func (s *Server) Watch(*healthv1.HealthCheckRequest, healthv1.Health_WatchServer) error { return nil }
