package events

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/example/fitness-checkin/services/checkin-service/internal/model"
	"github.com/redis/go-redis/v9"
	"testing"
	"time"
)

type repo struct {
	events    []model.OutboxEvent
	published int
	markErr   error
	released  int
}

func (r *repo) CreateWithEvent(context.Context, *model.Checkin, *model.OutboxEvent) error { return nil }
func (r *repo) List(context.Context, string, time.Time, time.Time, int, int) ([]model.Checkin, int64, error) {
	return nil, 0, nil
}
func (r *repo) ListDates(context.Context, string, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}
func (r *repo) PendingEvents(context.Context, int) ([]model.OutboxEvent, error) {
	for i := range r.events {
		r.events[i].LeaseID = "l"
	}
	return r.events, nil
}
func (r *repo) MarkPublished(context.Context, string, string, time.Time) error {
	r.published++
	return r.markErr
}
func (r *repo) ReleaseLease(context.Context, string, string) error { r.released++; return nil }
func TestPublisher(t *testing.T) {
	m := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: m.Addr()})
	r := &repo{events: []model.OutboxEvent{{EventID: "e", EventType: "WorkoutCompleted", UserID: "u", CheckinID: "c", CompletedAt: time.Now(), OccurredAt: time.Now()}}}
	if e := (&Publisher{Repo: r, Redis: c}).PublishPending(context.Background(), 1); e != nil {
		t.Fatal(e)
	}
	if r.published != 1 {
		t.Fatal(r.published)
	}
}

func TestMarkFailureReleasesLease(t *testing.T) {
	m := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: m.Addr()})
	r := &repo{markErr: context.DeadlineExceeded, events: []model.OutboxEvent{{EventID: "e2", EventType: "WorkoutCompleted", LeaseID: "l", CompletedAt: time.Now(), OccurredAt: time.Now()}}}
	if err := (&Publisher{Repo: r, Redis: c}).PublishPending(context.Background(), 1); err == nil {
		t.Fatal("expected mark failure")
	}
	if r.released != 1 {
		t.Fatalf("released=%d", r.released)
	}
}
