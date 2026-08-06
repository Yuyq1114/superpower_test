package service

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/services/checkin-service/internal/model"
	"testing"
	"time"
)

type fakeRepo struct {
	calls int
	err   error
	rows  []model.Checkin
}

func (f *fakeRepo) CreateWithEvent(_ context.Context, c *model.Checkin, e *model.OutboxEvent) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	return nil
}
func (f *fakeRepo) List(context.Context, string, time.Time, time.Time, int, int) ([]model.Checkin, int64, error) {
	return f.rows, int64(len(f.rows)), nil
}
func (f *fakeRepo) PendingEvents(context.Context, int) ([]model.OutboxEvent, error) { return nil, nil }
func (f *fakeRepo) MarkPublished(context.Context, string, time.Time) error          { return nil }

type checker struct{ err error }

func (c checker) CheckWorkoutItem(context.Context, string, string) error { return c.err }
func TestCompleteWritesOnceAndPropagatesTransactionFailure(t *testing.T) {
	r := &fakeRepo{}
	s := New(r, checker{})
	d, _ := time.Parse(time.RFC3339, "2026-02-28T23:30:00-08:00")
	c, e := s.Complete(context.Background(), "u1", "item1", d, "ok")
	if e != nil || c.UserID != "u1" || c.Date.Format("2006-01-02") != "2026-03-01" {
		t.Fatalf("complete=%+v err=%v", c, e)
	}
	if r.calls != 1 {
		t.Fatal(r.calls)
	}
	r.err = errors.New("rollback")
	if _, e = s.Complete(context.Background(), "u1", "item1", d, ""); e == nil {
		t.Fatal("expected transaction failure")
	}
}
func TestCalculateStreakAcrossMonthAndDuplicateDates(t *testing.T) {
	ds := []time.Time{time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC), time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC), time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC), time.Date(2026, 2, 2, 18, 0, 0, 0, time.UTC)}
	if got := CalculateStreak(ds); got != 3 {
		t.Fatalf("got %d", got)
	}
}
