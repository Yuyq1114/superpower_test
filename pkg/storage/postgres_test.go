package storage

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
	"gorm.io/gorm"
)

func TestOpenPostgresUsesBoundedSinglePing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := OpenPostgres(ctx, config.Config{DBDSN: "postgres://invalid:invalid@192.0.2.1:5432/invalid?sslmode=disable"})
	if err == nil || !strings.Contains(err.Error(), "ping PostgreSQL") {
		t.Fatalf("expected bounded ping error, got %v", err)
	}
}

func TestInitPostgresSchemasRequiresDatabase(t *testing.T) {
	if err := InitPostgresSchemas(context.Background(), nil); err == nil {
		t.Fatal("expected nil database error")
	}
}

func TestInitPostgresSchemasRequiresTargetSchema(t *testing.T) {
	if err := InitPostgresSchemas(context.Background(), &gorm.DB{}, PostgresSchemaTarget{Role: "auth_service"}); err == nil {
		t.Fatal("expected schema validation error")
	}
}

func TestPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set; skipping PostgreSQL integration test")
	}
	db, err := OpenPostgres(context.Background(), config.Config{DBDSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	if err := InitPostgresSchemas(context.Background(), db); err != nil {
		t.Fatalf("first schema initialization: %v", err)
	}
	if err := InitPostgresSchemas(context.Background(), db); err != nil {
		t.Fatalf("second schema initialization: %v", err)
	}

	for _, schema := range defaultPostgresSchemas {
		exists, err := PostgresSchemaExists(context.Background(), db, schema)
		if err != nil {
			t.Fatalf("query schema %s: %v", schema, err)
		}
		if !exists {
			t.Errorf("schema %s does not exist after initialization", schema)
		}
	}
}
