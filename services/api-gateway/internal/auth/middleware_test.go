package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func TestAuthenticateRejectsMissingExpiredAndBadSignature(t *testing.T) {
	for _, tc := range []struct{ name, header string }{{"missing", ""}, {"expired", "Bearer " + signed(t, "secret", time.Now().Add(-time.Hour))}, {"bad signature", "Bearer " + signed(t, "wrong", time.Now().Add(time.Hour))}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Authenticate(t.Context(), tc.header, "secret"); err == nil {
				t.Fatal("expected unauthenticated")
			}
		})
	}
}
func TestAuthenticateInjectsUserIDFromTrustedClaims(t *testing.T) {
	ctx, err := Authenticate(t.Context(), "Bearer "+signed(t, "secret", time.Now().Add(time.Hour)), "secret")
	if err != nil {
		t.Fatal(err)
	}
	if got := UserID(ctx); got != "user-1" {
		t.Fatalf("user id=%q", got)
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
