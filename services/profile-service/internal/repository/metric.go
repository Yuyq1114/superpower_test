package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/profile-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const DefaultSchema = "profile_schema"

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func quoteLiteral(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `''`) + `'`
}

func table(schema, name string) string {
	if schema == "" {
		schema = DefaultSchema
	}
	return quoteIdentifier(schema) + "." + quoteIdentifier(name)
}

func migrationSQL(schema string) []string {
	s := quoteIdentifier(schema)
	metrics := s + "." + quoteIdentifier("metrics")
	regclass := quoteLiteral(metrics) + "::regclass"
	return []string{
		"CREATE TABLE IF NOT EXISTS " + metrics + " (id text PRIMARY KEY,user_id text NOT NULL,metric_type text NOT NULL,value double precision NOT NULL,unit text NOT NULL,recorded_at timestamptz NOT NULL,idempotency_key text NOT NULL,request_fingerprint text NOT NULL DEFAULT '',created_at timestamptz NOT NULL,CONSTRAINT metrics_idempotency_key_length CHECK (char_length(idempotency_key) BETWEEN 1 AND 128))",
		"ALTER TABLE " + metrics + " ADD COLUMN IF NOT EXISTS idempotency_key text",
		"ALTER TABLE " + metrics + " ADD COLUMN IF NOT EXISTS request_fingerprint text NOT NULL DEFAULT ''",
		"DO $$ BEGIN IF EXISTS (SELECT 1 FROM " + metrics + " WHERE idempotency_key IS NULL OR char_length(idempotency_key) NOT BETWEEN 1 AND 128 LIMIT 1) THEN RAISE EXCEPTION 'profile metrics contain missing or invalid idempotency keys'; END IF; ALTER TABLE " + metrics + " ALTER COLUMN idempotency_key SET NOT NULL; IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname='metrics_idempotency_key_length' AND conrelid=" + regclass + ") THEN ALTER TABLE " + metrics + " DROP CONSTRAINT metrics_idempotency_key_length; END IF; ALTER TABLE " + metrics + " ADD CONSTRAINT metrics_idempotency_key_length CHECK (char_length(idempotency_key) BETWEEN 1 AND 128); END $$",
		"DO $$ BEGIN IF EXISTS (SELECT 1 FROM " + metrics + " WHERE NOT ((metric_type='weight' AND unit='kg' AND value > 0 AND value <= 500) OR (metric_type='body_fat' AND unit='percent' AND value >= 0 AND value <= 100)) LIMIT 1) THEN RAISE EXCEPTION 'profile metrics contain invalid type, unit, or value'; END IF; IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname='metrics_type_unit_range' AND conrelid=" + regclass + ") THEN ALTER TABLE " + metrics + " DROP CONSTRAINT metrics_type_unit_range; END IF; ALTER TABLE " + metrics + " ADD CONSTRAINT metrics_type_unit_range CHECK ((metric_type='weight' AND unit='kg' AND value > 0 AND value <= 500) OR (metric_type='body_fat' AND unit='percent' AND value >= 0 AND value <= 100)); END $$",
		"DROP INDEX IF EXISTS " + s + "." + quoteIdentifier("metrics_user_idempotency_unique"),
		"CREATE UNIQUE INDEX " + quoteIdentifier("metrics_user_idempotency_unique") + " ON " + metrics + "(user_id,idempotency_key)",
		"CREATE INDEX IF NOT EXISTS " + quoteIdentifier("metrics_user_recorded_at_idx") + " ON " + metrics + "(user_id,recorded_at DESC)",
	}
}

func Migrate(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).Exec("CREATE SCHEMA IF NOT EXISTS " + quoteIdentifier(DefaultSchema)).Error; err != nil {
		return fmt.Errorf("migration: %w", err)
	}
	return MigrateSchema(ctx, db, DefaultSchema)
}

func MigrateSchema(ctx context.Context, db *gorm.DB, schema string) error {
	if schema == "" {
		return fmt.Errorf("migration: schema is required")
	}
	for _, q := range migrationSQL(schema) {
		if e := db.WithContext(ctx).Exec(q).Error; e != nil {
			return fmt.Errorf("migration: %w", e)
		}
	}
	return nil
}

type Repository interface {
	Create(context.Context, *model.Metric) error
	List(context.Context, string, string, time.Time, time.Time) ([]model.Metric, error)
}

type GORM struct {
	DB     *gorm.DB
	Schema string
}

func (r GORM) Create(ctx context.Context, m *model.Metric) error {
	q := r.DB.WithContext(ctx).Table(table(r.Schema, "metrics"))
	x := q.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "idempotency_key"}}, DoNothing: true}).Create(m)
	if x.Error != nil {
		return x.Error
	}
	if x.RowsAffected == 0 {
		var old model.Metric
		if e := q.Where("user_id=? AND idempotency_key=?", m.UserID, m.IdempotencyKey).First(&old).Error; e != nil {
			return e
		}
		if old.RequestFingerprint != m.RequestFingerprint {
			return apperror.Conflict("idempotency key reused with different request")
		}
		*m = old
	}
	return nil
}

func (r GORM) List(ctx context.Context, u, t string, from, to time.Time) ([]model.Metric, error) {
	var out []model.Metric
	q := r.DB.WithContext(ctx).Table(table(r.Schema, "metrics")).Where("user_id=? AND recorded_at>=? AND recorded_at<=?", u, from, to)
	if t != "" {
		q = q.Where("metric_type=?", t)
	}
	e := q.Order("recorded_at DESC,id DESC").Find(&out).Error
	return out, e
}
