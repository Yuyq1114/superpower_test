package grpc

import (
	"context"
	profilev1 "github.com/example/fitness-checkin/proto/gen/profile/v1"
	"github.com/example/fitness-checkin/services/profile-service/internal/model"
	"github.com/example/fitness-checkin/services/profile-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

type emptyRepo struct{}

func (emptyRepo) Create(context.Context, *model.Metric) error { return nil }
func (emptyRepo) List(context.Context, string, string, time.Time, time.Time) ([]model.Metric, error) {
	return nil, nil
}
func TestStableNilAndInvalidErrors(t *testing.T) {
	s := NewServer(service.New(emptyRepo{}))
	if status.Code(func() error { _, e := s.RecordMetric(context.Background(), nil); return e }()) != codes.InvalidArgument {
		t.Fatal("nil request")
	}
	if status.Code(func() error {
		_, e := s.RecordMetric(context.Background(), &profilev1.RecordMetricRequest{UserId: "u", MetricType: "weight", Value: 1, Unit: "kg", RecordedAt: "bad"})
		return e
	}()) != codes.InvalidArgument {
		t.Fatal("invalid time")
	}
}
