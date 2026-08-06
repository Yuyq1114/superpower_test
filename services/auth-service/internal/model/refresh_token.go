package model

import "time"

type RefreshToken struct {
	ID        string `gorm:"primaryKey"`
	UserID    string `gorm:"index"`
	TokenHash string `gorm:"uniqueIndex"`
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (RefreshToken) TableName() string { return "auth_schema.refresh_tokens" }
