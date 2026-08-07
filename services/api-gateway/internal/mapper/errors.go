package mapper

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/pkg/apperror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
)

type Response struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func HTTPStatus(err error) int {
	if errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
		return http.StatusGatewayTimeout
	}
	if errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled {
		return 499
	}
	switch apperror.CodeOf(err) {
	case apperror.CodeInvalidArgument:
		return 400
	case apperror.CodeUnauthenticated:
		return 401
	case apperror.CodePermissionDenied:
		return 403
	case apperror.CodeNotFound:
		return 404
	case apperror.CodeConflict:
		return 409
	}
	switch status.Code(err) {
	case codes.InvalidArgument:
		return 400
	case codes.Unauthenticated:
		return 401
	case codes.PermissionDenied:
		return 403
	case codes.NotFound:
		return 404
	case codes.AlreadyExists:
		return 409
	case codes.ResourceExhausted:
		return 429
	}
	return 500
}
func Error(err error, requestID string) Response {
	code := string(apperror.CodeOf(err))
	if code == "" || code == string(apperror.CodeInternal) {
		code = "INTERNAL"
	}
	if s, ok := status.FromError(err); ok {
		switch s.Code() {
		case codes.Unauthenticated:
			code = "UNAUTHENTICATED"
		case codes.InvalidArgument:
			code = "INVALID_ARGUMENT"
		case codes.PermissionDenied:
			code = "PERMISSION_DENIED"
		case codes.NotFound:
			code = "NOT_FOUND"
		case codes.AlreadyExists:
			code = "CONFLICT"
		}
	}
	msg := "internal error"
	if HTTPStatus(err) != 500 {
		msg = err.Error()
	}
	return Response{code, msg, requestID}
}
