package mapper

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/pkg/apperror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"testing"
)

func TestHTTPStatusMapsAllGatewayErrors(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{{apperror.InvalidArgument("bad"), 400}, {status.Error(codes.Unauthenticated, "secret token"), 401}, {status.Error(codes.PermissionDenied, "internal ACL"), 403}, {status.Error(codes.NotFound, "sql detail"), 404}, {status.Error(codes.AlreadyExists, "duplicate key"), 409}, {status.Error(codes.ResourceExhausted, "quota internals"), 429}, {status.Error(codes.Unavailable, "dial tcp detail"), 503}, {context.DeadlineExceeded, 504}, {context.Canceled, 499}, {&http.MaxBytesError{Limit: 10}, 413}}
	for _, tc := range cases {
		if got := HTTPStatus(tc.err); got != tc.want {
			t.Errorf("HTTPStatus(%v)=%d want %d", tc.err, got, tc.want)
		}
	}
}
func TestErrorUsesStableSafeMessages(t *testing.T) {
	for _, err := range []error{status.Error(codes.Internal, "password=db-secret"), status.Error(codes.Unavailable, "dial tcp 10.0.0.1"), status.Error(codes.ResourceExhausted, "redis detail"), errors.New("token=secret")} {
		out := Error(err, "req-1")
		if out.RequestID != "req-1" || out.Message == err.Error() {
			t.Fatalf("unsafe response: %+v", out)
		}
	}
}
