package events

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/example/fitness-checkin/pkg/workoutevent"
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

// The v1 wire format is unchanged by the logical-date semantics: both
// timestamps are still RFC3339, so events already sitting unpublished in the
// outbox stay parseable by the statistics consumer. Only the MEANING of
// `completed_at` changed (logical workout date instead of write instant).
func TestPublishedEventKeepsRFC3339TimestampsForLogicalDates(t *testing.T) {
	m := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: m.Addr()})
	logicalDate := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	writtenAt := time.Date(2026, 8, 11, 9, 30, 15, 0, time.UTC)
	r := &repo{events: []model.OutboxEvent{{EventID: "e-logical", EventType: "WorkoutCompleted", UserID: "u", CheckinID: "c", CompletedAt: logicalDate, OccurredAt: writtenAt}}}

	if err := (&Publisher{Repo: r, Redis: c}).PublishPending(context.Background(), 1); err != nil {
		t.Fatal(err)
	}

	messages, err := c.XRange(context.Background(), Stream, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("published %d messages, want 1", len(messages))
	}
	event, err := workoutevent.Parse(messages[0].Values)
	if err != nil {
		t.Fatal(err)
	}
	if event.CompletedAt != "2026-08-05T00:00:00Z" {
		t.Fatalf("completed_at=%q, want the logical workout date at UTC midnight", event.CompletedAt)
	}
	if event.OccurredAt != "2026-08-11T09:30:15Z" {
		t.Fatalf("occurred_at=%q, want the write instant", event.OccurredAt)
	}
	if _, err := time.Parse(time.RFC3339Nano, event.CompletedAt); err != nil {
		t.Fatalf("completed_at must stay RFC3339 for already-queued events: %v", err)
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
