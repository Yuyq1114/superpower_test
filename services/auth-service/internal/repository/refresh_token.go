package repository

import (
	"context"
	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"gorm.io/gorm"
	"time"
)

type RefreshToken interface {
	Create(context.Context, *model.RefreshToken) error
	Rotate(context.Context, string, *model.RefreshToken, time.Time) (string, error)
	Revoke(context.Context, string, time.Time) error
}
type GORMRefreshToken struct{ DB *gorm.DB }

func (r GORMRefreshToken) Create(c context.Context, t *model.RefreshToken) error {
	return r.DB.WithContext(c).Create(t).Error
}
func (r GORMRefreshToken) Rotate(c context.Context, h string, n *model.RefreshToken, now time.Time) (string, error) {
	var uid string
	e := r.DB.WithContext(c).Transaction(func(tx *gorm.DB) error {
		var old model.RefreshToken
		if e := tx.Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", h, now).First(&old).Error; e != nil {
			return e
		}
		uid = old.UserID
		if e := tx.Model(&old).Update("revoked_at", now).Error; e != nil {
			return e
		}
		return tx.Create(n).Error
	})
	return uid, e
}
func (r GORMRefreshToken) Revoke(c context.Context, h string, now time.Time) error {
	res := r.DB.WithContext(c).Model(&model.RefreshToken{}).Where("token_hash = ? AND revoked_at IS NULL", h).Update("revoked_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
