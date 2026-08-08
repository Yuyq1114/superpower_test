package auth

import (
	"context"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/pkg/authclaims"
	"strings"
)

type contextKey string

const userIDKey contextKey = "gateway-user-id"

func UserID(ctx context.Context) string { v, _ := ctx.Value(userIDKey).(string); return v }
func Authenticate(ctx context.Context, header, secret string) (context.Context, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ctx, apperror.Unauthenticated("unauthenticated")
	}
	uid, err := authclaims.ParseAccess(parts[1], []byte(secret), nil)
	if err != nil {
		return ctx, apperror.Unauthenticated("unauthenticated")
	}
	return context.WithValue(ctx, userIDKey, uid), nil
}
