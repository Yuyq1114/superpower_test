package identity

import (
	"context"
	"github.com/example/fitness-checkin/pkg/authclaims"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"regexp"
	"strings"
)

type trusted struct{ UserID, TraceID, RequestID string }
type key struct{}

var correlationID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func WithTrusted(ctx context.Context, u, t, r string) context.Context {
	if !correlationID.MatchString(t) {
		t = uuid.NewString()
	}
	if !correlationID.MatchString(r) {
		r = uuid.NewString()
	}
	return context.WithValue(ctx, key{}, trusted{u, t, r})
}
func FromContext(ctx context.Context) (string, string, string, bool) {
	v, ok := ctx.Value(key{}).(trusted)
	return v.UserID, v.TraceID, v.RequestID, ok && v.UserID != ""
}
func UnaryServerInterceptor(secret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		if strings.HasPrefix(info.FullMethod, "/grpc.health.v1.Health/") {
			return h(ctx, req)
		}
		md, _ := metadata.FromIncomingContext(ctx)
		a := md.Get("authorization")
		if len(a) != 1 || !strings.HasPrefix(a[0], "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}
		u, e := authclaims.ParseAccess(strings.TrimSpace(strings.TrimPrefix(a[0], "Bearer ")), []byte(secret), nil)
		if e != nil {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}
		if req == nil {
			return nil, status.Error(codes.InvalidArgument, "request required")
		}
		identified, ok := req.(interface{ GetUserId() string })
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "request user identity required")
		}
		if identified.GetUserId() != u {
			return nil, status.Error(codes.PermissionDenied, "permission denied")
		}
		trace, request := "", ""
		if x := md.Get("x-trace-id"); len(x) == 1 && correlationID.MatchString(x[0]) {
			trace = x[0]
		}
		if x := md.Get("x-request-id"); len(x) == 1 && correlationID.MatchString(x[0]) {
			request = x[0]
		}
		return h(WithTrusted(ctx, u, trace, request), req)
	}
}

func StreamServerInterceptor(secret string) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if info.FullMethod == "/grpc.health.v1.Health/Watch" {
			return handler(srv, stream)
		}
		md, _ := metadata.FromIncomingContext(stream.Context())
		values := md.Get("authorization")
		if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
			return status.Error(codes.Unauthenticated, "unauthenticated")
		}
		if _, err := authclaims.ParseAccess(strings.TrimSpace(strings.TrimPrefix(values[0], "Bearer ")), []byte(secret), nil); err != nil {
			return status.Error(codes.Unauthenticated, "unauthenticated")
		}
		return status.Error(codes.Unimplemented, "stream method unsupported")
	}
}
