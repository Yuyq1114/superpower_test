package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/example/fitness-checkin/pkg/observability"
	"github.com/example/fitness-checkin/services/statistics-service/internal/consumer"
	"github.com/example/fitness-checkin/services/statistics-service/internal/identity"
	"github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"time"
)

func signedToken(t *testing.T, secret, user string) string {
	t.Helper()
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user, "jti": "jti", "iat": time.Now().Add(-time.Second).Unix(), "exp": time.Now().Add(time.Minute).Unix()}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestUnauthenticatedRPCIsMeasuredAndSafelyLogged(t *testing.T) {
	reg := observability.NewRegistry()
	metrics := observability.NewMetrics(reg)
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	auth := identity.UnaryServerInterceptor("secret")
	observed := metricsInterceptor(metrics, logger)
	chain := chainUnary(observed, auth)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer secret-token", "x-trace-id", "bad trace"))
	_, err := chain(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/statistics.v1.StatisticsService/GetSummary"}, func(context.Context, any) (any, error) { t.Fatal("handler called"); return nil, nil })
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code=%s", status.Code(err))
	}
	if got := testutil.ToFloat64(metrics.RequestsTotal.WithLabelValues("statistics-service", "/statistics.v1.StatisticsService/GetSummary")); got != 1 {
		t.Fatalf("requests=%v", got)
	}
	if got := testutil.ToFloat64(metrics.ErrorsTotal.WithLabelValues("statistics-service", "/statistics.v1.StatisticsService/GetSummary")); got != 1 {
		t.Fatalf("errors=%v", got)
	}
	log := output.String()
	if strings.Contains(log, "secret-token") || strings.Contains(log, "bad trace") {
		t.Fatalf("unsafe log=%s", log)
	}
	if strings.Contains(log, `"trace_id":""`) || strings.Contains(log, `"request_id":""`) || !strings.Contains(log, "trace_id") || !strings.Contains(log, "request_id") {
		t.Fatalf("missing correlation IDs: %s", log)
	}
}

func chainUnary(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		chained := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			current, next := interceptors[i], chained
			chained = func(ctx context.Context, req any) (any, error) { return current(ctx, req, info, next) }
		}
		return chained(ctx, req)
	}
}

func TestRuntimeFailureIsReturnedAfterShutdownSignal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	failures := make(chan error, 1)
	failures <- errors.New("serve failed")
	err := waitForStop(ctx, failures)
	if err == nil || err.Error() != "serve failed" {
		t.Fatalf("err=%v", err)
	}
	cancel()
}

func TestAuthenticatedRPCLogsTrustedUser(t *testing.T) {
	reg := observability.NewRegistry()
	metrics := observability.NewMetrics(reg)
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	chain := chainUnary(metricsInterceptor(metrics, logger), identity.UnaryServerInterceptor("secret"))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+signedToken(t, "secret", "trusted-user")))
	_, err := chain(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(context.Context, any) (any, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"user_id":"trusted-user"`) {
		t.Fatalf("log=%s", output.String())
	}
}

func TestBuildConsumerRejectsInvalidStartupSettings(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*consumer.Consumer)
	}{
		{name: "sub-second dedupe TTL", configure: func(c *consumer.Consumer) { c.DedupeTTL = 500 * time.Millisecond }},
		{name: "Redis Cluster", configure: func(c *consumer.Consumer) { c.RedisCluster = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := buildConsumer(nil, nil, "test", tt.configure); err == nil {
				t.Fatal("expected startup assembly validation error")
			}
		})
	}
}

func TestBuildConsumerAcceptsProductionDefaults(t *testing.T) {
	c, err := buildConsumer(nil, nil, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.DedupeTTL < time.Second || c.RedisCluster {
		t.Fatalf("consumer=%#v", c)
	}
}
