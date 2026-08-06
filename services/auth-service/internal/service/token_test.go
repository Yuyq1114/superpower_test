package service

import (
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func TestAccessClaimsAndExpiry(t *testing.T) {
	now := time.Now().UTC()
	m := NewTokenManager([]byte("0123456789abcdef0123456789abcdef"), time.Minute, time.Hour)
	m.now = func() time.Time { return now }
	p, h, e := m.Issue("u1")
	if e != nil || h == p.RefreshToken {
		t.Fatal("issue failed")
	}
	tok, e := jwt.Parse(p.AccessToken, func(*jwt.Token) (any, error) { return m.secret, nil })
	if e != nil {
		t.Fatal(e)
	}
	c := tok.Claims.(jwt.MapClaims)
	if c["sub"] != "u1" || c["jti"] == "" || int64(c["iat"].(float64)) != now.Unix() || int64(c["exp"].(float64)) != now.Add(time.Minute).Unix() {
		t.Fatalf("claims: %#v", c)
	}
}
func TestExpiredAccessRejected(t *testing.T) {
	m := NewTokenManager([]byte("0123456789abcdef0123456789abcdef"), -time.Minute, time.Hour)
	p, _, _ := m.Issue("u1")
	if _, e := m.ParseAccess(p.AccessToken); e == nil {
		t.Fatal("expired accepted")
	}
}
