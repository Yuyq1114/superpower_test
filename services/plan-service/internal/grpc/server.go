package grpc

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/pkg/apperror"
	planv1 "github.com/example/fitness-checkin/proto/gen/plan/v1"
	"github.com/example/fitness-checkin/services/plan-service/internal/model"
	"github.com/example/fitness-checkin/services/plan-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"strings"
	"time"
)

type Server struct {
	planv1.UnimplementedPlanServiceServer
	Svc *service.Service
}

func NewServer(s *service.Service) *Server { return &Server{Svc: s} }
func mapErr(e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return status.Error(codes.NotFound, "not found")
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}
	if errors.Is(e, context.Canceled) {
		return status.Error(codes.Canceled, "canceled")
	}
	if strings.Contains(strings.ToLower(e.Error()), "duplicate key") || strings.Contains(e.Error(), "23505") {
		return status.Error(codes.AlreadyExists, "resource already exists")
	}
	switch apperror.CodeOf(e) {
	case apperror.CodeUnauthenticated:
		return status.Error(codes.Unauthenticated, e.Error())
	case apperror.CodeInvalidArgument:
		return status.Error(codes.InvalidArgument, e.Error())
	case apperror.CodeNotFound:
		return status.Error(codes.NotFound, "not found")
	case apperror.CodePermissionDenied:
		return status.Error(codes.PermissionDenied, e.Error())
	case apperror.CodeConflict:
		return status.Error(codes.AlreadyExists, e.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
func formatTime(v time.Time) string {
	if v.IsZero() {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}
func plan(p model.Plan) *planv1.Plan {
	return &planv1.Plan{Id: p.ID, UserId: p.UserID, Name: p.Name, Status: p.Status, CreatedAt: formatTime(p.CreatedAt), UpdatedAt: formatTime(p.UpdatedAt)}
}
func day(d model.WorkoutDay) *planv1.WorkoutDay {
	date := ""
	if d.Date != nil {
		date = d.Date.Format("2006-01-02")
	}
	return &planv1.WorkoutDay{Id: d.ID, PlanId: d.PlanID, Date: date, CreatedAt: formatTime(d.CreatedAt), UpdatedAt: formatTime(d.UpdatedAt)}
}
func item(i model.WorkoutItem) *planv1.WorkoutItem {
	return &planv1.WorkoutItem{Id: i.ID, WorkoutDayId: i.WorkoutDayID, Name: i.Name, Sets: int32(i.Sets), Repetitions: int32(i.Repetitions), Weight: i.Weight, DurationSeconds: int32(i.DurationSeconds), CreatedAt: formatTime(i.CreatedAt), UpdatedAt: formatTime(i.UpdatedAt)}
}
func (s *Server) CreatePlan(c context.Context, r *planv1.CreatePlanRequest) (*planv1.PlanResponse, error) {
	p, e := s.Svc.CreatePlan(c, r.UserId, service.CreatePlanInput{Name: r.Name, IdempotencyKey: r.IdempotencyKey})
	if e != nil {
		return nil, mapErr(e)
	}
	return &planv1.PlanResponse{Plan: plan(p)}, nil
}
func (s *Server) GetPlan(c context.Context, r *planv1.GetPlanRequest) (*planv1.PlanResponse, error) {
	p, e := s.Svc.GetPlan(c, r.UserId, r.PlanId)
	if e != nil {
		return nil, mapErr(e)
	}
	return &planv1.PlanResponse{Plan: plan(p)}, nil
}
func (s *Server) UpdatePlan(c context.Context, r *planv1.UpdatePlanRequest) (*planv1.PlanResponse, error) {
	p, e := s.Svc.UpdatePlan(c, r.UserId, r.PlanId, service.UpdatePlanInput{Name: r.Name, Status: r.Status})
	if e != nil {
		return nil, mapErr(e)
	}
	return &planv1.PlanResponse{Plan: plan(p)}, nil
}
func (s *Server) DeletePlan(c context.Context, r *planv1.DeletePlanRequest) (*planv1.Empty, error) {
	if e := s.Svc.DeletePlan(c, r.UserId, r.PlanId); e != nil {
		return nil, mapErr(e)
	}
	return &planv1.Empty{}, nil
}
func (s *Server) ListPlans(c context.Context, r *planv1.ListPlansRequest) (*planv1.ListPlansResponse, error) {
	page, size := 1, 20
	if r.Page != nil {
		page, size = int(r.Page.Page), int(r.Page.PageSize)
	}
	p, e := s.Svc.ListPlans(c, r.UserId, page, size)
	if e != nil {
		return nil, mapErr(e)
	}
	out := make([]*planv1.Plan, len(p.Items))
	for i := range p.Items {
		out[i] = plan(p.Items[i])
	}
	return &planv1.ListPlansResponse{Plans: out, Page: &planv1.PageInfo{Page: int32(p.Page), PageSize: int32(p.PageSize), Total: p.Total}}, nil
}
func (s *Server) AddWorkoutDay(c context.Context, r *planv1.AddWorkoutDayRequest) (*planv1.WorkoutDayResponse, error) {
	d, e := s.Svc.AddWorkoutDay(c, r.UserId, r.PlanId, service.WorkoutDayInput{Date: parseDate(r.Date), IdempotencyKey: r.IdempotencyKey})
	if e != nil {
		return nil, mapErr(e)
	}
	return &planv1.WorkoutDayResponse{WorkoutDay: day(d)}, nil
}
func (s *Server) UpdateWorkoutDay(c context.Context, r *planv1.UpdateWorkoutDayRequest) (*planv1.WorkoutDayResponse, error) {
	d, e := s.Svc.UpdateWorkoutDay(c, r.UserId, r.PlanId, r.WorkoutDayId, service.WorkoutDayInput{Date: parseDate(r.Date)})
	if e != nil {
		return nil, mapErr(e)
	}
	return &planv1.WorkoutDayResponse{WorkoutDay: day(d)}, nil
}
func (s *Server) DeleteWorkoutDay(c context.Context, r *planv1.DeleteWorkoutDayRequest) (*planv1.Empty, error) {
	if e := s.Svc.DeleteWorkoutDay(c, r.UserId, r.PlanId, r.WorkoutDayId); e != nil {
		return nil, mapErr(e)
	}
	return &planv1.Empty{}, nil
}
func (s *Server) AddWorkoutItem(c context.Context, r *planv1.AddWorkoutItemRequest) (*planv1.WorkoutItemResponse, error) {
	i, e := s.Svc.AddWorkoutItem(c, r.UserId, r.WorkoutDayId, toInput(r.Item, r.IdempotencyKey))
	if e != nil {
		return nil, mapErr(e)
	}
	return &planv1.WorkoutItemResponse{Item: item(i)}, nil
}
func (s *Server) UpdateWorkoutItem(c context.Context, r *planv1.UpdateWorkoutItemRequest) (*planv1.WorkoutItemResponse, error) {
	i, e := s.Svc.UpdateWorkoutItem(c, r.UserId, r.WorkoutDayId, r.WorkoutItemId, toInput(r.Item))
	if e != nil {
		return nil, mapErr(e)
	}
	return &planv1.WorkoutItemResponse{Item: item(i)}, nil
}
func (s *Server) DeleteWorkoutItem(c context.Context, r *planv1.DeleteWorkoutItemRequest) (*planv1.Empty, error) {
	if e := s.Svc.DeleteWorkoutItem(c, r.UserId, r.WorkoutDayId, r.WorkoutItemId); e != nil {
		return nil, mapErr(e)
	}
	return &planv1.Empty{}, nil
}

func pageRequest(p *planv1.PageRequest) (int, int) {
	if p == nil {
		return 1, 20
	}
	return int(p.Page), int(p.PageSize)
}
func (s *Server) GetWorkoutDay(c context.Context, r *planv1.GetWorkoutDayRequest) (*planv1.WorkoutDayResponse, error) {
	d, e := s.Svc.GetWorkoutDay(c, r.UserId, r.PlanId, r.WorkoutDayId)
	if e != nil {
		return nil, mapErr(e)
	}
	return &planv1.WorkoutDayResponse{WorkoutDay: day(d)}, nil
}
func (s *Server) ListWorkoutDays(c context.Context, r *planv1.ListWorkoutDaysRequest) (*planv1.ListWorkoutDaysResponse, error) {
	p, z := pageRequest(r.Page)
	out, e := s.Svc.ListWorkoutDays(c, r.UserId, r.PlanId, p, z)
	if e != nil {
		return nil, mapErr(e)
	}
	days := make([]*planv1.WorkoutDay, len(out.Items))
	for i := range out.Items {
		days[i] = day(out.Items[i])
	}
	return &planv1.ListWorkoutDaysResponse{WorkoutDays: days, Page: &planv1.PageInfo{Page: int32(out.Page), PageSize: int32(out.PageSize), Total: out.Total}}, nil
}
func (s *Server) GetWorkoutItem(c context.Context, r *planv1.GetWorkoutItemRequest) (*planv1.WorkoutItemResponse, error) {
	i, e := s.Svc.GetWorkoutItem(c, r.UserId, r.WorkoutDayId, r.WorkoutItemId)
	if e != nil {
		return nil, mapErr(e)
	}
	return &planv1.WorkoutItemResponse{Item: item(i)}, nil
}
func (s *Server) ListWorkoutItems(c context.Context, r *planv1.ListWorkoutItemsRequest) (*planv1.ListWorkoutItemsResponse, error) {
	p, z := pageRequest(r.Page)
	out, e := s.Svc.ListWorkoutItems(c, r.UserId, r.WorkoutDayId, p, z)
	if e != nil {
		return nil, mapErr(e)
	}
	items := make([]*planv1.WorkoutItem, len(out.Items))
	for i := range out.Items {
		items[i] = item(out.Items[i])
	}
	return &planv1.ListWorkoutItemsResponse{Items: items, Page: &planv1.PageInfo{Page: int32(out.Page), PageSize: int32(out.PageSize), Total: out.Total}}, nil
}
func parseDate(v string) time.Time {
	if strings.TrimSpace(v) == "" {
		return time.Time{}
	}
	d, _ := time.Parse("2006-01-02", v)
	return d
}
func toInput(i *planv1.WorkoutItem, keys ...string) service.WorkoutItemInput {
	if i == nil {
		return service.WorkoutItemInput{}
	}
	key := ""
	if len(keys) > 0 {
		key = keys[0]
	}
	return service.WorkoutItemInput{IdempotencyKey: key, Name: i.Name, Sets: int(i.Sets), Repetitions: int(i.Repetitions), Weight: i.Weight, DurationSeconds: int(i.DurationSeconds)}
}
