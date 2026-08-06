package grpc

import (
	"github.com/example/fitness-checkin/pkg/apperror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func TestStableErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{{apperror.InvalidArgument("bad"), codes.InvalidArgument}, {apperror.NotFound("bad"), codes.NotFound}, {apperror.PermissionDenied("bad"), codes.PermissionDenied}, {apperror.Conflict("bad"), codes.AlreadyExists}} {
		if got := status.Code(mapErr(tc.err)); got != tc.code {
			t.Fatalf("%v => %v", tc.err, got)
		}
	}
}
