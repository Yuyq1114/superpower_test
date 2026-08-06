package service

import (
	"context"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/checkin-service/internal/model"
	"github.com/example/fitness-checkin/services/checkin-service/internal/repository"
	"github.com/google/uuid"
	"strings"
	"time"
)

type WorkoutItemChecker interface {
	CheckWorkoutItem(context.Context, string, string) error
}
type Page[T any] struct {
	Items          []T
	Page, PageSize int
	Total          int64
}
type Service struct {
	repo    repository.Repository
	checker WorkoutItemChecker
}

func New(r repository.Repository, c WorkoutItemChecker) *Service {
	return &Service{repo: r, checker: c}
}
func valid(n, v string) error {
	if strings.TrimSpace(v) == "" {
		return apperror.InvalidArgument(n + " is required")
	}
	return nil
}
func (s *Service) Complete(ctx context.Context, u, i string, d time.Time, note string) (model.Checkin, error) {
	if e := valid("user_id", u); e != nil {
		return model.Checkin{}, e
	}
	if e := valid("workout_item_id", i); e != nil {
		return model.Checkin{}, e
	}
	if d.IsZero() {
		return model.Checkin{}, apperror.InvalidArgument("date is required")
	}
	d = d.UTC()
	d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	if s.checker != nil {
		if e := s.checker.CheckWorkoutItem(ctx, u, i); e != nil {
			return model.Checkin{}, e
		}
	}
	now := time.Now().UTC()
	c := model.Checkin{ID: uuid.NewString(), UserID: u, WorkoutItemID: i, Date: d, Note: note, CompletedAt: now, CreatedAt: now}
	ev := model.OutboxEvent{EventID: uuid.NewString(), EventType: "WorkoutCompleted", UserID: u, CheckinID: c.ID, CompletedAt: now, OccurredAt: now}
	if e := s.repo.CreateWithEvent(ctx, &c, &ev); e != nil {
		return model.Checkin{}, e
	}
	return c, nil
}
func (s *Service) ListHistory(ctx context.Context, u string, from, to time.Time, p, z int) (Page[model.Checkin], error) {
	if e := valid("user_id", u); e != nil {
		return Page[model.Checkin]{}, e
	}
	if from.IsZero() || to.IsZero() || from.After(to) {
		return Page[model.Checkin]{}, apperror.InvalidArgument("invalid date range")
	}
	if p < 1 || z < 1 || z > 100 {
		return Page[model.Checkin]{}, apperror.InvalidArgument("invalid pagination")
	}
	x, n, e := s.repo.List(ctx, u, from.UTC(), to.UTC(), p, z)
	result := Page[model.Checkin]{Items: x, Page: p, PageSize: z, Total: n}
	return result, e

}
func CalculateStreak(ds []time.Time) int {
	seen := map[string]bool{}
	for _, d := range ds {
		seen[d.UTC().Format("2006-01-02")] = true
	}
	best := 0
	for k := range seen {
		d, _ := time.Parse("2006-01-02", k)
		n := 1
		for seen[d.AddDate(0, 0, -n).Format("2006-01-02")] {
			n++
		}
		if n > best {
			best = n
		}
	}
	return best
}
