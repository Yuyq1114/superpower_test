package storage

import (
	"context"
	"testing"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
)

func TestOpenPostgresReturnsConnectionError(t *testing.T) {
	cfg := config.Config{DBDSN: "postgres://invalid:invalid@127.0.0.1:1/invalid?sslmode=disable"}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := OpenPostgres(ctx, cfg); err == nil {
		t.Fatal("expected PostgreSQL connection error")
	}
}

func TestInitPostgresSchemasRequiresDatabase(t *testing.T) {
	if err := InitPostgresSchemas(context.Background(), nil); err == nil {
		t.Fatal("expected nil database error")
	}
}
