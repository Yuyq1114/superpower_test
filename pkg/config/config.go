package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Service       string
	DBDSN         string
	RedisAddr     string
	RedisPassword string
	JWTSecret     string
	HTTPPort      int
	GRPCPort      int
}

func Load(service string) (Config, error) {
	cfg := Config{Service: service, DBDSN: databaseDSN(), RedisAddr: getenv("REDIS_ADDR", "localhost:6379"), RedisPassword: os.Getenv("REDIS_PASSWORD"), JWTSecret: os.Getenv("JWT_SECRET"), HTTPPort: port("HTTP_PORT", 8080), GRPCPort: port("GRPC_PORT", 9090)}
	if cfg.Service == "" {
		return Config{}, fmt.Errorf("service is required")
	}
	if cfg.DBDSN == "" {
		return Config{}, fmt.Errorf("DB_DSN is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func port(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func databaseDSN() string {
	if dsn := os.Getenv("DB_DSN"); dsn != "" {
		return dsn
	}
	host, name, user, password := os.Getenv("DB_HOST"), os.Getenv("DB_NAME"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD")
	if host == "" || name == "" || user == "" || password == "" {
		return ""
	}
	query := url.Values{"sslmode": {getenv("DB_SSLMODE", "disable")}}
	if schema := os.Getenv("DB_SCHEMA"); schema != "" {
		query.Set("search_path", schema)
	}
	u := &url.URL{Scheme: "postgres", User: url.UserPassword(user, password), Host: host + ":" + getenv("DB_PORT", "5432"), Path: "/" + strings.TrimPrefix(name, "/"), RawQuery: query.Encode()}
	return u.String()
}
