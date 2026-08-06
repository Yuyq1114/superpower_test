package model

import "time"

type OutboxEvent struct {
	EventID     string     `gorm:"column:event_id;primaryKey"`
	EventType   string     `gorm:"column:event_type"`
	UserID      string     `gorm:"column:user_id"`
	CheckinID   string     `gorm:"column:checkin_id"`
	CompletedAt time.Time  `gorm:"column:completed_at"`
	OccurredAt  time.Time  `gorm:"column:occurred_at"`
	PublishedAt *time.Time `gorm:"column:published_at"`
}
