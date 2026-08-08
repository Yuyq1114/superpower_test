package main

import (
	"bytes"
	"context"
	"errors"
	"github.com/example/fitness-checkin/pkg/observability"
	"github.com/example/fitness-checkin/services/profile-service/internal/identity"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"log/slog"
	"net"
	"strings"
	"testing"
)

type forgedRequest struct{ user string }

func (f forgedRequest) GetUserId() string { return f.user }
func TestMetricsLoggerUsesTrustedIdentity(t *testing.T) {
	var b bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&b, nil))
	m := observability.NewMetrics(prometheus.NewRegistry())
	ctx := identity.WithTrusted(context.Background(), "trusted-user", "trace", "request")
	_, _ = metricsInterceptor(m, logger)(ctx, forgedRequest{"forged-user"}, &grpc.UnaryServerInfo{FullMethod: "/profile.v1/ProfileService/RecordMetric"}, func(context.Context, any) (any, error) { return nil, nil })
	got := b.String()
	if !strings.Contains(got, "trusted-user") || strings.Contains(got, "forged-user") {
		t.Fatalf("unexpected audit log: %s", got)
	}
}

type failingServer struct{}

func (failingServer) Serve(net.Listener) error { return errors.New("serve failed") }
func TestServeFailureClearsReadinessAndCancels(t *testing.T) {
	ctx := context.Background()
	state := &servingState{}
	cancelled := false
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer l.Close()
	serveGRPC(ctx, failingServer{}, l, state, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), func() { cancelled = true })
	if state.Ready() || !cancelled {
		t.Fatal("serve failure did not clear readiness and cancel")
	}
}
