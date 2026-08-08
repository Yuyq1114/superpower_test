package identity

import (
	"context"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
	"testing"
	"time"
)

func token(t *testing.T, secret, user, jti string) string {
	t.Helper()
	raw, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": user, "jti": jti, "iat": time.Now().Add(-time.Second).Unix(), "exp": time.Now().Add(time.Minute).Unix()}).SignedString([]byte(secret))
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func TestInterceptorUsesAuthCompatibleClaimsAndValidatedCorrelationIDs(t *testing.T) {
	i := UnaryServerInterceptor("secret")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("user_id", "forged", "x-trace-id", "trace_1", "x-request-id", "request-1", "authorization", "Bearer "+token(t, "secret", "trusted", "jti")))
	_, e := i(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		u, trace, request, ok := FromContext(ctx)
		if !ok || u != "trusted" || trace != "trace_1" || request != "request-1" {
			t.Fatal("trusted context mismatch")
		}
		return nil, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	missingJTI := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token(t, "secret", "trusted", "")))
	_, e = i(missingJTI, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(e) != codes.Unauthenticated {
		t.Fatal("missing jti accepted")
	}
	forged := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-trace-id", "forged"))
	_, e = i(forged, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
		t.Fatal("unauthenticated request propagated")
		return nil, nil
	})
	if status.Code(e) != codes.Unauthenticated {
		t.Fatal(e)
	}
}
func TestInvalidCorrelationIDsAreRegenerated(t *testing.T) {
	ctx := WithTrusted(context.Background(), "u", "contains space", strings.Repeat("x", 129))
	_, trace, request, _ := FromContext(ctx)
	if trace == "contains space" || len(request) > 128 {
		t.Fatal("invalid correlation ID accepted")
	}
}

func TestHealthCheckBypassesUserAuthentication(t *testing.T) {
	called := false
	_, err := UnaryServerInterceptor("secret")(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	if err != nil || !called {
		t.Fatalf("health check rejected: %v", err)
	}
}
