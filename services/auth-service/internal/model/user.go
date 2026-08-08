package model

import "time"

type User struct {
	ID           string `gorm:"column:id;primaryKey"`
	Email        string `gorm:"column:email;uniqueIndex"`
	PasswordHash string `gorm:"column:password_hash"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "auth_schema.users" }
