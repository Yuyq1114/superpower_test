package repository

import (
	"context"
	"fmt"
	"github.com/example/fitness-checkin/services/checkin-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const DefaultSchema = "checkin_schema"

func table(s, n string) string {
	if s == "" {
		s = DefaultSchema
	}
	return `"` + s + `"."` + n + `"`
}
func Migrate(ctx context.Context, db *gorm.DB) error {
	s := `"` + DefaultSchema + `"`
	qs := []string{`CREATE SCHEMA IF NOT EXISTS ` + s, `CREATE TABLE IF NOT EXISTS ` + s + `.checkins (id text PRIMARY KEY,user_id text NOT NULL,workout_item_id text NOT NULL,checkin_date date NOT NULL,note text NOT NULL DEFAULT '',completed_at timestamptz NOT NULL,created_at timestamptz NOT NULL)`, `CREATE TABLE IF NOT EXISTS ` + s + `.outbox_events (event_id text PRIMARY KEY,event_type text NOT NULL,user_id text NOT NULL,checkin_id text NOT NULL,completed_at timestamptz NOT NULL,occurred_at timestamptz NOT NULL,published_at timestamptz NULL)`, `CREATE UNIQUE INDEX IF NOT EXISTS checkins_user_item_date_unique ON ` + s + `.checkins(user_id,workout_item_id,checkin_date)`, `CREATE INDEX IF NOT EXISTS checkins_user_date_idx ON ` + s + `.checkins(user_id,checkin_date)`, `CREATE INDEX IF NOT EXISTS outbox_unpublished_idx ON ` + s + `.outbox_events(published_at,event_id)`}
	for _, q := range qs {
		if err := db.WithContext(ctx).Exec(q).Error; err != nil {
			return fmt.Errorf("migration: %w", err)
		}
	}
	return nil
}

type Repository interface {
	CreateWithEvent(context.Context, *model.Checkin, *model.OutboxEvent) error
	List(context.Context, string, time.Time, time.Time, int, int) ([]model.Checkin, int64, error)
	PendingEvents(context.Context, int) ([]model.OutboxEvent, error)
	MarkPublished(context.Context, string, time.Time) error
}
type GORM struct {
	DB     *gorm.DB
	Schema string
}

func (r GORM) CreateWithEvent(ctx context.Context, c *model.Checkin, e *model.OutboxEvent) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		x := tx.Table(table(r.Schema, "checkins")).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "workout_item_id"}, {Name: "checkin_date"}}, DoNothing: true}).Create(c)
		if x.Error != nil {
			return x.Error
		}
		if x.RowsAffected == 0 {
			return tx.Table(table(r.Schema, "checkins")).Where("user_id=? AND workout_item_id=? AND checkin_date=?", c.UserID, c.WorkoutItemID, c.Date).First(c).Error
		}
		return tx.Table(table(r.Schema, "outbox_events")).Create(e).Error
	})
}
func (r GORM) List(ctx context.Context, u string, from, to time.Time, p, z int) ([]model.Checkin, int64, error) {
	var out []model.Checkin
	var n int64
	q := r.DB.WithContext(ctx).Table(table(r.Schema, "checkins")).Where("user_id=? AND checkin_date>=? AND checkin_date<=?", u, from, to)
	if err := q.Count(&n).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("checkin_date DESC, completed_at DESC").Offset((p - 1) * z).Limit(z).Find(&out).Error
	return out, n, err
}
func (r GORM) PendingEvents(ctx context.Context, n int) ([]model.OutboxEvent, error) {
	var out []model.OutboxEvent
	err := r.DB.WithContext(ctx).Table(table(r.Schema, "outbox_events")).Where("published_at IS NULL").Order("occurred_at ASC").Limit(n).Find(&out).Error
	return out, err
}
func (r GORM) MarkPublished(ctx context.Context, id string, at time.Time) error {
	return r.DB.WithContext(ctx).Table(table(r.Schema, "outbox_events")).Where("event_id=? AND published_at IS NULL", id).Updates(map[string]any{"published_at": at}).Error
}
