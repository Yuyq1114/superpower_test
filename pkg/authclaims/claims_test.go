package authclaims

import (
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func sign(t *testing.T, secret []byte, c jwt.MapClaims) string {
	t.Helper()
	raw, e := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(secret)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func TestParseAccessMatchesAuthContract(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Now().UTC().Truncate(time.Second)
	valid := jwt.MapClaims{"sub": "u1", "jti": "id", "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()}
	if u, e := ParseAccess(sign(t, secret, valid), secret, func() time.Time { return now }); e != nil || u != "u1" {
		t.Fatal(u, e)
	}
	delete(valid, "jti")
	if _, e := ParseAccess(sign(t, secret, valid), secret, func() time.Time { return now }); e == nil {
		t.Fatal("missing jti accepted")
	}
}
