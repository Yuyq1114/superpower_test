package service

import (
	"context"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"gorm.io/gorm"
	"testing"
	"time"
)

type memUsers struct{ m map[string]model.User }

func (x *memUsers) Create(_ context.Context, u *model.User) error {
	if _, ok := x.m[u.Email]; ok {
		return gorm.ErrDuplicatedKey
	}
	x.m[u.Email] = *u
	return nil
}
func (x *memUsers) ByEmail(_ context.Context, e string) (model.User, error) {
	u, ok := x.m[e]
	if !ok {
		return u, gorm.ErrRecordNotFound
	}
	return u, nil
}
func (x *memUsers) ByID(_ context.Context, id string) (model.User, error) {
	for _, u := range x.m {
		if u.ID == id {
			return u, nil
		}
	}
	return model.User{}, gorm.ErrRecordNotFound
}

type memTokens struct{ m map[string]model.RefreshToken }

func (x *memTokens) Create(_ context.Context, t *model.RefreshToken) error {
	x.m[t.TokenHash] = *t
	return nil
}
func (x *memTokens) Rotate(_ context.Context, h string, n *model.RefreshToken, now time.Time) (string, error) {
	o, ok := x.m[h]
	if !ok || o.RevokedAt != nil || !o.ExpiresAt.After(now) {
		return "", gorm.ErrRecordNotFound
	}
	o.RevokedAt = &now
	x.m[h] = o
	n.UserID = o.UserID
	x.m[n.TokenHash] = *n
	return o.UserID, nil
}
func (x *memTokens) Revoke(_ context.Context, h string, now time.Time) error {
	o, ok := x.m[h]
	if !ok || o.RevokedAt != nil {
		return gorm.ErrRecordNotFound
	}
	o.RevokedAt = &now
	x.m[h] = o
	return nil
}
func setup() *AuthService {
	u := &memUsers{map[string]model.User{}}
	r := &memTokens{map[string]model.RefreshToken{}}
	return NewAuthService(u, r, NewTokenManager([]byte("0123456789abcdef0123456789abcdef"), time.Minute, time.Hour))
}
func TestRegisterLoginAndDuplicate(t *testing.T) {
	s := setup()
	u, p, e := s.Register(context.Background(), " User@example.com ", "ValidPass123")
	if e != nil || u.Email != "user@example.com" || u.PasswordHash != "" || p.AccessToken == "" {
		t.Fatalf("register: %#v %#v %v", u, p, e)
	}
	if _, _, e = s.Register(context.Background(), "user@example.com", "ValidPass123"); apperror.CodeOf(e) != apperror.CodeConflict {
		t.Fatalf("duplicate: %v", e)
	}
	if _, _, e = s.Login(context.Background(), "user@example.com", "WrongPass123"); apperror.CodeOf(e) != apperror.CodeUnauthenticated {
		t.Fatalf("credentials: %v", e)
	}
}
func TestValidation(t *testing.T) {
	s := setup()
	for _, v := range [][2]string{{"bad", "ValidPass123"}, {"a@example.com", "short"}, {"a@example.com", "alllowercase123"}} {
		if _, _, e := s.Register(context.Background(), v[0], v[1]); apperror.CodeOf(e) != apperror.CodeInvalidArgument {
			t.Fatalf("validation: %v", e)
		}
	}
}
func TestRefreshRotationLogoutAndRevocation(t *testing.T) {
	s := setup()
	_, p, e := s.Register(context.Background(), "a@example.com", "ValidPass123")
	if e != nil {
		t.Fatal(e)
	}
	n, e := s.Refresh(context.Background(), p.RefreshToken)
	if e != nil {
		t.Fatal(e)
	}
	if n.RefreshToken == p.RefreshToken {
		t.Fatal("not rotated")
	}
	if _, e = s.Refresh(context.Background(), p.RefreshToken); apperror.CodeOf(e) != apperror.CodeUnauthenticated {
		t.Fatalf("reuse: %v", e)
	}
	if e = s.Logout(context.Background(), n.RefreshToken); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Refresh(context.Background(), n.RefreshToken); apperror.CodeOf(e) != apperror.CodeUnauthenticated {
		t.Fatalf("revoked: %v", e)
	}
}
func TestGetUserNotFound(t *testing.T) {
	if _, e := setup().GetUser(context.Background(), "missing"); apperror.CodeOf(e) != apperror.CodeNotFound {
		t.Fatalf("not found: %v", e)
	}
}
