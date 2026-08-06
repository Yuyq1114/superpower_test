//go:build integration

package repository

import (
	"context"
	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/storage"
	"os"
	"testing"
)

func TestProductionMigrationRequiresDatabase(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set; skipping PostgreSQL integration test")
	}
	db, e := storage.OpenPostgres(context.Background(), config.Config{DBDSN: dsn})
	if e != nil {
		t.Fatal(e)
	}
	if e = Migrate(context.Background(), db); e != nil {
		t.Fatal(e)
	}
}
