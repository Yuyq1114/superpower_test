package repository

import (
	"context"

	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"gorm.io/gorm"
)

type UnitOfWork interface {
	CreateUserWithRefreshToken(context.Context, *model.User, *model.RefreshToken) error
}

type GORMUnitOfWork struct{ DB *gorm.DB }

func (r GORMUnitOfWork) CreateUserWithRefreshToken(c context.Context, u *model.User, token *model.RefreshToken) error {
	return r.DB.WithContext(c).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(u).Error; err != nil {
			return err
		}
		return tx.Create(token).Error
	})
}
