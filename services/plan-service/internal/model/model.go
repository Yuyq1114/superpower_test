package model

import "time"

type Plan struct {
	ID        string `gorm:"column:id;primaryKey"`
	UserID    string `gorm:"column:user_id;index"`
	Name      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
type WorkoutDay struct {
	ID        string    `gorm:"column:id;primaryKey"`
	UserID    string    `gorm:"column:user_id;index"`
	PlanID    string    `gorm:"column:plan_id;index"`
	Date      time.Time `gorm:"column:workout_date"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
type WorkoutItem struct {
	ID              string `gorm:"column:id;primaryKey"`
	UserID          string `gorm:"column:user_id;index"`
	WorkoutDayID    string `gorm:"column:workout_day_id;index"`
	Name            string
	Sets            int
	Repetitions     int
	Weight          float64
	DurationSeconds int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
