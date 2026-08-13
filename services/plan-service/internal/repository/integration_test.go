//go:build integration

package repository

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/example/fitness-checkin/pkg/config"
	"github.com/example/fitness-checkin/pkg/storage"
	"github.com/example/fitness-checkin/services/plan-service/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func openPostgres(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := storage.OpenPostgres(context.Background(), config.Config{DBDSN: dsn})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func integrationRepo(t *testing.T) (GORM, *gorm.DB) {
	t.Helper()
	adminDSN := os.Getenv("TEST_DATABASE_ADMIN_DSN")
	if adminDSN == "" {
		t.Fatal("TEST_DATABASE_ADMIN_DSN is required")
	}
	schema := "plan_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	admin := openPostgres(t, adminDSN)
	if err := admin.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
	})
	return GORM{DB: admin, Schema: schema}, admin
}

// migratedRepo isolates a schema and applies the production migration to it.
func migratedRepo(t *testing.T) GORM {
	t.Helper()
	repo, db := integrationRepo(t)
	if err := MigrateSchema(context.Background(), db, repo.Schema); err != nil {
		t.Fatalf("MigrateSchema: %v", err)
	}
	return repo
}

func TestIntegrationMigrateAndCreate(t *testing.T) {
	repo := migratedRepo(t)
	p := model.Plan{ID: uuid.NewString(), UserID: "u", Name: "test", Status: "draft"}
	if err := repo.CreatePlan(context.Background(), &p); err != nil {
		t.Fatal(err)
	}
}

// TestIntegrationCreatePlanSurvivesPostgresGenericPlanSwitch is a regression
// test for a real bug found via Task 9 E2E testing: CreatePlan's ON CONFLICT
// arbiter used a bound parameter (`clause.Neq{Column: "idempotency_key",
// Value: ""}`) for the partial index's WHERE predicate instead of a literal.
// PostgreSQL only replans a prepared statement's ON CONFLICT arbiter using
// the actual parameter value for its first 5 executions ("custom plan"); from
// the 6th execution on it reuses a "generic plan" that can't resolve a
// parameterized partial-index predicate, and fails with SQLSTATE 42P10 ("no
// unique or exclusion constraint matching the ON CONFLICT specification").
// Creating 7 plans with distinct non-empty idempotency keys on the same
// connection reproduces this reliably before the fix (using a literal
// `clause.Expr{SQL: "idempotency_key <> ''"}` instead, mirroring
// checkin-service's already-correct CreateWithEvent).
func TestIntegrationCreatePlanSurvivesPostgresGenericPlanSwitch(t *testing.T) {
	repo := migratedRepo(t)
	for i := 0; i < 7; i++ {
		p := model.Plan{ID: uuid.NewString(), UserID: "u", Name: "test", Status: "draft", IdempotencyKey: uuid.NewString()}
		if err := repo.CreatePlan(context.Background(), &p); err != nil {
			t.Fatalf("CreatePlan failed on attempt %d (PostgreSQL generic-plan ON CONFLICT arbiter bug): %v", i+1, err)
		}
	}
}

// TestIntegrationCreateDaySurvivesPostgresGenericPlanSwitch mirrors
// TestIntegrationCreatePlanSurvivesPostgresGenericPlanSwitch for CreateDay's
// identical ON CONFLICT arbiter pattern.
func TestIntegrationCreateDaySurvivesPostgresGenericPlanSwitch(t *testing.T) {
	repo := migratedRepo(t)
	plan := model.Plan{ID: uuid.NewString(), UserID: "u", Name: "test", Status: "draft"}
	if err := repo.CreatePlan(context.Background(), &plan); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		date := time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC)
		d := model.WorkoutDay{
			ID:             uuid.NewString(),
			UserID:         "u",
			PlanID:         plan.ID,
			IdempotencyKey: uuid.NewString(),
			Date:           &date,
			Weekday:        i + 1,
		}
		if err := repo.CreateDay(context.Background(), &d); err != nil {
			t.Fatalf("CreateDay failed on attempt %d (PostgreSQL generic-plan ON CONFLICT arbiter bug): %v", i+1, err)
		}
	}
}

// TestIntegrationCreateItemSurvivesPostgresGenericPlanSwitch mirrors
// TestIntegrationCreatePlanSurvivesPostgresGenericPlanSwitch for CreateItem's
// identical ON CONFLICT arbiter pattern.
func TestIntegrationCreateItemSurvivesPostgresGenericPlanSwitch(t *testing.T) {
	repo := migratedRepo(t)
	plan := model.Plan{ID: uuid.NewString(), UserID: "u", Name: "test", Status: "draft"}
	if err := repo.CreatePlan(context.Background(), &plan); err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day := model.WorkoutDay{ID: uuid.NewString(), UserID: "u", PlanID: plan.ID, Date: &date, Weekday: 4}
	if err := repo.CreateDay(context.Background(), &day); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		it := model.WorkoutItem{ID: uuid.NewString(), UserID: "u", WorkoutDayID: day.ID, IdempotencyKey: uuid.NewString(), Name: "squat"}
		if err := repo.CreateItem(context.Background(), &it); err != nil {
			t.Fatalf("CreateItem failed on attempt %d (PostgreSQL generic-plan ON CONFLICT arbiter bug): %v", i+1, err)
		}
	}
}
