//go:build integration

package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/storage"
	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
	"github.com/example/fitness-checkin/services/statistics-service/internal/repository"
)

func TestProductionServiceAndRepositoryDeduplicateReplayedEvent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set; skipping PostgreSQL integration test")
	}
	adminDSN := os.Getenv("TEST_DATABASE_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("TEST_DATABASE_ADMIN_DSN not set; PostgreSQL integration test requires an admin DSN to isolate schema")
	}
	ctx := context.Background()
	adminDB, err := storage.OpenPostgres(ctx, config.Config{DBDSN: adminDSN})
	if err != nil {
		t.Fatalf("open admin database: %v", err)
	}
	closeDB(t, adminDB)
	serviceDB, err := storage.OpenPostgres(ctx, config.Config{DBDSN: dsn})
	if err != nil {
		t.Fatalf("open service database: %v", err)
	}
	closeDB(t, serviceDB)

	var role string
	if err = serviceDB.Raw("SELECT current_user").Scan(&role).Error; err != nil {
		t.Fatalf("identify service database role: %v", err)
	}
	schema := randomTestSchema(t)
	quotedSchema := quoteTestIdentifier(schema)
	quotedRole := quoteTestIdentifier(role)
	if err = adminDB.Exec("CREATE SCHEMA " + quotedSchema + " AUTHORIZATION " + quotedRole).Error; err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		if err := adminDB.Exec("DROP SCHEMA IF EXISTS " + quotedSchema + " CASCADE").Error; err != nil {
			t.Errorf("drop isolated schema: %v", err)
		}
	})
	if err = adminDB.Exec("GRANT USAGE, CREATE ON SCHEMA " + quotedSchema + " TO " + quotedRole).Error; err != nil {
		t.Fatalf("grant isolated schema access: %v", err)
	}
	if err = repository.MigrateSchema(ctx, serviceDB, schema); err != nil {
		t.Fatalf("production MigrateSchema: %v", err)
	}

	svc := New(repository.GORM{DB: serviceDB, Schema: schema})
	at := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	event := model.WorkoutCompleted{EventID: "production-replay-event", EventType: model.WorkoutCompletedType, UserID: "production-replay-user", CheckinID: "production-replay-checkin", CompletedAt: at, OccurredAt: at}
	for i := 0; i < 2; i++ {
		if err := svc.ConsumeWorkoutCompleted(ctx, event); err != nil {
			t.Fatalf("consume replay %d: %v", i+1, err)
		}
	}

	summary, err := svc.GetSummary(ctx, event.UserID, model.PeriodWeek, at, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if summary.WorkoutCount != 1 || summary.ActiveDays != 1 {
		t.Fatalf("summary after replay = %#v", summary)
	}
	var processed int64
	if err := serviceDB.Table(quotedSchema+`."processed_events"`).Where("event_id = ?", event.EventID).Count(&processed).Error; err != nil {
		t.Fatal(err)
	}
	if processed != 1 {
		t.Fatalf("processed_events rows = %d, want 1", processed)
	}
}

func randomTestSchema(t *testing.T) string {
	t.Helper()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatalf("generate schema name: %v", err)
	}
	return "statistics_test_" + hex.EncodeToString(random[:])
}

func quoteTestIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func closeDB(t *testing.T, db interface{ DB() (*sql.DB, error) }) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}
