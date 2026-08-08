//go:build integration

package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/example/fitness-checkin/pkg/apperror"
	"github.com/example/fitness-checkin/services/profile-service/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func integrationDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	businessDSN := os.Getenv("TEST_DATABASE_DSN")
	if businessDSN == "" {
		t.Skip("TEST_DATABASE_DSN not set; skipping PostgreSQL profile migration integration test")
	}
	adminDSN := os.Getenv("TEST_DATABASE_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("TEST_DATABASE_ADMIN_DSN not set; PostgreSQL profile migration integration test requires an admin DSN to isolate schema")
	}

	adminDB, err := gorm.Open(postgres.Open(adminDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("open admin database: %v", err)
	}
	closeIntegrationDB(t, adminDB)
	businessDB, err := gorm.Open(postgres.Open(businessDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("open business database: %v", err)
	}
	closeIntegrationDB(t, businessDB)

	var role string
	if err = businessDB.Raw("SELECT current_user").Scan(&role).Error; err != nil {
		t.Fatalf("identify profile business role: %v", err)
	}
	schema := randomIntegrationSchema(t)
	quotedSchema, quotedRole := quoteIdentifier(schema), quoteIdentifier(role)
	if err = adminDB.Exec("CREATE SCHEMA " + quotedSchema + " AUTHORIZATION " + quotedRole).Error; err != nil {
		t.Fatalf("create isolated profile schema: %v", err)
	}
	t.Cleanup(func() {
		if err := adminDB.Exec("DROP SCHEMA IF EXISTS " + quotedSchema + " CASCADE").Error; err != nil {
			t.Errorf("drop isolated profile schema: %v", err)
		}
	})
	return businessDB, schema
}

func TestPostgresMigrationConstraintsUpgradeAndIdempotency(t *testing.T) {
	t.Parallel()
	db, schema := integrationDB(t)
	ctx := context.Background()
	if err := db.Exec("CREATE TABLE " + table(schema, "metrics") + " (id text PRIMARY KEY,user_id text NOT NULL,metric_type text NOT NULL,value double precision NOT NULL,unit text NOT NULL,recorded_at timestamptz NOT NULL,created_at timestamptz NOT NULL,idempotency_key text NOT NULL,request_fingerprint text NOT NULL DEFAULT '')").Error; err != nil {
		t.Fatal(err)
	}
	if err := MigrateSchema(ctx, db, schema); err != nil {
		t.Fatal(err)
	}
	bad := model.Metric{ID: "bad", UserID: "u", MetricType: "weight", Value: 0, Unit: "kg", RecordedAt: time.Now(), IdempotencyKey: "bad", RequestFingerprint: "bad", CreatedAt: time.Now()}
	if err := db.Table(table(schema, "metrics")).Create(&bad).Error; err == nil {
		t.Fatal("CHECK constraint accepted invalid metric")
	}
	r := GORM{DB: db, Schema: schema}
	a := model.Metric{ID: "a", UserID: "u", MetricType: "weight", Value: 70, Unit: "kg", RecordedAt: time.Now(), IdempotencyKey: "k", RequestFingerprint: "f", CreatedAt: time.Now()}
	if err := r.Create(ctx, &a); err != nil {
		t.Fatal(err)
	}
	same := a
	same.ID = "b"
	if err := r.Create(ctx, &same); err != nil || same.ID != "a" {
		t.Fatal("same fingerprint did not return original", err)
	}
	different := a
	different.ID = "c"
	different.RequestFingerprint = "other"
	if err := r.Create(ctx, &different); apperror.CodeOf(err) != apperror.CodeConflict {
		t.Fatal("expected idempotency conflict", err)
	}
}

func TestPostgresMigrationRejectsExistingDirtyData(t *testing.T) {
	t.Parallel()
	db, schema := integrationDB(t)
	ctx := context.Background()
	if err := db.Exec("CREATE TABLE " + table(schema, "metrics") + " (id text PRIMARY KEY,user_id text NOT NULL,metric_type text NOT NULL,value double precision NOT NULL,unit text NOT NULL,recorded_at timestamptz NOT NULL,created_at timestamptz NOT NULL,idempotency_key text,request_fingerprint text NOT NULL DEFAULT ''); INSERT INTO " + table(schema, "metrics") + " (id,user_id,metric_type,value,unit,recorded_at,created_at,idempotency_key) VALUES ('dirty','u','weight',70,'kg',now(),now(),'')").Error; err != nil {
		t.Fatal(err)
	}
	err := MigrateSchema(ctx, db, schema)
	if err == nil || !strings.Contains(err.Error(), "profile metrics contain missing or invalid idempotency keys") {
		t.Fatalf("expected explicit dirty data failure: %v", err)
	}
}

func randomIntegrationSchema(t *testing.T) string {
	t.Helper()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatalf("generate profile test schema: %v", err)
	}
	return "profile_test_" + hex.EncodeToString(random[:])
}

func closeIntegrationDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
}
