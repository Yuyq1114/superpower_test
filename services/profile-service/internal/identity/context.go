package identity

import (
	"context"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
	"time"
)

type trusted struct{ UserID, TraceID, RequestID string }
type key struct{}

func WithTrusted(ctx context.Context, u, t, r string) context.Context {
	if t == "" {
		t = uuid.NewString()
	}
	if r == "" {
		r = uuid.NewString()
	}
	return context.WithValue(ctx, key{}, trusted{u, t, r})
}
func FromContext(ctx context.Context) (string, string, string, bool) {
	v, ok := ctx.Value(key{}).(trusted)
	return v.UserID, v.TraceID, v.RequestID, ok && v.UserID != ""
}

type claims struct{ jwt.RegisteredClaims }

func UnaryServerInterceptor(secret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		a := md.Get("authorization")
		if len(a) != 1 || !strings.HasPrefix(a[0], "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}
		c := &claims{}
		tok, e := jwt.ParseWithClaims(strings.TrimSpace(strings.TrimPrefix(a[0], "Bearer ")), c, func(t *jwt.Token) (any, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, errors.New("invalid signing method")
			}
			return []byte(secret), nil
		}, jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithLeeway(time.Second))
		if e != nil || !tok.Valid || strings.TrimSpace(c.Subject) == "" {
			return nil, status.Error(codes.Unauthenticated, "unauthenticated")
		}
		return h(WithTrusted(ctx, c.Subject, "", ""), req)
	}
}
