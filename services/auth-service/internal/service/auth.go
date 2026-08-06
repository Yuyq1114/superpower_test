package service

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"github.com/example/fitness-checkin/services/auth-service/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"strings"
	"time"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailExists        = errors.New("email exists")
)

type AuthService struct {
	users  repository.User
	tokens repository.RefreshToken
	tm     *TokenManager
}

func NewAuthService(u repository.User, t repository.RefreshToken, m *TokenManager) *AuthService {
	return &AuthService{u, t, m}
}
func (s *AuthService) issue(c context.Context, uid string) (TokenPair, error) {
	p, h, e := s.tm.Issue(uid)
	if e != nil {
		return TokenPair{}, apperror.Internal("unable to issue token")
	}
	if e = s.tokens.Create(c, &model.RefreshToken{ID: uuid.NewString(), UserID: uid, TokenHash: h, ExpiresAt: time.Now().Add(s.tm.refreshTTL)}); e != nil {
		return TokenPair{}, apperror.Internal("unable to issue token")
	}
	return p, nil
}
func (s *AuthService) Register(c context.Context, email, password string) (model.User, TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) || len(password) < 8 || !strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") || !strings.ContainsAny(password, "0123456789") {
		return model.User{}, TokenPair{}, apperror.InvalidArgument("invalid email or password")
	}
	h, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return model.User{}, TokenPair{}, apperror.Internal("unable to create user")
	}
	u := model.User{ID: uuid.NewString(), Email: email, PasswordHash: string(h), CreatedAt: time.Now().UTC()}
	if e = s.users.Create(c, &u); e != nil {
		if errors.Is(e, gorm.ErrDuplicatedKey) {
			return model.User{}, TokenPair{}, apperror.Conflict("email already registered")
		}
		return model.User{}, TokenPair{}, apperror.Internal("unable to create user")
	}
	p, e := s.issue(c, u.ID)
	if e != nil {
		return model.User{}, TokenPair{}, e
	}
	u.PasswordHash = ""
	return u, p, nil
}
func (s *AuthService) Login(c context.Context, email, password string) (model.User, TokenPair, error) {
	u, e := s.users.ByEmail(c, strings.ToLower(strings.TrimSpace(email)))
	if e != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return model.User{}, TokenPair{}, apperror.Unauthenticated("invalid credentials")
	}
	p, e := s.issue(c, u.ID)
	if e != nil {
		return model.User{}, TokenPair{}, e
	}
	u.PasswordHash = ""
	return u, p, nil
}
func (s *AuthService) Refresh(c context.Context, raw string) (TokenPair, error) {
	if raw == "" {
		return TokenPair{}, apperror.Unauthenticated("invalid refresh token")
	}
	p, h, e := s.tm.Issue("pending")
	if e != nil {
		return TokenPair{}, apperror.Internal("unable to issue token")
	}
	n := &model.RefreshToken{ID: uuid.NewString(), TokenHash: h, ExpiresAt: time.Now().Add(s.tm.refreshTTL)}
	uid, e := s.tokens.Rotate(c, HashRefreshToken(raw), n, time.Now())
	if e != nil {
		return TokenPair{}, apperror.Unauthenticated("invalid refresh token")
	}
	n.UserID = uid
	return p, nil
}
func (s *AuthService) Logout(c context.Context, raw string) error {
	if raw == "" {
		return apperror.InvalidArgument("refresh token is required")
	}
	if e := s.tokens.Revoke(c, HashRefreshToken(raw), time.Now()); e != nil {
		return apperror.Unauthenticated("invalid refresh token")
	}
	return nil
}
func (s *AuthService) GetUser(c context.Context, id string) (model.User, error) {
	u, e := s.users.ByID(c, id)
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return model.User{}, apperror.NotFound("user not found")
	}
	if e != nil {
		return model.User{}, apperror.Internal("unable to load user")
	}
	u.PasswordHash = ""
	return u, nil
}
