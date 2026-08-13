package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/example/fitness-checkin/pkg/storage"
	"gorm.io/gorm"
)

// weeklyScheduleMigrationVersion identifies the move from dated workout days to
// ISO weekday slots (1=Monday .. 7=Sunday). It is recorded only after every
// statement of the migration transaction has succeeded, so a failed run leaves
// no half-migrated schema and can be retried once the data is fixed.
const weeklyScheduleMigrationVersion int64 = 2026081301

// WeeklyConflict describes one group of legacy workout days that would collapse
// onto the same weekday slot of the same plan.
type WeeklyConflict struct {
	UserID    string
	PlanID    string
	Weekday   int
	RecordIDs []string
}

// WeeklyMigrationConflictError aborts the migration instead of silently merging
// or dropping workout days; an operator must resolve the listed records first.
type WeeklyMigrationConflictError struct {
	Conflicts []WeeklyConflict
}

func (e *WeeklyMigrationConflictError) Error() string {
	groups := make([]string, len(e.Conflicts))
	for i, c := range e.Conflicts {
		groups[i] = fmt.Sprintf("plan %s weekday %d has %s", c.PlanID, c.Weekday, strings.Join(c.RecordIDs, ", "))
	}
	return "weekly schedule migration aborted: workout days map to a duplicate weekday (" + strings.Join(groups, "; ") + "); resolve them and rerun"
}

func migrateWeeklySchedule(ctx context.Context, db *gorm.DB, schema string) error {
	if err := storage.RequirePostgresSchema(ctx, db, schema); err != nil {
		return err
	}
	applied, err := weeklyMigrationApplied(ctx, db, schema)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}
	days, plans, items := table(schema, "workout_days"), table(schema, "plans"), table(schema, "workout_items")
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, stmt := range []string{
			`CREATE TABLE IF NOT EXISTS ` + table(schema, "schema_migrations") + ` (version bigint PRIMARY KEY,applied_at timestamptz NOT NULL)`,
			`ALTER TABLE ` + days + ` ADD COLUMN IF NOT EXISTS weekday integer`,
			`ALTER TABLE ` + days + ` ADD COLUMN IF NOT EXISTS title text`,
			`ALTER TABLE ` + plans + ` ADD COLUMN IF NOT EXISTS request_fingerprint text`,
			`ALTER TABLE ` + days + ` ALTER COLUMN workout_date DROP NOT NULL`,
			`UPDATE ` + days + ` SET weekday = EXTRACT(ISODOW FROM workout_date) WHERE weekday IS NULL AND workout_date IS NOT NULL`,
		} {
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("prepare weekly schedule columns: %w", err)
			}
		}
		conflicts, err := weeklyConflicts(tx, days)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			return &WeeklyMigrationConflictError{Conflicts: conflicts}
		}
		// An active plan must have a full schedule; anything incomplete goes
		// back to draft rather than staying active with missing exercises.
		demote := `UPDATE ` + plans + ` p SET status = 'draft', updated_at = ? WHERE p.status = 'active' AND (NOT EXISTS (SELECT 1 FROM ` + days + ` d WHERE d.plan_id = p.id) OR EXISTS (SELECT 1 FROM ` + days + ` d WHERE d.plan_id = p.id AND NOT EXISTS (SELECT 1 FROM ` + items + ` i WHERE i.workout_day_id = d.id)))`
		now := time.Now().UTC()
		if err := tx.Exec(demote, now).Error; err != nil {
			return fmt.Errorf("demote incomplete active plans: %w", err)
		}
		for _, stmt := range []string{
			`DROP INDEX IF EXISTS ` + table(schema, "plan_days_unique"),
			`ALTER TABLE ` + days + ` ADD CONSTRAINT workout_days_weekday_range CHECK (weekday IS NULL OR (weekday >= 1 AND weekday <= 7))`,
			`ALTER TABLE ` + days + ` ADD CONSTRAINT workout_days_user_plan_weekday_unique UNIQUE (user_id,plan_id,weekday)`,
		} {
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("apply weekly schedule constraints: %w", err)
			}
		}
		insert := `INSERT INTO ` + table(schema, "schema_migrations") + ` (version,applied_at) VALUES (?,?) ON CONFLICT (version) DO NOTHING`
		if err := tx.Exec(insert, weeklyScheduleMigrationVersion, now).Error; err != nil {
			return fmt.Errorf("record migration version %d: %w", weeklyScheduleMigrationVersion, err)
		}
		return nil
	})
}

func weeklyMigrationApplied(ctx context.Context, db *gorm.DB, schema string) (bool, error) {
	var historyExists bool
	query := `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = ? AND table_name = 'schema_migrations')`
	if err := db.WithContext(ctx).Raw(query, schema).Scan(&historyExists).Error; err != nil {
		return false, fmt.Errorf("inspect migration history: %w", err)
	}
	if !historyExists {
		return false, nil
	}
	var applied bool
	query = `SELECT EXISTS (SELECT 1 FROM ` + table(schema, "schema_migrations") + ` WHERE version = ?)`
	if err := db.WithContext(ctx).Raw(query, weeklyScheduleMigrationVersion).Scan(&applied).Error; err != nil {
		return false, fmt.Errorf("read migration history: %w", err)
	}
	return applied, nil
}

func weeklyConflicts(tx *gorm.DB, days string) ([]WeeklyConflict, error) {
	type conflictRow struct {
		UserID  string
		PlanID  string
		Weekday int
		ID      string
	}
	query := `SELECT d.user_id,d.plan_id,d.weekday,d.id FROM ` + days + ` d JOIN (SELECT user_id,plan_id,weekday FROM ` + days + ` WHERE weekday IS NOT NULL GROUP BY user_id,plan_id,weekday HAVING count(*) > 1) c ON c.user_id = d.user_id AND c.plan_id = d.plan_id AND c.weekday = d.weekday ORDER BY d.user_id,d.plan_id,d.weekday,d.id`
	var rows []conflictRow
	if err := tx.Raw(query).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("detect duplicate weekdays: %w", err)
	}
	var conflicts []WeeklyConflict
	for _, row := range rows {
		last := len(conflicts) - 1
		if last >= 0 && conflicts[last].UserID == row.UserID && conflicts[last].PlanID == row.PlanID && conflicts[last].Weekday == row.Weekday {
			conflicts[last].RecordIDs = append(conflicts[last].RecordIDs, row.ID)
			continue
		}
		conflicts = append(conflicts, WeeklyConflict{UserID: row.UserID, PlanID: row.PlanID, Weekday: row.Weekday, RecordIDs: []string{row.ID}})
	}
	return conflicts, nil
}
