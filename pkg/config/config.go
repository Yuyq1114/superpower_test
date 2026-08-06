package config

import (
	"fmt"
	"os"
	"strconv"
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
	cfg := Config{Service: service, DBDSN: os.Getenv("DB_DSN"), RedisAddr: getenv("REDIS_ADDR", "localhost:6379"), RedisPassword: os.Getenv("REDIS_PASSWORD"), JWTSecret: os.Getenv("JWT_SECRET"), HTTPPort: port("HTTP_PORT", 8080), GRPCPort: port("GRPC_PORT", 9090)}
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
