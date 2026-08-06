package grpc

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/pkg/apperror"
	checkinv1 "github.com/example/fitness-checkin/proto/gen/checkin/v1"
	"github.com/example/fitness-checkin/services/checkin-service/internal/model"
	"github.com/example/fitness-checkin/services/checkin-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"strings"
	"time"
)

type Server struct {
	checkinv1.UnimplementedCheckinServiceServer
	Svc *service.Service
}

func NewServer(s *service.Service) *Server { return &Server{Svc: s} }
func mapErr(e error) error {
	if e == nil {
		return nil
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}
	if errors.Is(e, context.Canceled) {
		return status.Error(codes.Canceled, "canceled")
	}
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return status.Error(codes.NotFound, "not found")
	}
	switch apperror.CodeOf(e) {
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
func parseDate(v string) (time.Time, error) {
	if strings.TrimSpace(v) == "" {
		return time.Time{}, apperror.InvalidArgument("date is required")
	}
	d, e := time.Parse("2006-01-02", v)
	if e != nil {
		return time.Time{}, apperror.InvalidArgument("date must be YYYY-MM-DD")
	}
	return d, nil
}
func out(c model.Checkin) *checkinv1.Checkin {
	return &checkinv1.Checkin{Id: c.ID, UserId: c.UserID, WorkoutItemId: c.WorkoutItemID, Date: c.Date.Format("2006-01-02"), Note: c.Note, CompletedAt: c.CompletedAt.UTC().Format(time.RFC3339Nano)}
}
func (s *Server) Complete(ctx context.Context, r *checkinv1.CompleteRequest) (*checkinv1.CompleteResponse, error) {
	d, e := parseDate(r.Date)
	if e != nil {
		return nil, mapErr(e)
	}
	c, e := s.Svc.Complete(ctx, r.UserId, r.WorkoutItemId, d, r.Note)
	if e != nil {
		return nil, mapErr(e)
	}
	return &checkinv1.CompleteResponse{Checkin: out(c)}, nil
}
func (s *Server) ListHistory(ctx context.Context, r *checkinv1.ListHistoryRequest) (*checkinv1.ListHistoryResponse, error) {
	from, e := parseDate(r.From)
	if e != nil {
		return nil, mapErr(e)
	}
	to, e := parseDate(r.To)
	if e != nil {
		return nil, mapErr(e)
	}
	p, z := 1, 20
	if r.Page != nil {
		p, z = int(r.Page.Page), int(r.Page.PageSize)
	}
	x, e := s.Svc.ListHistory(ctx, r.UserId, from, to, p, z)
	if e != nil {
		return nil, mapErr(e)
	}
	items := make([]*checkinv1.Checkin, len(x.Items))
	for i := range x.Items {
		items[i] = out(x.Items[i])
	}
	return &checkinv1.ListHistoryResponse{Checkins: items, Page: &checkinv1.PageInfo{Page: int32(x.Page), PageSize: int32(x.PageSize), Total: x.Total}}, nil
}
