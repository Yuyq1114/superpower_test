package grpc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/example/fitness-checkin/pkg/apperror"
	statisticsv1 "github.com/example/fitness-checkin/proto/gen/statistics/v1"
	"github.com/example/fitness-checkin/services/statistics-service/internal/identity"
	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
	"github.com/example/fitness-checkin/services/statistics-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	statisticsv1.UnimplementedStatisticsServiceServer
	Svc *service.Service
}

func NewServer(s *service.Service) *Server { return &Server{Svc: s} }

func (s *Server) GetSummary(ctx context.Context, r *statisticsv1.GetSummaryRequest) (*statisticsv1.GetSummaryResponse, error) {
	if r == nil {
		return nil, mapErr(apperror.InvalidArgument("request is required"))
	}
	trusted, _, _, ok := identity.FromContext(ctx)
	if !ok {
		return nil, mapErr(apperror.Unauthenticated("unauthenticated"))
	}
	if trusted != r.UserId {
		return nil, mapErr(apperror.PermissionDenied("permission denied"))
	}
	period, err := periodFromProto(r.Period)
	if err != nil {
		return nil, mapErr(err)
	}
	start, err := parseTime(r.Start, true)
	if err != nil {
		return nil, mapErr(err)
	}
	end, err := parseTime(r.End, false)
	if err != nil {
		return nil, mapErr(err)
	}
	summary, err := s.Svc.GetSummary(ctx, r.UserId, period, start, end)
	if err != nil {
		return nil, mapErr(err)
	}
	return &statisticsv1.GetSummaryResponse{Summary: &statisticsv1.Summary{UserId: summary.UserID, Period: periodToProto(summary.Period), Start: summary.Start.UTC().Format(time.RFC3339Nano), End: summary.End.UTC().Format(time.RFC3339Nano), WorkoutCount: summary.WorkoutCount, ActiveDays: summary.ActiveDays, TotalDurationSeconds: summary.TotalDurationSeconds}}, nil
}

func periodFromProto(p statisticsv1.Period) (model.Period, error) {
	switch p {
	case statisticsv1.Period_PERIOD_WEEK:
		return model.PeriodWeek, nil
	case statisticsv1.Period_PERIOD_MONTH:
		return model.PeriodMonth, nil
	default:
		return "", apperror.InvalidArgument("period must be week or month")
	}
}
func periodToProto(p model.Period) statisticsv1.Period {
	if p == model.PeriodMonth {
		return statisticsv1.Period_PERIOD_MONTH
	}
	return statisticsv1.Period_PERIOD_WEEK
}
func parseTime(raw string, required bool) (time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		if required {
			return time.Time{}, apperror.InvalidArgument("start is required")
		}
		return time.Time{}, nil
	}
	at, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, apperror.InvalidArgument("time must be RFC3339")
	}
	return at, nil
}
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "canceled")
	}
	switch apperror.CodeOf(err) {
	case apperror.CodeInvalidArgument:
		return status.Error(codes.InvalidArgument, err.Error())
	case apperror.CodeUnauthenticated:
		return status.Error(codes.Unauthenticated, "unauthenticated")
	case apperror.CodePermissionDenied:
		return status.Error(codes.PermissionDenied, "permission denied")
	case apperror.CodeNotFound:
		return status.Error(codes.NotFound, "not found")
	case apperror.CodeConflict:
		return status.Error(codes.AlreadyExists, "conflict")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
