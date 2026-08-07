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

func WithRequestContext(ctx context.Context) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)
	trace, request := "", ""
	if x := md.Get("x-trace-id"); len(x) == 1 {
		trace = x[0]
	}
	if x := md.Get("x-request-id"); len(x) == 1 {
		request = x[0]
	}
	return WithTrusted(ctx, "", trace, request)
}

func WithTrusted(ctx context.Context, u, t, r string) context.Context {
	if !correlationID.MatchString(t) {
		t = uuid.NewString()
	}
	if !correlationID.MatchString(r) {
		r = uuid.NewString()
	}
	if current, ok := ctx.Value(key{}).(*trusted); ok {
		current.UserID, current.TraceID, current.RequestID = u, t, r
		return ctx
	}
	return context.WithValue(ctx, key{}, &trusted{u, t, r})
}
func FromContext(ctx context.Context) (string, string, string, bool) {
	v, ok := ctx.Value(key{}).(*trusted)
	if !ok || v == nil {
		return "", "", "", false
	}
	return v.UserID, v.TraceID, v.RequestID, v.UserID != ""
}
func UnaryServerInterceptor(secret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		a := md.Get("authorization")
		if len(a) != 1 || !strings.HasPrefix(a[0], "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}
		u, e := authclaims.ParseAccess(strings.TrimSpace(strings.TrimPrefix(a[0], "Bearer ")), []byte(secret), nil)
		if e != nil {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
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
