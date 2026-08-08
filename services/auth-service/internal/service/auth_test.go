package service

import (
	"context"
	"errors"
	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"sync"
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

type memTokens struct {
	mu sync.Mutex
	m  map[string]model.RefreshToken
}

func (x *memTokens) Create(_ context.Context, t *model.RefreshToken) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.m[t.TokenHash] = *t
	return nil
}
func (x *memTokens) Rotate(_ context.Context, h string, now time.Time, issue func(string) (*model.RefreshToken, error)) (model.RefreshToken, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	o, ok := x.m[h]
	if !ok || o.RevokedAt != nil || !o.ExpiresAt.After(now) {
		return model.RefreshToken{}, gorm.ErrRecordNotFound
	}
	n, err := issue(o.UserID)
	if err != nil {
		return model.RefreshToken{}, err
	}
	o.RevokedAt = &now
	x.m[h] = o
	n.UserID = o.UserID
	x.m[n.TokenHash] = *n
	return *n, nil
}
func (x *memTokens) Revoke(_ context.Context, h string, now time.Time) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	o, ok := x.m[h]
	if !ok || o.RevokedAt != nil {
		return gorm.ErrRecordNotFound
	}
	o.RevokedAt = &now
	x.m[h] = o
	return nil
}

type memUnitOfWork struct {
	users         *memUsers
	tokens        *memTokens
	failAfterUser bool
	fail          error
}

func (x memUnitOfWork) CreateUserWithRefreshToken(c context.Context, u *model.User, token *model.RefreshToken) error {
	userSnapshot := make(map[string]model.User, len(x.users.m))
	for key, value := range x.users.m {
		userSnapshot[key] = value
	}
	tokenSnapshot := make(map[string]model.RefreshToken, len(x.tokens.m))
	for key, value := range x.tokens.m {
		tokenSnapshot[key] = value
	}
	if err := x.users.Create(c, u); err != nil {
		return err
	}
	if x.failAfterUser {
		x.users.m = userSnapshot
		x.tokens.m = tokenSnapshot
		return x.fail
	}
	return x.tokens.Create(c, token)
}
func TestRegisterTransactionFailureLeavesNoUser(t *testing.T) {
	users := &memUsers{map[string]model.User{}}
	tokens := &memTokens{m: map[string]model.RefreshToken{}}
	s := NewAuthService(users, tokens, memUnitOfWork{users: users, tokens: tokens, failAfterUser: true, fail: errors.New("token insert failed")}, NewTokenManager([]byte("0123456789abcdef0123456789abcdef"), time.Minute, time.Hour))
	if _, _, err := s.Register(context.Background(), "rollback@example.com", "ValidPass123"); apperror.CodeOf(err) != apperror.CodeInternal {
		t.Fatalf("register: %v", err)
	}
	if len(users.m) != 0 || len(tokens.m) != 0 {
		t.Fatalf("partial registration persisted")
	}
}

func setup() *AuthService {
	u := &memUsers{map[string]model.User{}}
	r := &memTokens{m: map[string]model.RefreshToken{}}
	return NewAuthService(u, r, memUnitOfWork{users: u, tokens: r}, NewTokenManager([]byte("0123456789abcdef0123456789abcdef"), time.Minute, time.Hour))
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

func TestConcurrentRefreshOnlyOneSucceeds(t *testing.T) {
	s := setup()
	_, pair, err := s.Register(context.Background(), "concurrent@example.com", "ValidPass123")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Refresh(context.Background(), pair.RefreshToken); err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("successful refreshes = %d", success)
	}
}

func TestRegisterMapsPostgresUniqueViolationToConflict(t *testing.T) {
	u := &memUsers{map[string]model.User{}}
	r := &memTokens{m: map[string]model.RefreshToken{}}
	err := &pgconn.PgError{Code: "23505"}
	s := NewAuthService(u, r, memUnitOfWork{users: u, tokens: r, failAfterUser: true, fail: err}, NewTokenManager([]byte("0123456789abcdef0123456789abcdef"), time.Minute, time.Hour))
	if _, _, got := s.Register(context.Background(), "unique@example.com", "ValidPass123"); apperror.CodeOf(got) != apperror.CodeConflict {
		t.Fatalf("error: %v", got)
	}
}
