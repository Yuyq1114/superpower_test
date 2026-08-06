package grpc

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/pkg/apperror"
	profilev1 "github.com/example/fitness-checkin/proto/gen/profile/v1"
	"github.com/example/fitness-checkin/services/profile-service/internal/identity"
	"github.com/example/fitness-checkin/services/profile-service/internal/model"
	"github.com/example/fitness-checkin/services/profile-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"time"
)

type Server struct {
	profilev1.UnimplementedProfileServiceServer
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
	switch apperror.CodeOf(e) {
	case apperror.CodeInvalidArgument:
		return status.Error(codes.InvalidArgument, e.Error())
	case apperror.CodeUnauthenticated:
		return status.Error(codes.Unauthenticated, "unauthenticated")
	case apperror.CodePermissionDenied:
		return status.Error(codes.PermissionDenied, "permission denied")
	case apperror.CodeConflict:
		return status.Error(codes.AlreadyExists, "conflict")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
func authorize(ctx context.Context, u string) error {
	trusted, _, _, ok := identity.FromContext(ctx)
	if !ok {
		return apperror.Unauthenticated("unauthenticated")
	}
	if trusted != u {
		return apperror.PermissionDenied("permission denied")
	}
	return nil
}
func parseTime(v string) (time.Time, error) {
	if strings.TrimSpace(v) == "" {
		return time.Time{}, apperror.InvalidArgument("recorded_at is required")
	}
	t, e := time.Parse(time.RFC3339Nano, v)
	if e != nil {
		return time.Time{}, apperror.InvalidArgument("time must be RFC3339")
	}
	return t, nil
}
func out(m model.Metric) *profilev1.Metric {
	return &profilev1.Metric{Id: m.ID, UserId: m.UserID, MetricType: m.MetricType, Value: m.Value, Unit: m.Unit, RecordedAt: m.RecordedAt.UTC().Format(time.RFC3339Nano)}
}
func (s *Server) RecordMetric(ctx context.Context, r *profilev1.RecordMetricRequest) (*profilev1.RecordMetricResponse, error) {
	if r == nil {
		return nil, mapErr(apperror.InvalidArgument("request is required"))
	}
	if e := authorize(ctx, r.UserId); e != nil {
		return nil, mapErr(e)
	}
	t, e := parseTime(r.RecordedAt)
	if e != nil {
		return nil, mapErr(e)
	}
	m, e := s.Svc.RecordMetric(ctx, r.UserId, service.MetricInput{MetricType: r.MetricType, Value: r.Value, Unit: r.Unit, RecordedAt: t, IdempotencyKey: r.IdempotencyKey})
	if e != nil {
		return nil, mapErr(e)
	}
	return &profilev1.RecordMetricResponse{Metric: out(m)}, nil
}
func (s *Server) ListMetrics(ctx context.Context, r *profilev1.ListMetricsRequest) (*profilev1.ListMetricsResponse, error) {
	if r == nil {
		return nil, mapErr(apperror.InvalidArgument("request is required"))
	}
	if e := authorize(ctx, r.UserId); e != nil {
		return nil, mapErr(e)
	}
	var from, to time.Time
	var e error
	if r.From != "" {
		from, e = parseTime(r.From)
		if e != nil {
			return nil, mapErr(e)
		}
	}
	if r.To != "" {
		to, e = parseTime(r.To)
		if e != nil {
			return nil, mapErr(e)
		}
	}
	ms, e := s.Svc.ListMetrics(ctx, r.UserId, r.MetricType, from, to)
	if e != nil {
		return nil, mapErr(e)
	}
	outm := make([]*profilev1.Metric, len(ms))
	for i := range ms {
		outm[i] = out(ms[i])
	}
	return &profilev1.ListMetricsResponse{Metrics: outm}, nil
}
