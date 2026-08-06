package repository

import (
	"context"
	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type RefreshToken interface {
	Create(context.Context, *model.RefreshToken) error
	Rotate(context.Context, string, time.Time, func(string) (*model.RefreshToken, error)) (model.RefreshToken, error)
	Revoke(context.Context, string, time.Time) error
}
type GORMRefreshToken struct {
	DB     *gorm.DB
	Schema string
}

func (r GORMRefreshToken) Create(c context.Context, t *model.RefreshToken) error {
	return r.DB.WithContext(c).Table(scopedTable(r.Schema, "refresh_tokens")).Create(t).Error
}
func (r GORMRefreshToken) Rotate(c context.Context, h string, now time.Time, issue func(string) (*model.RefreshToken, error)) (model.RefreshToken, error) {
	var out model.RefreshToken
	err := r.DB.WithContext(c).Transaction(func(tx *gorm.DB) error {
		var old model.RefreshToken
		if err := tx.Table(scopedTable(r.Schema, "refresh_tokens")).Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", h, now).First(&old).Error; err != nil {
			return err
		}
		next, err := issue(old.UserID)
		if err != nil {
			return err
		}
		next.UserID = old.UserID
		if err := tx.Table(scopedTable(r.Schema, "refresh_tokens")).Where("id = ?", old.ID).UpdateColumn("revoked_at", now).Error; err != nil {
			return err
		}
		if err := tx.Create(next).Error; err != nil {
			return err
		}
		out = *next
		return nil
	})
	return out, err
}
func (r GORMRefreshToken) Revoke(c context.Context, h string, now time.Time) error {
	res := r.DB.WithContext(c).Table(scopedTable(r.Schema, "refresh_tokens")).Where("token_hash = ? AND revoked_at IS NULL", h).Update("revoked_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
