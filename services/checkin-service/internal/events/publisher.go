package events

import (
	"context"
	"github.com/example/fitness-checkin/pkg/storage"
	"github.com/example/fitness-checkin/services/checkin-service/internal/repository"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"time"
)

const Stream = "workout-events"

type Publisher struct {
	Repo                       repository.Repository
	Redis                      redis.UniversalClient
	Logger                     *slog.Logger
	Published, Failed, Retried func()
}

func (p *Publisher) PublishPending(ctx context.Context, n int) error {
	xs, e := p.Repo.PendingEvents(ctx, n)
	if e != nil {
		return e
	}
	for _, x := range xs {
		if x.LeaseID == "" {
			continue
		}
		if _, e = storage.AddStreamMessage(ctx, p.Redis, Stream, map[string]any{"event_id": x.EventID, "event_type": x.EventType, "user_id": x.UserID, "checkin_id": x.CheckinID, "completed_at": x.CompletedAt.UTC().Format(time.RFC3339Nano), "occurred_at": x.OccurredAt.UTC().Format(time.RFC3339Nano)}); e != nil {
			if p.Failed != nil {
				p.Failed()
			}
			_ = p.Repo.ReleaseLease(ctx, x.EventID, x.LeaseID)
			return e
		}
		if e = p.Repo.MarkPublished(ctx, x.EventID, x.LeaseID, time.Now().UTC()); e != nil {
			if p.Retried != nil {
				p.Retried()
			}
			return e
		}
		if p.Published != nil {
			p.Published()
		}
	}
	return nil
}
