package auth

import (
	"context"
	"github.com/example/fitness-checkin/pkg/authclaims"
	"net/http"
	"strings"
)

type contextKey string

const userIDKey contextKey = "gateway-user-id"

func UserID(ctx context.Context) string { v, _ := ctx.Value(userIDKey).(string); return v }
func Middleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := strings.TrimSpace(r.Header.Get("Authorization"))
			parts := strings.SplitN(raw, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				http.Error(w, `{"code":"UNAUTHENTICATED","message":"unauthenticated"}`, http.StatusUnauthorized)
				return
			}
			uid, err := authclaims.ParseAccess(strings.TrimSpace(parts[1]), []byte(secret), nil)
			if err != nil {
				http.Error(w, `{"code":"UNAUTHENTICATED","message":"unauthenticated"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey, uid)))
		})
	}
}
