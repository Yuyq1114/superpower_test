package service

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/services/checkin-service/internal/model"
	"testing"
	"time"
)

type fakeRepo struct {
	err   error
	calls int
}

func (f *fakeRepo) CreateWithEvent(context.Context, *model.Checkin, *model.OutboxEvent) error {
	f.calls++
	return f.err
}
func (f *fakeRepo) List(context.Context, string, time.Time, time.Time, int, int) ([]model.Checkin, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) ListDates(context.Context, string, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}
func (f *fakeRepo) PendingEvents(context.Context, int) ([]model.OutboxEvent, error) { return nil, nil }
func (f *fakeRepo) MarkPublished(context.Context, string, string, time.Time) error  { return nil }
func (f *fakeRepo) ReleaseLease(context.Context, string, string) error              { return nil }

type checker struct{ err error }

func (c checker) CheckWorkoutItem(context.Context, string, string) error { return c.err }
func TestCompleteAndFailure(t *testing.T) {
	r := &fakeRepo{}
	s := New(r, checker{})
	d := time.Date(2026, 2, 28, 23, 30, 0, 0, time.FixedZone("PST", -8*3600))
	c, e := s.Complete(context.Background(), "u", "i", d, "n", "k")
	if e != nil || c.IdempotencyKey != "k" {
		t.Fatal(c, e)
	}
	r.err = errors.New("rollback")
	if _, e = s.Complete(context.Background(), "u", "i", d, "", "k2"); e == nil {
		t.Fatal("expected failure")
	}
}
func TestCurrentStreak(t *testing.T) {
	now := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	ds := []time.Time{now.AddDate(0, 0, -2), now.AddDate(0, 0, -1), now, now.AddDate(0, -1, 0), now.AddDate(0, 0, 1)}
	if got := CurrentStreak(ds, now); got != 3 {
		t.Fatal(got)
	}
	if got := CurrentStreak(ds, now.AddDate(0, 0, 2)); got != 0 {
		t.Fatal(got)
	}
}
