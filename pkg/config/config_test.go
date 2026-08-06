package config

import "testing"

func TestLoadRejectsMissingRequiredConfiguration(t *testing.T) {
	t.Setenv("DB_DSN", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("REDIS_ADDR", "")
	if _, err := Load("auth-service"); err == nil {
		t.Fatal("Load() error = nil, want missing required configuration error")
	}
}

func TestLoadUsesDefaultsAndEnvironment(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://localhost/app")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("REDIS_ADDR", "")
	cfg, err := Load("checkin-service")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Service != "checkin-service" || cfg.RedisAddr != "localhost:6379" || cfg.HTTPPort != 8080 || cfg.GRPCPort != 9090 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}
