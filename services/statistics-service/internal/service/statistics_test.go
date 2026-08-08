package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
)

type fakeRepository struct {
	summary model.Summary
	err     error
	user    string
	period  model.Period
	start   time.Time
	end     time.Time
}

func (f *fakeRepository) ConsumeWorkoutCompleted(context.Context, model.WorkoutCompleted) error {
	return f.err
}
func (f *fakeRepository) GetSummary(_ context.Context, user string, period model.Period, start, end time.Time) (model.Summary, error) {
	f.user, f.period, f.start, f.end = user, period, start, end
	return f.summary, f.err
}

func TestGetSummaryNormalizesWeekToUTCBoundaries(t *testing.T) {
	r := &fakeRepository{}
	s := New(r)
	start := time.Date(2026, 8, 5, 12, 0, 0, 0, time.FixedZone("local", 8*60*60))
	got, err := s.GetSummary(context.Background(), "user-1", model.PeriodWeek, start, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	wantStart := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	if !r.start.Equal(wantStart) || !r.end.Equal(wantStart.AddDate(0, 0, 7)) {
		t.Fatalf("bounds = %s..%s", r.start, r.end)
	}
	if got.UserID != "user-1" || got.Period != model.PeriodWeek {
		t.Fatalf("summary identity = %#v", got)
	}
}

func TestGetSummaryNormalizesMonthAndHonorsExplicitRange(t *testing.T) {
	r := &fakeRepository{}
	s := New(r)
	start := time.Date(2026, 2, 18, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	_, err := s.GetSummary(context.Background(), "user-2", model.PeriodMonth, start, end)
	if err != nil {
		t.Fatal(err)
	}
	if !r.start.Equal(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)) || !r.end.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("bounds = %s..%s", r.start, r.end)
	}
}

func TestGetSummaryRejectsInvalidInput(t *testing.T) {
	s := New(&fakeRepository{})
	for _, tc := range []struct {
		user   string
		period model.Period
	}{{"", model.PeriodWeek}, {"u", "day"}} {
		if _, err := s.GetSummary(context.Background(), tc.user, tc.period, time.Now(), time.Time{}); err == nil {
			t.Fatal("expected validation error")
		}
	}
}

func TestConsumeValidatesStableEvent(t *testing.T) {
	s := New(&fakeRepository{})
	e := model.WorkoutCompleted{EventID: "event", EventType: model.WorkoutCompletedType, UserID: "user", CheckinID: "checkin", CompletedAt: time.Now()}
	if err := s.ConsumeWorkoutCompleted(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	e.EventID = ""
	if err := s.ConsumeWorkoutCompleted(context.Background(), e); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRepositoryErrorPropagates(t *testing.T) {
	want := errors.New("database unavailable")
	s := New(&fakeRepository{err: want})
	_, got := s.GetSummary(context.Background(), "u", model.PeriodWeek, time.Now(), time.Time{})
	if !errors.Is(got, want) {
		t.Fatalf("error = %v", got)
	}
}
