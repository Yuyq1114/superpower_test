package repository

import (
	"context"
	"fmt"
	"github.com/example/fitness-checkin/services/plan-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const DefaultSchema = "plan_schema"

func table(s, t string) string {
	if s == "" {
		s = DefaultSchema
	}
	return `"` + s + `"."` + t + `"`
}
func Migrate(ctx context.Context, db *gorm.DB) error { return migrateSchema(ctx, db, DefaultSchema) }

func migrateSchema(ctx context.Context, db *gorm.DB, schema string) error {
	q := `"` + schema + `"`
	for _, x := range []string{`CREATE SCHEMA IF NOT EXISTS ` + q, `CREATE TABLE IF NOT EXISTS ` + q + `.plans (id text PRIMARY KEY,user_id text NOT NULL,idempotency_key text NOT NULL DEFAULT '',name text NOT NULL,status text NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`, `CREATE TABLE IF NOT EXISTS ` + q + `.workout_days (id text PRIMARY KEY,user_id text NOT NULL,plan_id text NOT NULL REFERENCES ` + q + `.plans(id) ON DELETE CASCADE,idempotency_key text NOT NULL DEFAULT '',workout_date date NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`, `CREATE TABLE IF NOT EXISTS ` + q + `.workout_items (id text PRIMARY KEY,user_id text NOT NULL,workout_day_id text NOT NULL REFERENCES ` + q + `.workout_days(id) ON DELETE CASCADE,idempotency_key text NOT NULL DEFAULT '',name text NOT NULL,sets integer NOT NULL DEFAULT 0,repetitions integer NOT NULL DEFAULT 0,weight double precision NOT NULL DEFAULT 0,duration_seconds integer NOT NULL DEFAULT 0,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`, `ALTER TABLE ` + q + `.plans ADD COLUMN IF NOT EXISTS idempotency_key text NOT NULL DEFAULT ''`, `ALTER TABLE ` + q + `.workout_days ADD COLUMN IF NOT EXISTS idempotency_key text NOT NULL DEFAULT ''`, `ALTER TABLE ` + q + `.workout_items ADD COLUMN IF NOT EXISTS idempotency_key text NOT NULL DEFAULT ''`, `CREATE UNIQUE INDEX IF NOT EXISTS plan_days_unique ON ` + q + `.workout_days(plan_id,workout_date)`, `CREATE UNIQUE INDEX IF NOT EXISTS plans_idempotency_unique ON ` + q + `.plans(user_id,idempotency_key) WHERE idempotency_key <> ''`, `DROP INDEX IF EXISTS ` + q + `.workout_days_idempotency_unique`, `CREATE UNIQUE INDEX IF NOT EXISTS workout_days_idempotency_unique ON ` + q + `.workout_days(user_id,plan_id,idempotency_key) WHERE idempotency_key <> ''`, `DROP INDEX IF EXISTS ` + q + `.workout_items_idempotency_unique`, `CREATE UNIQUE INDEX IF NOT EXISTS workout_items_idempotency_unique ON ` + q + `.workout_items(user_id,workout_day_id,idempotency_key) WHERE idempotency_key <> ''`, `CREATE INDEX IF NOT EXISTS plans_user_id_idx ON ` + q + `.plans(user_id)`, `CREATE INDEX IF NOT EXISTS workout_days_user_plan_idx ON ` + q + `.workout_days(user_id,plan_id)`, `CREATE INDEX IF NOT EXISTS workout_items_user_day_idx ON ` + q + `.workout_items(user_id,workout_day_id)`} {
		if e := db.WithContext(ctx).Exec(x).Error; e != nil {
			return fmt.Errorf("migration: %w", e)
		}
	}
	return nil
}

type GORM struct {
	DB     *gorm.DB
	Schema string
}

func (r GORM) CreatePlan(c context.Context, p *model.Plan) error {
	if p.IdempotencyKey == "" {
		return r.DB.WithContext(c).Table(table(r.Schema, "plans")).Create(p).Error
	}
	result := r.DB.WithContext(c).Table(table(r.Schema, "plans")).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "idempotency_key"}}, DoNothing: true, TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Neq{Column: "idempotency_key", Value: ""}}}}).Create(p)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return r.DB.WithContext(c).Table(table(r.Schema, "plans")).Where("user_id=? AND idempotency_key=?", p.UserID, p.IdempotencyKey).First(p).Error
	}
	return nil
}
func (r GORM) GetPlan(c context.Context, u, id string) (model.Plan, error) {
	var p model.Plan
	e := r.DB.WithContext(c).Table(table(r.Schema, "plans")).Where("user_id = ? AND id = ?", u, id).First(&p).Error
	return p, e
}
func (r GORM) UpdatePlan(c context.Context, p *model.Plan) error {
	return r.DB.WithContext(c).Table(table(r.Schema, "plans")).Save(p).Error
}
func (r GORM) DeletePlan(c context.Context, u, id string) error {
	result := r.DB.WithContext(c).Table(table(r.Schema, "plans")).Where("user_id = ? AND id = ?", u, id).Delete(&model.Plan{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r GORM) ListPlans(c context.Context, u string, page, size int) ([]model.Plan, int64, error) {
	var p []model.Plan
	var n int64
	q := r.DB.WithContext(c).Table(table(r.Schema, "plans")).Where("user_id = ?", u)
	if e := q.Count(&n).Error; e != nil {
		return nil, 0, e
	}
	e := q.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&p).Error
	return p, n, e
}
func (r GORM) CreateDay(c context.Context, d *model.WorkoutDay) error {
	if d.IdempotencyKey == "" {
		return r.DB.WithContext(c).Table(table(r.Schema, "workout_days")).Create(d).Error
	}
	result := r.DB.WithContext(c).Table(table(r.Schema, "workout_days")).Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "user_id"}, {Name: "plan_id"}, {Name: "idempotency_key"}},
		DoNothing:   true,
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Neq{Column: "idempotency_key", Value: ""}}},
	}).Create(d)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return r.DB.WithContext(c).Table(table(r.Schema, "workout_days")).Where("user_id=? AND plan_id=? AND idempotency_key=?", d.UserID, d.PlanID, d.IdempotencyKey).First(d).Error
	}
	return nil
}
func (r GORM) ListDays(c context.Context, u, pid string, page, size int) ([]model.WorkoutDay, int64, error) {
	var out []model.WorkoutDay
	var n int64
	q := r.DB.WithContext(c).Table(table(r.Schema, "workout_days")).Where("user_id = ? AND plan_id = ?", u, pid)
	if e := q.Count(&n).Error; e != nil {
		return nil, 0, e
	}
	e := q.Order("workout_date ASC").Offset((page - 1) * size).Limit(size).Find(&out).Error
	return out, n, e
}
func (r GORM) GetDay(c context.Context, u, pid, id string) (model.WorkoutDay, error) {
	var d model.WorkoutDay
	q := r.DB.WithContext(c).Table(table(r.Schema, "workout_days")).Where("user_id = ? AND id = ?", u, id)
	if pid != "" {
		q = q.Where("plan_id = ?", pid)
	}
	e := q.First(&d).Error
	return d, e
}
func (r GORM) UpdateDay(c context.Context, d *model.WorkoutDay) error {
	return r.DB.WithContext(c).Table(table(r.Schema, "workout_days")).Save(d).Error
}
func (r GORM) DeleteDay(c context.Context, u, pid, id string) error {
	result := r.DB.WithContext(c).Table(table(r.Schema, "workout_days")).Where("user_id = ? AND plan_id = ? AND id = ?", u, pid, id).Delete(&model.WorkoutDay{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r GORM) CreateItem(c context.Context, i *model.WorkoutItem) error {
	if i.IdempotencyKey == "" {
		return r.DB.WithContext(c).Table(table(r.Schema, "workout_items")).Create(i).Error
	}
	result := r.DB.WithContext(c).Table(table(r.Schema, "workout_items")).Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "user_id"}, {Name: "workout_day_id"}, {Name: "idempotency_key"}},
		DoNothing:   true,
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Neq{Column: "idempotency_key", Value: ""}}},
	}).Create(i)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return r.DB.WithContext(c).Table(table(r.Schema, "workout_items")).Where("user_id=? AND workout_day_id=? AND idempotency_key=?", i.UserID, i.WorkoutDayID, i.IdempotencyKey).First(i).Error
	}
	return nil
}
func (r GORM) ListItems(c context.Context, u, did string, page, size int) ([]model.WorkoutItem, int64, error) {
	var out []model.WorkoutItem
	var n int64
	q := r.DB.WithContext(c).Table(table(r.Schema, "workout_items")).Where("user_id = ? AND workout_day_id = ?", u, did)
	if e := q.Count(&n).Error; e != nil {
		return nil, 0, e
	}
	e := q.Order("created_at ASC").Offset((page - 1) * size).Limit(size).Find(&out).Error
	return out, n, e
}
func (r GORM) GetItem(c context.Context, u, did, id string) (model.WorkoutItem, error) {
	var i model.WorkoutItem
	e := r.DB.WithContext(c).Table(table(r.Schema, "workout_items")).Where("user_id = ? AND workout_day_id = ? AND id = ?", u, did, id).First(&i).Error
	return i, e
}
func (r GORM) UpdateItem(c context.Context, i *model.WorkoutItem) error {
	return r.DB.WithContext(c).Table(table(r.Schema, "workout_items")).Save(i).Error
}
func (r GORM) DeleteItem(c context.Context, u, did, id string) error {
	result := r.DB.WithContext(c).Table(table(r.Schema, "workout_items")).Where("user_id = ? AND workout_day_id = ? AND id = ?", u, did, id).Delete(&model.WorkoutItem{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
