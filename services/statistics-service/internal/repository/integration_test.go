package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/storage"
	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
	"gorm.io/gorm"
)

func integrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set; skipping PostgreSQL integration test")
	}
	db, err := storage.OpenPostgres(context.Background(), config.Config{DBDSN: dsn})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err = Migrate(context.Background(), db); err != nil {
		t.Fatalf("production Migrate: %v", err)
	}
	if err = db.Exec(`TRUNCATE TABLE statistics_schema.active_days, statistics_schema.summaries, statistics_schema.processed_events CASCADE`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestConsumeIsIdempotentAndUserIsolated(t *testing.T) {
	db := integrationDB(t)
	r := GORM{DB: db, Schema: DefaultSchema}
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
	db := integrationDB(t)
	r := GORM{DB: db, Schema: DefaultSchema}
	ctx := context.Background()
	if err := db.Exec(`ALTER TABLE statistics_schema.summaries ADD CONSTRAINT reject_test_user CHECK (user_id <> 'rollback-user')`).Error; err != nil {
		t.Fatal(err)
	}
	defer db.Exec(`ALTER TABLE statistics_schema.summaries DROP CONSTRAINT IF EXISTS reject_test_user`)
	e := model.WorkoutCompleted{EventID: "rollback-event", EventType: model.WorkoutCompletedType, UserID: "rollback-user", CheckinID: "c", CompletedAt: time.Now().UTC()}
	if err := r.ConsumeWorkoutCompleted(ctx, e); err == nil {
		t.Fatal("expected transaction failure")
	}
	var count int64
	if err := db.Table(table(DefaultSchema, "processed_events")).Where("event_id=?", e.EventID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("processed event committed after rollback: %d", count)
	}
}

func TestMigrationObjectsStayInStatisticsSchema(t *testing.T) {
	db := integrationDB(t)
	var count int64
	if err := db.Raw(`SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('processed_events','summaries','active_days')`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("found %d statistics tables in public schema", count)
	}
}
