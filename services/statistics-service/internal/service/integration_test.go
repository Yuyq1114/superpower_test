//go:build integration

package service

import (
	"context"
	"os"
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
		adminDSN = dsn
	}
	ctx := context.Background()
	adminDB, err := storage.OpenPostgres(ctx, config.Config{DBDSN: adminDSN})
	if err != nil {
		t.Fatalf("open admin database: %v", err)
	}
	if err = repository.Migrate(ctx, adminDB); err != nil {
		t.Fatalf("production Migrate: %v", err)
	}
	if err = adminDB.Exec(`TRUNCATE TABLE statistics_schema.active_days, statistics_schema.summaries, statistics_schema.processed_events CASCADE`).Error; err != nil {
		t.Fatal(err)
	}
	serviceDB, err := storage.OpenPostgres(ctx, config.Config{DBDSN: dsn})
	if err != nil {
		t.Fatalf("open service database: %v", err)
	}

	svc := New(repository.GORM{DB: serviceDB, Schema: repository.DefaultSchema})
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
	if err := serviceDB.Table(`statistics_schema.processed_events`).Where("event_id = ?", event.EventID).Count(&processed).Error; err != nil {
		t.Fatal(err)
	}
	if processed != 1 {
		t.Fatalf("processed_events rows = %d, want 1", processed)
	}
}
