package repository

import (
	"context"
	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"gorm.io/gorm"
)

type User interface {
	Create(context.Context, *model.User) error
	ByEmail(context.Context, string) (model.User, error)
	ByID(context.Context, string) (model.User, error)
}
type GORMUser struct {
	DB     *gorm.DB
	Schema string
}

func (r GORMUser) Create(c context.Context, u *model.User) error {
	return r.DB.WithContext(c).Table(scopedTable(r.Schema, "users")).Create(u).Error
}
func (r GORMUser) ByEmail(c context.Context, e string) (model.User, error) {
	var u model.User
	err := r.DB.WithContext(c).Table(scopedTable(r.Schema, "users")).Where("email = ?", e).First(&u).Error
	return u, err
}
func (r GORMUser) ByID(c context.Context, id string) (model.User, error) {
	var u model.User
	err := r.DB.WithContext(c).Table(scopedTable(r.Schema, "users")).First(&u, "id = ?", id).Error
	return u, err
}
