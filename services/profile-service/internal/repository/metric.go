package repository

import (
	"context"
	"fmt"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/profile-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const DefaultSchema = "profile_schema"

func table(schema, name string) string {
	if schema == "" {
		schema = DefaultSchema
	}
	return `"` + schema + `"."` + name + `"`
}
func Migrate(ctx context.Context, db *gorm.DB) error {
	s := `"` + DefaultSchema + `"`
	qs := []string{
		`CREATE SCHEMA IF NOT EXISTS ` + s,
		`CREATE TABLE IF NOT EXISTS ` + s + `.metrics (id text PRIMARY KEY,user_id text NOT NULL,metric_type text NOT NULL,value double precision NOT NULL,unit text NOT NULL,recorded_at timestamptz NOT NULL,idempotency_key text NOT NULL DEFAULT '',request_fingerprint text NOT NULL DEFAULT '',created_at timestamptz NOT NULL,CONSTRAINT metrics_type_unit_range CHECK ((metric_type='weight' AND unit='kg' AND value > 0 AND value <= 500) OR (metric_type='body_fat' AND unit='percent' AND value >= 0 AND value <= 100)))`,
		`ALTER TABLE ` + s + `.metrics ADD COLUMN IF NOT EXISTS idempotency_key text NOT NULL DEFAULT ''`, `ALTER TABLE ` + s + `.metrics ADD COLUMN IF NOT EXISTS request_fingerprint text NOT NULL DEFAULT ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS metrics_user_idempotency_unique ON ` + s + `.metrics(user_id,idempotency_key) WHERE idempotency_key <> ''`, `CREATE INDEX IF NOT EXISTS metrics_user_recorded_at_idx ON ` + s + `.metrics(user_id,recorded_at DESC)`}
	for _, q := range qs {
		if err := db.WithContext(ctx).Exec(q).Error; err != nil {
			return fmt.Errorf("migration: %w", err)
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
	if m.IdempotencyKey == "" {
		return q.Create(m).Error
	}
	x := q.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "idempotency_key"}}, DoNothing: true, TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Neq{Column: "idempotency_key", Value: ""}}}}).Create(m)
	if x.Error != nil {
		return x.Error
	}
	if x.RowsAffected == 0 {
		var old model.Metric
		if err := q.Where("user_id=? AND idempotency_key=?", m.UserID, m.IdempotencyKey).First(&old).Error; err != nil {
			return err
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
