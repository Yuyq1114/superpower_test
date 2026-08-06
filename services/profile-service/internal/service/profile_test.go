package service

import (
	"context"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/profile-service/internal/model"
	"testing"
	"time"
)

type fakeRepo struct{ items []model.Metric }

func (r *fakeRepo) Create(_ context.Context, m *model.Metric) error {
	for _, x := range r.items {
		if x.UserID == m.UserID && x.IdempotencyKey != "" && x.IdempotencyKey == m.IdempotencyKey {
			if x.RequestFingerprint != m.RequestFingerprint {
				return apperror.Conflict("idempotency key reused with different request")
			}
			*m = x
			return nil
		}
	}
	r.items = append(r.items, *m)
	return nil
}
func (r *fakeRepo) List(_ context.Context, u, t string, f, to time.Time) ([]model.Metric, error) {
	var o []model.Metric
	for _, x := range r.items {
		if x.UserID == u && (t == "" || x.MetricType == t) && !x.RecordedAt.Before(f) && !x.RecordedAt.After(to) {
			o = append(o, x)
		}
	}
	return o, nil
}
func TestMetricRulesIdempotencyIsolationAndRange(t *testing.T) {
	r := &fakeRepo{}
	s := New(r)
	at := time.Date(2026, 8, 1, 1, 0, 0, 0, time.FixedZone("x", 8*3600))
	m, e := s.RecordMetric(context.Background(), "u1", MetricInput{"weight", 70, "kg", at, "k"})
	if e != nil || m.RecordedAt.Location() != time.UTC {
		t.Fatal(e)
	}
	again, e := s.RecordMetric(context.Background(), "u1", MetricInput{"weight", 70, "kg", at, "k"})
	if e != nil || again.ID != m.ID {
		t.Fatal("idempotency failed")
	}
	if _, e = s.RecordMetric(context.Background(), "u1", MetricInput{"weight", 71, "kg", at, "k"}); apperror.CodeOf(e) != apperror.CodeConflict {
		t.Fatal("expected conflict")
	}
	if _, e = s.RecordMetric(context.Background(), "u2", MetricInput{"body_fat", 20, "percent", at, "k"}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.RecordMetric(context.Background(), "u1", MetricInput{"body_fat", 101, "percent", at, "x"}); apperror.CodeOf(e) != apperror.CodeInvalidArgument {
		t.Fatal("expected range error")
	}
}
func TestListValidatesRange(t *testing.T) {
	s := New(&fakeRepo{})
	_, e := s.ListMetrics(context.Background(), "u", "", time.Now(), time.Now().Add(-time.Hour))
	if apperror.CodeOf(e) != apperror.CodeInvalidArgument {
		t.Fatal(e)
	}
}
