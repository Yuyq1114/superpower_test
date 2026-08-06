package model

import "time"

type Metric struct {
	ID                 string    `gorm:"column:id;primaryKey"`
	UserID             string    `gorm:"column:user_id;index"`
	MetricType         string    `gorm:"column:metric_type"`
	Value              float64   `gorm:"column:value"`
	Unit               string    `gorm:"column:unit"`
	RecordedAt         time.Time `gorm:"column:recorded_at;index"`
	IdempotencyKey     string    `gorm:"column:idempotency_key"`
	RequestFingerprint string    `gorm:"column:request_fingerprint"`
	CreatedAt          time.Time `gorm:"column:created_at"`
}
