//go:build integration

package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"
)

// createLegacyPlanSchema recreates the pre-weekday plan schema: a mandatory
// workout_date and the (plan_id, workout_date) uniqueness that the weekly
// model replaces.
func createLegacyPlanSchema(t *testing.T, db *gorm.DB, schema string) {
	t.Helper()
	q := `"` + schema + `"`
	for _, stmt := range []string{
		`CREATE TABLE ` + q + `.plans (id text PRIMARY KEY,user_id text NOT NULL,idempotency_key text NOT NULL DEFAULT '',name text NOT NULL,status text NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`,
		`CREATE TABLE ` + q + `.workout_days (id text PRIMARY KEY,user_id text NOT NULL,plan_id text NOT NULL REFERENCES ` + q + `.plans(id) ON DELETE CASCADE,idempotency_key text NOT NULL DEFAULT '',workout_date date NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`,
		`CREATE TABLE ` + q + `.workout_items (id text PRIMARY KEY,user_id text NOT NULL,workout_day_id text NOT NULL REFERENCES ` + q + `.workout_days(id) ON DELETE CASCADE,idempotency_key text NOT NULL DEFAULT '',name text NOT NULL,sets integer NOT NULL DEFAULT 0,repetitions integer NOT NULL DEFAULT 0,weight double precision NOT NULL DEFAULT 0,duration_seconds integer NOT NULL DEFAULT 0,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`,
		`CREATE UNIQUE INDEX plan_days_unique ON ` + q + `.workout_days(plan_id,workout_date)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create legacy schema: %v", err)
		}
	}
}

func insertLegacyActivePlan(t *testing.T, db *gorm.DB, schema, planID string) {
	t.Helper()
	stmt := `INSERT INTO ` + table(schema, "plans") + ` (id,user_id,idempotency_key,name,status,created_at,updated_at) VALUES (?,?,'',?,'active',now(),now()) ON CONFLICT (id) DO NOTHING`
	if err := db.Exec(stmt, planID, legacyUser(planID), planID).Error; err != nil {
		t.Fatalf("insert legacy plan %s: %v", planID, err)
	}
}

func insertLegacyDay(t *testing.T, db *gorm.DB, schema, planID, date string) string {
	t.Helper()
	dayID := planID + "-" + date
	stmt := `INSERT INTO ` + table(schema, "workout_days") + ` (id,user_id,plan_id,idempotency_key,workout_date,created_at,updated_at) VALUES (?,?,?,'',?::date,now(),now())`
	if err := db.Exec(stmt, dayID, legacyUser(planID), planID, date).Error; err != nil {
		t.Fatalf("insert legacy day %s: %v", dayID, err)
	}
	return dayID
}

func insertLegacyPlanDayAndItem(t *testing.T, db *gorm.DB, schema, planID, date string) {
	t.Helper()
	insertLegacyActivePlan(t, db, schema, planID)
	dayID := insertLegacyDay(t, db, schema, planID, date)
	stmt := `INSERT INTO ` + table(schema, "workout_items") + ` (id,user_id,workout_day_id,idempotency_key,name,sets,repetitions,weight,duration_seconds,created_at,updated_at) VALUES (?,?,?,'','squat',3,10,20,0,now(),now())`
	if err := db.Exec(stmt, dayID+"-item", legacyUser(planID), dayID).Error; err != nil {
		t.Fatalf("insert legacy item for %s: %v", dayID, err)
	}
}

// insertLegacyPlanDayWithoutItem models an active plan whose schedule is
// incomplete because one of its days has no exercises.
func insertLegacyPlanDayWithoutItem(t *testing.T, db *gorm.DB, schema, planID, date string) {
	t.Helper()
	insertLegacyActivePlan(t, db, schema, planID)
	insertLegacyDay(t, db, schema, planID, date)
}

func insertLegacyEmptyActivePlan(t *testing.T, db *gorm.DB, schema, planID string) {
	t.Helper()
	insertLegacyActivePlan(t, db, schema, planID)
}

func legacyUser(planID string) string { return "user-" + planID }

func assertDayWeekday(t *testing.T, db *gorm.DB, schema, planID string, want int) {
	t.Helper()
	var got []int
	if err := db.Raw(`SELECT weekday FROM `+table(schema, "workout_days")+` WHERE plan_id = ? ORDER BY id`, planID).Scan(&got).Error; err != nil {
		t.Fatalf("read weekday for plan %s: %v", planID, err)
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("plan %s weekdays = %v, want [%d]", planID, got, want)
	}
}

func assertPlanStatus(t *testing.T, db *gorm.DB, schema, planID, want string) {
	t.Helper()
	var got string
	if err := db.Raw(`SELECT status FROM `+table(schema, "plans")+` WHERE id = ?`, planID).Scan(&got).Error; err != nil {
		t.Fatalf("read status for plan %s: %v", planID, err)
	}
	if got != want {
		t.Fatalf("plan %s status = %q, want %q", planID, got, want)
	}
}

func assertColumnMissing(t *testing.T, db *gorm.DB, schema, tableName, column string) {
	t.Helper()
	var count int64
	if err := db.Raw(`SELECT count(*) FROM information_schema.columns WHERE table_schema = ? AND table_name = ? AND column_name = ?`, schema, tableName, column).Scan(&count).Error; err != nil {
		t.Fatalf("inspect %s.%s: %v", tableName, column, err)
	}
	if count != 0 {
		t.Fatalf("%s.%s exists after a failed migration; the transaction must roll back every statement", tableName, column)
	}
}

func TestWeeklyMigrationBackfillsISOWeekdayAndDemotesIncompleteActivePlan(t *testing.T) {
	repo, db := integrationRepo(t)
	createLegacyPlanSchema(t, db, repo.Schema)
	insertLegacyPlanDayAndItem(t, db, repo.Schema, "complete", "2026-08-10")
	insertLegacyEmptyActivePlan(t, db, repo.Schema, "empty")

	if err := migrateWeeklySchedule(context.Background(), db, repo.Schema); err != nil {
		t.Fatal(err)
	}

	assertDayWeekday(t, db, repo.Schema, "complete", 1)
	assertPlanStatus(t, db, repo.Schema, "complete", "active")
	assertPlanStatus(t, db, repo.Schema, "empty", "draft")
}

func TestWeeklyMigrationRejectsDuplicateMappedWeekdaysWithoutPartialWrites(t *testing.T) {
	repo, db := integrationRepo(t)
	createLegacyPlanSchema(t, db, repo.Schema)
	insertLegacyPlanDayAndItem(t, db, repo.Schema, "p1", "2026-08-10")
	insertLegacyPlanDayAndItem(t, db, repo.Schema, "p1", "2026-08-17")

	err := migrateWeeklySchedule(context.Background(), db, repo.Schema)
	var conflict *WeeklyMigrationConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want WeeklyMigrationConflictError", err)
	}
	if len(conflict.Conflicts) != 1 || conflict.Conflicts[0].PlanID != "p1" || conflict.Conflicts[0].Weekday != 1 {
		t.Fatalf("conflicts = %#v", conflict.Conflicts)
	}
	assertColumnMissing(t, db, repo.Schema, "workout_days", "weekday")
}

// The conflict report has to name the colliding records so an operator can fix
// the data before retrying; silently merging them would drop a workout day.
func TestWeeklyMigrationConflictNamesEveryCollidingRecord(t *testing.T) {
	repo, db := integrationRepo(t)
	createLegacyPlanSchema(t, db, repo.Schema)
	insertLegacyPlanDayAndItem(t, db, repo.Schema, "p1", "2026-08-10")
	insertLegacyPlanDayAndItem(t, db, repo.Schema, "p1", "2026-08-17")

	err := migrateWeeklySchedule(context.Background(), db, repo.Schema)
	var conflict *WeeklyMigrationConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want WeeklyMigrationConflictError", err)
	}
	want := []string{"p1-2026-08-10", "p1-2026-08-17"}
	got := conflict.Conflicts[0].RecordIDs
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("record IDs = %v, want %v", got, want)
	}
	for _, fragment := range []string{"p1", "weekday 1", "p1-2026-08-10", "p1-2026-08-17"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("error message %q must mention %q", err.Error(), fragment)
		}
	}
}

func TestWeeklyMigrationDemotesActivePlanWhoseDayHasNoItems(t *testing.T) {
	repo, db := integrationRepo(t)
	createLegacyPlanSchema(t, db, repo.Schema)
	insertLegacyPlanDayWithoutItem(t, db, repo.Schema, "half", "2026-08-11")

	if err := migrateWeeklySchedule(context.Background(), db, repo.Schema); err != nil {
		t.Fatal(err)
	}

	assertDayWeekday(t, db, repo.Schema, "half", 2)
	assertPlanStatus(t, db, repo.Schema, "half", "draft")
}

func TestWeeklyMigrationRecordsVersionAndSkipsSecondRun(t *testing.T) {
	repo, db := integrationRepo(t)
	createLegacyPlanSchema(t, db, repo.Schema)
	insertLegacyPlanDayAndItem(t, db, repo.Schema, "complete", "2026-08-16")

	if err := migrateWeeklySchedule(context.Background(), db, repo.Schema); err != nil {
		t.Fatal(err)
	}
	var version int64
	if err := db.Raw(`SELECT version FROM ` + table(repo.Schema, "schema_migrations")).Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if version != weeklyScheduleMigrationVersion {
		t.Fatalf("recorded version = %d, want %d", version, weeklyScheduleMigrationVersion)
	}

	// A second run must be a no-op instead of re-adding constraints.
	if err := migrateWeeklySchedule(context.Background(), db, repo.Schema); err != nil {
		t.Fatalf("second run: %v", err)
	}
	assertDayWeekday(t, db, repo.Schema, "complete", 7)
}

// The weekly uniqueness rule and the weekday range have to be enforced by the
// database, not only by the service layer.
func TestWeeklyMigrationEnforcesWeekdayConstraints(t *testing.T) {
	repo, db := integrationRepo(t)
	createLegacyPlanSchema(t, db, repo.Schema)
	insertLegacyPlanDayAndItem(t, db, repo.Schema, "p1", "2026-08-10")

	if err := migrateWeeklySchedule(context.Background(), db, repo.Schema); err != nil {
		t.Fatal(err)
	}

	insert := `INSERT INTO ` + table(repo.Schema, "workout_days") + ` (id,user_id,plan_id,idempotency_key,weekday,created_at,updated_at) VALUES (?,?,'p1','',?,now(),now())`
	if err := db.Exec(insert, "dup", legacyUser("p1"), 1).Error; err == nil {
		t.Fatal("a second Monday in the same plan must violate the (user_id, plan_id, weekday) unique constraint")
	}
	if err := db.Exec(insert, "out-of-range", legacyUser("p1"), 8).Error; err == nil {
		t.Fatal("weekday 8 must violate the ISO weekday range constraint")
	}
	var indexes int64
	if err := db.Raw(`SELECT count(*) FROM pg_indexes WHERE schemaname = ? AND indexname = 'plan_days_unique'`, repo.Schema).Scan(&indexes).Error; err != nil {
		t.Fatal(err)
	}
	if indexes != 0 {
		t.Fatal("plan_days_unique must be dropped: a weekly plan no longer keys days by date")
	}
}
