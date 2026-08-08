package apperror

import (
	"errors"
	"strings"
	"testing"
)

func TestStableCodesAndMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want Code
	}{
		{"invalid argument", InvalidArgument("bad input"), CodeInvalidArgument},
		{"unauthenticated", Unauthenticated("login required"), CodeUnauthenticated},
		{"permission denied", PermissionDenied("forbidden"), CodePermissionDenied},
		{"not found", NotFound("missing"), CodeNotFound},
		{"conflict", Conflict("duplicate"), CodeConflict},
		{"internal", Internal("failure"), CodeInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CodeOf(tc.err); got != tc.want {
				t.Fatalf("CodeOf() = %v, want %v", got, tc.want)
			}
		})
	}
	if got := CodeOf(errors.New("unknown")); got != CodeInternal {
		t.Fatalf("unknown code = %v, want internal", got)
	}
}

func TestWrapKeepsCauseWithoutLeakingMessage(t *testing.T) {
	cause := errors.New("database password=top-secret")
	wrapped := Wrap(CodeInternal, "request failed", cause)
	if strings.Contains(wrapped.Error(), "top-secret") {
		t.Fatalf("public message leaked cause: %q", wrapped.Error())
	}
	if !errors.Is(wrapped, cause) {
		t.Fatal("wrapped error should unwrap to cause")
	}
	if got := CodeOf(wrapped); got != CodeInternal {
		t.Fatalf("CodeOf() = %v, want %v", got, CodeInternal)
	}
}
