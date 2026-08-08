//go:build integration

package repository

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/example/fitness-checkin/services/auth-service/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func integrationDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	adminDSN := os.Getenv("TEST_DATABASE_ADMIN_DSN")
	businessDSN := os.Getenv("TEST_DATABASE_DSN")
	if adminDSN == "" || businessDSN == "" {
		t.Skip("TEST_DATABASE_ADMIN_DSN and TEST_DATABASE_DSN are required; skipping PostgreSQL integration test")
	}

	ctx := context.Background()
	businessDB, err := gorm.Open(postgres.Open(businessDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var businessRole string
	if err := businessDB.WithContext(ctx).Raw("SELECT current_user").Scan(&businessRole).Error; err != nil {
		t.Fatal(err)
	}
	if !validSchema(businessRole) {
		t.Fatalf("business database role is not a safe identifier: %q", businessRole)
	}

	adminDB, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := "auth_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	if err := adminDB.WithContext(ctx).Exec("CREATE SCHEMA " + quoteIdentifier(schema) + " AUTHORIZATION " + quoteIdentifier(businessRole)).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = adminDB.Exec("DROP SCHEMA IF EXISTS " + quoteIdentifier(schema) + " CASCADE").Error
		if sqlDB, err := businessDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
		if sqlDB, err := adminDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := MigrateSchema(ctx, businessDB, schema); err != nil {
		t.Fatal(err)
	}
	return businessDB, schema
}

func TestIntegrationCreateUserWithRefreshTokenRollsBack(t *testing.T) {
	db, schema := integrationDB(t)
	uow := GORMUnitOfWork{DB: db, Schema: schema}
	user := &model.User{ID: uuid.NewString(), Email: "rollback@integration.test", PasswordHash: "hash", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	token := &model.RefreshToken{ID: uuid.NewString(), UserID: "missing-user", TokenHash: uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now().UTC()}
	if err := uow.CreateUserWithRefreshToken(context.Background(), user, token); err == nil {
		t.Fatal("expected foreign key failure")
	}
	var count int64
	if err := db.Table(scopedTable(schema, "users")).Where("id = ?", user.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("user was not rolled back: %d", count)
	}
}

func TestIntegrationConcurrentRotateOnlyOneSucceeds(t *testing.T) {
	db, schema := integrationDB(t)
	repo := GORMRefreshToken{DB: db, Schema: schema}
	userID := uuid.NewString()
	user := &model.User{ID: userID, Email: "rotate@integration.test", PasswordHash: "hash", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := db.Table(scopedTable(schema, "users")).Create(user).Error; err != nil {
		t.Fatal(err)
	}
	old := &model.RefreshToken{ID: uuid.NewString(), UserID: userID, TokenHash: "old-token", ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now().UTC()}
	if err := db.Table(scopedTable(schema, "refresh_tokens")).Create(old).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Rotate(context.Background(), old.TokenHash, time.Now().UTC(), func(uid string) (*model.RefreshToken, error) {
				return &model.RefreshToken{ID: uuid.NewString(), UserID: uid, TokenHash: uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now().UTC()}, nil
			})
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful rotations = %d", successes)
	}
}
