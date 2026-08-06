package repository

import (
	"context"

	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"gorm.io/gorm"
)

type UnitOfWork interface {
	CreateUserWithRefreshToken(context.Context, *model.User, *model.RefreshToken) error
}

type GORMUnitOfWork struct {
	DB     *gorm.DB
	Schema string
}

func (r GORMUnitOfWork) CreateUserWithRefreshToken(c context.Context, u *model.User, token *model.RefreshToken) error {
	return r.DB.WithContext(c).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(scopedTable(r.Schema, "users")).Create(u).Error; err != nil {
			return err
		}
		return tx.Table(scopedTable(r.Schema, "refresh_tokens")).Create(token).Error
	})
}
