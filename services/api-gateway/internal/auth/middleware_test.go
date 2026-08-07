package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiddlewareRejectsMissingExpiredAndBadSignature(t *testing.T) {
	for _, tc := range []struct {
		name  string
		token string
	}{
		{"missing", ""},
		{"expired", signed(t, "secret", time.Now().Add(-time.Hour))},
		{"bad signature", signed(t, "wrong", time.Now().Add(time.Hour))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			rec := httptest.NewRecorder()
			Middleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d", rec.Code)
			}
		})
	}
}

func TestMiddlewareInjectsUserIDFromTrustedClaims(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer "+signed(t, "secret", time.Now().Add(time.Hour)))
	rec := httptest.NewRecorder()
	Middleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := UserID(r.Context()); got != "user-1" {
			t.Fatalf("user id = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func signed(t *testing.T, secret string, exp time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user-1", "jti": "jti-1", "iat": time.Now().Unix(), "exp": exp.Unix()})
	v, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return v
}
