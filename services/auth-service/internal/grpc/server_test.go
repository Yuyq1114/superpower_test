package grpc

import (
	"context"
	"github.com/example/fitness-checkin/pkg/apperror"
	authv1 "github.com/example/fitness-checkin/proto/gen/auth/v1"
	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"github.com/example/fitness-checkin/services/auth-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	"time"
)

type fake struct{ e error }

func (f fake) Register(context.Context, string, string) (model.User, service.TokenPair, error) {
	return model.User{ID: "u", Email: "a@b.com", CreatedAt: time.Unix(1, 0)}, service.TokenPair{AccessToken: "a"}, f.e
}
func (f fake) Login(context.Context, string, string) (model.User, service.TokenPair, error) {
	return model.User{}, service.TokenPair{}, f.e
}
func (f fake) Refresh(context.Context, string) (service.TokenPair, error) {
	return service.TokenPair{}, f.e
}
func (f fake) Logout(context.Context, string) error                { return f.e }
func (f fake) GetUser(context.Context, string) (model.User, error) { return model.User{}, f.e }
func TestRegisterResponse(t *testing.T) {
	r, e := NewServer(fake{}).Register(context.Background(), &authv1.RegisterRequest{})
	if e != nil || r.User.Id != "u" || r.Tokens.AccessToken != "a" {
		t.Fatalf("response: %#v %v", r, e)
	}
}
func TestStableErrors(t *testing.T) {
	for _, x := range []struct {
		e error
		c codes.Code
	}{{apperror.InvalidArgument("bad"), codes.InvalidArgument}, {apperror.Unauthenticated("bad"), codes.Unauthenticated}, {apperror.PermissionDenied("bad"), codes.PermissionDenied}, {apperror.NotFound("bad"), codes.NotFound}, {apperror.Conflict("bad"), codes.AlreadyExists}, {service.ErrInvalidCredentials, codes.Internal}} {
		_, e := NewServer(fake{x.e}).Login(context.Background(), &authv1.LoginRequest{})
		if status.Code(e) != x.c {
			t.Fatalf("%v => %v", x.e, status.Code(e))
		}
	}
}
