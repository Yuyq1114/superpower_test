package service

import (
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func signClaims(t *testing.T, m *TokenManager, claims jwt.MapClaims) string {
	t.Helper()
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

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

func TestAccessRequiresAllClaims(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	m := NewTokenManager([]byte("0123456789abcdef0123456789abcdef"), time.Minute, time.Hour)
	m.now = func() time.Time { return now }
	valid := jwt.MapClaims{"sub": "u1", "jti": "token-id", "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()}
	for _, name := range []string{"sub", "jti", "iat", "exp"} {
		claims := jwt.MapClaims{}
		for key, value := range valid {
			claims[key] = value
		}
		delete(claims, name)
		if _, err := m.ParseAccess(signClaims(t, m, claims)); err == nil {
			t.Fatalf("missing %s accepted", name)
		}
	}
}

func TestAccessRejectsInvalidClaimValuesAndTiming(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	m := NewTokenManager([]byte("0123456789abcdef0123456789abcdef"), time.Minute, time.Hour)
	m.now = func() time.Time { return now }
	tests := map[string]jwt.MapClaims{
		"empty subject":    {"sub": "", "jti": "id", "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()},
		"empty jwt id":     {"sub": "u1", "jti": "", "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()},
		"future issued at": {"sub": "u1", "jti": "id", "iat": now.Add(time.Minute).Unix(), "exp": now.Add(2 * time.Minute).Unix()},
		"reversed window":  {"sub": "u1", "jti": "id", "iat": now.Unix(), "exp": now.Add(-time.Minute).Unix()},
	}
	for name, claims := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := m.ParseAccess(signClaims(t, m, claims)); err == nil {
				t.Fatal("invalid claims accepted")
			}
		})
	}
}
