package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/example/fitness-checkin/pkg/observability"
	authv1 "github.com/example/fitness-checkin/proto/gen/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestRequestLoggerFieldsAndSensitiveData(t *testing.T) {
	var output bytes.Buffer
	logger := observability.NewLogger("auth-service", &output)
	interceptor := requestLogger(logger)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "request-1", "x-trace-id", "trace-1", "authorization", "Bearer secret-token"))
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
	if entry["request_id"] != "request-1" || entry["trace_id"] != "trace-1" || entry["user_id"] != "" {
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

func TestRequestLoggerUsesTrustedContextIdentity(t *testing.T) {
	var output bytes.Buffer
	interceptor := requestLogger(observability.NewLogger("auth-service", &output))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "spoofed"))
	ctx = WithAuthenticatedUserID(ctx, "verified-user")
	_, _ = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, func(context.Context, any) (any, error) { return nil, nil })
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["user_id"] != "verified-user" {
		t.Fatalf("user id: %#v", entry["user_id"])
	}
}

func TestRequestLoggerUsesAuthenticatedResponseIdentity(t *testing.T) {
	var output bytes.Buffer
	interceptor := requestLogger(observability.NewLogger("auth-service", &output))
	response := &authv1.AuthResponse{User: &authv1.User{Id: "response-user"}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-user-id", "spoofed"))
	_, _ = interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/auth.v1.AuthService/Login"}, func(context.Context, any) (any, error) { return response, nil })
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["user_id"] != "response-user" {
		t.Fatalf("user id: %#v", entry["user_id"])
	}
}
