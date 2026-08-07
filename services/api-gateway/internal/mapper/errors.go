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
	if err == nil {
		return 200
	}
	var max *http.MaxBytesError
	if errors.As(err, &max) {
		return 413
	}
	if errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
		return 504
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
	case codes.Unavailable:
		return 503
	}
	return 500
}
func Error(err error, requestID string) Response {
	statusCode := HTTPStatus(err)
	code, message := "INTERNAL", "internal error"
	switch statusCode {
	case 400:
		code, message = "INVALID_ARGUMENT", "invalid request"
	case 401:
		code, message = "UNAUTHENTICATED", "unauthenticated"
	case 403:
		code, message = "PERMISSION_DENIED", "permission denied"
	case 404:
		code, message = "NOT_FOUND", "not found"
	case 409:
		code, message = "CONFLICT", "conflict"
	case 413:
		code, message = "INVALID_ARGUMENT", "request body too large"
	case 429:
		code, message = "RESOURCE_EXHAUSTED", "too many requests"
	case 499:
		code, message = "CANCELED", "request canceled"
	case 503:
		code, message = "UNAVAILABLE", "service unavailable"
	case 504:
		code, message = "DEADLINE_EXCEEDED", "upstream timeout"
	}
	if statusCode < 500 {
		var app *apperror.Error
		if errors.As(err, &app) && app.Message != "" {
			message = app.Message
		}
	}
	return Response{code, message, requestID}
}
