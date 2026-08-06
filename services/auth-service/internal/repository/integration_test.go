package repository

import (
	"context"
	"os"
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
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set; skipping PostgreSQL integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	schema := "auth_test_" + uuid.NewString()[:8]
	if err := MigrateSchema(context.Background(), db, schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP SCHEMA IF EXISTS " + quoteIdentifier(schema) + " CASCADE").Error
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	return db, schema
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
