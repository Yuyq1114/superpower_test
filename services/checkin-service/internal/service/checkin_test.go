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
	dates []time.Time
}

func (f *fakeRepo) CreateWithEvent(context.Context, *model.Checkin, *model.OutboxEvent) error {
	f.calls++
	return f.err
}
func (f *fakeRepo) List(context.Context, string, time.Time, time.Time, int, int) ([]model.Checkin, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) ListDates(context.Context, string, time.Time, time.Time) ([]time.Time, error) {
	return f.dates, nil
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

// The caller (the API gateway/frontend) sends its own local notion of
// "today" as the `to` bound of the requested date range -- that's the only
// definition of "today" the streak should ever use. If `ListHistory`
// instead asks the server's own wall clock what day it is, the two can
// disagree by exactly one day for roughly half of each 24h period for any
// caller whose local timezone isn't UTC, silently truncating a real streak
// to 0 even though every requested date was actually consecutive.
func TestListHistoryStreakUsesCallerSuppliedToNotServerWallClock(t *testing.T) {
	to := time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -30)
	r := &fakeRepo{dates: []time.Time{to, to.AddDate(0, 0, -1), to.AddDate(0, 0, -2)}}
	s := New(r, checker{})

	page, e := s.ListHistory(context.Background(), "u", from, to, 1, 10)
	if e != nil {
		t.Fatal(e)
	}
	if page.Streak != 3 {
		t.Fatalf(
			"expected streak computed against the caller-supplied `to` (%s) to be 3 consecutive days, got %d -- "+
				"this only comes out wrong if the streak is computed against the server's real wall clock "+
				"instead of `to`, which would consider every seeded date to be far in the past and report 0",
			to, page.Streak,
		)
	}
}
