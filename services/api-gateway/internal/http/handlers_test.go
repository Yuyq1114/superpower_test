package http

import (
	"bytes"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func token(t *testing.T, secret string, exp time.Time) string {
	t.Helper()
	v, e := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "trusted-user", "jti": "j", "iat": time.Now().Unix(), "exp": exp.Unix()}).SignedString([]byte(secret))
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestProtectedAuthErrorsAreUniformJSON(t *testing.T) {
	r := NewRouter(&Dependencies{JWTSecret: "secret"})
	for _, tok := range []string{"", token(t, "secret", time.Now().Add(-time.Hour)), token(t, "wrong", time.Now().Add(time.Hour))} {
		req := httptest.NewRequest("GET", "/api/v1/plans", nil)
		req.Header.Set("X-Request-ID", "req-1")
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("status/content-type=%d/%q body=%s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
		}
		var out map[string]any
		if json.Unmarshal(w.Body.Bytes(), &out) != nil || out["request_id"] != "req-1" {
			t.Fatalf("body=%s", w.Body.String())
		}
	}
}
func TestOversizedBodyReturnsUniform413(t *testing.T) {
	r := NewRouter(&Dependencies{})
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(make([]byte, (1<<20)+1)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", "req-big")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != nethttp.StatusRequestEntityTooLarge || !strings.Contains(w.Body.String(), `"request_id":"req-big"`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
func TestInvalidStatisticsPeriodReturns400(t *testing.T) {
	r := NewRouter(&Dependencies{JWTSecret: "secret"})
	req := httptest.NewRequest("GET", "/api/v1/statistics/summary?period=year", nil)
	req.Header.Set("Authorization", "Bearer "+token(t, "secret", time.Now().Add(time.Hour)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
