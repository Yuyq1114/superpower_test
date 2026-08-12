package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetRefreshCookieUsesSecureBrowserAttributes(t *testing.T) {
	w := httptest.NewRecorder()
	setRefreshCookie(w, RefreshCookieConfig{Name: "fitness_refresh", Secure: true}, "secret", 3600)
	cookie := w.Result().Cookies()[0]
	if cookie.Name != "fitness_refresh" || cookie.Value != "secret" || !cookie.HttpOnly || !cookie.Secure {
		t.Fatalf("unexpected cookie: %#v", cookie)
	}
	if cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api/v1/auth" || cookie.MaxAge != 3600 {
		t.Fatalf("unsafe cookie attributes: %#v", cookie)
	}
}

func TestRequireAllowedOriginRejectsMissingAndCrossOrigin(t *testing.T) {
	cfg := RefreshCookieConfig{AllowedOrigins: map[string]struct{}{"http://localhost:5173": {}}}
	for _, origin := range []string{"", "https://evil.example"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(""))
		req.Header.Set("Origin", origin)
		if err := requireAllowedOrigin(req, cfg); err == nil {
			t.Fatalf("origin %q was accepted", origin)
		}
	}
}
