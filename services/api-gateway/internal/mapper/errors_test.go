package mapper

import (
	"context"
	"github.com/example/fitness-checkin/pkg/apperror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func TestHTTPStatusMapsAppAndGRPCErrors(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{apperror.InvalidArgument("bad"), 400},
		{status.Error(codes.Unauthenticated, "no"), 401},
		{status.Error(codes.NotFound, "no"), 404},
		{status.Error(codes.AlreadyExists, "no"), 409},
		{context.DeadlineExceeded, 504},
	}
	for _, tc := range cases {
		if got := HTTPStatus(tc.err); got != tc.want {
			t.Errorf("HTTPStatus(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}
