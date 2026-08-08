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
	t.Setenv("HTTP_PORT", "18080")
	t.Setenv("GRPC_PORT", "19090")
	cfg, err := Load("checkin-service")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Service != "checkin-service" || cfg.RedisAddr != "localhost:6379" || cfg.HTTPPort != 18080 || cfg.GRPCPort != 19090 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadUsesDefaultsForInvalidPorts(t *testing.T) {
	t.Setenv("DB_DSN", "postgres://localhost/app")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("HTTP_PORT", "not-a-port")
	t.Setenv("GRPC_PORT", "-1")
	cfg, err := Load("auth-service")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPPort != 8080 || cfg.GRPCPort != 9090 {
		t.Fatalf("invalid ports should default: %+v", cfg)
	}
}

func TestLoadBuildsDSNFromSplitDatabaseConfiguration(t *testing.T) {
	t.Setenv("DB_DSN", "")
	t.Setenv("DB_HOST", "postgres")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "fitness")
	t.Setenv("DB_USER", "auth_service")
	t.Setenv("DB_PASSWORD", "p@ss word")
	t.Setenv("DB_SCHEMA", "auth_schema")
	t.Setenv("JWT_SECRET", "test-secret")
	cfg, err := Load("auth-service")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := "postgres://auth_service:p%40ss%20word@postgres:5432/fitness?search_path=auth_schema&sslmode=disable"
	if cfg.DBDSN != want {
		t.Fatalf("DBDSN = %q, want %q", cfg.DBDSN, want)
	}
}
