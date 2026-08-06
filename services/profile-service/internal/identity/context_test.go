package identity

import (
	"context"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

func token(t *testing.T, secret, user string) string {
	t.Helper()
	raw, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: user, IssuedAt: jwt.NewNumericDate(time.Now().Add(-time.Second)), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}).SignedString([]byte(secret))
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func TestInterceptorRejectsForgedMetadataAndInjectsVerifiedSubject(t *testing.T) {
	i := UnaryServerInterceptor("secret")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("user_id", "forged", "x-trace-id", "forged-trace", "x-request-id", "forged-request", "authorization", "Bearer "+token(t, "secret", "trusted")))
	_, e := i(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		u, trace, request, ok := FromContext(ctx)
		if !ok || u != "trusted" || trace == "forged-trace" || request == "forged-request" {
			t.Fatal("untrusted identity")
		}
		return nil, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	bad := metadata.NewIncomingContext(context.Background(), metadata.Pairs("user_id", "forged"))
	_, e = i(bad, nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(e) != codes.Unauthenticated {
		t.Fatal(e)
	}
}
