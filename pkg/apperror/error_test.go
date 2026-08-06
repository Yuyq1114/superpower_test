package apperror

import (
	"errors"
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
			if !errors.Is(tc.err, tc.err) {
				t.Fatal("error must support errors.Is")
			}
		})
	}
	if got := CodeOf(errors.New("unknown")); got != CodeInternal {
		t.Fatalf("unknown code = %v, want internal", got)
	}
}
