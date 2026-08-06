package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/example/fitness-checkin/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRequestLoggerFieldsAndSensitiveData(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger("auth-service", &output)
	interceptor := requestLogger(logger)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "request-1", "x-trace-id", "trace-1", "x-user-id", "user-42", "authorization", "Bearer secret-token"))
	_, err := interceptor(ctx, struct{ Password string }{Password: "secret-password"}, &grpc.UnaryServerInfo{FullMethod: "/auth.v1.AuthService/Login"}, func(context.Context, any) (any, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"service", "level", "trace_id", "request_id", "user_id", "message", "timestamp"} {
		if _, ok := entry[key]; !ok {
			t.Errorf("missing log field %s", key)
		}
	}
	if entry["request_id"] != "request-1" || entry["trace_id"] != "trace-1" || entry["user_id"] != "user-42" {
		t.Fatalf("request identifiers: %#v", entry)
	}
	if bytes.Contains(output.Bytes(), []byte("secret-token")) || bytes.Contains(output.Bytes(), []byte("secret-password")) {
		t.Fatal("sensitive request data logged")
	}
}

func TestRequestLoggerGeneratesIdentifiers(t *testing.T) {
	var output bytes.Buffer
	interceptor := requestLogger(observability.NewLogger("auth-service", &output))
	_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(context.Context, any) (any, error) { return nil, nil })
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["request_id"] == "" || entry["trace_id"] == "" {
		t.Fatal("identifiers were not generated")
	}
}

var _ = slog.LevelInfo
