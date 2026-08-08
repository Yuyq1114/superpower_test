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

func TestMetricsLoggerIgnoresRequestUserID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	m := observability.NewMetrics(prometheus.NewRegistry())
	ctx := withAuditIdentity(context.Background(), "trusted-user")
	req := fakeUserRequest{user: "forged-user"}
	_, _ = metricsInterceptor(m, logger)(ctx, req, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(context.Context, any) (any, error) { return nil, nil })
	out := buf.String()
	if strings.Contains(out, "forged-user") {
		t.Fatal("request user leaked into audit log")
	}
	if !strings.Contains(out, "trusted-user") || strings.Contains(out, "password") || strings.Contains(out, "token") {
		t.Fatalf("unexpected log %s", out)
	}
}

type fakeUserRequest struct{ user string }

func (f fakeUserRequest) GetUserId() string { return f.user }
