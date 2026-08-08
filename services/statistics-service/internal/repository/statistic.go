package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const DefaultSchema = "statistics_schema"

func table(schema, name string) string {
	if schema == "" {
		schema = DefaultSchema
	}
	return quoteIdentifier(schema) + "." + quoteIdentifier(name)
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func migrationSQL(schema string, createSchema bool) []string {
	s := quoteIdentifier(schema)
	queries := []string{
		"CREATE TABLE IF NOT EXISTS " + s + ".processed_events (event_id text PRIMARY KEY, processed_at timestamptz NOT NULL)",
		"CREATE TABLE IF NOT EXISTS " + s + ".summaries (user_id text NOT NULL, period text NOT NULL CHECK (period IN ('week','month')), bucket_start timestamptz NOT NULL, workout_count bigint NOT NULL DEFAULT 0 CHECK (workout_count >= 0), active_days bigint NOT NULL DEFAULT 0 CHECK (active_days >= 0), total_duration_seconds bigint NOT NULL DEFAULT 0 CHECK (total_duration_seconds >= 0), updated_at timestamptz NOT NULL, PRIMARY KEY(user_id,period,bucket_start))",
		"CREATE TABLE IF NOT EXISTS " + s + ".active_days (user_id text NOT NULL, period text NOT NULL CHECK (period IN ('week','month')), bucket_start timestamptz NOT NULL, activity_date date NOT NULL, PRIMARY KEY(user_id,period,bucket_start,activity_date), FOREIGN KEY(user_id,period,bucket_start) REFERENCES " + s + ".summaries(user_id,period,bucket_start) ON DELETE CASCADE)",
		"CREATE INDEX IF NOT EXISTS summaries_user_period_bucket_idx ON " + s + ".summaries(user_id,period,bucket_start)",
	}
	if createSchema {
		queries = append([]string{"CREATE SCHEMA IF NOT EXISTS " + s}, queries...)
	}
	return queries
}

func Migrate(ctx context.Context, db *gorm.DB) error {
	return migrateSchema(ctx, db, DefaultSchema, true)
}

func MigrateSchema(ctx context.Context, db *gorm.DB, schema string) error {
	return migrateSchema(ctx, db, schema, false)
}

func migrateSchema(ctx context.Context, db *gorm.DB, schema string, createSchema bool) error {
	for _, query := range migrationSQL(schema, createSchema) {
		if err := db.WithContext(ctx).Exec(query).Error; err != nil {
			return fmt.Errorf("statistics migration: %w", err)
		}
	}
	return nil
}

type GORM struct {
	DB     *gorm.DB
	Schema string
}

type processedEvent struct {
	EventID     string    `gorm:"column:event_id;primaryKey"`
	ProcessedAt time.Time `gorm:"column:processed_at"`
}
type activeDay struct {
	UserID       string       `gorm:"column:user_id"`
	Period       model.Period `gorm:"column:period"`
	BucketStart  time.Time    `gorm:"column:bucket_start"`
	ActivityDate time.Time    `gorm:"column:activity_date;type:date"`
}

func (r GORM) ConsumeWorkoutCompleted(ctx context.Context, event model.WorkoutCompleted) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		pe := processedEvent{EventID: event.EventID, ProcessedAt: time.Now().UTC()}
		result := tx.Table(table(r.Schema, "processed_events")).Clauses(clause.OnConflict{DoNothing: true}).Create(&pe)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		for _, period := range []model.Period{model.PeriodWeek, model.PeriodMonth} {
			bucket := bucketStart(period, event.CompletedAt)
			now := time.Now().UTC()
			summary := model.Aggregate{UserID: event.UserID, Period: period, BucketStart: bucket, WorkoutCount: 1}
			if err := tx.Table(table(r.Schema, "summaries")).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "period"}, {Name: "bucket_start"}}, DoUpdates: clause.Assignments(map[string]any{"workout_count": gorm.Expr(table(r.Schema, "summaries") + `."workout_count" + 1`), "updated_at": now})}).Create(map[string]any{"user_id": summary.UserID, "period": summary.Period, "bucket_start": summary.BucketStart, "workout_count": 1, "active_days": 0, "total_duration_seconds": 0, "updated_at": now}).Error; err != nil {
				return err
			}
			day := activeDay{UserID: event.UserID, Period: period, BucketStart: bucket, ActivityDate: dayStart(event.CompletedAt)}
			insert := tx.Table(table(r.Schema, "active_days")).Clauses(clause.OnConflict{DoNothing: true}).Create(&day)
			if insert.Error != nil {
				return insert.Error
			}
			if insert.RowsAffected == 1 {
				if err := tx.Table(table(r.Schema, "summaries")).Where("user_id=? AND period=? AND bucket_start=?", event.UserID, period, bucket).UpdateColumn("active_days", gorm.Expr("active_days + 1")).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r GORM) GetSummary(ctx context.Context, userID string, period model.Period, start, end time.Time) (model.Summary, error) {
	var row struct{ WorkoutCount, ActiveDays, TotalDurationSeconds int64 }
	err := r.DB.WithContext(ctx).Table(table(r.Schema, "summaries")).Select("COALESCE(SUM(workout_count),0) workout_count, COALESCE(SUM(active_days),0) active_days, COALESCE(SUM(total_duration_seconds),0) total_duration_seconds").Where("user_id=? AND period=? AND bucket_start>=? AND bucket_start<?", userID, period, start, end).Scan(&row).Error
	return model.Summary{UserID: userID, Period: period, Start: start, End: end, WorkoutCount: row.WorkoutCount, ActiveDays: row.ActiveDays, TotalDurationSeconds: row.TotalDurationSeconds}, err
}

func bucketStart(period model.Period, at time.Time) time.Time {
	at = at.UTC()
	if period == model.PeriodMonth {
		return time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	day := dayStart(at)
	return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
}
func dayStart(at time.Time) time.Time {
	at = at.UTC()
	return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
}
