package main

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"log/slog"
	"time"

	"github.com/example/fitness-checkin/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func requestLogger(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		requestID, traceID := requestIDs(ctx)
		resp, err := handler(ctx, req)
		logger.LogAttrs(ctx, slog.LevelInfo, "request completed", slog.String("request_id", requestID), slog.String("trace_id", traceID), slog.String("user_id", userID(ctx)), slog.String("method", info.FullMethod), slog.Any("error", err))
		return resp, err
	}
}

func userID(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	return firstMetadata(md, "x-user-id")
}

func requestIDs(ctx context.Context) (string, string) {
	md, _ := metadata.FromIncomingContext(ctx)
	requestID := firstMetadata(md, "x-request-id")
	traceID := firstMetadata(md, "x-trace-id")
	if requestID == "" {
		requestID = uuid.NewString()
	}
	if traceID == "" {
		traceID = uuid.NewString()
	}
	return requestID, traceID
}

func firstMetadata(md metadata.MD, key string) string {
	if values := md.Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}

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
