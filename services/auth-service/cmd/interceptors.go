package main

import (
	"context"
	"time"

	"github.com/example/fitness-checkin/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func metricsInterceptor(metrics *observability.Metrics) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		metrics.RequestsTotal.WithLabelValues("auth-service", info.FullMethod).Inc()
		resp, err := handler(ctx, req)
		metrics.DurationSeconds.WithLabelValues("auth-service", info.FullMethod).Observe(time.Since(start).Seconds())
		if err != nil {
			metrics.ErrorsTotal.WithLabelValues("auth-service", info.FullMethod).Inc()
		}
		return resp, err
	}
}

func defaultDeadlineInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := ctx.Deadline(); ok {
			return handler(ctx, req)
		}
		deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return handler(deadlineCtx, req)
	}
}

var _ = status.Code
