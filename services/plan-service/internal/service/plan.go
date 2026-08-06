package service

import (
	"context"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/plan-service/internal/model"
	"github.com/google/uuid"
	"time"
)

type PlanRepository interface {
	CreatePlan(context.Context, *model.Plan) error
	GetPlan(context.Context, string, string) (model.Plan, error)
	UpdatePlan(context.Context, *model.Plan) error
	DeletePlan(context.Context, string, string) error
	ListPlans(context.Context, string, int, int) ([]model.Plan, int64, error)
	CreateDay(context.Context, *model.WorkoutDay) error
	GetDay(context.Context, string, string, string) (model.WorkoutDay, error)
	UpdateDay(context.Context, *model.WorkoutDay) error
	DeleteDay(context.Context, string, string, string) error
	CreateItem(context.Context, *model.WorkoutItem) error
	GetItem(context.Context, string, string, string) (model.WorkoutItem, error)
	UpdateItem(context.Context, *model.WorkoutItem) error
	DeleteItem(context.Context, string, string, string) error
}
type CreatePlanInput struct {
	Name           string
	IdempotencyKey string
}
type UpdatePlanInput struct {
	Name   string
	Status string
}
type WorkoutDayInput struct {
	Date           time.Time
	IdempotencyKey string
}
type WorkoutItemInput struct {
	IdempotencyKey                     string
	Name                               string
	Sets, Repetitions, DurationSeconds int
	Weight                             float64
}
type Page[T any] struct {
	Items          []T
	Page, PageSize int
	Total          int64
}
type dayLister interface {
	ListDays(context.Context, string, string, int, int) ([]model.WorkoutDay, int64, error)
}
type itemLister interface {
	ListItems(context.Context, string, string, int, int) ([]model.WorkoutItem, int64, error)
}
type Service struct{ repo PlanRepository }

