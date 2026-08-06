package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/profile-service/internal/model"
	"github.com/example/fitness-checkin/services/profile-service/internal/repository"
	"github.com/google/uuid"
	"math"
	"strings"
	"time"
)

type MetricInput struct {
	MetricType     string
	Value          float64
	Unit           string
	RecordedAt     time.Time
	IdempotencyKey string
}
type Service struct{ repo repository.Repository }

func New(r repository.Repository) *Service { return &Service{repo: r} }
func fingerprint(i MetricInput) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%.17g|%s|%s", i.MetricType, i.Value, i.Unit, i.RecordedAt.UTC().Format(time.RFC3339Nano))))
	return hex.EncodeToString(h[:])
}
func (s *Service) RecordMetric(ctx context.Context, u string, i MetricInput) (model.Metric, error) {
	u = strings.TrimSpace(u)
	i.MetricType = strings.TrimSpace(i.MetricType)
	i.Unit = strings.TrimSpace(i.Unit)
	i.IdempotencyKey = strings.TrimSpace(i.IdempotencyKey)
	if u == "" {
		return model.Metric{}, apperror.InvalidArgument("user_id is required")
	}
	if len(i.IdempotencyKey) < 1 || len(i.IdempotencyKey) > 128 {
		return model.Metric{}, apperror.InvalidArgument("idempotency_key length must be between 1 and 128")
	}
	if i.MetricType != "weight" && i.MetricType != "body_fat" {
		return model.Metric{}, apperror.InvalidArgument("metric_type must be weight or body_fat")
	}
	want := map[string]string{"weight": "kg", "body_fat": "percent"}[i.MetricType]
	if i.Unit != want {
		return model.Metric{}, apperror.InvalidArgument("invalid unit for metric_type")
	}
	if math.IsNaN(i.Value) || math.IsInf(i.Value, 0) || (i.MetricType == "weight" && (i.Value <= 0 || i.Value > 500)) || (i.MetricType == "body_fat" && (i.Value < 0 || i.Value > 100)) {
		return model.Metric{}, apperror.InvalidArgument("value is outside the allowed range")
	}
	if i.RecordedAt.IsZero() {
		return model.Metric{}, apperror.InvalidArgument("recorded_at is required")
	}
	now := time.Now().UTC()
	m := model.Metric{ID: uuid.NewString(), UserID: u, MetricType: i.MetricType, Value: i.Value, Unit: i.Unit, RecordedAt: i.RecordedAt.UTC(), IdempotencyKey: i.IdempotencyKey, RequestFingerprint: fingerprint(i), CreatedAt: now}
	if e := s.repo.Create(ctx, &m); e != nil {
		return model.Metric{}, e
	}
	return m, nil
}
func (s *Service) ListMetrics(ctx context.Context, u, t string, from, to time.Time) ([]model.Metric, error) {
	u = strings.TrimSpace(u)
	t = strings.TrimSpace(t)
	if u == "" {
		return nil, apperror.InvalidArgument("user_id is required")
	}
	if t != "" && t != "weight" && t != "body_fat" {
		return nil, apperror.InvalidArgument("metric_type must be weight or body_fat")
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return nil, apperror.InvalidArgument("invalid time range")
	}
	if from.IsZero() {
		from = time.Unix(0, 0)
	}
	if to.IsZero() {
		to = time.Now()
	}
	return s.repo.List(ctx, u, t, from.UTC(), to.UTC())
}
