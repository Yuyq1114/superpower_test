package grpc

import (
	"context"
	"testing"
	"time"

	statisticsv1 "github.com/example/fitness-checkin/proto/gen/statistics/v1"
	"github.com/example/fitness-checkin/services/statistics-service/internal/identity"
	"github.com/example/fitness-checkin/services/statistics-service/internal/model"
	"github.com/example/fitness-checkin/services/statistics-service/internal/service"
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type repo struct{}

func (repo) ConsumeWorkoutCompleted(context.Context, model.WorkoutCompleted) error { return nil }
func (repo) GetSummary(_ context.Context, user string, period model.Period, start, end time.Time) (model.Summary, error) {
	return model.Summary{UserID: user, Period: period, Start: start, End: end, WorkoutCount: 2}, nil
}

func TestGetSummaryRequiresRequestAndTrustedMatchingUser(t *testing.T) {
	s := NewServer(service.New(repo{}))
	if _, err := s.GetSummary(context.Background(), nil); status.Code(err) != grpcCodes.InvalidArgument {
		t.Fatalf("nil code=%s", status.Code(err))
	}
	r := &statisticsv1.GetSummaryRequest{UserId: "user-1", Period: statisticsv1.Period_PERIOD_WEEK, Start: "2026-08-03T00:00:00Z"}
	if _, err := s.GetSummary(context.Background(), r); status.Code(err) != grpcCodes.Unauthenticated {
		t.Fatalf("auth code=%s", status.Code(err))
	}
	ctx := identity.WithTrusted(context.Background(), "user-2", "trace", "request")
	if _, err := s.GetSummary(ctx, r); status.Code(err) != grpcCodes.PermissionDenied {
		t.Fatalf("ownership code=%s", status.Code(err))
	}
}

func TestGetSummaryMapsPeriodAndResponse(t *testing.T) {
	s := NewServer(service.New(repo{}))
	ctx := identity.WithTrusted(context.Background(), "user-1", "trace", "request")
	r := &statisticsv1.GetSummaryRequest{UserId: "user-1", Period: statisticsv1.Period_PERIOD_MONTH, Start: "2026-08-07T00:00:00Z"}
	out, err := s.GetSummary(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if out.Summary.WorkoutCount != 2 || out.Summary.Period != statisticsv1.Period_PERIOD_MONTH || out.Summary.Start != "2026-08-01T00:00:00Z" {
		t.Fatalf("summary=%#v", out.Summary)
	}
}

func TestGetSummaryRejectsBadPeriodAndTime(t *testing.T) {
	s := NewServer(service.New(repo{}))
	ctx := identity.WithTrusted(context.Background(), "u", "t", "r")
	for _, r := range []*statisticsv1.GetSummaryRequest{{UserId: "u", Period: statisticsv1.Period_PERIOD_UNSPECIFIED, Start: "2026-08-01T00:00:00Z"}, {UserId: "u", Period: statisticsv1.Period_PERIOD_WEEK, Start: "bad"}} {
		if _, err := s.GetSummary(ctx, r); status.Code(err) != grpcCodes.InvalidArgument {
			t.Fatalf("code=%s err=%v", status.Code(err), err)
		}
	}
}
