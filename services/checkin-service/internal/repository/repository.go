package repository

import (
	"context"

	"errors"

	"fmt"

	"github.com/example/fitness-checkin/pkg/apperror"

	"github.com/example/fitness-checkin/services/checkin-service/internal/model"

	"github.com/jackc/pgx/v5/pgconn"

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

	qs := []string{

		`CREATE SCHEMA IF NOT EXISTS ` + s,

		`CREATE TABLE IF NOT EXISTS ` + s + `.checkins (id text PRIMARY KEY,user_id text NOT NULL,workout_item_id text NOT NULL,idempotency_key text NOT NULL DEFAULT '',request_fingerprint text NOT NULL DEFAULT '',checkin_date date NOT NULL,note text NOT NULL DEFAULT '',completed_at timestamptz NOT NULL,created_at timestamptz NOT NULL)`,

		`ALTER TABLE ` + s + `.checkins ADD COLUMN IF NOT EXISTS idempotency_key text NOT NULL DEFAULT ''`,
		`ALTER TABLE ` + s + `.checkins ADD COLUMN IF NOT EXISTS request_fingerprint text NOT NULL DEFAULT ''`,

		`CREATE TABLE IF NOT EXISTS ` + s + `.outbox_events (event_id text PRIMARY KEY,event_type text NOT NULL,user_id text NOT NULL,checkin_id text NOT NULL,completed_at timestamptz NOT NULL,occurred_at timestamptz NOT NULL,published_at timestamptz NULL,lease_id text NOT NULL DEFAULT '',lease_until timestamptz NULL)`,

		`ALTER TABLE ` + s + `.outbox_events ADD COLUMN IF NOT EXISTS lease_id text NOT NULL DEFAULT ''`,

		`ALTER TABLE ` + s + `.outbox_events ADD COLUMN IF NOT EXISTS lease_until timestamptz NULL`,

		`CREATE UNIQUE INDEX IF NOT EXISTS checkins_user_key_unique ON ` + s + `.checkins(user_id,idempotency_key) WHERE idempotency_key <> ''`,

		`CREATE UNIQUE INDEX IF NOT EXISTS checkins_user_item_date_unique ON ` + s + `.checkins(user_id,workout_item_id,checkin_date)`,

		`CREATE INDEX IF NOT EXISTS checkins_user_date_idx ON ` + s + `.checkins(user_id,checkin_date)`,

		`CREATE INDEX IF NOT EXISTS outbox_unpublished_idx ON ` + s + `.outbox_events(published_at,lease_until,event_id)`,
	}

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

	ListDates(context.Context, string, time.Time, time.Time) ([]time.Time, error)

	PendingEvents(context.Context, int) ([]model.OutboxEvent, error)

	MarkPublished(context.Context, string, string, time.Time) error

	ReleaseLease(context.Context, string, string) error
}
type GORM struct {
	DB *gorm.DB

	Schema string
}

func (r GORM) CreateWithEvent(ctx context.Context, c *model.Checkin, e *model.OutboxEvent) error {

	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		x := tx.Table(table(r.Schema, "checkins")).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "idempotency_key"}}, TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "idempotency_key <> ''"}}}, DoNothing: true}).Create(c)

		if x.Error != nil {

			return x.Error

		}

		if x.RowsAffected == 0 {

			var existing model.Checkin

			if err := tx.Table(table(r.Schema, "checkins")).Where("user_id=? AND idempotency_key=?", c.UserID, c.IdempotencyKey).First(&existing).Error; err != nil {

				return err

			}

			if existing.RequestFingerprint != c.RequestFingerprint {

				return apperror.Conflict("idempotency key reused with different request")

			}

			*c = existing

			return nil

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
func (r GORM) ListDates(ctx context.Context, u string, from, to time.Time) ([]time.Time, error) {

	var out []time.Time

	err := r.DB.WithContext(ctx).Table(table(r.Schema, "checkins")).Where("user_id=? AND checkin_date>=? AND checkin_date<=?", u, from, to).Order("checkin_date ASC").Pluck("checkin_date", &out).Error

	return out, err
}
func (r GORM) PendingEvents(ctx context.Context, n int) ([]model.OutboxEvent, error) {

	var out []model.OutboxEvent

	lease := time.Now().UTC()

	id := fmt.Sprintf("lease-%d", lease.UnixNano())

	until := lease.Add(30 * time.Second)

	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		q := tx.Table(table(r.Schema, "outbox_events")).Where("published_at IS NULL AND (lease_until IS NULL OR lease_until < ?)", lease).Order("occurred_at ASC").Limit(n).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})

		if err := q.Find(&out).Error; err != nil {

			return err

		}

		if len(out) == 0 {

			return nil

		}

		ids := make([]string, len(out))

		for i := range out {

			ids[i] = out[i].EventID

		}

		result := tx.Table(table(r.Schema, "outbox_events")).Where("event_id IN ? AND published_at IS NULL AND (lease_until IS NULL OR lease_until < ?)", ids, lease).Updates(map[string]any{"lease_id": id, "lease_until": until})

		if result.Error != nil {

			return result.Error

		}

		if result.RowsAffected != int64(len(ids)) {

			return apperror.Conflict("outbox lease claim lost")

		}

		for i := range out {

			out[i].LeaseID = id

			out[i].LeaseUntil = &until

		}

		return nil

	})

	return out, err
}
func (r GORM) MarkPublished(ctx context.Context, id, lease string, at time.Time) error {

	result := r.DB.WithContext(ctx).Table(table(r.Schema, "outbox_events")).Where("event_id=? AND lease_id=? AND published_at IS NULL", id, lease).Updates(map[string]any{"published_at": at, "lease_id": "", "lease_until": nil})

	if result.Error != nil {

		return result.Error

	}

	if result.RowsAffected != 1 {

		return apperror.Conflict("outbox mark published lost")

	}

	return nil
}
func (r GORM) ReleaseLease(ctx context.Context, id, lease string) error {

	result := r.DB.WithContext(ctx).Table(table(r.Schema, "outbox_events")).Where("event_id=? AND lease_id=? AND published_at IS NULL", id, lease).Updates(map[string]any{"lease_id": "", "lease_until": nil})

	if result.Error != nil {

		return result.Error

	}

	if result.RowsAffected != 1 {

		return apperror.Conflict("outbox lease release lost")

	}

	return nil
}

func ConstraintError(err error) error {

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" {

		return apperror.Conflict("check-in conflicts with an existing record")

	}

	return err
}
