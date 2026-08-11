package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	Streak         int
}
type Service struct {
	repo    repository.Repository
	checker WorkoutItemChecker
}

func New(r repository.Repository, c WorkoutItemChecker) *Service {
	return &Service{repo: r, checker: c}
}
func fingerprint(item string, date time.Time, note string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s", item, date.Format("2006-01-02"), note)))
	return hex.EncodeToString(sum[:])
}
func valid(n, v string) error {
	if strings.TrimSpace(v) == "" {
		return apperror.InvalidArgument(n + " is required")
	}
	return nil
}
func (s *Service) Complete(ctx context.Context, u, i string, d time.Time, note, key string) (model.Checkin, error) {
	if e := valid("user_id", u); e != nil {
		return model.Checkin{}, e
	}
	if e := valid("workout_item_id", i); e != nil {
		return model.Checkin{}, e
	}
	if e := valid("idempotency_key", key); e != nil {
		return model.Checkin{}, e
	}
	if d.IsZero() {
		return model.Checkin{}, apperror.InvalidArgument("date is required")
	}
	d = d.UTC()
	d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	if s.checker != nil {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if e := s.checker.CheckWorkoutItem(checkCtx, u, i); e != nil {
			return model.Checkin{}, e
		}
	}
	now := time.Now().UTC()
	c := model.Checkin{ID: uuid.NewString(), UserID: u, WorkoutItemID: i, IdempotencyKey: key, RequestFingerprint: fingerprint(i, d, note), Date: d, Note: note, CompletedAt: now, CreatedAt: now}
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
	if e != nil {
		return Page[model.Checkin]{}, e
	}
	result := Page[model.Checkin]{Items: x, Page: p, PageSize: z, Total: n}
	dates, e := s.repo.ListDates(ctx, u, from.UTC(), to.UTC())
	if e != nil {
		return Page[model.Checkin]{}, e
	}
	// `to` is the caller's own local notion of "today" (validated non-zero
	// above); using the server's own wall clock here instead would silently
	// disagree with it for roughly half of each day for any caller whose
	// timezone isn't UTC, truncating a real streak to 0.
	result.Streak = CurrentStreak(dates, to.UTC())
	return result, nil
}
func CurrentStreak(ds []time.Time, now time.Time) int {
	seen := map[string]bool{}
	for _, d := range ds {
		seen[d.UTC().Format("2006-01-02")] = true
	}
	today := now.UTC().Format("2006-01-02")
	if !seen[today] {
		return 0
	}
	d, _ := time.Parse("2006-01-02", today)
	n := 0
	for seen[d.Format("2006-01-02")] {
		n++
		d = d.AddDate(0, 0, -1)
	}
	return n
}
func CalculateStreak(ds []time.Time) int { return CurrentStreak(ds, time.Now().UTC()) }
