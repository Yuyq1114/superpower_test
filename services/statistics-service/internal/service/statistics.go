package service

import (
	"context"
	"strings"
	"time"

	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
)

type Repository interface {
	ConsumeWorkoutCompleted(context.Context, model.WorkoutCompleted) error
	GetSummary(context.Context, string, model.Period, time.Time, time.Time) (model.Summary, error)
}

type Service struct{ repo Repository }

func New(repo Repository) *Service { return &Service{repo: repo} }

func (s *Service) ConsumeWorkoutCompleted(ctx context.Context, event model.WorkoutCompleted) error {
	if strings.TrimSpace(event.EventID) == "" || strings.TrimSpace(event.UserID) == "" || strings.TrimSpace(event.CheckinID) == "" || event.CompletedAt.IsZero() {
		return apperror.InvalidArgument("event_id, user_id, checkin_id and completed_at are required")
	}
	if event.EventType != model.WorkoutCompletedType {
		return apperror.InvalidArgument("unsupported event_type")
	}
	event.CompletedAt = event.CompletedAt.UTC()
	event.OccurredAt = event.OccurredAt.UTC()
	return s.repo.ConsumeWorkoutCompleted(ctx, event)
}

func (s *Service) GetSummary(ctx context.Context, userID string, period model.Period, start, end time.Time) (model.Summary, error) {
	if strings.TrimSpace(userID) == "" {
		return model.Summary{}, apperror.InvalidArgument("user_id is required")
	}
	if period != model.PeriodWeek && period != model.PeriodMonth {
		return model.Summary{}, apperror.InvalidArgument("period must be week or month")
	}
	if start.IsZero() {
		start = time.Now().UTC()
	}
	start = bucketStart(period, start.UTC())
	if end.IsZero() {
		end = nextBucket(period, start)
	} else {
		end = nextBucket(period, bucketStart(period, end.UTC()))
	}
	if !end.After(start) {
		return model.Summary{}, apperror.InvalidArgument("end must be after start")
	}
	out, err := s.repo.GetSummary(ctx, userID, period, start, end)
	if err != nil {
		return model.Summary{}, err
	}
	out.UserID, out.Period, out.Start, out.End = userID, period, start, end
	return out, nil
}

func bucketStart(period model.Period, at time.Time) time.Time {
	at = at.UTC()
	if period == model.PeriodMonth {
		return time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

func nextBucket(period model.Period, at time.Time) time.Time {
	if period == model.PeriodMonth {
		return at.AddDate(0, 1, 0)
	}
	return at.AddDate(0, 0, 7)
}
