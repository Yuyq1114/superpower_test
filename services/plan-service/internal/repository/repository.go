package repository

import (
	"context"
	"fmt"
	"github.com/example/fitness-checkin/services/plan-service/internal/model"
	"gorm.io/gorm"
)

const DefaultSchema = "plan_schema"

func table(s, t string) string {
	if s == "" {
		s = DefaultSchema
	}
	return `"` + s + `"."` + t + `"`
}
func Migrate(ctx context.Context, db *gorm.DB) error {
	q := `"` + DefaultSchema + `"`
	for _, x := range []string{`CREATE SCHEMA IF NOT EXISTS ` + q, `CREATE TABLE IF NOT EXISTS ` + q + `.plans (id text PRIMARY KEY,user_id text NOT NULL,name text NOT NULL,status text NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`, `CREATE TABLE IF NOT EXISTS ` + q + `.workout_days (id text PRIMARY KEY,user_id text NOT NULL,plan_id text NOT NULL REFERENCES ` + q + `.plans(id) ON DELETE CASCADE,workout_date date NOT NULL,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`, `CREATE UNIQUE INDEX IF NOT EXISTS plan_days_unique ON ` + q + `.workout_days(plan_id,workout_date)`, `CREATE TABLE IF NOT EXISTS ` + q + `.workout_items (id text PRIMARY KEY,user_id text NOT NULL,workout_day_id text NOT NULL REFERENCES ` + q + `.workout_days(id) ON DELETE CASCADE,name text NOT NULL,sets integer NOT NULL DEFAULT 0,repetitions integer NOT NULL DEFAULT 0,weight double precision NOT NULL DEFAULT 0,duration_seconds integer NOT NULL DEFAULT 0,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL)`} {
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
	return r.DB.WithContext(c).Table(table(r.Schema, "plans")).Create(p).Error
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
	return r.DB.WithContext(c).Table(table(r.Schema, "plans")).Where("user_id = ? AND id = ?", u, id).Delete(&model.Plan{}).Error
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
	return r.DB.WithContext(c).Table(table(r.Schema, "workout_days")).Create(d).Error
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
	return r.DB.WithContext(c).Table(table(r.Schema, "workout_days")).Where("user_id = ? AND plan_id = ? AND id = ?", u, pid, id).Delete(&model.WorkoutDay{}).Error
}
func (r GORM) CreateItem(c context.Context, i *model.WorkoutItem) error {
	return r.DB.WithContext(c).Table(table(r.Schema, "workout_items")).Create(i).Error
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
	return r.DB.WithContext(c).Table(table(r.Schema, "workout_items")).Where("user_id = ? AND workout_day_id = ? AND id = ?", u, did, id).Delete(&model.WorkoutItem{}).Error
}
