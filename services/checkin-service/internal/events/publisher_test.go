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
}

func (r *repo) CreateWithEvent(context.Context, *model.Checkin, *model.OutboxEvent) error { return nil }
func (r *repo) List(context.Context, string, time.Time, time.Time, int, int) ([]model.Checkin, int64, error) {
	return nil, 0, nil
}
func (r *repo) PendingEvents(context.Context, int) ([]model.OutboxEvent, error) { return r.events, nil }
func (r *repo) MarkPublished(context.Context, string, time.Time) error          { r.published++; return nil }
func TestPublisherPublishesAndMarks(t *testing.T) {
	m := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: m.Addr()})
	r := &repo{events: []model.OutboxEvent{{EventID: "e1", EventType: "WorkoutCompleted", UserID: "u", CheckinID: "c", CompletedAt: time.Now(), OccurredAt: time.Now()}}}
	if e := (&Publisher{Repo: r, Redis: c}).PublishPending(context.Background(), 10); e != nil {
		t.Fatal(e)
	}
	if r.published != 1 {
		t.Fatal(r.published)
	}
	n, e := c.XLen(context.Background(), Stream).Result()
	if e != nil || n != 1 {
		t.Fatalf("len=%d err=%v", n, e)
	}
}
