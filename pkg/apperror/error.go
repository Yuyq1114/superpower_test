package apperror

type Code string

const (
	CodeInvalidArgument  Code = "INVALID_ARGUMENT"
	CodeUnauthenticated  Code = "UNAUTHENTICATED"
	CodePermissionDenied Code = "PERMISSION_DENIED"
	CodeNotFound         Code = "NOT_FOUND"
	CodeConflict         Code = "CONFLICT"
	CodeInternal         Code = "INTERNAL"
)

type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string            { return e.Message }
func (e *Error) Unwrap() error            { return e.Cause }
func New(code Code, message string) error { return &Error{Code: code, Message: message} }
func wrap(code Code, message string, cause error) error {
	return &Error{Code: code, Message: message, Cause: cause}
}
func InvalidArgument(message string) error  { return New(CodeInvalidArgument, message) }
func Unauthenticated(message string) error  { return New(CodeUnauthenticated, message) }
func PermissionDenied(message string) error { return New(CodePermissionDenied, message) }
func NotFound(message string) error         { return New(CodeNotFound, message) }
func Conflict(message string) error         { return New(CodeConflict, message) }
func Internal(message string) error         { return New(CodeInternal, message) }
func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	var appErr *Error
	if ok := As(err, &appErr); ok {
		return appErr.Code
	}
	return CodeInternal
}
func As(err error, target any) bool {
	switch t := target.(type) {
	case **Error:
		for err != nil {
			if e, ok := err.(*Error); ok {
				*t = e
				return true
			}
			err = unwrap(err)
		}
	}
	return false
}
func unwrap(err error) error {
	if u, ok := err.(interface{ Unwrap() error }); ok {
		return u.Unwrap()
	}
	return nil
}
func Wrap(code Code, message string, cause error) error { return wrap(code, message, cause) }
