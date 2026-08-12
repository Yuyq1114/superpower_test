package http

import (
	"fmt"
	nethttp "net/http"
)

const defaultRefreshCookieName = "fitness_refresh"

type RefreshCookieConfig struct {
	Name           string
	Secure         bool
	AllowedOrigins map[string]struct{}
}

func (c RefreshCookieConfig) name() string {
	if c.Name == "" {
		return defaultRefreshCookieName
	}
	return c.Name
}

func setRefreshCookie(w nethttp.ResponseWriter, cfg RefreshCookieConfig, token string, ttlSeconds int64) {
	nethttp.SetCookie(w, &nethttp.Cookie{
		Name: cfg.name(), Value: token, Path: "/api/v1/auth",
		MaxAge: int(ttlSeconds), HttpOnly: true, Secure: cfg.Secure,
		SameSite: nethttp.SameSiteStrictMode,
	})
}

func clearRefreshCookie(w nethttp.ResponseWriter, cfg RefreshCookieConfig) {
	nethttp.SetCookie(w, &nethttp.Cookie{
		Name: cfg.name(), Value: "", Path: "/api/v1/auth",
		MaxAge: -1, HttpOnly: true, Secure: cfg.Secure,
		SameSite: nethttp.SameSiteStrictMode,
	})
}

func readRefreshCookie(r *nethttp.Request, cfg RefreshCookieConfig) (string, error) {
	cookie, err := r.Cookie(cfg.name())
	if err != nil || cookie.Value == "" {
		return "", fmt.Errorf("refresh cookie is required")
	}
	return cookie.Value, nil
}

func requireAllowedOrigin(r *nethttp.Request, cfg RefreshCookieConfig) error {
	origin := r.Header.Get("Origin")
	if _, ok := cfg.AllowedOrigins[origin]; !ok || origin == "" {
		return fmt.Errorf("origin is not allowed")
	}
	return nil
}
