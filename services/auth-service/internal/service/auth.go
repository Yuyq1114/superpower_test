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
	uow    repository.UnitOfWork
	tm     *TokenManager
}

func NewAuthService(u repository.User, t repository.RefreshToken, uow repository.UnitOfWork, m *TokenManager) *AuthService {
	return &AuthService{users: u, tokens: t, uow: uow, tm: m}
}
func (s *AuthService) issue(uid string, now time.Time) (TokenPair, model.RefreshToken, error) {
	pair, hash, err := s.tm.IssueAt(uid, now)
	if err != nil {
		return TokenPair{}, model.RefreshToken{}, apperror.Internal("unable to issue token")
	}
	return pair, model.RefreshToken{ID: uuid.NewString(), UserID: uid, TokenHash: hash, ExpiresAt: now.Add(s.tm.refreshTTL), CreatedAt: now}, nil
}
func (s *AuthService) Register(c context.Context, email, password string) (model.User, TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !validEmail(email) || len(password) < 8 || !strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") || !strings.ContainsAny(password, "0123456789") {
		return model.User{}, TokenPair{}, apperror.InvalidArgument("invalid email or password")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, TokenPair{}, apperror.Internal("unable to create user")
	}
	now := s.tm.now().UTC()
	u := model.User{ID: uuid.NewString(), Email: email, PasswordHash: string(hash), CreatedAt: now, UpdatedAt: now}
	pair, token, err := s.issue(u.ID, now)
	if err != nil {
		return model.User{}, TokenPair{}, err
	}
	if err = s.uow.CreateUserWithRefreshToken(c, &u, &token); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return model.User{}, TokenPair{}, apperror.Conflict("email already registered")
		}
		return model.User{}, TokenPair{}, apperror.Internal("unable to create user")
	}
	u.PasswordHash = ""
	return u, pair, nil
}
func (s *AuthService) Login(c context.Context, email, password string) (model.User, TokenPair, error) {
	u, e := s.users.ByEmail(c, strings.ToLower(strings.TrimSpace(email)))
	if e != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return model.User{}, TokenPair{}, apperror.Unauthenticated("invalid credentials")
	}
	now := s.tm.now().UTC()
	p, token, e := s.issue(u.ID, now)
	if e != nil {
		return model.User{}, TokenPair{}, e
	}
	if e = s.tokens.Create(c, &token); e != nil {
		return model.User{}, TokenPair{}, apperror.Internal("unable to issue token")
	}
	u.PasswordHash = ""
	return u, p, nil
}
func (s *AuthService) Refresh(c context.Context, raw string) (TokenPair, error) {
	if raw == "" {
		return TokenPair{}, apperror.Unauthenticated("invalid refresh token")
	}
	now := s.tm.now().UTC()
	var pair TokenPair
	_, err := s.tokens.Rotate(c, HashRefreshToken(raw), now, func(uid string) (*model.RefreshToken, error) {
		issued, token, issueErr := s.issue(uid, now)
		if issueErr != nil {
			return nil, issueErr
		}
		pair = issued
		return &token, nil
	})
	if err != nil {
		return TokenPair{}, apperror.Unauthenticated("invalid refresh token")
	}
	return pair, nil
}
func (s *AuthService) Logout(c context.Context, raw string) error {
	if raw == "" {
		return apperror.InvalidArgument("refresh token is required")
	}
	if e := s.tokens.Revoke(c, HashRefreshToken(raw), s.tm.now().UTC()); e != nil {
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
