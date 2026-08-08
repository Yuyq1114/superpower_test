package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"github.com/example/fitness-checkin/pkg/authclaims"
	"github.com/golang-jwt/jwt/v5"
	"strings"
	"time"
)

type TokenPair struct {
	AccessToken, RefreshToken         string
	AccessExpiresIn, RefreshExpiresIn int64
}
type TokenManager struct {
	secret                []byte
	accessTTL, refreshTTL time.Duration
	now                   func() time.Time
}

func NewTokenManager(s []byte, a, r time.Duration) *TokenManager {
	return &TokenManager{secret: s, accessTTL: a, refreshTTL: r, now: time.Now}
}
func (m *TokenManager) Issue(uid string) (TokenPair, string, error) {
	return m.IssueAt(uid, m.now().UTC())
}

func (m *TokenManager) IssueAt(uid string, now time.Time) (TokenPair, string, error) {
	j := make([]byte, 16)
	if _, e := rand.Read(j); e != nil {
		return TokenPair{}, "", e
	}
	claims := jwt.MapClaims{"sub": uid, "jti": base64.RawURLEncoding.EncodeToString(j), "iat": now.Unix(), "exp": now.Add(m.accessTTL).Unix()}
	tok, e := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if e != nil {
		return TokenPair{}, "", e
	}
	r := make([]byte, 32)
	if _, e = rand.Read(r); e != nil {
		return TokenPair{}, "", e
	}
	raw := base64.RawURLEncoding.EncodeToString(r)
	return TokenPair{tok, raw, int64(m.accessTTL.Seconds()), int64(m.refreshTTL.Seconds())}, HashRefreshToken(raw), nil
}
func HashRefreshToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func (m *TokenManager) ParseAccess(raw string) (string, error) {
	return authclaims.ParseAccess(raw, m.secret, m.now)
}
func validEmail(e string) bool {
	return strings.Contains(e, "@") && !strings.ContainsAny(e, " \t\n") && strings.LastIndex(e, ".") > strings.Index(e, "@")
}
