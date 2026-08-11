package service

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/services/checkin-service/internal/model"
	"testing"
	"time"
)

type fakeRepo struct {
	err         error
	calls       int
	dates       []time.Time
	lastCheckin model.Checkin
	lastEvent   model.OutboxEvent
}

func (f *fakeRepo) CreateWithEvent(_ context.Context, c *model.Checkin, e *model.OutboxEvent) error {
	f.calls++
	if c != nil {
		f.lastCheckin = *c
	}
	if e != nil {
		f.lastEvent = *e
	}
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

// Statistics buckets a WorkoutCompleted event by its `completed_at`. If that
// field carried the write instant, a check-in backfilled today for last
// Wednesday would be counted in THIS week's summary, permanently disagreeing
// with the history list (which uses the logical date) for that user. The v1
// contract is therefore: `completed_at` = the logical workout date,
// `occurred_at` = when the row was written.
func TestOutboxEventCarriesLogicalWorkoutDateNotWriteInstant(t *testing.T) {
	r := &fakeRepo{}
	s := New(r, checker{})
	backfilled := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	before := time.Now().UTC()

	c, err := s.Complete(context.Background(), "u", "i", backfilled, "n", "k")
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC()

	if !r.lastEvent.CompletedAt.Equal(backfilled) {
		t.Fatalf("event completed_at=%s, want the logical workout date %s", r.lastEvent.CompletedAt, backfilled)
	}
	if r.lastEvent.OccurredAt.Before(before) || r.lastEvent.OccurredAt.After(after) {
		t.Fatalf("event occurred_at=%s, want the write instant between %s and %s", r.lastEvent.OccurredAt, before, after)
	}
	// The user-visible record keeps the real completion instant.
	if c.CompletedAt.Before(before) || c.CompletedAt.After(after) {
		t.Fatalf("checkin completed_at=%s, want the real write instant", c.CompletedAt)
	}
	if c.Date != backfilled {
		t.Fatalf("checkin date=%s, want %s", c.Date, backfilled)
	}
}

// A date supplied in a zone ahead of UTC still normalizes to that calendar
// day's UTC midnight, so the event's logical date matches the stored date.
func TestOutboxEventLogicalDateMatchesNormalizedCheckinDate(t *testing.T) {
	r := &fakeRepo{}
	s := New(r, checker{})
	d := time.Date(2026, 2, 28, 23, 30, 0, 0, time.FixedZone("PST", -8*3600))

	if _, err := s.Complete(context.Background(), "u", "i", d, "n", "k"); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !r.lastEvent.CompletedAt.Equal(want) {
		t.Fatalf("event completed_at=%s, want %s", r.lastEvent.CompletedAt, want)
	}
	if !r.lastCheckin.Date.Equal(r.lastEvent.CompletedAt) {
		t.Fatalf("checkin date=%s and event completed_at=%s must be the same logical day", r.lastCheckin.Date, r.lastEvent.CompletedAt)
	}
}

// The check-in date input is client-supplied, so the `max=<today>` attribute
// in the UI is a hint, not a control: a future logical date would otherwise
// pre-populate a future statistics bucket.
func TestCompleteRejectsFutureLogicalDates(t *testing.T) {
	r := &fakeRepo{}
	s := New(r, checker{})
	future := time.Now().UTC().AddDate(0, 0, 3)

	if _, err := s.Complete(context.Background(), "u", "i", future, "n", "k"); err == nil {
		t.Fatal("expected a future check-in date to be rejected")
	}
	if r.calls != 0 {
		t.Fatalf("repository was called %d times for a rejected future date", r.calls)
	}

	// A caller in a timezone ahead of UTC legitimately reports "today" as the
	// UTC date plus one, so exactly one day of slack must stay accepted.
	tomorrow := time.Now().UTC().AddDate(0, 0, 1)
	if _, err := s.Complete(context.Background(), "u", "i", tomorrow, "n", "k2"); err != nil {
		t.Fatalf("a caller one day ahead of UTC must still be able to check in: %v", err)
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
