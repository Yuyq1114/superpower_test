package model

import "time"

const WorkoutCompletedType = "WorkoutCompleted"

type Period string

const (
	PeriodWeek  Period = "week"
	PeriodMonth Period = "month"
)

type WorkoutCompleted struct {
	EventID     string
	EventType   string
	UserID      string
	CheckinID   string
	CompletedAt time.Time
	OccurredAt  time.Time
}

type Summary struct {
	UserID               string
	Period               Period
	Start                time.Time
	End                  time.Time
	WorkoutCount         int64
	ActiveDays           int64
	TotalDurationSeconds int64
}

type Aggregate struct {
	UserID               string    `gorm:"column:user_id"`
	Period               Period    `gorm:"column:period"`
	BucketStart          time.Time `gorm:"column:bucket_start"`
	WorkoutCount         int64     `gorm:"column:workout_count"`
	ActiveDays           int64     `gorm:"column:active_days"`
	TotalDurationSeconds int64     `gorm:"column:total_duration_seconds"`
}
