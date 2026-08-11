package repository

import (
	"context"
	"github.com/example/fitness-checkin/services/plan-service/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"testing"
	"time"
)

func TestIntegrationMigrateAndCreate(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set; skipping PostgreSQL integration test")
	}
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "plan_test_" + uuid.NewString()[:8]
	old := DefaultSchema
	_ = old
	if e = db.Exec("CREATE SCHEMA \"" + schema + "\"").Error; e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Exec("DROP SCHEMA IF EXISTS \"" + schema + "\" CASCADE") })
	if e = MigrateSchema(context.Background(), db, schema); e != nil {
		t.Fatal(e)
	}
	p := model.Plan{ID: uuid.NewString(), UserID: "u", Name: "test", Status: "draft"}
	if e = (GORM{DB: db, Schema: schema}).CreatePlan(context.Background(), &p); e != nil {
		t.Fatal(e)
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
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set; skipping PostgreSQL integration test")
	}
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "plan_test_" + uuid.NewString()[:8]
	if e = db.Exec("CREATE SCHEMA \"" + schema + "\"").Error; e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Exec("DROP SCHEMA IF EXISTS \"" + schema + "\" CASCADE") })
	if e = MigrateSchema(context.Background(), db, schema); e != nil {
		t.Fatal(e)
	}
	repo := GORM{DB: db, Schema: schema}
	for i := 0; i < 7; i++ {
		p := model.Plan{ID: uuid.NewString(), UserID: "u", Name: "test", Status: "draft", IdempotencyKey: uuid.NewString()}
		if e = repo.CreatePlan(context.Background(), &p); e != nil {
			t.Fatalf("CreatePlan failed on attempt %d (PostgreSQL generic-plan ON CONFLICT arbiter bug): %v", i+1, e)
		}
	}
}

// TestIntegrationCreateDaySurvivesPostgresGenericPlanSwitch mirrors
// TestIntegrationCreatePlanSurvivesPostgresGenericPlanSwitch for CreateDay's
// identical ON CONFLICT arbiter pattern.
func TestIntegrationCreateDaySurvivesPostgresGenericPlanSwitch(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set; skipping PostgreSQL integration test")
	}
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "plan_test_" + uuid.NewString()[:8]
	if e = db.Exec("CREATE SCHEMA \"" + schema + "\"").Error; e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Exec("DROP SCHEMA IF EXISTS \"" + schema + "\" CASCADE") })
	if e = MigrateSchema(context.Background(), db, schema); e != nil {
		t.Fatal(e)
	}
	repo := GORM{DB: db, Schema: schema}
	plan := model.Plan{ID: uuid.NewString(), UserID: "u", Name: "test", Status: "draft"}
	if e = repo.CreatePlan(context.Background(), &plan); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 7; i++ {
		d := model.WorkoutDay{
			ID:             uuid.NewString(),
			UserID:         "u",
			PlanID:         plan.ID,
			IdempotencyKey: uuid.NewString(),
			Date:           time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC),
		}
		if e = repo.CreateDay(context.Background(), &d); e != nil {
			t.Fatalf("CreateDay failed on attempt %d (PostgreSQL generic-plan ON CONFLICT arbiter bug): %v", i+1, e)
		}
	}
}

// TestIntegrationCreateItemSurvivesPostgresGenericPlanSwitch mirrors
// TestIntegrationCreatePlanSurvivesPostgresGenericPlanSwitch for CreateItem's
// identical ON CONFLICT arbiter pattern.
func TestIntegrationCreateItemSurvivesPostgresGenericPlanSwitch(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set; skipping PostgreSQL integration test")
	}
	db, e := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	schema := "plan_test_" + uuid.NewString()[:8]
	if e = db.Exec("CREATE SCHEMA \"" + schema + "\"").Error; e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Exec("DROP SCHEMA IF EXISTS \"" + schema + "\" CASCADE") })
	if e = MigrateSchema(context.Background(), db, schema); e != nil {
		t.Fatal(e)
	}
	repo := GORM{DB: db, Schema: schema}
	plan := model.Plan{ID: uuid.NewString(), UserID: "u", Name: "test", Status: "draft"}
	if e = repo.CreatePlan(context.Background(), &plan); e != nil {
		t.Fatal(e)
	}
	day := model.WorkoutDay{ID: uuid.NewString(), UserID: "u", PlanID: plan.ID, Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	if e = repo.CreateDay(context.Background(), &day); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 7; i++ {
		it := model.WorkoutItem{ID: uuid.NewString(), UserID: "u", WorkoutDayID: day.ID, IdempotencyKey: uuid.NewString(), Name: "squat"}
		if e = repo.CreateItem(context.Background(), &it); e != nil {
			t.Fatalf("CreateItem failed on attempt %d (PostgreSQL generic-plan ON CONFLICT arbiter bug): %v", i+1, e)
		}
	}
}
