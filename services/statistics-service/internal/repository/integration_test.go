//go:build integration

package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/storage"
	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
	"gorm.io/gorm"
)

func integrationDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
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
	closeIntegrationDB(t, adminDB)
	db, err := storage.OpenPostgres(ctx, config.Config{DBDSN: dsn})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	closeIntegrationDB(t, db)

	var role string
	if err = db.Raw("SELECT current_user").Scan(&role).Error; err != nil {
		t.Fatalf("identify service database role: %v", err)
	}
	schema := randomIntegrationSchema(t)
	quotedSchema, quotedRole := quoteIdentifier(schema), quoteIdentifier(role)
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
	if err = MigrateSchema(ctx, db, schema); err != nil {
		t.Fatalf("production MigrateSchema: %v", err)
	}
	return db, schema
}

func TestConsumeIsIdempotentAndUserIsolated(t *testing.T) {
	db, schema := integrationDB(t)
	r := GORM{DB: db, Schema: schema}
	ctx := context.Background()
	at := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	e := model.WorkoutCompleted{EventID: "e1", EventType: model.WorkoutCompletedType, UserID: "u1", CheckinID: "c1", CompletedAt: at, OccurredAt: at}
	if err := r.ConsumeWorkoutCompleted(ctx, e); err != nil {
		t.Fatal(err)
	}
	if err := r.ConsumeWorkoutCompleted(ctx, e); err != nil {
		t.Fatal(err)
	}
	e.EventID, e.CheckinID, e.CompletedAt = "e2", "c2", at.Add(time.Hour)
	if err := r.ConsumeWorkoutCompleted(ctx, e); err != nil {
		t.Fatal(err)
	}
	e.EventID, e.UserID, e.CheckinID = "e3", "u2", "c3"
	if err := r.ConsumeWorkoutCompleted(ctx, e); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	got, err := r.GetSummary(ctx, "u1", model.PeriodWeek, start, start.AddDate(0, 0, 7))
	if err != nil {
		t.Fatal(err)
	}
	if got.WorkoutCount != 2 || got.ActiveDays != 1 {
		t.Fatalf("u1 summary=%#v", got)
	}
	other, _ := r.GetSummary(ctx, "u2", model.PeriodWeek, start, start.AddDate(0, 0, 7))
	if other.WorkoutCount != 1 {
		t.Fatalf("u2 summary=%#v", other)
	}
}

func TestTransactionFailureRollsBackProcessedEvent(t *testing.T) {
	db, schema := integrationDB(t)
	r := GORM{DB: db, Schema: schema}
	ctx := context.Background()
	if err := db.Exec("ALTER TABLE " + table(schema, "summaries") + " ADD CONSTRAINT reject_test_user CHECK (user_id <> 'rollback-user')").Error; err != nil {
		t.Fatal(err)
	}
	e := model.WorkoutCompleted{EventID: "rollback-event", EventType: model.WorkoutCompletedType, UserID: "rollback-user", CheckinID: "c", CompletedAt: time.Now().UTC()}
	if err := r.ConsumeWorkoutCompleted(ctx, e); err == nil {
		t.Fatal("expected transaction failure")
	}
	var count int64
	if err := db.Table(table(schema, "processed_events")).Where("event_id=?", e.EventID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("processed event committed after rollback: %d", count)
	}
}

func TestMigrationObjectsStayInRequestedSchema(t *testing.T) {
	db, schema := integrationDB(t)
	var count int64
	if err := db.Raw(`SELECT count(*) FROM information_schema.tables WHERE table_schema = ? AND table_name IN ('processed_events','summaries','active_days')`, schema).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("found %d statistics tables in isolated schema, want 3", count)
	}
}

func randomIntegrationSchema(t *testing.T) string {
	t.Helper()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatalf("generate schema name: %v", err)
	}
	return "statistics_test_" + hex.EncodeToString(random[:])
}

func closeIntegrationDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}
