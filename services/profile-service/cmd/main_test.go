package main

import (
	"bytes"
	"context"
	"github.com/example/fitness-checkin/pkg/observability"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"log/slog"
	"strings"
	"testing"
)

type forgedRequest struct{ user string }

func (f forgedRequest) GetUserId() string { return f.user }
func TestMetricsLoggerUsesTrustedIdentity(t *testing.T) {
	var b bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&b, nil))
	m := observability.NewMetrics(prometheus.NewRegistry())
	ctx := withAuditIdentity(context.Background(), "trusted-user")
	_, _ = metricsInterceptor(m, logger)(ctx, forgedRequest{"forged-user"}, &grpc.UnaryServerInfo{FullMethod: "/profile.v1/ProfileService/RecordMetric"}, func(context.Context, any) (any, error) { return nil, nil })
	got := b.String()
	if !strings.Contains(got, "trusted-user") || strings.Contains(got, "forged-user") {
		t.Fatalf("unexpected audit log: %s", got)
	}
}
