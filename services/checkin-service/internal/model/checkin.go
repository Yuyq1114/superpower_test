package model

import "time"

type Checkin struct {
	ID            string    `gorm:"column:id;primaryKey"`
	UserID        string    `gorm:"column:user_id;index"`
	WorkoutItemID string    `gorm:"column:workout_item_id;index"`
	Date          time.Time `gorm:"column:checkin_date"`
	Note          string
	CompletedAt   time.Time
	CreatedAt     time.Time
}
