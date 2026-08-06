package authclaims

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"strings"
	"time"
)

type Claims struct {
	Subject string `json:"sub"`
	JWTID   string `json:"jti"`
	jwt.RegisteredClaims
}

func ParseAccess(raw string, secret []byte, now func() time.Time) (string, error) {
	if now == nil {
		now = time.Now
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("invalid signing algorithm")
		}
		return secret, nil
	}, jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithTimeFunc(now))
	if err != nil || !token.Valid || strings.TrimSpace(claims.Subject) == "" || strings.TrimSpace(claims.JWTID) == "" || claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return "", errors.New("invalid token")
	}
	n := now()
	if claims.IssuedAt.Time.After(n) || !claims.ExpiresAt.Time.After(claims.IssuedAt.Time) {
		return "", errors.New("invalid token")
	}
	return claims.Subject, nil
}
