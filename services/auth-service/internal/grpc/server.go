package grpc

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/pkg/apperror"
	authv1 "github.com/example/fitness-checkin/proto/gen/auth/v1"
	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"github.com/example/fitness-checkin/services/auth-service/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Auth interface {
	Register(context.Context, string, string) (model.User, service.TokenPair, error)
	Login(context.Context, string, string) (model.User, service.TokenPair, error)
	Refresh(context.Context, string) (service.TokenPair, error)
	Logout(context.Context, string) error
	GetUser(context.Context, string) (model.User, error)
}
type Server struct {
	authv1.UnimplementedAuthServiceServer
	svc Auth
}

func NewServer(s Auth) *Server { return &Server{svc: s} }
func mapErr(e error) error {
	if e == nil {
		return nil
	}
	switch apperror.CodeOf(e) {
	case apperror.CodeInvalidArgument:
		return status.Error(codes.InvalidArgument, e.Error())
	case apperror.CodeUnauthenticated:
		return status.Error(codes.Unauthenticated, e.Error())
	case apperror.CodeNotFound:
		return status.Error(codes.NotFound, e.Error())
	case apperror.CodeConflict:
		return status.Error(codes.AlreadyExists, e.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
func user(u model.User) *authv1.User {
	return &authv1.User{Id: u.ID, Email: u.Email, CreatedAt: u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")}
}
func pair(p service.TokenPair) *authv1.TokenPair {
	return &authv1.TokenPair{AccessToken: p.AccessToken, RefreshToken: p.RefreshToken, AccessExpiresIn: p.AccessExpiresIn, RefreshExpiresIn: p.RefreshExpiresIn}
}
func (s *Server) Register(c context.Context, r *authv1.RegisterRequest) (*authv1.AuthResponse, error) {
	u, p, e := s.svc.Register(c, r.Email, r.Password)
	if e != nil {
		return nil, mapErr(e)
	}
	return &authv1.AuthResponse{User: user(u), Tokens: pair(p)}, nil
}
func (s *Server) Login(c context.Context, r *authv1.LoginRequest) (*authv1.AuthResponse, error) {
	u, p, e := s.svc.Login(c, r.Email, r.Password)
	if e != nil {
		return nil, mapErr(e)
	}
	return &authv1.AuthResponse{User: user(u), Tokens: pair(p)}, nil
}
func (s *Server) Refresh(c context.Context, r *authv1.RefreshRequest) (*authv1.RefreshResponse, error) {
	p, e := s.svc.Refresh(c, r.RefreshToken)
	if e != nil {
		return nil, mapErr(e)
	}
	return &authv1.RefreshResponse{Tokens: pair(p)}, nil
}
func (s *Server) Logout(c context.Context, r *authv1.LogoutRequest) (*authv1.Empty, error) {
	if e := s.svc.Logout(c, r.RefreshToken); e != nil {
		return nil, mapErr(e)
	}
	return &authv1.Empty{}, nil
}
func (s *Server) GetUser(c context.Context, r *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
	u, e := s.svc.GetUser(c, r.UserId)
	if e != nil {
		return nil, mapErr(e)
	}
	return &authv1.GetUserResponse{User: user(u)}, nil
}

var _ = errors.New