func New(r PlanRepository) *Service { return &Service{repo: r} }
func validateUser(id string) error {
	if id == "" {
		return apperror.InvalidArgument("user_id is required")
	}
	return nil
}
func (s *Service) CreatePlan(c context.Context, u string, in CreatePlanInput) (model.Plan, error) {
	if e := validateUser(u); e != nil {
		return model.Plan{}, e
	}
	if in.Name == "" {
		return model.Plan{}, apperror.InvalidArgument("name is required")
	}
	now := time.Now().UTC()
	p := model.Plan{ID: uuid.NewString(), UserID: u, Name: in.Name, Status: "draft", IdempotencyKey: in.IdempotencyKey, CreatedAt: now, UpdatedAt: now}
	return p, s.repo.CreatePlan(c, &p)
}
func (s *Service) GetPlan(c context.Context, u, id string) (model.Plan, error) {
	if e := validateUser(u); e != nil {
		return model.Plan{}, e
	}
	if id == "" {
		return model.Plan{}, apperror.InvalidArgument("plan_id is required")
	}
	return s.repo.GetPlan(c, u, id)
}
func (s *Service) UpdatePlan(c context.Context, u, id string, in UpdatePlanInput) (model.Plan, error) {
	p, e := s.GetPlan(c, u, id)
	if e != nil {
		return p, e
	}
	if in.Name == "" {
		return p, apperror.InvalidArgument("name is required")
	}
	if in.Status != "" && in.Status != "draft" && in.Status != "active" && in.Status != "archived" {
		return p, apperror.InvalidArgument("invalid plan status")
	}
	p.Name = in.Name
	if in.Status != "" {
		p.Status = in.Status
	}
	p.UpdatedAt = time.Now().UTC()
	return p, s.repo.UpdatePlan(c, &p)
}
func (s *Service) DeletePlan(c context.Context, u, id string) error {
	if _, e := s.GetPlan(c, u, id); e != nil {
		return e
	}
	return s.repo.DeletePlan(c, u, id)
}
func (s *Service) ListPlans(c context.Context, u string, page, size int) (Page[model.Plan], error) {
	if e := validateUser(u); e != nil {
		return Page[model.Plan]{}, e
	}
	if page < 1 || size < 1 || size > 100 {
		return Page[model.Plan]{}, apperror.InvalidArgument("invalid pagination")
	}
	items, total, e := s.repo.ListPlans(c, u, page, size)
	return Page[model.Plan]{Items: items, Page: page, PageSize: size, Total: total}, e
}
func (s *Service) AddWorkoutDay(c context.Context, u, pid string, in WorkoutDayInput) (model.WorkoutDay, error) {
	if _, e := s.GetPlan(c, u, pid); e != nil {
		return model.WorkoutDay{}, e
	}
	if in.Date.IsZero() {
		return model.WorkoutDay{}, apperror.InvalidArgument("date is required")
	}
	now := time.Now().UTC()
	d := model.WorkoutDay{ID: uuid.NewString(), UserID: u, PlanID: pid, Date: in.Date.UTC(), IdempotencyKey: in.IdempotencyKey, CreatedAt: now, UpdatedAt: now}
	return d, s.repo.CreateDay(c, &d)
}
func (s *Service) UpdateWorkoutDay(c context.Context, u, pid, did string, in WorkoutDayInput) (model.WorkoutDay, error) {
	if e := validateUser(u); e != nil {
		return model.WorkoutDay{}, e
	}
	if pid == "" {
		return model.WorkoutDay{}, apperror.InvalidArgument("plan_id is required")
	}
	if did == "" {
		return model.WorkoutDay{}, apperror.InvalidArgument("workout_day_id is required")
	}
	if in.Date.IsZero() {
		return model.WorkoutDay{}, apperror.InvalidArgument("date is required")
	}
	d, e := s.repo.GetDay(c, u, pid, did)
	if e != nil {
		return d, e
	}
	d.Date = in.Date.UTC()
	d.UpdatedAt = time.Now().UTC()
	return d, s.repo.UpdateDay(c, &d)
}
func (s *Service) DeleteWorkoutDay(c context.Context, u, pid, did string) error {
	if e := validateUser(u); e != nil {
		return e
	}
	if pid == "" || did == "" {
		return apperror.InvalidArgument("plan_id and workout_day_id are required")
	}
	if _, e := s.repo.GetDay(c, u, pid, did); e != nil {
		return e
	}
	return s.repo.DeleteDay(c, u, pid, did)
}
func (s *Service) ListWorkoutDays(c context.Context, u, pid string, page, size int) (Page[model.WorkoutDay], error) {
	if e := validateUser(u); e != nil {
		return Page[model.WorkoutDay]{}, e
	}
	if pid == "" {
		return Page[model.WorkoutDay]{}, apperror.InvalidArgument("plan_id is required")
	}
	if _, e := s.GetPlan(c, u, pid); e != nil {
		return Page[model.WorkoutDay]{}, e
	}
	if page < 1 || size < 1 || size > 100 {
		return Page[model.WorkoutDay]{}, apperror.InvalidArgument("invalid pagination")
	}
	l, ok := s.repo.(dayLister)
	if !ok {
		return Page[model.WorkoutDay]{}, apperror.Internal("workout day listing unavailable")
	}
	items, total, e := l.ListDays(c, u, pid, page, size)
	return Page[model.WorkoutDay]{Items: items, Page: page, PageSize: size, Total: total}, e
}
func validItem(in WorkoutItemInput) error {
	if in.Name == "" {
		return apperror.InvalidArgument("name is required")
	}
	if in.Sets < 0 || in.Sets > 1000 || in.Repetitions < 0 || in.Repetitions > 1000 || in.DurationSeconds < 0 || in.DurationSeconds > 86400 || in.Weight < 0 || in.Weight > 1000 {
		return apperror.InvalidArgument("workout parameters must be non-negative")
	}
	if in.Sets == 0 && in.Repetitions == 0 && in.DurationSeconds == 0 {
		return apperror.InvalidArgument("at least sets, repetitions, or duration is required")
	}
	return nil
}
func (s *Service) AddWorkoutItem(c context.Context, u, did string, in WorkoutItemInput) (model.WorkoutItem, error) {
	if e := validateUser(u); e != nil {
		return model.WorkoutItem{}, e
	}
	if did == "" {
		return model.WorkoutItem{}, apperror.InvalidArgument("workout_day_id is required")
	}
	if e := validItem(in); e != nil {
		return model.WorkoutItem{}, e
	}
	now := time.Now().UTC()
	i := model.WorkoutItem{ID: uuid.NewString(), UserID: u, WorkoutDayID: did, IdempotencyKey: in.IdempotencyKey, Name: in.Name, Sets: in.Sets, Repetitions: in.Repetitions, Weight: in.Weight, DurationSeconds: in.DurationSeconds, CreatedAt: now, UpdatedAt: now}
	if _, e := s.repo.GetDay(c, u, "", did); e != nil {
		return i, e
	}
	return i, s.repo.CreateItem(c, &i)
}
func (s *Service) UpdateWorkoutItem(c context.Context, u, did, iid string, in WorkoutItemInput) (model.WorkoutItem, error) {
	if e := validateUser(u); e != nil {
		return model.WorkoutItem{}, e
	}
	if did == "" {
		return model.WorkoutItem{}, apperror.InvalidArgument("workout_day_id is required")
	}
	if iid == "" {
		return model.WorkoutItem{}, apperror.InvalidArgument("workout_item_id is required")
	}
	if e := validItem(in); e != nil {
		return model.WorkoutItem{}, e
	}
	i, e := s.repo.GetItem(c, u, did, iid)
	if e != nil {
		return i, e
	}
	i.Name = in.Name
	i.Sets = in.Sets
	i.Repetitions = in.Repetitions
	i.Weight = in.Weight
	i.DurationSeconds = in.DurationSeconds
	i.UpdatedAt = time.Now().UTC()
	return i, s.repo.UpdateItem(c, &i)
}
func (s *Service) DeleteWorkoutItem(c context.Context, u, did, iid string) error {
	if e := validateUser(u); e != nil {
		return e
	}
	if did == "" || iid == "" {
		return apperror.InvalidArgument("workout_day_id and workout_item_id are required")
	}
	if _, e := s.repo.GetItem(c, u, did, iid); e != nil {
		return e
	}
	return s.repo.DeleteItem(c, u, did, iid)
}
func (s *Service) GetWorkoutDay(c context.Context, u, pid, did string) (model.WorkoutDay, error) {
	if e := validateUser(u); e != nil {
		return model.WorkoutDay{}, e
	}
	if pid == "" || did == "" {
		return model.WorkoutDay{}, apperror.InvalidArgument("plan_id and workout_day_id are required")
	}
	return s.repo.GetDay(c, u, pid, did)
}
func (s *Service) GetWorkoutItem(c context.Context, u, did, iid string) (model.WorkoutItem, error) {
	if e := validateUser(u); e != nil {
		return model.WorkoutItem{}, e
	}
	if did == "" || iid == "" {
		return model.WorkoutItem{}, apperror.InvalidArgument("workout_day_id and workout_item_id are required")
	}
	return s.repo.GetItem(c, u, did, iid)
}
func (s *Service) ListWorkoutItems(c context.Context, u, did string, page, size int) (Page[model.WorkoutItem], error) {
	if e := validateUser(u); e != nil {
		return Page[model.WorkoutItem]{}, e
	}
	if did == "" {
		return Page[model.WorkoutItem]{}, apperror.InvalidArgument("workout_day_id is required")
	}
	if _, e := s.repo.GetDay(c, u, "", did); e != nil {
		return Page[model.WorkoutItem]{}, e
	}
	if page < 1 || size < 1 || size > 100 {
		return Page[model.WorkoutItem]{}, apperror.InvalidArgument("invalid pagination")
	}
	l, ok := s.repo.(itemLister)
	if !ok {
		return Page[model.WorkoutItem]{}, apperror.Internal("workout item listing unavailable")
	}
	items, total, e := l.ListItems(c, u, did, page, size)
	return Page[model.WorkoutItem]{Items: items, Page: page, PageSize: size, Total: total}, e
}
