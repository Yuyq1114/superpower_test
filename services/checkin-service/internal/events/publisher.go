package events

import (
	"context"
	"github.com/example/fitness-checkin/pkg/storage"
	"github.com/example/fitness-checkin/services/checkin-service/internal/repository"
	"github.com/redis/go-redis/v9"
	"time"
)

const Stream = "workout-events"

type Publisher struct {
	Repo  repository.Repository
	Redis redis.UniversalClient
}

func (p *Publisher) PublishPending(ctx context.Context, n int) error {
	xs, e := p.Repo.PendingEvents(ctx, n)
	if e != nil {
		return e
	}
	for _, x := range xs {
		if _, e = storage.AddStreamMessage(ctx, p.Redis, Stream, map[string]any{"event_id": x.EventID, "event_type": x.EventType, "user_id": x.UserID, "checkin_id": x.CheckinID, "completed_at": x.CompletedAt.UTC().Format(time.RFC3339Nano), "occurred_at": x.OccurredAt.UTC().Format(time.RFC3339Nano)}); e != nil {
			return e
		}
		if e = p.Repo.MarkPublished(ctx, x.EventID, time.Now().UTC()); e != nil {
			return e
		}
	}
	return nil
}
