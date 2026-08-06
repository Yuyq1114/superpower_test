package grpc

import (
	"context"
	profilev1 "github.com/example/fitness-checkin/proto/gen/profile/v1"
	"github.com/example/fitness-checkin/services/profile-service/internal/identity"
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
func TestStableNilInvalidAndIdentityErrors(t *testing.T) {
	s := NewServer(service.New(emptyRepo{}))
	if status.Code(func() error { _, e := s.RecordMetric(context.Background(), nil); return e }()) != codes.InvalidArgument {
		t.Fatal("nil request")
	}
	trusted := identity.WithTrusted(context.Background(), "u", "", "")
	if status.Code(func() error {
		_, e := s.RecordMetric(trusted, &profilev1.RecordMetricRequest{UserId: "u", MetricType: "weight", Value: 1, Unit: "kg", RecordedAt: "bad", IdempotencyKey: "k"})
		return e
	}()) != codes.InvalidArgument {
		t.Fatal("invalid time")
	}
	if status.Code(func() error {
		_, e := s.ListMetrics(identity.WithTrusted(context.Background(), "trusted", "", ""), &profilev1.ListMetricsRequest{UserId: "forged"})
		return e
	}()) != codes.PermissionDenied {
		t.Fatal("forged request identity")
	}
	if status.Code(func() error {
		_, e := s.ListMetrics(context.Background(), &profilev1.ListMetricsRequest{UserId: "u"})
		return e
	}()) != codes.Unauthenticated {
		t.Fatal("missing trusted identity")
	}
}
