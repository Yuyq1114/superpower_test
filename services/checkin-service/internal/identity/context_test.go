package identity

import (
	"context"
	checkinv1 "github.com/example/fitness-checkin/proto/gen/checkin/v1"
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
	_, e := i(ctx, &checkinv1.ListHistoryRequest{UserId: "trusted"}, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
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
	_, e = i(missingJTI, &checkinv1.ListHistoryRequest{UserId: "trusted"}, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(e) != codes.Unauthenticated {
		t.Fatal("missing jti accepted")
	}
	forged := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-trace-id", "forged"))
	_, e = i(forged, &checkinv1.ListHistoryRequest{UserId: "trusted"}, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) {
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
func TestRejectsRequestUserDifferentFromJWTSubject(t *testing.T) {
	raw := token(t, "secret", "user-a", "jti")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+raw))
	req := &checkinv1.ListHistoryRequest{UserId: "user-b"}
	called := false
	_, err := UnaryServerInterceptor("secret")(ctx, req, &grpc.UnaryServerInfo{FullMethod: "/checkin.v1.CheckinService/ListHistory"}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	if status.Code(err) != codes.PermissionDenied || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}
func TestAllCheckinRequestsEnforceUserMatch(t *testing.T) {
	requests := []interface{ GetUserId() string }{&checkinv1.CompleteRequest{UserId: "other"}, &checkinv1.ListHistoryRequest{UserId: "other"}}
	for _, req := range requests {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token(t, "secret", "trusted", "j")))
		_, err := UnaryServerInterceptor("secret")(ctx, req, &grpc.UnaryServerInfo{FullMethod: "/checkin.v1.CheckinService/Test"}, func(context.Context, any) (any, error) { t.Fatal("mismatch propagated"); return nil, nil })
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("%T err=%v", req, err)
		}
	}
}

type unknownRequest struct{}
type streamStub struct {
	grpc.ServerStream
	ctx context.Context
}

func (s streamStub) Context() context.Context { return s.ctx }
func TestUnaryRejectsNilAndUnknownRequests(t *testing.T) {
	for _, req := range []any{nil, unknownRequest{}} {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token(t, "secret", "trusted", "j")))
		called := false
		_, err := UnaryServerInterceptor("secret")(ctx, req, &grpc.UnaryServerInfo{FullMethod: "/service/Test"}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		if status.Code(err) != codes.InvalidArgument || called {
			t.Fatalf("req=%T err=%v called=%v", req, err, called)
		}
	}
}
func TestStreamAuthenticationDefaultsClosed(t *testing.T) {
	interceptor := StreamServerInterceptor("secret")
	healthCalled := false
	err := interceptor(nil, streamStub{ctx: context.Background()}, &grpc.StreamServerInfo{FullMethod: "/grpc.health.v1.Health/Watch"}, func(any, grpc.ServerStream) error { healthCalled = true; return nil })
	if err != nil || !healthCalled {
		t.Fatalf("health watch err=%v called=%v", err, healthCalled)
	}
	for _, auth := range []string{"", "Bearer invalid"} {
		ctx := context.Background()
		if auth != "" {
			ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", auth))
		}
		called := false
		err = interceptor(nil, streamStub{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: "/service/Stream"}, func(any, grpc.ServerStream) error { called = true; return nil })
		if status.Code(err) != codes.Unauthenticated || called {
			t.Fatalf("auth=%q err=%v called=%v", auth, err, called)
		}
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token(t, "secret", "trusted", "j")))
	called := false
	err = interceptor(nil, streamStub{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: "/service/Stream"}, func(any, grpc.ServerStream) error { called = true; return nil })
	if status.Code(err) != codes.Unimplemented || called {
		t.Fatalf("valid token err=%v called=%v", err, called)
	}
}
