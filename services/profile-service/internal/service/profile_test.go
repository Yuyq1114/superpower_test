package service

import (
	"context"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/profile-service/internal/model"
	"math"
	"sort"
	"testing"
	"time"
)

type fakeRepo struct{ items []model.Metric }

func (r *fakeRepo) Create(_ context.Context, m *model.Metric) error {
	for _, x := range r.items {
		if x.UserID == m.UserID && x.IdempotencyKey == m.IdempotencyKey {
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
	sort.Slice(o, func(i, j int) bool {
		if o[i].RecordedAt.Equal(o[j].RecordedAt) {
			return o[i].ID > o[j].ID
		}
		return o[i].RecordedAt.After(o[j].RecordedAt)
	})
	return o, nil
}
func TestRecordValidationBoundaries(t *testing.T) {
	s := New(&fakeRepo{})
	at := time.Now()
	tests := []MetricInput{{"weight", 0, "kg", at, "k"}, {"weight", 500.1, "kg", at, "k"}, {"weight", 70, "percent", at, "k"}, {"body_fat", -0.1, "percent", at, "k"}, {"body_fat", 100.1, "percent", at, "k"}, {"body_fat", 20, "kg", at, "k"}, {"weight", math.NaN(), "kg", at, "k"}, {"weight", math.Inf(1), "kg", at, "k"}, {"weight", 70, "kg", at, " "}, {"weight", 70, "kg", at, string(make([]byte, 129))}}
	for _, in := range tests {
		if _, e := s.RecordMetric(context.Background(), "u", in); apperror.CodeOf(e) != apperror.CodeInvalidArgument {
			t.Fatalf("expected invalid for %#v: %v", in, e)
		}
	}
	for _, in := range []MetricInput{{"weight", 0.01, "kg", at, "a"}, {"weight", 500, "kg", at, "b"}, {"body_fat", 0, "percent", at, "c"}, {"body_fat", 100, "percent", at, "d"}} {
		if _, e := s.RecordMetric(context.Background(), "u", in); e != nil {
			t.Fatal(e)
		}
	}
}
func TestIdempotencyConflictAndStableSort(t *testing.T) {
	r := &fakeRepo{}
	s := New(r)
	at := time.Now().UTC()
	a, e := s.RecordMetric(context.Background(), "u", MetricInput{"weight", 70, "kg", at, "k"})
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.RecordMetric(context.Background(), "u", MetricInput{"weight", 70, "kg", at, " k "})
	if e != nil || again.ID != a.ID {
		t.Fatal(e)
	}
	if _, e = s.RecordMetric(context.Background(), "u", MetricInput{"weight", 71, "kg", at, "k"}); apperror.CodeOf(e) != apperror.CodeConflict {
		t.Fatal(e)
	}
	r.items = append(r.items, model.Metric{ID: "zz", UserID: "u", MetricType: "weight", RecordedAt: at})
	got, e := s.ListMetrics(context.Background(), "u", "weight", at.Add(-time.Second), at.Add(time.Second))
	if e != nil || got[0].ID != "zz" {
		t.Fatal("unstable order")
	}
}
func TestListRejectsInvalidTypeAndRange(t *testing.T) {
	s := New(&fakeRepo{})
	if _, e := s.ListMetrics(context.Background(), "u", "height", time.Time{}, time.Time{}); apperror.CodeOf(e) != apperror.CodeInvalidArgument {
		t.Fatal(e)
	}
	if _, e := s.ListMetrics(context.Background(), "u", "", time.Now(), time.Now().Add(-time.Hour)); apperror.CodeOf(e) != apperror.CodeInvalidArgument {
		t.Fatal(e)
	}
}
