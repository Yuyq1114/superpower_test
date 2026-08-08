package service

import (
	"context"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/plan-service/internal/model"
	"testing"
	"time"
)

type fakeRepo struct{ item model.WorkoutItem }

func (fakeRepo) CreatePlan(context.Context, *model.Plan) error { return nil }
func (fakeRepo) GetPlan(context.Context, string, string) (model.Plan, error) {
	return model.Plan{ID: "p", UserID: "u", Status: "draft"}, nil
}
func (fakeRepo) UpdatePlan(context.Context, *model.Plan) error    { return nil }
func (fakeRepo) DeletePlan(context.Context, string, string) error { return nil }
func (fakeRepo) ListPlans(context.Context, string, int, int) ([]model.Plan, int64, error) {
	return nil, 0, nil
}
func (fakeRepo) CreateDay(context.Context, *model.WorkoutDay) error { return nil }
func (fakeRepo) GetDay(context.Context, string, string, string) (model.WorkoutDay, error) {
	return model.WorkoutDay{}, nil
}
func (fakeRepo) UpdateDay(context.Context, *model.WorkoutDay) error      { return nil }
func (fakeRepo) DeleteDay(context.Context, string, string, string) error { return nil }
func (fakeRepo) CreateItem(context.Context, *model.WorkoutItem) error    { return nil }
func (f fakeRepo) GetItem(context.Context, string, string, string) (model.WorkoutItem, error) {
	return f.item, nil
}
func (fakeRepo) UpdateItem(context.Context, *model.WorkoutItem) error     { return nil }
func (fakeRepo) DeleteItem(context.Context, string, string, string) error { return nil }
func TestValidation(t *testing.T) {
	s := New(fakeRepo{})
	if apperror.CodeOf(func() error {
		_, e := s.UpdatePlan(context.Background(), "u", "p", UpdatePlanInput{Name: "x", Status: "bad"})
		return e
	}()) != apperror.CodeInvalidArgument {
		t.Fatal("status")
	}
	if apperror.CodeOf(func() error {
		_, e := s.AddWorkoutItem(context.Background(), "u", "d", WorkoutItemInput{Name: "x"})
		return e
	}()) != apperror.CodeInvalidArgument {
		t.Fatal("parameters")
	}
	if apperror.CodeOf(func() error {
		_, e := s.AddWorkoutDay(context.Background(), "u", "p", WorkoutDayInput{Date: time.Time{}})
		return e
	}()) != apperror.CodeInvalidArgument {
		t.Fatal("date")
	}
	if apperror.CodeOf(func() error { _, e := s.ListPlans(context.Background(), "u", 0, 10); return e }()) != apperror.CodeInvalidArgument {
		t.Fatal("page")
	}
}

func TestGetWorkoutItemAllowsLookupByItemIDWithoutDayID(t *testing.T) {
	repo := fakeRepo{item: model.WorkoutItem{ID: "item-1", UserID: "user-1"}}
	got, err := New(repo).GetWorkoutItem(context.Background(), "user-1", "", "item-1")
	if err != nil || got.ID != "item-1" {
		t.Fatalf("GetWorkoutItem() = %+v, %v", got, err)
	}
}
